// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// ContentHash is the hash of everything this compiled content is: the
// lower-case hex SHA-256 of its JSON (rpg-project#1001). A host carries it
// into [encounter.SetupInput.Content], so a run's record names the content
// it was a run of, and a saved run can tell whether the content open now is
// the same.
//
// IT IS OF THE COMPILED FORM, NOT THE SOURCE BYTES. Anything the compiler
// keeps changes it: a line's words, a check's DC, a machine's step, a post,
// a creature, a region, the key and the name. Anything it drops does not:
// comments, key order, quoting and layout in the YAML. Two compiles of one
// file hash alike, and so does a reload. A compiler that compiles the same
// file differently (a new engine version) gives it a new hash, which is
// true: it is different content to run.
//
// Errors: the compiled form does not marshal, which a compile never produces.
func (c *Compiled) ContentHash() (encounter.ContentHash, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("content hash: %w", err)
	}
	sum := sha256.Sum256(b)
	return encounter.ContentHash(hex.EncodeToString(sum[:])), nil
}
