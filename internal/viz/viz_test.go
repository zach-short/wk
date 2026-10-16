package viz

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
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

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}
