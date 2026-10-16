//go:build !darwin

package clone

import "errors"

// Dir refuses: clonefile(2) is APFS only, and a byte copy of node_modules would cost the disk
// that wk exists to save (PLAN.md section 4).
func Dir(_, _ string) error {
	return errors.New("wk needs clonefile(2), which only macOS on APFS has")
}
