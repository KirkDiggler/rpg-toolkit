// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// ErrBadSpec is returned for every refusal in this package: a file that does
// not decode, and a file that decodes into something that is not a dungeon.
//
// ONE SENTINEL, not two, and that is a judgement rather than an oversight. A
// caller's options are the same either way — tell the author which line is
// wrong — and the difference between "yaml the parser hated" and "yaml that
// parsed into nonsense" is a fact about the parser rather than about the
// dungeon. The MESSAGE always names the offending field; the sentinel exists so
// a host can tell a bad spec from an I/O failure.
var ErrBadSpec = errors.New("bad dungeon spec")

// Decode reads a dungeon spec from YAML, strictly.
//
// # Strictly means three refusals
//
// UNKNOWN KEYS FAIL. A typo (`hieght: 8`) and a stale dialect key are the same
// event from the file's point of view — the author wrote something they meant
// and the program did not act on it — and yaml.Unmarshal's default is to drop
// both without a word. KnownFields(true) is the difference between an author
// debugging why their dungeon is 0 tall and an error naming the line.
//
// EMPTY INPUT FAILS, rather than decoding to a zero-value Spec that would then
// be refused by Validate for missing everything. The message an author needs
// for an empty file is "this file is empty", not a list of every field it does
// not have.
//
// A SECOND DOCUMENT FAILS. yaml.v3 will happily read `---` and hand back only
// the first document, silently discarding the rest; a dungeon is one document
// and a file with two of them is a file whose author believes something untrue
// about what will run.
//
// Decode does NOT check that the result is a sensible dungeon — see [Validate].
// The split is so an error can say "this is not YAML I can read" without
// pretending to know what a room is.
func Decode(raw []byte) (*Spec, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)

	var spec Spec
	if err := dec.Decode(&spec); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("the dungeon spec is empty: %w", ErrBadSpec)
		}

		return nil, fmt.Errorf("%v: %w", err, ErrBadSpec)
	}

	// A second Decode reads the next document in the stream, if any. EOF is
	// the good case here, which is why this reads inverted from the check
	// above.
	var extra Spec
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, fmt.Errorf("%v: %w", err, ErrBadSpec)
		}

		return nil, fmt.Errorf(
			"a dungeon spec is one document, and this file has more than one: %w", ErrBadSpec)
	}

	return &spec, nil
}
