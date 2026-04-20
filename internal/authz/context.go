package authz

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Gabrielbdd/gospa/db/sqlc"
)

// Caller is the resolved team-member identity attached to a request
// context after the authz middleware succeeds. Handlers read it via
// CallerFromContext / ContactID / Role rather than re-querying the
// database for every action.
type Caller struct {
	// ContactID is the local contacts.id of the authenticated team
	// member. Always valid when Caller is in context.
	ContactID pgtype.UUID
	// Role is the workspace_grants.role at request time. Always
	// 'admin' or 'technician' — the middleware rejects every other
	// state before attaching the Caller.
	Role sqlc.WorkspaceRole
}

type contextKey struct{}

// WithCaller returns ctx augmented with c. Internal to the middleware;
// handlers should not call it directly.
func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, contextKey{}, c)
}

// CallerFromContext returns the Caller attached to ctx by the authz
// middleware. The boolean is false when no Caller is present (which
// should only happen for public procedures or in tests that bypassed
// the middleware).
func CallerFromContext(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(contextKey{}).(Caller)
	return c, ok
}

// ContactID is a sugar over CallerFromContext for handlers that only
// care about the actor identity.
func ContactID(ctx context.Context) (pgtype.UUID, bool) {
	c, ok := CallerFromContext(ctx)
	return c.ContactID, ok
}

// Role is a sugar over CallerFromContext for handlers that need to
// branch on the caller's role inside the handler body (rare — most
// authz decisions are made by the middleware via the policy map).
func Role(ctx context.Context) (sqlc.WorkspaceRole, bool) {
	c, ok := CallerFromContext(ctx)
	return c.Role, ok
}
