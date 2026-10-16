// Package viz serves the hive page and the live gc report it reads.
package viz

import (
	"bytes"
	"embed"
	"io"
	"net/http"
)

//go:embed index.html
var assets embed.FS

// Handler serves the page at / and the live report at /gc.json. report writes the JSON for one
// request, so every load shows the current state of the worktrees.
func Handler(report func(io.Writer) error) http.Handler {
	page, err := assets.ReadFile("index.html")
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("/gc.json", func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if err := report(&buf); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(buf.Bytes())
	})
	return mux
}
