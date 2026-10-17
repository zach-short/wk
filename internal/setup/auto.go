package setup

import (
	"slices"
	"strings"
)

// Defaults is what the wizard offers with no key pressed: every option that starts on, and
// nothing that asks for a standing change. Schedule and Warm stay false, because the rollout
// runs those as their own commands.
func Defaults(f Found) Choices {
	c := Choices{Base: defaultBase(f.Branches)}
	c.CloneDirs = taken(f.CloneDirs)
	c.CopyFiles = taken(f.CopyFiles)
	for i, o := range f.Installs {
		if o.On {
			c.Installs = append(c.Installs, f.installs[i])
		}
	}
	return c
}

// defaultBase is the integration branch. The checked-out branch of a main checkout can be a
// feature branch, and a workspace must start from the integration branch, so the current branch
// comes last.
func defaultBase(branches []string) string {
	for _, want := range []string{"main", "master"} {
		if slices.Contains(branches, want) {
			return want
		}
	}
	if len(branches) > 0 {
		return branches[0]
	}
	return "main"
}

func taken(opts []Option) []string {
	var out []string
	for _, o := range opts {
		if o.On {
			out = append(out, o.Value)
		}
	}
	return out
}

// Describe lists the choices as one line each, for a person or an agent to read back.
func Describe(c Choices) []string {
	lines := []string{"base " + c.Base}
	for _, d := range c.CloneDirs {
		lines = append(lines, "clone dir "+d)
	}
	for _, f := range c.CopyFiles {
		lines = append(lines, "copy file "+f)
	}
	for _, in := range c.Installs {
		lines = append(lines, "install "+strings.Join(in.Run, " ")+" (rebuilds on "+in.Lockfile+")")
	}
	return lines
}
