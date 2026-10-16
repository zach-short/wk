// Package clone makes copy-on-write copies of whole directory trees.
package clone

import "golang.org/x/sys/unix"

// Dir clones the tree at src to dst in one clonefile(2) call. dst must not exist and its parent
// must. One call for the whole tree is what makes this fast: `cp -c -R` clones file by file and
// took 23 s on the root node_modules where this takes under 5 s (DESIGN.md section 1.3).
func Dir(src, dst string) error {
	return unix.Clonefile(src, dst, unix.CLONE_NOFOLLOW)
}
