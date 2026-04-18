package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html
var assets embed.FS

func Handler() http.Handler {
	root, err := fs.Sub(assets, ".")
	if err != nil {
		panic(err)
	}

	return http.FileServerFS(root)
}
