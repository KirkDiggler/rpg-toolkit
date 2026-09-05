// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package scenarios

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// holdout.go is the second scenario (rpg-project#375, the hold-out design
// §2, §3.10): a camp is hostile to the party until its mind learns better;
// convince it and the run ends.
//
// ONE BINDING AND ONE ENDING. The form asks which faction the party is here
// to convince; the run ends when that faction's stance toward the party
// folds to neutral — which is exactly the disposition's own `until`
// holding, so this scenario declares nothing the file did not already say.
// `convince: goblins` is sugar for an ending on
// `{ stance: { between: [goblins, party], is: neutral } }` (R10), and once
// endings are authorable in the file this package has nothing left to do —
// the north star's own test.
//
// # The refusals are the guidance
//
// The dungeon ALLOWS an until fact no record reveals (R8, pre-release: show
// the cost); this scenario refuses it, because a hold-out nobody can win is
// a scenario, not a dungeon defect. Each sentence below is both the form's
// help text and the error the author sees.

// HoldOutID is this scenario's key in a dungeon's `scenarios:` map, and the
// key of the ending it declares.
const HoldOutID = "hold-out"

// FieldConvince names the faction the party is there to convince.
const FieldConvince = "convince"

// convinceGuidance is the guidance, and the refusal text.
const convinceGuidance = "this scenario needs a faction to convince — which faction holds out against the party " +
	"until its mind learns better"

// holdOut is the scenario itself. Stateless, like every scenario.
type holdOut struct{}

func init() { register(holdOut{}) }

// ID is this scenario's key in a dungeon's `scenarios:` map.
func (holdOut) ID() string { return HoldOutID }

// Name is what the builder shows in the picker.
func (holdOut) Name() string { return "The hold-out" }

// Fields is the form: one field.
func (holdOut) Fields() []Field {
	return []Field{{
		Key: FieldConvince, Label: "Faction to convince",
		Type: FieldEntityRef, Kind: "faction", Guidance: convinceGuidance,
	}}
}

// New reads one filled-in form against the dungeon it is bound to.
//
// Refusals, each in the words the form itself uses (design §2):
//
//   - no faction named, or one this dungeon does not have;
//   - a faction with no mind — nothing can come to know anything for it;
//   - no hostile disposition with an until fact between it and `party`;
//   - an until fact no record in the dungeon reveals — "a hold-out nobody
//     can win".
//
// NOTHING IS DEFAULTED (rpg-toolkit#1033).
func (holdOut) New(cfg map[string]string, facts *DungeonFacts) (Declared, error) {
	if facts == nil {
		return Declared{}, fmt.Errorf("%s: no dungeon to bind to", HoldOutID)
	}

	convince := cfg[FieldConvince]
	if convince == "" {
		return Declared{}, fmt.Errorf("%s: %s", FieldConvince, convinceGuidance)
	}
	faction, has := facts.Factions[convince]
	if !has {
		return Declared{}, fmt.Errorf("%s: %q is not a faction this dungeon has — %s",
			FieldConvince, convince, convinceGuidance)
	}
	if !faction.CanLearn {
		return Declared{}, fmt.Errorf(
			"%s: %q has no mind, so nothing can come to know anything for it — name a mind under `factions:`, or %s",
			FieldConvince, convince, convinceGuidance)
	}
	fact, hostile := faction.UntilFact[encounter.FactionParty]
	if !hostile {
		return Declared{}, fmt.Errorf(
			"%s: %q is not hostile to the party until a fact — declare `{ between: [%s, party], stance: hostile, "+
				"until: { fact: <id> } }`, or %s",
			FieldConvince, convince, convince, convinceGuidance)
	}
	if !facts.Reveals[fact] {
		return Declared{}, fmt.Errorf(
			"%s: nothing in this dungeon reveals %q, so %q can never learn it — a hold-out nobody can win: "+
				"place an intel record with `reveals: { fact: %s }`, or %s",
			FieldConvince, fact, convince, fact, convinceGuidance)
	}

	return Declared{
		Endings: []encounter.EndingInput{{
			Key: HoldOutID,
			Trigger: encounter.TriggerStance{
				Between: [2]encounter.FactionID{convince, encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
			},
		}},
		Convince: convince,
	}, nil
}
