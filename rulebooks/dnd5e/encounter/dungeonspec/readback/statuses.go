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
		s := fmt.Sprintf("The status %s, named %s, is a status effect a line's row can give its speaker. What it changes "+
			"and what ends it are written in its own words under %s, and the rules judge them when it is given.",
			quote(id), said(name), quote("statuses."+id))
		out = append(out, b.fstmt(KindStatus, Subject{Kind: ConnStatus, ID: id}, "statuses."+id, s,
			foldInfo{site: "the status " + quote(id)}))
	}
	return out
}

// statusGrantStatement says a row's `apply_status`: the speaker is given the
// status, by name, with a connection to it.
func (b *builder) statusGrantStatement(g encounter.StatusGrant, lead, path string, t lineTarget, f foldInfo) Statement {
	name := b.statusName(g.ID)
	who := "the speaker"
	if t.done {
		who = "the one who did it"
	}
	s := fmt.Sprintf("%s, %s is given the status %s (%s). The rules apply it to them; a second giving does what the status's "+
		"`again` says.", lead, who, said(name), quote(g.ID))
	f.text = "gives the speaker the status " + said(name)
	return b.fstmt(KindLineGrant, t.about, path+".apply_status", s, f, Connection{Kind: ConnStatus, ID: g.ID, Role: RoleGives})
}
