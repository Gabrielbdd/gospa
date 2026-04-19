//go:build dev

package web

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

// Handler returns a reverse proxy to the Vite dev server at
// http://localhost:5173. Compiled only when the binary is built with
// `-tags dev`; the default build uses the embedded-dist Handler in
// embed.go.
func Handler() http.Handler {
	target, err := url.Parse("http://localhost:5173")
	if err != nil {
		panic(err)
	}
	return httputil.NewSingleHostReverseProxy(target)
}
