package slot

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadMachineReadsEveryDial(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	writeFile(t, filepath.Join(f.home, machineFile), `[slots]
dir = "`+filepath.Join(f.home, "places")+`"
order = "any"
max_wait_seconds = 30
min_free_mb = 2048

[slots.heavy]
capacity = 1

[slots.check]
capacity = 2
`)
	m, found, err := LoadMachine(f.home)
	want := Machine{Dir: filepath.Join(f.home, "places"), Order: "any", MaxWaitSeconds: 30, MinFreeMB: 2048,
		Classes: map[string]Class{"heavy": {Capacity: 1}, "check": {Capacity: 2}}}
	if err != nil || !found || !reflect.DeepEqual(m, want) {
		t.Fatalf("LoadMachine = %+v, %v, %v; want %+v", m, found, err, want)
	}
}

func TestLoadMachineDefaults(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	m, found, err := LoadMachine(f.home)
	if err != nil || found || m.Dir != f.slots || m.Order != orderFIFO || len(m.Classes) != 0 {
		t.Fatalf("missing file: %+v, found %v, %v; want the defaults and no classes", m, found, err)
	}
	writeFile(t, filepath.Join(f.home, machineFile), capacityOne)
	m, found, err = LoadMachine(f.home)
	if err != nil || !found || m.Dir != f.slots || m.MaxWaitSeconds != 0 || m.MinFreeMB != 0 {
		t.Fatalf("classes only: %+v, found %v, %v; want the default dials", m, found, err)
	}
}

func TestLoadMachineRefuses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, toml, want string }{
		{"syntax", "[slots\n", "machine.toml"},
		{"unknown top-level table", capacityOne + "[gates]\nx = 1\n", "unknown keys: gates"},
		{"unknown class key", capacityOne + "colour = \"red\"\n", "unknown keys: slots.heavy.colour"},
		{"typo of a dial", "[slots]\nmax_wait = 5\n", "unknown keys: slots.max_wait"},
		{"bad order", "[slots]\norder = \"lifo\"\n", `slots.order "lifo"`},
		{"relative dir", "[slots]\ndir = \"slots\"\n", "must be an absolute path"},
		{"negative wait", "[slots]\nmax_wait_seconds = -1\n", "slots.max_wait_seconds -1"},
		{"negative memory", "[slots]\nmin_free_mb = -1\n", "slots.min_free_mb -1"},
		{"zero capacity", "[slots.heavy]\ncapacity = 0\n", "slots.heavy.capacity 0"},
		{"no capacity", "[slots.heavy]\n", "slots.heavy.capacity 0"},
		{"class name with a dot", "[slots.\"a.b\"]\ncapacity = 1\n", "a class name holds only"},
		{"wrong type", "[slots]\norder = 3\n", "machine.toml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, tc.toml)
			_, found, err := LoadMachine(f.home)
			if err == nil || !found || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("LoadMachine: found %v, %v; want an error naming %q", found, err, tc.want)
			}
		})
	}
}

func TestLoadMachineRefusesARelativeHome(t *testing.T) {
	t.Parallel()
	if _, _, err := LoadMachine("relative/home"); err == nil {
		t.Fatal("LoadMachine took a relative WK_HOME")
	}
}
