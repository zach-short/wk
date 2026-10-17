// Package viz serves the hive page, the live gc report it reads, and the one action the page
// takes: removing a dead worktree.
package viz

import (
	"bytes"
	"embed"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
)

//go:embed index.html assets/*.webp
var assets embed.FS

// Handler serves the page at /, its artwork under /assets/, the live report at /gc.json and the
// removal at POST /remove. report writes the JSON for one request, so every load shows the
// current state of the worktrees. remove deletes one worktree by path and writes what it did.
func Handler(report func(io.Writer) error, remove func(out io.Writer, path string) error) http.Handler {
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
	mux.Handle("/assets/", http.FileServerFS(assets))
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
	mux.HandleFunc("/remove", func(w http.ResponseWriter, r *http.Request) {
		// Only the page sends this header. A form on another site cannot set it without a
		// preflight request, and this server does not answer preflights.
		if r.Method != http.MethodPost || r.Header.Get("X-Wk-Action") != "remove" {
			http.Error(w, "POST /remove with the X-Wk-Action: remove header", http.StatusBadRequest)
			return
		}
		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil || req.Path == "" {
			http.Error(w, "the body must be JSON with a path", http.StatusBadRequest)
			return
		}
		var buf bytes.Buffer
		if err := remove(&buf, req.Path); err != nil {
			http.Error(w, strings.TrimSpace(err.Error()), http.StatusConflict)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(buf.Bytes())
	})
	return hostOnly(mux)
}

// hostOnly refuses any request whose Host is not this machine. A web page on another site can
// point its name at 127.0.0.1, but the browser still sends that site's name in Host, so the
// page cannot read the report or remove anything.
func hostOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			http.Error(w, "wk viz answers only to this machine", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
