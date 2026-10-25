// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

// name.go is THE VALUE A `when` KIND TAKES FOR AN AUTHORED NAME
// (rpg-project#684 L1–L3, rpg-project#670 R16): a sound's kind for `heard`, a
// fact's id for `fact`.
//
// CONTENT SPEAKS IN AUTHORED NAMES. A pattern matches names a line in the
// author's file gave, never a field of an engine payload: this module hands
// [Name.Matches] a sound's kind or a fact's id and nothing else, and it never
// sees a beat or a testimony's wire shape. Whether a pattern can match any
// name the file authored is the dialect's to refuse, because only the dialect
// holds the file.

// Name is a literal name or a pattern over names, and exactly one of the two
// (sealed by [When.Validate]).
//
// A BARE STRING IS A LITERAL, and that is the whole of the compatibility
// claim: a `fact: alarm-raised` written before patterns existed decodes,
// matches and marshals exactly as the plain string did. A string written
// between slashes, `/clang|bell/`, is a pattern, and marshals back the same
// way. A literal that itself begins and ends with a slash cannot be written,
// and no authored name in this workspace does.
//
// A PATTERN MATCHES THE WHOLE NAME. `/clang|bell/` matches `clang` and `bell`
// and never `clanging`: an author listing names means those names, and a
// pattern that silently matched inside a longer one would fire on a sound
// nobody wrote it for.
type Name struct {
	// Literal is the one name this matches, exactly. Empty when Pattern is
	// set.
	Literal string

	// Pattern is a regular expression (Go's RE2 syntax) matched against the
	// whole name, written here without its slashes. Empty when Literal is
	// set.
	Pattern string
}

// IsZero reports whether neither form is set: the condition is not named.
// It is what `omitzero` reads, so an unset name is omitted as the empty
// string was.
func (n Name) IsZero() bool { return n.Literal == "" && n.Pattern == "" }

// String is the name as an author writes it: the literal, or the pattern
// between slashes. It is what a row verdict's shorthand prints.
func (n Name) String() string {
	if n.Pattern != "" {
		return "/" + n.Pattern + "/"
	}

	return n.Literal
}

// Matches reports whether an authored name is this one: equal to the
// literal, or matched whole by the pattern. A pattern that does not compile
// matches nothing; [When.Validate] refuses it by name.
func (n Name) Matches(name string) bool {
	if n.Pattern == "" {
		return name == n.Literal
	}
	re, err := compiled(n.Pattern)
	if err != nil {
		return false
	}

	return re.MatchString(name)
}

// validate refuses both forms at once and a pattern that does not compile.
// what is the condition as the author wrote it ("`fact`", "`heard` kind").
func (n Name) validate(what string) error {
	if n.Literal != "" && n.Pattern != "" {
		return fmt.Errorf("%s names both %q and /%s/; a name is a literal or a pattern, not both: %w",
			what, n.Literal, n.Pattern, ErrBadWhen)
	}
	if n.Pattern == "" {
		return nil
	}
	if _, err := regexp.Compile(n.Pattern); err != nil {
		return fmt.Errorf("%s /%s/ is not a pattern: %s: %w", what, n.Pattern, err.Error(), ErrBadWhen)
	}

	return nil
}

// errEmptyPattern refuses `//` at decode: a pattern between empty slashes
// would decode to the zero Name, an unset condition wearing a pattern's
// clothes.
var errEmptyPattern = errors.New("behavior: a pattern between slashes is empty")

// MarshalJSON writes the name as an author writes it ([Name.String]).
func (n Name) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.String())
}

// UnmarshalJSON reads a bare string as a literal and a `/…/` string as a
// pattern.
func (n *Name) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	*n = Name{}
	if len(s) >= 2 && strings.HasPrefix(s, "/") && strings.HasSuffix(s, "/") {
		if len(s) == 2 {
			return errEmptyPattern
		}
		n.Pattern = s[1 : len(s)-1]

		return nil
	}
	n.Literal = s

	return nil
}

// patterns caches each pattern's anchored compilation. Bounded by the
// patterns authors wrote, and read on every pick.
var patterns sync.Map // string → *regexp.Regexp

// compiled is the pattern anchored to the whole name, compiled once.
func compiled(pattern string) (*regexp.Regexp, error) {
	if re, ok := patterns.Load(pattern); ok {
		return re.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(`^(?:` + pattern + `)$`)
	if err != nil {
		return nil, err
	}
	patterns.Store(pattern, re)

	return re, nil
}
