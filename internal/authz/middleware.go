package authz

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	runtimeauth "github.com/Gabrielbdd/gofra/runtime/auth"

	"github.com/Gabrielbdd/gospa/db/sqlc"
)

// Queries is the narrow subset of sqlc the middleware consumes. Kept
// as an interface so test doubles stay trivial.
type Queries interface {
	ResolveTeamCallerByZitadelUserID(ctx context.Context, zitadelUserID pgtype.Text) (sqlc.ResolveTeamCallerByZitadelUserIDRow, error)
	ActivatePendingGrant(ctx context.Context, contactID pgtype.UUID) error
	TouchLastSeen(ctx context.Context, contactID pgtype.UUID) error
}

// LastSeenThrottle is the minimum time between TouchLastSeen writes
// for any single contact. Exposed as a package var so tests can shrink
// it; in production the value below is the operating point.
var LastSeenThrottle = 60 * time.Second

// Middleware enforces the per-RPC policy after Gofra's auth gate
// validates the JWT and attaches a User to the request context.
//
// Decision order on every request:
//
//  1. Skip authz for non-Connect paths (assets, health, /_gofra).
//  2. Skip authz for public Connect procedures (install RPCs,
//     public config) — same matcher the gate uses.
//  3. Read User from context. Missing User → 401 (this should be
//     unreachable: the gate would have already rejected, but the
//     belt-and-suspenders check keeps a forgotten public-list entry
//     from accidentally going to a handler unauthenticated).
//  4. Resolve the local caller via ResolveTeamCallerByZitadelUserID.
//     pgx.ErrNoRows → 401 not_provisioned. Other DB errors → 500.
//  5. No grant on the contact → 403 no_grant (reserved for future
//     client-portal contacts that authenticate without a workspace
//     grant; team members always have a grant).
//  6. status = 'suspended' → 401 account_suspended.
//  7. status = 'not_signed_in_yet' → ActivatePendingGrant
//     (idempotent), then proceed.
//  8. Throttled TouchLastSeen (~60s per contact_id, in-memory).
//  9. Apply the policy map: missing procedure → 403 policy_undefined;
//     LevelAdminOnly with role != admin → 403 admin_required.
// 10. Attach Caller to ctx and proceed.
type Middleware struct {
	queries  Queries
	isPublic runtimeauth.ProcedureMatcher
	logger   *slog.Logger

	seenMu   sync.Mutex
	lastSeen map[pgtype.UUID]time.Time
}

// New constructs a Middleware. isPublic is the same matcher the auth
// gate uses to skip install / public config; pass it explicitly so
// the two layers cannot drift.
func New(queries Queries, isPublic runtimeauth.ProcedureMatcher, logger *slog.Logger) *Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return &Middleware{
		queries:  queries,
		isPublic: isPublic,
		logger:   logger,
		lastSeen: make(map[pgtype.UUID]time.Time),
	}
}

// Wrap returns a chi-compatible middleware. Mount AFTER the auth gate
// so the gate's JWT validation has already populated runtimeauth.User
// in context by the time Wrap runs.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// (1) Non-Connect paths bypass authz entirely.
		if !isConnectProcedure(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		// (2) Public Connect procedures bypass authz.
		if m.isPublic != nil && m.isPublic(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()

		// (3) Belt-and-suspenders: gate should have rejected if no User.
		user, ok := runtimeauth.UserFromContext(ctx)
		if !ok {
			m.logger.WarnContext(ctx, "authz: no User in context for private procedure",
				"procedure", r.URL.Path,
				"hint", "missing public-procedures entry or gate not mounted")
			writeError(w, http.StatusUnauthorized, "unauthenticated", "missing authenticated user")
			return
		}

		// (4) Resolve the caller in one query.
		row, err := m.queries.ResolveTeamCallerByZitadelUserID(ctx, pgtype.Text{String: user.ID, Valid: true})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				m.logger.WarnContext(ctx, "authz: user not provisioned in Gospa",
					"zitadel_user_id", user.ID,
					"procedure", r.URL.Path)
				writeError(w, http.StatusUnauthorized, "not_provisioned",
					"user authenticated at ZITADEL but not provisioned in Gospa")
				return
			}
			m.logger.ErrorContext(ctx, "authz: resolve caller failed",
				"error", err,
				"zitadel_user_id", user.ID)
			writeError(w, http.StatusInternalServerError, "internal", "authz lookup failed")
			return
		}

		// (5) Contact exists but no workspace grant.
		if !row.GrantRole.Valid {
			m.logger.WarnContext(ctx, "authz: contact has no workspace grant",
				"contact_id", uuidString(row.ContactID),
				"procedure", r.URL.Path)
			writeError(w, http.StatusForbidden, "no_grant",
				"contact has no workspace grant")
			return
		}

		// (6) Suspended grant — reject.
		if row.GrantStatus.Valid && row.GrantStatus.GrantStatus == sqlc.GrantStatusSuspended {
			writeError(w, http.StatusUnauthorized, "account_suspended",
				"workspace access has been suspended")
			return
		}

		// (7) Pending → flip to active. Best-effort: if the DB write
		// fails we still let the request proceed (the in-memory
		// resolution is correct; the DB will catch up on the next
		// successful request).
		if row.GrantStatus.Valid && row.GrantStatus.GrantStatus == sqlc.GrantStatusNotSignedInYet {
			if err := m.queries.ActivatePendingGrant(ctx, row.ContactID); err != nil {
				m.logger.WarnContext(ctx, "authz: activate pending grant failed",
					"error", err,
					"contact_id", uuidString(row.ContactID))
			}
		}

		// (8) Throttled last-seen.
		if m.shouldTouch(row.ContactID) {
			if err := m.queries.TouchLastSeen(ctx, row.ContactID); err != nil {
				m.logger.WarnContext(ctx, "authz: TouchLastSeen failed",
					"error", err,
					"contact_id", uuidString(row.ContactID))
			}
		}

		// (9) Policy enforcement.
		level, ok := LevelFor(r.URL.Path)
		if !ok {
			m.logger.WarnContext(ctx, "authz: procedure not in policy map",
				"procedure", r.URL.Path,
				"hint", "add it to internal/authz/policy.go")
			writeError(w, http.StatusForbidden, "policy_undefined",
				"procedure has no policy entry — refusing by default")
			return
		}
		if level == LevelAdminOnly && row.GrantRole.WorkspaceRole != sqlc.WorkspaceRoleAdmin {
			writeError(w, http.StatusForbidden, "admin_required",
				"this action requires admin role")
			return
		}

		// (10) Attach caller and proceed.
		ctx = WithCaller(ctx, Caller{
			ContactID: row.ContactID,
			Role:      row.GrantRole.WorkspaceRole,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// shouldTouch returns true when the in-memory throttle says it's time
// to update last_seen_at for the given contact. Updates the timestamp
// optimistically — concurrent calls for the same contact may both
// return true exactly once (negligible cost).
func (m *Middleware) shouldTouch(contactID pgtype.UUID) bool {
	m.seenMu.Lock()
	defer m.seenMu.Unlock()
	last, ok := m.lastSeen[contactID]
	if ok && time.Since(last) < LastSeenThrottle {
		return false
	}
	m.lastSeen[contactID] = time.Now()
	return true
}

// isConnectProcedure mirrors the helper in authgate.go (kept local so
// the two middleware layers share the structural test for "this looks
// like a Connect call"). Connect procedures have the form
// "/<package>.<Service>/<Method>" — first segment contains a dot
// and is followed by a second segment.
func isConnectProcedure(path string) bool {
	if len(path) < 2 || path[0] != '/' {
		return false
	}
	rest := path[1:]
	i := strings.IndexByte(rest, '/')
	if i < 0 {
		return false
	}
	seg := rest[:i]
	return strings.ContainsRune(seg, '.')
}

// connectError is the JSON wire format Connect clients deserialize on
// non-200. The shape matches what authgate also emits for its own
// 401s, so callers (the SPA, buf-curl) get a consistent surface.
type connectError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(connectError{Code: code, Message: message})
}

// uuidString returns the canonical hex form of a pgtype.UUID for log
// fields. Empty when the UUID is invalid.
func uuidString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	v, err := u.Value()
	if err != nil || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
