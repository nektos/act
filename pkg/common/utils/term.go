package utils

import (
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

func CheckIfColorable(w io.Writer) bool {
	if !CheckIfTerminal(w) {
		return false
	}

	// https://no-color.org/
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}

	// https://bixense.com/clicolors/
	if f, ok := os.LookupEnv("CLICOLOR_FORCE"); ok && f != "0" {
		return true
	}

	if c, ok := os.LookupEnv("CLICOLOR"); ok {
		if c != "0" {
			return true
		} else if c == "0" {
			return false
		}
	}

	if t, ok := os.LookupEnv("TERM"); ok {
		switch t {
		// safeguard against weird terminals
		case "dumb", "unknown", "linux":
			return false
		}
	}

	return true
}

func CheckIfTerminal(w io.Writer) bool {
	switch v := w.(type) {
	case *os.File:
		return isatty.IsTerminal(v.Fd()) || isatty.IsCygwinTerminal(v.Fd())
	default:
		return false
	}
}
