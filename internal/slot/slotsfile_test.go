package slot

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func slotsFileTestWrite(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "repo.slots.toml")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSlotsFileMissing(t *testing.T) {
	sf, found, err := LoadSlotsFile(filepath.Join(t.TempDir(), "none.slots.toml"))
	if err != nil || found {
		t.Fatalf("found=%v err=%v; want not found and no error", found, err)
	}
	if want := (SlotsFile{Hook: HookConfig{FailOpen: true, TimeoutMS: 200}}); !reflect.DeepEqual(sf, want) {
		t.Errorf("got %#v; want %#v", sf, want)
	}
}

func TestLoadSlotsFileReadsEntriesAndHook(t *testing.T) {
	path := slotsFileTestWrite(t, `
[[slots.match]]
spelling = "bun run build"
class = "heavy"
trim_after = ["web/.next/cache"]

[[slots.match]]
spelling = "go test"
class = "check"

[hook]
fail_open = false
timeout_ms = 150
`)
	sf, found, err := LoadSlotsFile(path)
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	want := SlotsFile{
		Match: []MatchEntry{
			{Spelling: "bun run build", Class: "heavy", TrimAfter: []string{"web/.next/cache"}},
			{Spelling: "go test", Class: "check"},
		},
		Hook: HookConfig{FailOpen: false, TimeoutMS: 150},
	}
	if !reflect.DeepEqual(sf, want) {
		t.Errorf("got %#v\nwant %#v", sf, want)
	}
}

func TestLoadSlotsFileHookDefaults(t *testing.T) {
	sf, _, err := LoadSlotsFile(slotsFileTestWrite(t, "[[slots.match]]\nspelling = \"tsc\"\nclass = \"check\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if want := (HookConfig{FailOpen: true, TimeoutMS: 200}); sf.Hook != want {
		t.Errorf("got %#v; want %#v", sf.Hook, want)
	}
}

func TestLoadSlotsFileRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, text, err string
		hook            HookConfig
	}{
		{"an unknown key keeps the decoded hook", "[hook]\nfail_open = false\nfail_closed = true\n",
			"unknown keys: hook.fail_closed", HookConfig{FailOpen: false, TimeoutMS: 200}},
		{"an unknown key in an entry", "[[slots.match]]\nspelling = \"tsc\"\nclass = \"check\"\ntrim = [\"x\"]\n",
			"unknown keys: slots.match.trim", HookConfig{FailOpen: true, TimeoutMS: 200}},
		{"an empty spelling", "[[slots.match]]\nspelling = \"  \"\nclass = \"check\"\n",
			"slots.match[0]: spelling is empty", HookConfig{FailOpen: true, TimeoutMS: 200}},
		{"a class with a slash", "[[slots.match]]\nspelling = \"tsc\"\nclass = \"a/b\"\n",
			`slots.match[0]: class "a/b" must be letters, digits, - or _`, HookConfig{FailOpen: true, TimeoutMS: 200}},
		{"an absolute trim", "[[slots.match]]\nspelling = \"tsc\"\nclass = \"check\"\ntrim_after = [\"/tmp\"]\n",
			`slots.match[0]: trim_after "/tmp" must be a relative path`, HookConfig{FailOpen: true, TimeoutMS: 200}},
		{"a trim with ..", "[[slots.match]]\nspelling = \"tsc\"\nclass = \"check\"\ntrim_after = [\"web/../..\"]\n",
			`slots.match[0]: trim_after "web/../.." may not hold a .. segment`, HookConfig{FailOpen: true, TimeoutMS: 200}},
		{"a spelling listed twice", "[[slots.match]]\nspelling = \"go test\"\nclass = \"check\"\n[[slots.match]]\nspelling = \"go  test\"\nclass = \"heavy\"\n",
			`slots.match[1]: spelling "go test" is listed twice`, HookConfig{FailOpen: true, TimeoutMS: 200}},
		{"a zero timeout", "[hook]\nfail_open = false\ntimeout_ms = 0\n",
			"hook.timeout_ms is 0; it must be above 0", HookConfig{FailOpen: false, TimeoutMS: 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := slotsFileTestWrite(t, tc.text)
			sf, found, err := LoadSlotsFile(path)
			if !found || err == nil || err.Error() != path+": "+tc.err {
				t.Fatalf("found=%v err=%v; want %q", found, err, path+": "+tc.err)
			}
			if sf.Match != nil || sf.Hook != tc.hook {
				t.Errorf("got %#v; want no entries and hook %#v", sf, tc.hook)
			}
		})
	}
}

// A file that does not decode cannot be trusted to say fail_open = false, so it keeps the
// default and the hook fails open.
func TestLoadSlotsFileSyntaxErrorKeepsDefaults(t *testing.T) {
	path := slotsFileTestWrite(t, "[hook]\nfail_open = false\n[[slots.match\n")
	sf, found, err := LoadSlotsFile(path)
	if !found || err == nil || !strings.HasPrefix(err.Error(), path+": ") {
		t.Fatalf("found=%v err=%v; want a decode error naming the file", found, err)
	}
	if want := (HookConfig{FailOpen: true, TimeoutMS: 200}); sf.Hook != want {
		t.Errorf("got %#v; want %#v", sf.Hook, want)
	}
}
