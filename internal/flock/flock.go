// Package flock takes flock(2) locks on a file, shared or exclusive.
package flock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

type Lock struct{ f *os.File }

// Take locks path, creating it if needed. onWait runs once, before blocking, when the lock is
// held elsewhere, so the caller can say why it is waiting.
func Take(path string, exclusive bool, onWait func()) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	how := unix.LOCK_SH
	if exclusive {
		how = unix.LOCK_EX
	}
	err = unix.Flock(int(f.Fd()), how|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		if onWait != nil {
			onWait()
		}
		err = flockRetryingEINTR(f, how)
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

func flockRetryingEINTR(f *os.File, how int) error {
	for {
		err := unix.Flock(int(f.Fd()), how)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

// Release drops the lock. Closing the file is enough: flock(2) locks belong to the open file.
func (l *Lock) Release() error {
	return l.f.Close()
}
