package main

import (
	"errors"

	"github.com/rilus/mbtiles-inspect/internal/mbtiles"
)

// exitCodeFor maps an error to a process exit code per SPEC.md `verify` rules.
//
//	0: success
//	1: validation failure (gaps, corruption)
//	2: file unreadable / not an mbtiles
//	3: invocation error (bad flags, missing args)
func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, mbtiles.ErrNotMBTiles) {
		return 2
	}
	var ve verifyError
	if errors.As(err, &ve) {
		return 1
	}
	var ie invocationError
	if errors.As(err, &ie) {
		return 3
	}
	return 1
}

type verifyError struct{ msg string }

func (e verifyError) Error() string { return e.msg }

type invocationError struct{ msg string }

func (e invocationError) Error() string { return e.msg }
