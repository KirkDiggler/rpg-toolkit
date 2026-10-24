// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// tablebeat.go is THE RECORD CARRIES ITS OWN SYMBOLS (rpg-project#674).
//
// An [BeatAnswered] beat names its rows by index: the entry that fired and a
// verdict for every row of the key. An index means nothing without the table
// it indexes, and the operator's ruling is that the record should read like a
// binary with its debug symbols loaded — no trip to the file to see what
// "entry 2" was. So the table itself rides in the record, as a [BeatTable]
// beat, ONCE: written immediately before the first `answered` a creature
// makes on a key, and again only when the table it would describe differs
// from the one it last described.
//
// THE MEMBER REMEMBERS WHAT IT LAST DESCRIBED as a hash per scope
// ([memberRecord.TableHash], persisted as [MemberData.TableHash]), so a
// reloaded encounter does not describe a table the record already holds, and
// a table edited in the studio and played again is described exactly once
// more.
//
// RECORD ONLY. A table is debug, not an act: the session's act-family table
// lists it record-only, so it never folds into what a player is shown.
//
// Rejected (#674): row text copied into every `answered` (duplication, and it
// cannot show the rows that did not fire), and a debug RPC that returns the
// tables (a second owner for a fact the record can carry; a replayed record
// file would lose its symbols).

// BeatTable is the "beat" value of the story beat that describes a
// creature's table for one key: every row as authored, and the temperament
// that loads it. Read beside [BeatAnswered], which names those rows by index.
//
// EXPORTED BECAUSE A DECODER READS IT, [BeatAnswered]'s reason.
const BeatTable = "table"

// tableScope is the name a creature's described table goes by in
// [memberRecord.TableHash]: the key alone for the creature's own table, and
// the option and key for a spoken option's rows — two options on one
// creature carry different rows under the same verdict key, and one hash for
// both would describe each afresh every time the other was answered.
func tableScope(option string, key AnswerKey) string {
	if option == "" {
		return string(key)
	}

	return option + "/" + string(key)
}

// describeTable appends the [BeatTable] for this creature's rows under this
// scope when the table it would describe differs from the one it last
// described, and remembers the new one. A table already described is
// silence.
//
// THE CREATURE'S OWN SUBJECT AUDIENCE, the audience its `time` pick is
// written to: the table is a fact about the creature, whoever witnessed the
// answer that made it worth describing.
func (e *Encounter) describeTable(
	creature MemberID, option string, key AnswerKey, rows []Answer, temper Temper, at uint64,
) error {
	m, ok := e.members[creature]
	if !ok {
		return fmt.Errorf("table beat: creature %q: %w", creature, ErrNotMember)
	}

	payload, err := json.Marshal(tableBeatBody(creature, option, key, rows, temper))
	if err != nil {
		return fmt.Errorf("table beat: marshal: %w", err)
	}
	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])
	scope := tableScope(option, key)
	if m.TableHash[scope] == hash {
		return nil
	}

	if _, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(subjectBeat, creature),
		Tags:     map[string]string{"tag": BeatTable},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("table beat: %w", err)
	}
	if m.TableHash == nil {
		m.TableHash = map[string]string{}
	}
	m.TableHash[scope] = hash

	return nil
}

// tableBeatBody is the [BeatTable] payload: the creature, the option (empty
// for its own table), the key, the temperament word and the percent it loads
// each word with, and every row as authored.
//
// EVERY FIELD IS WRITTEN UNCONDITIONALLY, the `answered` beat's rule, so a
// reader never gets a third state out of an absent key.
func tableBeatBody(
	creature MemberID, option string, key AnswerKey, rows []Answer, temper Temper,
) map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rows))
	for i, row := range rows {
		when := make([]string, 0, len(row.When))
		for j := range row.When {
			when = append(when, whenShorthand(&row.When[j]))
		}
		var search interface{}
		if row.Search != nil {
			search = map[string]interface{}{
				"distance": row.Search.Distance, "heading": string(row.Search.Heading),
			}
		}
		out = append(out, map[string]interface{}{
			"entry":    i,
			"when":     when,
			"word":     answerWord(row),
			"selector": selectorWord(row),
			"weight":   row.Weight,
			"say":      row.Say,
			"fact":     string(row.Fact),
			"search":   search,
		})
	}

	return map[string]interface{}{
		"beat":     BeatTable,
		"creature": string(creature),
		"option":   option,
		"key":      string(key),
		"temper":   temper.Word,
		"percents": temperPercents(temper),
		"rows":     out,
	}
}

// temperPercents is the percent the temperament loads each acting word with
// — 100 across the board for a soldier, the zero profile, exactly as
// [behavior.Pick] reads it. A search walks toward, and a fact or a bare line
// is never loaded (100), so those five numbers are the whole temperament.
func temperPercents(temper Temper) map[string]int {
	p := temper.Profile
	if p == (behavior.TemperProfile{}) {
		return map[string]int{"attack": 100, "toward": 100, "away": 100, "flee": 100, "hold": 100}
	}

	return map[string]int{"attack": p.Attack, "toward": p.Toward, "away": p.Away, "flee": p.Flee, "hold": p.Hold}
}

// whenShorthand is one condition in the file's own keys — the compact
// readout the studio shows beside a row ("enemy reach", "heard clang within
// 2", "fact alarm-raised", "not enemy seen", "an ally attacked within 3"), so
// a reader matches a row in the record to the row in the file.
//
// THE SAME ORDER [behavior.When]'s evaluator reads a condition in, so a When
// that set two things (refused at every door) reads here as it is judged.
func whenShorthand(w *When) string {
	switch {
	case w.Not != nil:
		return "not " + whenShorthand(w.Not)
	case w.Heard != nil:
		return "heard " + w.Heard.Kind + " within " + strconv.Itoa(w.Heard.Within)
	case w.Fact != "":
		return "fact " + w.Fact
	case w.Enemy != "":
		return "enemy " + string(w.Enemy)
	}

	deed := w.Deed + " within " + strconv.Itoa(w.Within)
	switch w.Scope {
	case behavior.ScopeAlly:
		return "an ally " + deed
	case behavior.ScopeActor:
		return "I " + deed
	default:
		return deed
	}
}

// rowVerdicts is the `rows` of an [BeatAnswered] payload: one verdict per row
// of the key, in authored order — whether its conditions held, whether it was
// affordable, and for a candidate its weight, percent and load.
func rowVerdicts(chosen *Pick) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(chosen.Rows))
	for _, r := range chosen.Rows {
		out = append(out, map[string]interface{}{
			"entry": r.Entry, "held": r.Held, "affordable": r.Affordable,
			"weight": r.Weight, "percent": r.Percent, "loaded": r.Loaded,
		})
	}

	return out
}
