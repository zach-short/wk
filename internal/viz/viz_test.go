package viz

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func noRemove(io.Writer, string) error { return nil }

func TestHandlerServesThePageAndTheLiveReport(t *testing.T) {
	calls := 0
	h := Handler(func(w io.Writer) error {
		calls++
		_, err := io.WriteString(w, `{"base":"main"}`)
		return err
	}, noRemove)

	page := get(t, h, "/")
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "wk hive") {
		t.Fatalf("page: %d\n%s", page.Code, page.Body.String())
	}
	if ct := page.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("page content type = %q", ct)
	}

	for range 2 {
		rep := get(t, h, "/gc.json")
		if rep.Code != http.StatusOK || rep.Body.String() != `{"base":"main"}` {
			t.Fatalf("report: %d %q", rep.Code, rep.Body.String())
		}
	}
	if calls != 2 {
		t.Fatalf("report built %d times for two loads, want 2 (no caching)", calls)
	}
}

func TestHandlerRefusesOtherPaths(t *testing.T) {
	h := Handler(func(io.Writer) error { return nil }, noRemove)
	if rec := get(t, h, "/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerReportsAFailure(t *testing.T) {
	h := Handler(func(io.Writer) error { return io.ErrUnexpectedEOF }, noRemove)
	rec := get(t, h, "/gc.json")
	if rec.Code != http.StatusInternalServerError || !bytes.Contains(rec.Body.Bytes(), []byte("unexpected EOF")) {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestRemoveNeedsThePostAndTheHeader(t *testing.T) {
	var removed []string
	h := Handler(func(io.Writer) error { return nil }, func(_ io.Writer, p string) error {
		removed = append(removed, p)
		return nil
	})
	body := `{"path":".claude/worktrees/wk-29"}`

	if rec := get(t, h, "/remove"); rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /remove = %d, want 400", rec.Code)
	}
	if rec := send(t, h, "/remove", body, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("POST without the header = %d, want 400", rec.Code)
	}
	if len(removed) != 0 {
		t.Fatalf("removed %v without the header", removed)
	}
}

func TestRemoveRunsTheActionAndReportsItsOutcome(t *testing.T) {
	h := Handler(func(io.Writer) error { return nil }, func(out io.Writer, p string) error {
		if p == "wk-kept" {
			return io.ErrNoProgress
		}
		_, err := io.WriteString(out, "removed "+p)
		return err
	})
	hdr := map[string]string{"X-Wk-Action": "remove"}

	ok := send(t, h, "/remove", `{"path":"wk-29"}`, hdr)
	if ok.Code != http.StatusOK || ok.Body.String() != "removed wk-29" {
		t.Fatalf("remove: %d %q", ok.Code, ok.Body.String())
	}
	refused := send(t, h, "/remove", `{"path":"wk-kept"}`, hdr)
	if refused.Code != http.StatusConflict {
		t.Fatalf("refused remove = %d, want 409", refused.Code)
	}
	if rec := send(t, h, "/remove", `not json`, hdr); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", rec.Code)
	}
}

func TestOtherHostsAreRefused(t *testing.T) {
	h := Handler(func(io.Writer) error { return nil }, noRemove)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/gc.json", nil)
	req.Host = "evil.example:8787"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("foreign host = %d, want 403", rec.Code)
	}
}

// Every asset the page names must be embedded and served as an image, and the artwork together
// stays under the 3 MB budget.
func TestHandlerServesEveryAssetThePageNames(t *testing.T) {
	h := Handler(func(io.Writer) error { return nil }, noRemove)
	page := get(t, h, "/").Body.String()
	names := regexp.MustCompile(`assets/[a-z-]+\.webp`).FindAllString(page, -1)
	if len(names) == 0 {
		t.Fatalf("page names no assets, want the bioluminescent texture")
	}
	for _, name := range names {
		rec := get(t, h, "/"+name)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/webp" {
			t.Fatalf("%s: %d %q", name, rec.Code, rec.Header().Get("Content-Type"))
		}
	}

	total := 0
	err := fs.WalkDir(assets, "assets", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		total += int(info.Size())
		return err
	})
	if err != nil || total > 3<<20 {
		t.Fatalf("assets total %d bytes (err %v), want under 3 MB", total, err)
	}
}

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	return send(t, h, path, "", nil)
}

// send makes a request as the page would: from this machine, with the given headers and body.
func send(t *testing.T, h http.Handler, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	method := http.MethodGet
	if body != "" {
		method = http.MethodPost
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "127.0.0.1:8787"
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
