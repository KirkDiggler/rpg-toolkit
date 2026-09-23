// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// strikerSeam is this package's real encounter.Striker, bound to the live
// scope.enc a write verb is already operating on (rpg-project#254: "supply
// Striker from the strike machinery bound to the live scope.enc").
//
// A STRUCT HOLDING A *writeScope RATHER THAN A CLOSURE OVER ONE, so it can
// be constructed BEFORE scope.enc exists — encounter.LoadEncounter needs a
// Striker to construct the very *encounter.Encounter this seam's own Strike
// method later receives as a parameter, so scope is allocated with its data
// (and later enc) fields set in the order openForWrite needs, and this seam
// only ever reads scope AT CALL TIME, well after that ordering is settled.
//
// NO SECOND LOAD: attacker and target sheets come out of scope.data.NPCs
// and m.characters exactly as castFor already reads them for a player's own
// swing — this is the identical machinery, reached from the other
// direction.
type strikerSeam struct {
	m     *Manager
	scope *writeScope
}

// compile-time proof the seam satisfies what it is handed to.
var _ encounter.Striker = strikerSeam{}

// Strike resolves attacker's declared action against target and records the
// outcome itself via enc.Record — the same public verb Manager.Attack's own
// swing already uses, so a monster's blow and a player's land on the story
// through one path.
//
// NEVER ADOPTS A NEW *encounter.Encounter. enc is the SAME live object
// driveMonsterTurns is mid-loop over — swapping it out from inside a call
// enc itself made would orphan that loop. The world resolution.Resolve
// returns is read for its dirty sheets alone (out.DirtyCharacters,
// out.DirtyMonsters) and otherwise discarded; hit points and conditions
// live on SHEETS, not on the encounter's own story-and-position state, so
// nothing about them requires swapping enc.
//
// Attacker is always a monster: nothing outside this package's own
// TurnDriver-driven turns ever reaches this method, and only an unplayed
// member — always KindMonster today — has a TurnDriver in the first place.
func (s strikerSeam) Strike(
	ctx context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	var attackerData *monster.Data
	for i := range s.scope.data.NPCs {
		if s.scope.data.NPCs[i].ID == string(attacker) {
			attackerData = &s.scope.data.NPCs[i]
			break
		}
	}
	if attackerData == nil {
		return fmt.Errorf("strike: attacker %q: %w", attacker, ErrNoSheet)
	}

	var definition combatActions.Definition
	var found bool
	for i := range attackerData.Actions {
		if attackerData.Actions[i].Ref == action {
			definition = attackerData.Actions[i].Clone()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("strike: attacker %q: action %q: %w", attacker, action.String(), ErrBadAttack)
	}

	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("strike: %w", translate(err))
	}

	// No readied sheet: a monster attacker has no action-economy readying
	// step the way a character's own swing does (Manager.priceSwing). If
	// persisted content declares a non-nil cost anyway, resolution sees it at
	// the same door and refuses the monster payer rather than silently
	// inventing monster economy here.
	cast, err := s.m.castFor(ctx, s.scope, roster, nil)
	if err != nil {
		return fmt.Errorf("strike: %w", err)
	}

	machine, err := resolution.NewAction(&resolution.ActionInput{
		Definition: definition,
		AttackerID: string(attacker),
		TargetID:   string(target),
		// The attacker's WHOLE action list, because a sequence's steps name
		// components off it and this seam is the one place holding the stat
		// block they live on. Supplied for every arm: a strike ignores it,
		// and branching here on which arm the definition populated would put
		// profile dispatch on this side of the seam.
		Components: attackerData.Actions,
		Roller:     &diceSeam{roller: s.m.dice},
	})
	if err != nil {
		return fmt.Errorf("strike: attacker %q: %w: %v", attacker, ErrBadAttack, err)
	}

	var cost *resolution.Cost
	if definition.Cost != nil {
		cost = &resolution.Cost{PayerID: string(attacker), Profile: definition.Cost}
	}

	// A pure view for resolution's Input.World — a mid-verb read, never the
	// storage boundary (encounter v0.43.0, #1385).
	world := enc.WorldView()
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:        world,
		Participants: cast,
		Initiative:   s.m.initiative,
		Standing:     s.scope.standing,
		Sight:        &sightSeam{members: worldMembers(world)},
		Equipment:    equipmentBeside(s.scope.standing),
		TurnDriver:   s.scope.driver,
		// The concealment pair (rpg-toolkit#1378), bound to the same live
		// scope openForWrite and adopt bind — the one-seam consistency law:
		// a concealed world refuses to reconstruct without them, and
		// resolution carries them without consulting either, since no verb
		// runs inside an interaction.
		CheckResolver: checkSeam(s),
		Witness:       witnessSeam{scope: s.scope},
		Cost:          cost,
		Machine:       machine,
		Roller:        &diceSeam{roller: s.m.dice},
	})
	if err != nil {
		return fmt.Errorf("strike: %w", translateResolution(err))
	}

	if out.Posed != nil {
		if out.Posed.BeforeRoll || out.Posed.Sequence != nil {
			if err := s.m.saveDirty(ctx, s.scope, out); err != nil {
				return err
			}
			p := pendingAttackWindowPayload{Attacker: string(attacker), Target: string(target), Definition: definition, Components: attackerData.Actions}
			if out.Posed.Sequence != nil {
				if err := s.m.recordPendingSequence(s.scope, &p, *out.Posed.Sequence); err != nil {
					return err
				}
			}
			if err := posePendingAttackWindow(s.scope, out.Posed, p); err != nil {
				return err
			}
			return encounter.ErrStrikePaused
		}
		if out.Posed.SettledStrike == nil {
			return fmt.Errorf("strike: %w: unsupported pre-hit monster question", ErrInvalidWorld)
		}
		if err := s.m.saveDirty(ctx, s.scope, out); err != nil {
			return err
		}
		in := &AttackInput{Attacker: string(attacker), Target: string(target)}
		if _, err := enc.Record(recordFor(in, *out.Posed.SettledStrike, definition, "", out)); err != nil {
			return translate(err)
		}
		if err := posePostHitWindow(s.scope, out.Posed); err != nil {
			return err
		}
		return encounter.ErrStrikePaused
	}
	// Sheets are written ONCE for the whole interaction, before any beat is
	// recorded, whether the action landed one blow or several.
	if err := s.m.saveDirty(ctx, s.scope, out); err != nil {
		return fmt.Errorf("strike: %w", err)
	}

	in := &AttackInput{Attacker: string(attacker), Target: string(target)}

	switch produced := out.Outcome.(type) {
	case resolution.StrikeOutcome:
		// NO PRESENTATION TOKEN: nobody declared this roll. A monster's swing
		// is resolved by the driver, no client simulated its die, and there is
		// therefore no throw for a witness to correlate against — see recordFor.
		if _, err := enc.Record(recordFor(in, produced, definition, "", out)); err != nil {
			return fmt.Errorf("strike: %w", translate(err))
		}
		return nil

	case resolution.SequenceOutcome:
		return s.recordSequence(enc, in, produced, attackerData.Actions)

	default:
		return fmt.Errorf("strike: %w: strike produced %T", ErrInvalidWorld, out.Outcome)
	}
}

// recordSequence writes ONE BEAT PER SWING.
//
// # Why not one beat for the whole multiattack
//
// Because a beat is a roll. Every field the story keeps about an attack — the
// d20 and its keep record, the total, the AC it was compared against, the
// damage components, whether it crit — is singular, and a beat carrying two
// swings could only answer each of those once. The client that draws a die
// tray draws one throw per beat for the same reason.
//
// So the goblin boss's Multiattack is two Struck/Missed beats in order, each
// naming the COMPONENT that swung — "Scimitar", not "Multiattack" — because
// that is the identity a client maps to a model, an icon and a damage type,
// and it is what actually hit.
//
// # The deed is landed twice and keyed once
//
// Encounter.Record lands an attack deed keyed actor#verb, so the second swing
// overwrites the first's. That is the right answer rather than a gap: a `when`
// condition reading "this creature was attacked" wants the fact, not a count,
// and two swings in one action are one attack as far as a witness is
// concerned.
//
// # Concentration rides the swing that forced it
//
// Each step carries its OWN checks and breaks (Kirk's ruling, 2026-09-20), so
// this seam copies two slice headers per beat and still knows nothing about
// what is in them. That is the same arrangement recordStrike already documents
// for a lone swing, and it is why the split lives in resolution rather than
// here: which blow caused which break is a rule, and a seam that worked it out
// would be a seam having an opinion.
//
// out.ConcentrationChecks and out.ConcentrationBreaks are EMPTY for a
// sequence, deliberately, so reading both places cannot record one save twice.
func (s strikerSeam) recordSequence(
	enc *encounter.Encounter, in *AttackInput, sequence resolution.SequenceOutcome,
	repertoire []combatActions.Definition,
) error {
	if len(sequence.Steps) == 0 {
		// Refused rather than returned as a quiet success: a sequence that
		// produced no swing is a machine defect, and a silent return would
		// look exactly like a turn where nothing was in reach.
		return fmt.Errorf("strike: %w: %s swung nothing", ErrInvalidWorld, sequence.Action.String())
	}

	for index, step := range sequence.Steps {
		component, found := definitionFor(repertoire, step.Action)
		if !found {
			// Unreachable through resolution, which resolved these very refs
			// against this very list before the first swing. Named anyway,
			// because a beat labelled with the wrong weapon is worse than a
			// turn that failed.
			return fmt.Errorf("strike: %w: %s swung %s, which the attacker does not carry",
				ErrBadAttack, sequence.Action.String(), step.Action.String())
		}

		recorded := recordStrike(in.Attacker, in.Target, step.Strike,
			attackRefFor(component), "", step.ConcentrationChecks, step.ConcentrationBreaks)
		if _, err := enc.Record(recorded); err != nil {
			return fmt.Errorf("strike: step %d: %w", index, translate(err))
		}
	}

	return nil
}

// definitionFor finds one action in the attacker's own list. Linear, over a
// stat block's worth of entries.
func definitionFor(
	repertoire []combatActions.Definition, ref core.Ref,
) (combatActions.Definition, bool) {
	for i := range repertoire {
		if repertoire[i].Ref == ref {
			return repertoire[i], true
		}
	}

	return combatActions.Definition{}, false
}
