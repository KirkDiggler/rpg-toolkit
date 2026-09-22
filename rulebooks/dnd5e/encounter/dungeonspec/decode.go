// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrBadSpec is returned for every refusal in this package: a file that does
// not decode, and a file that decodes into something that is not a dungeon.
//
// ONE SENTINEL, not two. A caller's options are the same either way — tell the
// author which line is wrong — and the difference between "yaml the parser
// hated" and "yaml that parsed into nonsense" is a fact about the parser rather
// than about the dungeon. The MESSAGE always names the offending field; the
// sentinel exists so a host can tell a bad spec from an I/O failure. Every
// error this package returns also carries its [FieldError]s — see
// [ValidationError].
var ErrBadSpec = errors.New("bad dungeon spec")

// FieldError is one defect, at the YAML path of the thing that is wrong.
//
// Path is the author's own address for it — `regions[1].cells[0][3]`,
// `walls[3]`, `doors[0].edges[1]`, `place[7].blocks_los`, `start` — so a
// builder can draw the refusal on the canvas at the cell or edge it names. An
// UNKNOWN KEY is one of these too (`factions[0].tempre`): the decoder reports
// it by line, and unknown_key.go walks the source back to the path
// (rpg-project#481, R2). What is left carrying a line instead of a path is
// yaml the parser could not read at all, which has no key to address.
type FieldError struct {
	Path    string
	Message string
}

// Error renders one defect as "path: message".
func (e FieldError) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return e.Path + ": " + e.Message
}

// ValidationError is every defect [Validate] (or [Decode]) found, as one
// error. It unwraps to [ErrBadSpec] so `errors.Is(err, ErrBadSpec)` holds,
// and exposes the list so a host can hand each defect to the builder.
type ValidationError struct {
	Errors []FieldError
}

// Error lists every defect, one per line, after the sentinel's own words.
func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Errors)+1)
	parts = append(parts, ErrBadSpec.Error())
	for _, fe := range e.Errors {
		parts = append(parts, fe.Error())
	}
	return strings.Join(parts, "\n  ")
}

// Unwrap makes every validation error an [ErrBadSpec].
func (e *ValidationError) Unwrap() error { return ErrBadSpec }

// Decode reads a dungeon spec from YAML, strictly.
//
// # Strictly means three refusals
//
// UNKNOWN KEYS FAIL. A typo (`hieght: 8`) and a stale dialect key (version
// 1's `rooms:`, `connectors:`, `height:`) are the same event from the file's
// point of view — the author wrote something they meant and the program did
// not act on it — and yaml.Unmarshal's default is to drop both without a
// word. KnownFields(true) is the difference between an author debugging why
// their dungeon has no floor and an error naming the key — at its PATH, and in
// this package's own words rather than Go's, which unknown_key.go explains.
//
// EMPTY INPUT FAILS, rather than decoding to a zero-value Spec that would then
// be refused by Validate for missing everything.
//
// A SECOND DOCUMENT FAILS. yaml.v3 will happily read `---` and hand back only
// the first document, silently discarding the rest; a dungeon is one document.
//
// Decode does NOT check that the result is a sensible dungeon — see [Validate].
func Decode(raw []byte) (*Spec, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)

	var spec Spec
	if err := dec.Decode(&spec); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, &ValidationError{Errors: []FieldError{{Message: "the dungeon spec is empty"}}}
		}
		return nil, &ValidationError{Errors: decodeErrors(err, raw)}
	}

	// A second Decode reads the next document in the stream, if any. EOF is
	// the good case here, which is why this reads inverted from the check
	// above.
	var extra Spec
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, &ValidationError{Errors: decodeErrors(err, raw)}
		}
		return nil, &ValidationError{Errors: []FieldError{{
			Message: "a dungeon spec is one document, and this file has more than one"}}}
	}

	return &spec, nil
}

// decodeErrors splits yaml.v3's multi-line unmarshal error into one
// FieldError per defect.
//
// EVERY UNKNOWN KEY IN THE FILE, not the first one: KnownFields collects them
// all into one TypeError and this keeps them all, because an author fixing a
// typo should not have to run the file again to find the next. Each is walked
// back to its own path in the source — see unknown_key.go — and every other
// decode defect keeps the "line N:" the parser gave it, which is the nearest
// thing to a path yaml the parser could not read has.
func decodeErrors(err error, raw []byte) []FieldError {
	keys := indexAuthoredKeys(raw)
	var out []FieldError
	for _, line := range strings.Split(err.Error(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "yaml: unmarshal errors:" {
			continue
		}
		line = strings.TrimPrefix(line, "yaml: ")
		path := ""
		if i := strings.Index(line, ": "); i > 0 && strings.HasPrefix(line, "line ") {
			path, line = line[:i], line[i+2:]
		}
		if defect, unknown := unknownKeyDefect(path, line, keys); unknown {
			out = append(out, defect)
			continue
		}
		out = append(out, FieldError{Path: path, Message: line})
	}
	if len(out) == 0 {
		out = append(out, FieldError{Message: err.Error()})
	}
	return out
}
