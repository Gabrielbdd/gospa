// Package authgate exposes a dynamic JWT middleware whose authenticated
// state can be flipped on at runtime. The app starts with the gate in
// pass-through mode so the /install wizard (which runs before any user
// exists) remains reachable; once the install orchestrator finishes it
// calls Activate, which wraps the underlying chain with JWT validation
// against the newly-provisioned ZITADEL project. No process restart is
// required.
package authgate

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"

	runtimeauth "github.com/Gabrielbdd/gofra/runtime/auth"
)

// ErrMiddlewareNotMounted is returned by Activate when the middleware
// has not been attached to the router yet (pre-flight misuse).
var ErrMiddlewareNotMounted = errors.New("authgate: Middleware has not been mounted yet")

// Gate holds the dynamic JWT middleware state.
type Gate struct {
	issuer   string
	isPublic runtimeauth.ProcedureMatcher

	// passthrough is the chain the middleware delegates to while
	// inactive. Set when Middleware is mounted; read when Activate
	// wraps it with JWT validation.
	passthrough atomic.Pointer[http.Handler]

	// active holds the authenticated wrapper once Activate has run.
	// Nil means pass-through. Read on every request for a lock-free
	// fast path.
	active atomic.Pointer[http.Handler]
}

// New returns a Gate that will validate JWTs against the given issuer
// when Activate is called. isPublic lists Connect procedures that skip
// authentication even after activation — typically the install RPCs.
func New(issuer string, isPublic runtimeauth.ProcedureMatcher) *Gate {
	return &Gate{issuer: issuer, isPublic: isPublic}
}

// Middleware is a chi-compatible middleware. It starts as a
// pass-through and swaps to authenticated once Activate is called.
func (g *Gate) Middleware(next http.Handler) http.Handler {
	g.passthrough.Store(&next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h := g.active.Load(); h != nil {
			(*h).ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Activate constructs a JWT verifier for the given audience and swaps
// the middleware into authenticated mode. Safe to call multiple times:
// the latest verifier wins (useful if the workspace is re-installed).
// Returns an error if the verifier cannot be constructed (OIDC
// discovery network failure, malformed issuer) or if the middleware
// has not been mounted yet.
func (g *Gate) Activate(ctx context.Context, audience string) error {
	// Fail fast on misuse before doing any network I/O. If the caller
	// invoked Activate before mounting Middleware, OIDC discovery would
	// otherwise mask the real bug behind a network error.
	pt := g.passthrough.Load()
	if pt == nil {
		return ErrMiddlewareNotMounted
	}
	verifier, err := runtimeauth.NewJWTVerifier(ctx, g.issuer, audience)
	if err != nil {
		return err
	}
	wrapped := runtimeauth.NewMiddleware(verifier, g.isPublic)(*pt)
	g.active.Store(&wrapped)
	slog.Info("auth gate activated", "issuer", g.issuer, "audience", audience)
	return nil
}

// IsActive reports whether the gate is currently authenticating
// requests. Primarily useful for diagnostics and tests.
func (g *Gate) IsActive() bool {
	return g.active.Load() != nil
}
