// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// statuses.go is A STATUS AN AUTHOR WROTE (rpg-project#1024): the root's
// `statuses:` — each a named piece that changes what its holder can do, with
// rules for how long it stays — and a line's row `apply_status: <id>`, which
// gives one to the row's speaker.
//
//	statuses:
//	  well-fed:
//	    name: Well fed
//	    changes:
//	      - advantage: checks
//	        when: { ability: charisma }
//	    maintenance:
//	      - ends_on: run
//	    again: ignored
//
// THIS FILE JUDGES THE SHAPE, NOT THE WORDS INSIDE IT. The four top-level
// words are settled; what may stand inside `changes` and `maintenance` is the
// rules' own table, which this module may not import (C1). So a status is
// refused here for a missing or misspelt top-level word and for an empty
// list, and carried as canonical JSON in the author's words; the session
// compiles it with the rules when it launches and when a row gives it.

// StatusSpec is one authored status, kept in the author's words: `name`,
// `changes`, `maintenance` and `again`. Only those four keys and their
// presence are judged here.
type StatusSpec map[string]any

// The four words a status is written with, in the order a refusal lists them.
const (
	statusName        = "name"
	statusChanges     = "changes"
	statusMaintenance = "maintenance"
	statusAgain       = "again"
)

var statusWords = []string{statusName, statusChanges, statusMaintenance, statusAgain}

// sortedStatusIDs is every declared status id, in order.
func sortedStatusIDs(statuses map[string]StatusSpec) []string {
	ids := make([]string, 0, len(statuses))
	for id := range statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// statusesDeclared judges every status under `statuses:` at its path and
// returns the ids a row's `apply_status:` may name: every declared one, a
// refused one included, so a misshapen status is reported once, at itself,
// and not again at every row that names it.
func statusesDeclared(statuses map[string]StatusSpec, add errSink) map[string]bool {
	declared := make(map[string]bool, len(statuses))
	for _, id := range sortedStatusIDs(statuses) {
		at := "statuses." + id
		declared[id] = true
		if strings.TrimSpace(id) == "" {
			add("statuses", "a status has an empty id")
			continue
		}
		st := statuses[id]
		if st == nil {
			add(at, "a status is written with "+strings.Join(statusWords, ", "))
			continue
		}
		for _, key := range sortedKeys(st) {
			if !containsWord(statusWords, key) {
				add(at+"."+key, fmt.Sprintf("%q is not a word of a status: a status is written with %s",
					key, strings.Join(statusWords, ", ")))
			}
		}
		if name, ok := st[statusName].(string); !ok || strings.TrimSpace(name) == "" {
			add(at+"."+statusName, "a status needs a name: the words a player sees")
		}
		for _, list := range []string{statusChanges, statusMaintenance} {
			items, ok := st[list].([]any)
			if !ok || len(items) == 0 {
				what := "what the status changes"
				if list == statusMaintenance {
					what = "what ends the status"
				}
				add(at+"."+list, fmt.Sprintf("a status needs %s as a non-empty list: %s", list, what))
			}
		}
		if _, ok := st[statusAgain]; !ok {
			add(at+"."+statusAgain, "a status says what a second giving does: write again")
		}
	}
	return declared
}

// containsWord reports whether words holds w.
func containsWord(words []string, w string) bool {
	for _, x := range words {
		if x == w {
			return true
		}
	}
	return false
}

// statusesOf compiles every declared status to canonical JSON — keys sorted
// at every depth — so a status hashes the same however its YAML was laid out.
// Nil when the site declares none.
func statusesOf(statuses map[string]StatusSpec) map[string]json.RawMessage {
	if len(statuses) == 0 {
		return nil
	}
	out := make(map[string]json.RawMessage, len(statuses))
	for id, st := range statuses {
		raw, err := json.Marshal(map[string]any(st))
		if err != nil {
			// A YAML document decodes only to JSON's shapes; a value that
			// does not marshal is one the grammar refused already.
			continue
		}
		out[id] = raw
	}
	return out
}

// statusGrantOf is a row's `apply_status:` as the encounter carries it: the
// id and the status's canonical JSON. Nil when the row gives none or names a
// status the site does not have (refused by the grammar first).
func statusGrantOf(id string, statuses map[string]json.RawMessage) *encounter.StatusGrant {
	raw, ok := statuses[id]
	if id == "" || !ok {
		return nil
	}
	return &encounter.StatusGrant{ID: id, Status: append(json.RawMessage(nil), raw...)}
}

// declaredStatusList is the declared ids as a refusal lists them.
func declaredStatusList(declared map[string]bool) string {
	if len(declared) == 0 {
		return "this site declares no statuses"
	}
	ids := make([]string, 0, len(declared))
	for id := range declared {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return "the statuses declared are " + strings.Join(ids, ", ")
}
