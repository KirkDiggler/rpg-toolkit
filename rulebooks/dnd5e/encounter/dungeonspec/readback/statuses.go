// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// statuses.go says the site's status effects (rpg-project#1024): one
// statement per declared status, and one under each row that gives one. What
// a status changes and how long it lasts are the rules' to say when it is
// given; the read-back says only its id and name.

// ConnStatus is the kind of a connection to a declared status: {status, <id>}.
// A row that gives one carries it with role [RoleGives].
const ConnStatus = "status"

// RoleGives is the role of a row's connection to the status it gives.
const RoleGives = "gives"

// statusName is a declared status's name, or its id when it has none (a
// status the compiler refused never reaches here).
func (b *builder) statusName(id string) string {
	if st, ok := b.spec.Statuses[id]; ok {
		if name, ok := st["name"].(string); ok && name != "" {
			return name
		}
	}
	return id
}

// statusStatements is one statement per declared status, in id order.
func (b *builder) statusStatements() []Statement {
	ids := make([]string, 0, len(b.spec.Statuses))
	for id := range b.spec.Statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Statement, 0, len(ids))
	for _, id := range ids {
		name := b.statusName(id)
		s := fmt.Sprintf("The status %s, named %s, is a status effect a line's row can give. What it changes "+
			"and what ends it are written in its own words under %s, and the rules judge them when it is given.",
			quote(id), said(name), quote("statuses."+id))
		out = append(out, b.fstmt(KindStatus, Subject{Kind: ConnStatus, ID: id}, "statuses."+id, s,
			foldInfo{site: "the status " + quote(id)}))
	}
	return out
}

// RoleReceives is the role of a status grant's connection to who receives
// it: {who, <word>, receives, at: <status id>} (rpg-project#990).
const RoleReceives = "receives"

// statusGrantStatement says a row's `apply_status`: who receives the status,
// in plain words, and the status by name, with a connection to the status
// and one to the who-word that receives it.
func (b *builder) statusGrantStatement(g encounter.StatusGrant, lead, path string, t lineTarget, f foldInfo) Statement {
	name := b.statusName(g.ID)
	who, short := receiverWords(g.To, t)
	// The ending written where it is given (rpg-project#1080): a clause between
	// the id's closing bracket and the full stop, and "for N rounds" folded.
	var until, foldedFor string
	if g.Until != nil {
		unit, passed := "rounds", "have"
		if g.Until.Rounds == 1 {
			unit, passed = "round", "has"
		}
		until = fmt.Sprintf(" until %d %s of world time %s passed, or sooner if one of the status's own endings comes first",
			g.Until.Rounds, unit, passed)
		foldedFor = fmt.Sprintf(" for %d %s", g.Until.Rounds, unit)
	}
	s := fmt.Sprintf("%s, %s is given the status %s (%s)%s. The rules apply it to them; a second giving does what the status's "+
		"`again` says.", lead, who, said(name), quote(g.ID), until)
	f.text = "gives " + short + " the status " + said(name) + foldedFor
	return b.fstmt(KindLineGrant, t.about, path+".apply_status", s, f,
		Connection{Kind: ConnStatus, ID: g.ID, Role: RoleGives},
		Connection{Kind: ConnWho, ID: g.To, Role: RoleReceives, At: g.ID})
}

// receiverWords is who receives a status, in full and folded: the speaker
// (the one who did it, for a thing done) whether or not anyone sees them;
// the speaker's party the creature spoken to sees; or that creature.
func receiverWords(to string, t lineTarget) (full, short string) {
	switch to {
	case string(encounter.RecognitionSpeaker):
		if t.done {
			return "the one who did it, whether or not anyone sees them,", "the one who did it"
		}
		return "the speaker, whether or not anyone sees them,", "the speaker"
	case string(encounter.RecognitionPartyInSight):
		return fmt.Sprintf("each member of the speaker's party that %s sees at that moment (the speaker too, if seen)", t.name),
			"the speaker's side in its sight"
	case string(encounter.RecognitionSpokenTo):
		return fmt.Sprintf("the one the line is said to, %s,", t.name), "the one spoken to"
	}
	return fmt.Sprintf("%q (a word this build cannot say)", to), fmt.Sprintf("%q", to)
}
