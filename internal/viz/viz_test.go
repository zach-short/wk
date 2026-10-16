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

func TestHandlerServesThePageAndTheLiveReport(t *testing.T) {
	calls := 0
	h := Handler(func(w io.Writer) error {
		calls++
		_, err := io.WriteString(w, `{"base":"main"}`)
		return err
	})

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
	h := Handler(func(io.Writer) error { return nil })
	if rec := get(t, h, "/nope"); rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerReportsAFailure(t *testing.T) {
	h := Handler(func(io.Writer) error { return io.ErrUnexpectedEOF })
	rec := get(t, h, "/gc.json")
	if rec.Code != http.StatusInternalServerError || !bytes.Contains(rec.Body.Bytes(), []byte("unexpected EOF")) {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
}

// Every asset the page names must be embedded and served as an image, and the artwork together
// stays under the 3 MB budget.
func TestHandlerServesEveryAssetThePageNames(t *testing.T) {
	h := Handler(func(io.Writer) error { return nil })
	page := get(t, h, "/").Body.String()
	names := regexp.MustCompile(`assets/[a-z-]+\.webp`).FindAllString(page, -1)
	if len(names) < 7 {
		t.Fatalf("page names %d assets, want the 7 direction images: %v", len(names), names)
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
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}
