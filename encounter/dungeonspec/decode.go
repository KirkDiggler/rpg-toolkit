// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Decode strictly decodes a dungeon spec from YAML bytes. Unknown fields
// (typos, stale keys) fail loudly instead of being silently dropped — this
// is why yaml.NewDecoder with KnownFields(true) is used instead of the
// plain yaml.Unmarshal, which would drop them silently. A dungeon spec is
// exactly one YAML document: empty input and a stray second `---` document
// both fail loudly here rather than silently decoding to a zero-value spec
// or silently dropping everything after the first document.
func Decode(raw []byte) (*DungeonSpec, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)

	var spec DungeonSpec
	if err := dec.Decode(&spec); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("empty dungeon spec")
		}
		return nil, fmt.Errorf("decode dungeon spec: %w", err)
	}

	// A second call to Decode reads the next YAML document in the stream,
	// if any. yaml.v3 otherwise accepts multi-document input and silently
	// discards everything past the first document — the same silent-drop
	// class KnownFields already closes for unknown fields, closed here for
	// stray `---` documents.
	var extra DungeonSpec
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return nil, errors.New("multi-document YAML not supported: a dungeon spec is one document")
	}

	return &spec, nil
}
