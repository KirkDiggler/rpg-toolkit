// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// statuses.go is A STATUS AN AUTHOR WROTE (rpg-project#1024): the root's
// `statuses:` — each a named piece that changes what its holder can do, with
// rules for how long it stays — and a line's row
// `apply_status: { status: <id>, to: <who> }`, which gives one to whom its
// `to` names (rpg-project#990, [ApplyStatusSpec]).
//
//	statuses:
//	  feasted:
//	    name: Feasted
//	    changes:
//	      - max_hit_points: 2
//	    when_given:
//	      - heal: 2
//	    maintenance:
//	      - ends_on: run
//	    again: ignored
//
// A status may also write `when_given:` — what it does once, when it is first
// given (a heal, say) — as a non-empty list.
//
// THIS FILE JUDGES THE SHAPE, NOT THE WORDS INSIDE IT. The top-level words
// are settled; what may stand inside `changes`, `maintenance` and `when_given`
// is the rules' own table, which this module may not import (C1). So a status is
// refused here for a missing or misspelt top-level word and for an empty
// list, and carried as canonical JSON in the author's words; the session
// compiles it with the rules when it launches and when a row gives it.

// StatusSpec is one authored status, kept in the author's words: `name`,
// `changes`, `maintenance`, `again` and the optional `when_given`. Only those
// keys and their presence are judged here.
type StatusSpec map[string]any

// The words a status is written with, in the order a refusal lists them.
// when_given is the one a status may leave out.
const (
	statusName        = "name"
	statusChanges     = "changes"
	statusMaintenance = "maintenance"
	statusAgain       = "again"
	statusWhenGiven   = "when_given"
)

var statusWords = []string{statusName, statusChanges, statusMaintenance, statusAgain, statusWhenGiven}

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
		if raw, written := st[statusWhenGiven]; written {
			if items, ok := raw.([]any); !ok || len(items) == 0 {
				add(at+"."+statusWhenGiven, "when_given, when written, is a non-empty list: what the status does once, when it is first given")
			}
		}
		if _, ok := st[statusAgain]; !ok {
			add(at+"."+statusAgain, "a status says what a second giving does: write again")
		}
		if where, value, bad := unwritable(map[string]any(st), ""); bad {
			add(at, unwritableMessage(where, value))
		}
	}
	return declared
}

// unwritable finds the first value in a status that JSON cannot write — a
// YAML `.nan` or `.inf`, or a key that is not text — and says where it is,
// relative to the status, and what it is.
func unwritable(v any, where string) (string, string, bool) {
	switch x := v.(type) {
	case nil, bool, string, int, int64, uint64:
		return "", "", false
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return where, fmt.Sprint(x), true
		}
		return "", "", false
	case StatusSpec:
		return unwritable(map[string]any(x), where)
	case map[string]any:
		for _, k := range sortedKeys(x) {
			if w, val, bad := unwritable(x[k], joinPath(where, k)); bad {
				return w, val, true
			}
		}
		return "", "", false
	case []any:
		for i, item := range x {
			if w, val, bad := unwritable(item, fmt.Sprintf("%s[%d]", where, i)); bad {
				return w, val, true
			}
		}
		return "", "", false
	default:
		return where, fmt.Sprintf("%v", x), true
	}
}

func joinPath(where, key string) string {
	if where == "" {
		return key
	}
	return where + "." + key
}

// unwritableMessage is the refusal of a status holding a value it cannot
// carry.
func unwritableMessage(where, value string) string {
	return fmt.Sprintf("the value %s at %s cannot be written: a status holds only text, whole numbers, yes/no, "+
		"and lists and maps of those", value, where)
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
// Empty when the site declares none.
//
// FAILS CLOSED: a status that cannot be written is an error naming it, never
// a status quietly dropped — a dropped one would leave every row that gives
// it compiling with no grant.
func statusesOf(statuses map[string]StatusSpec) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(statuses))
	for _, id := range sortedStatusIDs(statuses) {
		st := statuses[id]
		if where, value, bad := unwritable(map[string]any(st), ""); bad {
			return nil, &statusError{id: id, message: unwritableMessage(where, value)}
		}
		raw, err := json.Marshal(map[string]any(st))
		if err != nil {
			return nil, &statusError{id: id, message: "it cannot be written: " + err.Error()}
		}
		out[id] = raw
	}
	return out, nil
}

// statusError is a declared status the compiler cannot carry, at its path.
type statusError struct{ id, message string }

func (e *statusError) Error() string { return "statuses." + e.id + ": " + e.message }

// statusGrantOf is a row's `apply_status:` as the encounter carries it: the
// id, the status's canonical JSON and who receives it.
//
// NEVER NIL FOR A NAMED STATUS: a status the site does not carry (refused by
// the grammar first) still compiles to a grant, with no block, and the
// encounter refuses that grant when it is built ("whose authored block is
// not a JSON object") — so a row that names a status can never compile
// quietly without its grant.
func statusGrantOf(a ApplyStatusSpec, statuses map[string]json.RawMessage) *encounter.StatusGrant {
	grant := &encounter.StatusGrant{ID: a.Status, Status: append(json.RawMessage(nil), statuses[a.Status]...), To: a.To}
	if a.Until != nil {
		grant.Until = &encounter.StatusUntil{Rounds: a.Until.Rounds}
	}
	return grant
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
