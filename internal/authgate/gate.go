// Package authgate exposes a dynamic JWT middleware whose authenticated
// state can be flipped on at runtime. The app starts with the gate in
// pass-through mode so the /install wizard (which runs before any user
// exists) remains reachable; once the install orchestrator finishes —
// or once cmd/app's eager startup activation reads the persisted
// contract on a Pod restart — Activate stores a verifier and the
// middleware lazily builds an authenticated chain on the next request.
//
// The lazy build is deliberate: chi's `app.Use(gate.Middleware)` does
// not invoke the middleware function until chi compiles the request
// chain (typically on the first request). Activate must therefore not
// require the wrapped `next` handler to be supplied at activation
// time. Storing the verifier and letting the wrapped chain materialise
// on demand decouples activation from middleware mount order.
package authgate

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	runtimeauth "github.com/Gabrielbdd/gofra/runtime/auth"
)

// Gate holds the dynamic JWT middleware state.
type Gate struct {
	isPublic runtimeauth.ProcedureMatcher

	// verifier is set when Activate succeeds. A non-nil pointer means
	// the middleware authenticates; nil means pass-through. atomic
	// pointer so each Activate (re-install, rotation) replaces it
	// atomically and the next request picks up the new value.
	verifier atomic.Pointer[runtimeauth.Verifier]
}

// New returns a Gate ready to be mounted. The issuer + audience pair
// is supplied per Activate call (not at construction) because both
// values are persisted on the workspace row and may differ between
// pre-install (no contract yet) and post-install (full contract from
// ZITADEL provisioning). isPublic lists Connect procedures that skip
// authentication even after activation — typically the install RPCs.
func New(isPublic runtimeauth.ProcedureMatcher) *Gate {
	return &Gate{isPublic: isPublic}
}

// Middleware is a chi-compatible middleware. While inactive it lets
// non-Connect paths and explicitly-public Connect procedures through;
// any other Connect procedure is a private app RPC that must not be
// reachable before install completes, so it gets a 401 immediately
// without invoking the wrapped handler. Once Activate has stored a
// verifier the next request lazily compiles the authenticated chain
// (cached per verifier pointer) and serves through it.
//
// The pre-install fail-closed branch is the gate's job, not each
// handler's: requireReady guards in app handlers stay as defense in
// depth, but a forgotten requireReady on a new private RPC is no
// longer the only thing standing between an unauthenticated caller
// and the handler body.
func (g *Gate) Middleware(next http.Handler) http.Handler {
	var (
		mu       sync.Mutex
		cachedVP *runtimeauth.Verifier
		cached   http.Handler
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vp := g.verifier.Load()
		if vp == nil {
			if isConnectProcedure(r.URL.Path) && (g.isPublic == nil || !g.isPublic(r.URL.Path)) {
				writeUnauthenticated(w, "workspace is not installed yet; complete /install first")
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		mu.Lock()
		if vp != cachedVP {
			cached = runtimeauth.NewMiddleware(*vp, g.isPublic)(next)
			cachedVP = vp
		}
		chain := cached
		mu.Unlock()
		chain.ServeHTTP(w, r)
	})
}

// Activate constructs a JWT verifier for the given issuer + audience
// pair and stores it; from the next request onward the middleware
// authenticates against it. Safe to call multiple times: the latest
// verifier wins (useful if the workspace is re-installed against a
// new ZITADEL). Returns an error if either argument is empty or if
// the verifier cannot be constructed (OIDC discovery network failure,
// malformed issuer).
//
// Activation does not require Middleware to have been invoked yet —
// chi mounts middleware lazily, so cmd/app's startup eager activation
// would otherwise race the first request. The middleware compiles its
// authenticated chain on demand.
func (g *Gate) Activate(ctx context.Context, issuer, audience string) error {
	if issuer == "" || audience == "" {
		return errors.New("authgate: issuer and audience are both required for activation")
	}
	verifier, err := runtimeauth.NewJWTVerifier(ctx, issuer, audience)
	if err != nil {
		return err
	}
	g.verifier.Store(&verifier)
	slog.Info("auth gate activated", "issuer", issuer, "audience", audience)
	return nil
}

// IsActive reports whether the gate is currently authenticating
// requests. Primarily useful for diagnostics and tests.
func (g *Gate) IsActive() bool {
	return g.verifier.Load() != nil
}

// isConnectProcedure reports whether the path looks like a Connect RPC
// procedure. Mirrors runtime/auth.isConnectProcedure (kept local so
// the pre-install fail-closed policy stays a product-level decision in
// gospa rather than widening Gofra's runtime surface). Connect
// procedures have the form "/<package>.<Service>/<Method>" — the
// first path segment contains a dot AND is followed by a second
// segment.
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

// connectError is the JSON wire format for Connect error responses.
type connectError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeUnauthenticated emits a Connect-compatible 401 JSON body so
// clients that speak Connect (the SPA's apiCall helper, buf-curl,
// etc.) surface a structured error instead of a plain "401
// Unauthorized" string.
func writeUnauthenticated(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(connectError{
		Code:    "unauthenticated",
		Message: msg,
	})
}
