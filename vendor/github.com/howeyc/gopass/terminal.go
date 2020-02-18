/*
 * Copyright (c) 2012 Chris Howey
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

// +build !solaris

package gopass

import "golang.org/x/crypto/ssh/terminal"

type terminalState struct {
	state *terminal.State
}

func isTerminal(fd uintptr) bool {
	return terminal.IsTerminal(int(fd))
}

func makeRaw(fd uintptr) (*terminalState, error) {
	state, err := terminal.MakeRaw(int(fd))

	return &terminalState{
		state: state,
	}, err
}

func restore(fd uintptr, oldState *terminalState) error {
	return terminal.Restore(int(fd), oldState.state)
}
