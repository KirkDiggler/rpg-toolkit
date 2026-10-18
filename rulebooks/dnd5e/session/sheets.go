// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// How fast everyone is, what they swing, how far they see and how hard they
// listen, answered where the sheets are (rpg-project#538, "Sheet facts asked
// at use time"; passive Perception rpg-project#569 R5).
//
// The composition holds no speed, sight, attack or targeting and cannot hold
// any (law C1, R6), so it asks through two capabilities — [encounter.Sheets]
// for speed, attacks and targeting, [encounter.Sight] for how far — and this
// file answers both from the sheets the verb holds: the host's character
// store for a player, the session's own stat blocks for a monster, and the
// session's recorded content for a world NPC. It is the same wire
// [standingSeam] and [equipmentSeam] are, built the same way.
//
// # One owner per fact, no copies
//
// Join, Launch and PlaceNPC hand the composition none of these facts; it asks
// at the moment it paces a walk, budgets a driven turn, tests reach, builds a
// driver's view or refreshes a percept. A level gained, a weapon swapped or a
// stat block changed is read at the next ask without any verb owning a
// refresh. This package's own push budget (castmove.go) asks the same answer.

// monsterWatchResetCells is the seeded re-arm distance, in cells, for a
// monster watcher (rpg-project#606, design R10 and open item O6): a sneaker
// must stand this far outside the monster's zone before the next entry rolls
// again, so a zone edge pacing over a still sneaker does not roll every step.
// A content fact with nowhere to live yet — the stat block states no watch
// terms — and it leaves when `senses` can say it.
const monsterWatchResetCells = 2

// sheetSeam answers the composition's sheet and sight questions out of the
// sheets this one verb holds.
//
// COMPILE-TIME PROOF that this package satisfies the composition's contract.
var _ encounter.Sheets = sheetSeam{}

// # It is per call, and it has to be
//
// The context rides on the struct for [standingSeam]'s reason: the
// capability's methods take no context because the composition calling them
// has none to give, so the verb's own is captured when the seam is built, and
// the seam is built fresh for every verb (S1, S4).
//
// # No cache, deliberately
//
// Every consult re-reads. A verb that equips a weapon or levels a character
// writes the sheet part-way through itself, and an answer remembered from the
// top of the call would describe a sheet that no longer exists — the exact
// copy rpg-project#538 retired from the composition, moved one layer up.
//
// THE SAME FIELDS AS [standingSeam], AND NONE OF ITS METHODS: a defined type
// over it, so the two capabilities share where the sheets are and which verb
// is asking while staying separate contracts answering separate questions.
type sheetSeam standingSeam

// unhalved is this seam answering as if nobody sneaks: the live encounter
// slot cleared on a copy, so every other fact is read exactly as before and
// the shared slot itself is untouched. For a move imposed on a member, which
// is not the member's own walking (see the halving in [sheetSeam.factsOf]).
func (s sheetSeam) unhalved() sheetSeam {
	s.live = nil
	return s
}

// sheetsBeside builds the sheet capability from the verb-scoped facts a
// [standingSeam] already carries, for [equipmentBeside]'s reason: the roster
// kind snapshot is the verb's one classification, and a second snapshot could
// disagree with it. The kinds map is shared, not copied, so a member [place]
// classifies mid-verb is known here before its own Join asks about it.
func sheetsBeside(s standingSeam) sheetSeam {
	return sheetSeam(s)
}

// Sheets reports each given member's speed, attacks and targeting, read off
// its sheet at this moment.
//
//   - A player answers from the resolution door's projection of its record:
//     walking speed from its race, and its attack from the main-hand fold
//     (an empty hand is an unarmed strike) — the same projection Join reads.
//     A player has no targeting strategy; empty is what its sheet says. Its
//     passive Perception is the loaded sheet's own answer.
//   - A monster answers from its stat block: SpeedData.Walk, every executable
//     action definition in author order, the targeting word the author's
//     placement wrote onto it at spawn, and the block's passive Perception.
//   - A world NPC is stationary and non-acting by construction (design N4):
//     its recorded content states no speed, no actions and no senses, so the
//     true answer from that content is zero speed, nothing to swing and a
//     passive Perception of zero. Answered only when its content is recorded.
//
// ONLY ABOUT WHO WAS ASKED: the loop is over the question, because the
// composition refuses a stranger in the answer (ErrNotMember).
//
// A member the verb holds no sheet for is refused, never answered with zero
// (zero is a real speed): [ErrNoCharacter] for a player whose record is not
// found, [ErrNoSheet] for a monster or world NPC the session never recorded.
// Any error aborts whatever verb was running, atomically (R5).
func (s sheetSeam) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		facts, err := s.factsOf(id)
		if err != nil {
			return nil, err
		}
		out[id] = facts
	}

	return out, nil
}

// factsOf reads one member's sheet facts.
func (s sheetSeam) factsOf(id encounter.MemberID) (encounter.SheetFacts, error) {
	held, err := s.sheetOf(id)
	if err != nil {
		return encounter.SheetFacts{}, err
	}

	switch {
	case held.character != nil:
		projected, projectErr := projectCharacter(s.ctx, string(id), held.character)
		if projectErr != nil {
			return encounter.SheetFacts{}, projectErr
		}
		passive, passiveErr := characterPassivePerception(s.ctx, string(id), held.character)
		if passiveErr != nil {
			return encounter.SheetFacts{}, passiveErr
		}
		speed := projected.Sheet.SpeedFeet
		// HALF SPEED WHILE SNEAKING (rpg-project#606, design R8): the cost
		// is time, so the answer every pace and budget asks for is the
		// session's, and the encounter never learns the rule. Integer feet,
		// rounded down. A fight engaging the sneaker ends the mode before
		// any turn budget asks (R7), so only free-roam pace sees the half.
		//
		// ONLY THE SNEAKER'S OWN WALKING (lead ruling on rpg-toolkit#2079,
		// operator may veto): a move imposed on the sneaker — a cast push
		// that runs it at its speed, any MoveImposed — runs at full speed,
		// the same own-steps line facing draws (R17). The push budget asks
		// [sheetSpeeds], which reads this seam with the slot cleared
		// ([sheetSeam.unhalved]).
		if s.live.sneaking(id) {
			speed /= 2
		}
		return encounter.SheetFacts{
			SpeedFeet:         speed,
			Actions:           memberActionsOf(projected.Actions),
			PassivePerception: passive,
			// A character is never a watcher (design R9), so it re-arms
			// nobody; zero is the encounter's no-hysteresis answer.
			WatchResetCells: 0,
		}, nil
	case held.monster != nil:
		bonusDisengage, err := bonusDisengageOf(held.monster, registryName)
		if err != nil {
			return encounter.SheetFacts{}, err
		}
		return encounter.SheetFacts{
			SpeedFeet:      held.monster.Speed.Walk,
			Actions:        memberActionsOf(held.monster.Actions),
			Targeting:      held.monster.Targeting.String(),
			BonusDisengage: bonusDisengage,
			// The stat block's own number, stored on it when the block was
			// assembled (monster.FromTemplate derives 10 + WIS, plus
			// proficiency when trained) and the same field
			// monster.Monster.PassivePerception answers once loaded. Read
			// off the record rather than a loaded monster for Speed's
			// reason: it is a stored fact, and loading a sheet to read one
			// field back would be the same number by a longer road.
			PassivePerception: held.monster.Senses.PassivePerception,
			// How far a sneaker must stand outside this watcher's zone
			// before the pair re-arms (design R10, O6). SEEDED HERE until
			// content states it on the stat block; the encounter picks no
			// distance of its own.
			WatchResetCells: monsterWatchResetCells,
		}, nil
	default:
		return encounter.SheetFacts{}, nil
	}
}

// characterPassivePerception is how hard a player listens without trying —
// the sheet's own answer ([character.Character.PassivePerception]: 10 plus
// its Perception modifier, proficiency and expertise included), asked of the
// stored record at this consult (rpg-project#569 R5).
//
// THE SHEET ANSWERS; THIS SEAM DOES NO ARITHMETIC. character.Load is
// bus-free — the same reconstitution [Manager.loadAttackSheet] makes — so it
// attaches nothing and subscribes nothing; it only turns the record into the
// sheet that knows its own skills.
//
// A record that will not load is refused as [ErrBadCharacter], never
// answered zero: zero is a real answer (deaf above DC 0), and a sheet that
// cannot be read has given none.
func characterPassivePerception(ctx context.Context, id string, record *character.Data) (int, error) {
	sheet, err := character.Load(ctx, record)
	if err != nil {
		return 0, fmt.Errorf("character %q: %w: %v", id, ErrBadCharacter, err)
	}

	return sheet.PassivePerception(), nil
}

// bonusDisengageOf names the trait that lets a monster Disengage as a bonus
// action, or the zero identity when its stat block carries none
// (rpg-toolkit#2032). The stored ref is the fact — read with
// [conditions.HoldsRef] off the sheet, loading nothing — and the name is the
// conditions registry's own rendering of it, so the story says what the
// descriptor says.
//
// A PLAYER IS NEVER ASKED: its bonus actions are its sheet's own economy,
// spent through Activate, so [sheetSeam.factsOf] leaves its identity zero.
//
// nameOf is the registry's name lookup — [registryName] in production —
// taken as a parameter so the refusal below is testable against a registry
// that lacks the trait.
//
// Refuses a stat block whose stored conditions cannot be read
// ([ErrBadNPC]), and a trait with no registry descriptor
// ([ErrUnknownContent]) rather than telling a nameless beat.
func bonusDisengageOf(
	block *monster.Data, nameOf func(core.Ref) (string, bool),
) (encounter.ActivationIdentity, error) {
	trait := refs.MonsterTraits.NimbleEscape()
	held, err := conditions.HoldsRef(block.Conditions, trait)
	if err != nil {
		return encounter.ActivationIdentity{}, fmt.Errorf("monster %q: %w: %v", block.ID, ErrBadNPC, err)
	}
	if !held {
		return encounter.ActivationIdentity{}, nil
	}
	name, known := nameOf(*trait)
	if !known {
		return encounter.ActivationIdentity{}, fmt.Errorf("monster %q: trait %s has no display descriptor: %w",
			block.ID, trait.String(), ErrUnknownContent)
	}
	return encounter.ActivationIdentity{Ref: trait.String(), Name: name}, nil
}

// registryName is the conditions registry's display name for ref, and
// whether the registry knows it ([conditions.DisplayFor]).
func registryName(ref core.Ref) (string, bool) {
	display, known := conditions.DisplayFor(ref)
	return display.Name, known
}

// heldSheet is the one record a member's sheet facts are read from: exactly
// one of character or monster is set, or neither for a world NPC whose
// content was found.
type heldSheet struct {
	character *character.Data
	monster   *monster.Data
}

// sheetOf finds the sheet behind one member, by the verb's authoritative
// roster kind — never by guessing from whichever store happens to answer.
//
// It refuses, by name, a member it holds no sheet for: a player whose record
// the host's store does not have ([ErrNoCharacter]), and a monster or world
// NPC the session never recorded ([ErrNoSheet]). This is where it differs
// from [standingSeam.recordsFor], which tolerates a sheetless member because
// "is it down" has a true answer for a body nobody can read; "how fast is it"
// does not.
func (s sheetSeam) sheetOf(id encounter.MemberID) (heldSheet, error) {
	name := string(id)
	kind, ok := s.kinds[name]
	if !ok {
		return heldSheet{}, fmt.Errorf("member %q has no roster kind: %w", name, ErrInvalidSession)
	}

	switch kind {
	case encounter.KindWorld:
		if s.data == nil {
			return heldSheet{}, fmt.Errorf("world npc %q: %w", name, ErrNoSheet)
		}
		if _, err := findWorldNPCIndex(s.data, name); err != nil {
			return heldSheet{}, err
		}
		return heldSheet{}, nil
	case encounter.KindMonster:
		sheet, found := npcSheet(s.data, name)
		if !found {
			return heldSheet{}, fmt.Errorf("monster %q: %w", name, ErrNoSheet)
		}
		return heldSheet{monster: sheet}, nil
	case encounter.KindPlayer:
	default:
		return heldSheet{}, fmt.Errorf("member %q has unknown roster kind %q: %w", name, kind, ErrInvalidSession)
	}

	data, err := s.sheets.load(s.ctx, "character", name)
	if err != nil {
		return heldSheet{}, err
	}

	return heldSheet{character: data}, nil
}
