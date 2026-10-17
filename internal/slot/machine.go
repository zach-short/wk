package slot

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

const machineFile = "machine.toml"

// Machine is <WK_HOME>/machine.toml (SB-3): one file for every repository on the machine, so
// the cap holds across all of them.
type Machine struct {
	Dir            string // slots.dir, absolute
	Order          string // "fifo" or "any"
	MaxWaitSeconds int
	MinFreeMB      int
	Classes        map[string]Class // [slots.<name>]
}

type Class struct{ Capacity int }

const (
	orderFIFO = "fifo"
	orderAny  = "any"
)

// A class name becomes a directory under slots.dir, so it may not hold a separator or dots.
var classNameRE = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func machinePath(home string) string { return filepath.Join(home, machineFile) }

func defaultSlotsDir(home string) string { return filepath.Join(home, "slots") }

func defaultMachine(home string) Machine {
	return Machine{Dir: defaultSlotsDir(home), Order: orderFIFO, Classes: map[string]Class{}}
}

// LoadMachine reads <home>/machine.toml (SB-3). A missing file is found=false with no classes;
// an unknown key or a bad value is an error, as config.Load does.
func LoadMachine(home string) (m Machine, found bool, err error) {
	if !filepath.IsAbs(home) {
		return Machine{}, false, fmt.Errorf("WK_HOME %q is not an absolute path", home)
	}
	path := machinePath(home)
	var raw struct {
		Slots map[string]toml.Primitive `toml:"slots"`
	}
	md, err := toml.DecodeFile(path, &raw)
	if errors.Is(err, fs.ErrNotExist) {
		return defaultMachine(home), false, nil
	}
	if err == nil {
		m, err = decodeMachine(home, &md, raw.Slots)
	}
	if err != nil {
		return Machine{}, true, fmt.Errorf("%s: %w", path, err)
	}
	return m, true, nil
}

func decodeMachine(home string, md *toml.MetaData, slots map[string]toml.Primitive) (Machine, error) {
	m := defaultMachine(home)
	for name, prim := range slots {
		if err := decodeSlotsKey(md, name, prim, &m); err != nil {
			return Machine{}, err
		}
	}
	if err := unknownKeys(md); err != nil {
		return Machine{}, err
	}
	return m, m.validate()
}

// decodeSlotsKey sorts the keys of [slots]: four are dials, and every table is a class, so no
// class name is compiled in. Any other scalar is a typo of a dial and must not become a class.
func decodeSlotsKey(md *toml.MetaData, name string, prim toml.Primitive, m *Machine) error {
	dials := map[string]any{
		"dir":              &m.Dir,
		"order":            &m.Order,
		"max_wait_seconds": &m.MaxWaitSeconds,
		"min_free_mb":      &m.MinFreeMB,
	}
	if dst, ok := dials[name]; ok {
		return md.PrimitiveDecode(prim, dst)
	}
	if md.Type("slots", name) != "Hash" {
		return fmt.Errorf("unknown keys: slots.%s", name)
	}
	var c struct {
		Capacity int `toml:"capacity"`
	}
	if err := md.PrimitiveDecode(prim, &c); err != nil {
		return fmt.Errorf("slots.%s: %w", name, err)
	}
	m.Classes[name] = Class(c)
	return nil
}

func unknownKeys(md *toml.MetaData) error {
	undecoded := md.Undecoded()
	if len(undecoded) == 0 {
		return nil
	}
	keys := make([]string, len(undecoded))
	for i, k := range undecoded {
		keys[i] = k.String()
	}
	sort.Strings(keys)
	return fmt.Errorf("unknown keys: %s", strings.Join(keys, ", "))
}

func (m Machine) validate() error {
	switch {
	case !filepath.IsAbs(m.Dir):
		return fmt.Errorf("slots.dir %q must be an absolute path", m.Dir)
	case m.Order != orderFIFO && m.Order != orderAny:
		return fmt.Errorf("slots.order %q must be %q or %q", m.Order, orderFIFO, orderAny)
	case m.MaxWaitSeconds < 0:
		return fmt.Errorf("slots.max_wait_seconds %d must be 0 or more", m.MaxWaitSeconds)
	case m.MinFreeMB < 0:
		return fmt.Errorf("slots.min_free_mb %d must be 0 or more", m.MinFreeMB)
	}
	for _, name := range m.classNames() {
		if !classNameRE.MatchString(name) {
			return fmt.Errorf("slots.%s: a class name holds only letters, digits, '-' and '_'", name)
		}
		if m.Classes[name].Capacity < 1 {
			return fmt.Errorf("slots.%s.capacity %d must be 1 or more", name, m.Classes[name].Capacity)
		}
	}
	return nil
}

func (m Machine) classNames() []string {
	names := make([]string, 0, len(m.Classes))
	for name := range m.Classes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
