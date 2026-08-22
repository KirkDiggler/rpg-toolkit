// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
)

// Participant is one member's sheet on the way in. Exactly one of Character or
// Monster is set.
//
// A union rather than two slices so that "everyone in the interaction" is one
// ordered list — which is what R4's sorted attach order and R3's pass-everyone-in
// are both statements about.
type Participant struct {
	// Character is a player's sheet, loaded from the host's repository.
	Character *character.Data

	// Monster is a monster's sheet, either instantiated from the catalog or
	// loaded from the host's repository.
	Monster *monster.Data
}

// ID returns the participant's identity, or "" when the participant is empty.
func (p Participant) ID() string {
	switch {
	case p.Character != nil:
		return p.Character.ID
	case p.Monster != nil:
		return p.Monster.ID
	default:
		return ""
	}
}

func (p Participant) validate() error {
	if p.Character != nil && p.Monster != nil {
		// Both IDs, not p.ID() — which reads the character branch and would
		// silently hide the monster's when the two differ.
		return fmt.Errorf("%w: one participant carries character %q and monster %q",
			ErrBadParticipant, p.Character.ID, p.Monster.ID)
	}

	if p.Character == nil && p.Monster == nil {
		return fmt.Errorf("%w: neither a character nor a monster", ErrBadParticipant)
	}

	if p.ID() == "" {
		return fmt.Errorf("%w: no id", ErrBadParticipant)
	}

	return nil
}

// Input is everything one interaction needs. All of it is data (R2).
type Input struct {
	// World is the encounter the interaction happens in, as the host stored it.
	World encounter.EncounterData

	// Deciders re-attaches behaviour to non-player members. Nil is legal and
	// means no member acts on its own. Passed straight through to the
	// encounter, which owns what a decider may do.
	Deciders map[encounter.MemberID]encounter.Decider

	// Participants are the sheets of everyone in the interaction. Pass everyone
	// (R3) — applicability is the effect's predicate, not the caller's guess.
	Participants []Participant

	// Machine is the interaction to run.
	Machine Machine

	// Cost is what this interaction costs the actor who declared it. NIL IS A
	// FREE ACTION and is the common case today — only a caller that compiled a
	// price passes one.
	//
	// It is paid HERE, before the machine starts, and the machine is never told:
	// it cannot tell a swing that cost an action from one that cost nothing,
	// which is the ignorance the ruling asks for, because what an action costs
	// was already answered by whoever compiled the profile. See [Cost], and
	// "The door pays" in this package's doc.
	Cost *Cost

	// Initiative orders a fight that starts while this interaction runs.
	// REQUIRED.
	//
	// The composition asks for one at construction and refuses without it
	// (rpg-toolkit#964): sight starts fights on its own, so an encounter that
	// cannot order one is an encounter that cannot be loaded. This package
	// does not know what initiative is and does not want to — it hands over
	// what the caller supplied, the way Deciders one field up are handed over.
	Initiative encounter.InitiativeRoller

	// Standing reports which members are down. REQUIRED.
	//
	// Carried, never consulted. This package loads the world and reads it back
	// out as data; no verb that refreshes sight runs in between, so nothing
	// here ever asks the question. The composition still refuses to load
	// without one (rpg-toolkit#1077), and answering on the caller's behalf —
	// "nobody is down" — would be this package deciding a rule about hit
	// points it holds none of. So it is handed over, the way Deciders and
	// Initiative above are handed over, and the caller that owns the sheets
	// owns the answer (rpg-toolkit#1079).
	Standing encounter.Standing

	// Sight reports how far each member can see, in cells. REQUIRED.
	//
	// Carried, never consulted, for exactly the reason Standing one field up
	// is — and the mechanism is worth naming rather than asserting. The
	// composition asks how far somebody can see at one choke point, where it
	// rebuilds percepts; the only two callers of that are its own Setup and
	// its refreshSight, and this package calls neither. It loads a world and
	// reads it back out as data, so the question is never put.
	//
	// The composition still refuses to load without one (rpg-toolkit#1111),
	// and answering on the caller's behalf — any number at all — would be this
	// package deciding a 5e rule about light and darkvision it holds none of.
	// So it is handed over, the way Deciders, Initiative and Standing above
	// are handed over, and the caller that owns the sheets owns the answer.
	Sight encounter.Sight

	// Roller is used to reconstitute effects that need one — a monster's Undead
	// Fortitude, for instance, which rolls when it is triggered rather than
	// when it is loaded. REQUIRED. It is not the machine's roller: a machine
	// that rolls carries its own.
	//
	// It used to say "nil takes the default roller", and that stopped being
	// true when rpg-toolkit#1033 refused the default — a nil silently became
	// real randomness, which put unreproducible rolls into results that looked
	// fine. Validate has answered ErrNoRoller ever since; only this line had
	// not caught up.
	Roller dice.Roller

	// TurnDriver decides what a member with no player does when a fight's
	// clock lands on their turn. REQUIRED.
	//
	// Carried, never consulted — the same shape as Standing and Sight, and for
	// the same reason: this package loads the world, runs one interaction, and
	// reads it back out as data, so it never calls EndTurn, form, Transfer or
	// Exit itself. The composition still refuses to load without one
	// (rpg-toolkit#1162), and answering on the caller's behalf — "it passes" —
	// would be this package deciding a rule it holds no opinion on. So it is
	// handed over, the way Deciders, Initiative, Standing and Sight above are.
	TurnDriver encounter.TurnDriver
}

// Validate reports whether this input describes a resolvable interaction.
func (in *Input) Validate() error {
	if in == nil {
		return ErrNilInput
	}

	if in.Machine == nil {
		return ErrNoMachine
	}

	// CAPABILITIES ARE SUPPLIED, NEVER DEFAULTED. Both of these are things
	// only the caller can provide, and both used to be forgivable — Initiative
	// was absent entirely, and a nil Roller quietly became real randomness.
	// Kirk's ruling on the composition's own roller applies to each: "a nil
	// initiative is an error returned way upstream". A silent default masks
	// missing wiring, and for a roller it does it in the worst possible way,
	// by putting untestable randomness into a result that looks fine.
	if in.Initiative == nil {
		return ErrNoInitiative
	}
	if in.Standing == nil {
		return ErrNoStanding
	}
	if in.Sight == nil {
		return ErrNoSight
	}
	if in.TurnDriver == nil {
		return ErrNoTurnDriver
	}
	if in.Roller == nil {
		return ErrNoRoller
	}

	seen := make(map[string]struct{}, len(in.Participants))
	for _, p := range in.Participants {
		if err := p.validate(); err != nil {
			return err
		}

		if _, dup := seen[p.ID()]; dup {
			// Two sheets under one ID would attach twice and be written back
			// once, so the second load silently wins. Refusing is the only
			// honest answer.
			return fmt.Errorf("%w: %q appears twice", ErrBadParticipant, p.ID())
		}
		seen[p.ID()] = struct{}{}
	}

	// Last, because it is the only clause that talks about somebody in the cast
	// above: a cost names a payer, and whether the cast is one-sheet-per-ID is
	// the question that has to be settled before naming anybody in it.
	return in.Cost.validate()
}

// Output is everything the interaction produced. All of it is data (R2).
type Output struct {
	// World is the encounter after the interaction, ready to be stored.
	//
	// It round-trips even when the interaction never reads it. That is
	// deliberate: without it, Resolve is a saving throw with extra steps, and
	// load-act-save is asserted rather than demonstrated.
	World encounter.EncounterData

	// DirtyCharacters and DirtyMonsters are the sheets that changed, and only
	// those. A host writes back exactly what it is handed.
	DirtyCharacters []*character.Data
	DirtyMonsters   []*monster.Data

	// Outcome is what the machine produced.
	Outcome Outcome

	// Hooks is every subscription resolution granted, in the order granted.
	// It is the pre-execution picture of what was attached, the record of which
	// effect attached what, and the proof that an effect attached nothing.
	Hooks []Registration
}

// Participants is the loaded cast of an interaction: the sheets after they were
// reconstituted and attached, addressable by ID.
//
// A machine reads rules off these. It is what "step machines over data" means
// in practice — the machine gets the entities, never the bus.
type Participants struct {
	characters map[string]*character.Character
	monsters   map[string]*monster.Monster
	order      []string
}

// Character returns the loaded character with this ID.
func (p *Participants) Character(id string) (*character.Character, bool) {
	c, ok := p.characters[id]
	return c, ok
}

// Monster returns the loaded monster with this ID.
func (p *Participants) Monster(id string) (*monster.Monster, bool) {
	m, ok := p.monsters[id]
	return m, ok
}

// IDs returns every participant's ID, in the order they were attached.
func (p *Participants) IDs() []string {
	out := make([]string, len(p.order))
	copy(out, p.order)

	return out
}

// Resolve runs one interaction and returns what it produced.
//
// The body is ADR-0038's loop in its order: create the bus, load the world,
// attach every participant through an instrumented surface in sorted order,
// run the machine, tear everything down, hand back data. Nothing survives the
// call — not the bus, not a loaded sheet, not a subscription.
func Resolve(ctx context.Context, in *Input) (*Output, error) {
	// R1: one bus for this interaction, shared by everyone in it. Created here
	// and nowhere else, which is the whole claim this package makes.
	return resolveOn(ctx, in, newSurface(events.NewEventBus()))
}

// resolveOn is Resolve with the surface handed in, so a test can hold the bus
// underneath and check what is left on it afterwards. Unexported because a
// caller supplying its own bus would be a caller keeping one alive, which is
// the thing this package exists to prevent.
func resolveOn(ctx context.Context, in *Input, surf *surface) (*Output, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}

	roller := in.Roller

	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:       in.World,
		Deciders:   in.Deciders,
		Initiative: in.Initiative,
		Standing:   in.Standing,
		Sight:      in.Sight,
		TurnDriver: in.TurnDriver,
	})
	if err != nil {
		return nil, fmt.Errorf("resolution: load world: %w", err)
	}

	// INSTALL THE WORLD. One world, and it is installed every time.
	//
	// This is the map the encounter itself runs on — its own room, handed over
	// to be read (rpg-toolkit#1114). Not a reconstruction of it: this package
	// used to build a room out of the encounter's persisted description, with
	// its own copy of grid construction and a comment promising the two would
	// be kept in step, and no walls in it at all. What the rules read now is
	// what the composition enforces, because they are the same object.
	//
	// EVERY TIME, which is the other half. "Which room describes this
	// interaction" used to be a question, and the answer this package gave
	// when it could not decide — install nothing — silently switched off every
	// predicate that reads positions the moment one party member wandered off,
	// which in a dungeon is most of the time (rpg-toolkit#1090). There is one
	// map, so there is nothing to choose between, and no input can produce an
	// interaction without a world. TestNoCodePathProducesARoomlessInteraction
	// holds that structurally rather than by example.
	//
	// It is READ-ONLY: the composition refuses a write through it by name, and
	// this package has no business making one. Moving somebody is a verb of the
	// encounter's, and it is a different interaction.
	room, err := enc.Canvas()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadWorld, err)
	}
	ctx = gamectx.WithRoom(ctx, room)

	cast, err := attachAll(ctx, surf, in.Participants, roller)
	if err != nil {
		// Tear down whatever did attach before giving up: a half-attached bus
		// is about to be garbage either way, but leaving revocation to the
		// error path's silence is how leaks become normal.
		_ = surf.teardown(ctx)
		return nil, err
	}

	// THE DOOR PAYS, AND IT PAYS BEFORE THE MACHINE RUNS. A resolution nobody
	// can pay for never starts one, which is R5's clean case and is only true
	// while there is still nothing to undo. The teardown is joined rather than
	// swallowed the way the attach failure above swallows it: everything
	// attached successfully here, so a subscription outliving a refused
	// interaction is a leak rather than the tail of a half-finished attach.
	if payErr := payAtTheDoor(ctx, in.Cost, cast); payErr != nil {
		return nil, errors.Join(payErr, surf.teardown(ctx))
	}

	outcome, runErr := drive(ctx, surf, in.Machine, cast)

	// R5: revoke everything granted, whether or not the machine succeeded.
	tearErr := surf.teardown(ctx)

	if runErr != nil {
		// The machine's error leads, but a teardown failure on this path is
		// the one that must not be masked — it means subscriptions outlived a
		// failed interaction. Join keeps both reachable by errors.Is and drops
		// a nil tearErr. No extra prefix: the package's sentinels and its
		// machines' errors already name themselves, and re-wrapping here was
		// yielding "resolution: resolution: ...".
		return nil, errors.Join(runErr, tearErr)
	}
	if tearErr != nil {
		return nil, fmt.Errorf("resolution: teardown: %w", tearErr)
	}

	return &Output{
		World:           enc.ToData(),
		DirtyCharacters: dirtyCharacters(cast),
		DirtyMonsters:   dirtyMonsters(cast),
		Outcome:         outcome,
		Hooks:           surf.registrations(),
	}, nil
}

// attachAll reconstitutes every participant onto the surface, in sorted ID
// order (R4). Two resolutions over identical data must grant identical
// registrations in an identical order, or a suspension cannot be resumed into
// the world it left.
func attachAll(
	ctx context.Context, surf *surface, participants []Participant, roller dice.Roller,
) (*Participants, error) {
	ordered := make([]Participant, len(participants))
	copy(ordered, participants)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID() < ordered[j].ID() })

	cast := &Participants{
		characters: make(map[string]*character.Character),
		monsters:   make(map[string]*monster.Monster),
		order:      make([]string, 0, len(ordered)),
	}

	for _, p := range ordered {
		id := p.ID()

		// Every sheet gets its own view of the one bus, so a subscription can
		// be traced back to whose sheet was being read when it was made.
		view := surf.forParticipant(id)

		switch {
		case p.Character != nil:
			ch, err := attachCharacter(ctx, view, p.Character)
			if err != nil {
				return nil, fmt.Errorf("resolution: attach character %q: %w", id, err)
			}
			cast.characters[id] = ch

		case p.Monster != nil:
			m, err := attachMonster(ctx, view, p.Monster, roller)
			if err != nil {
				return nil, fmt.Errorf("resolution: attach monster %q: %w", id, err)
			}
			cast.monsters[id] = m
		}

		cast.order = append(cast.order, id)
	}

	return cast, nil
}

// attachCharacter loads a sheet and puts it on this participant's view of the
// bus — two calls, and the split between them is the point.
//
// character.Load is data → sheet: no bus, no subscriptions, nothing applied.
// character.Attach is the loop this package used to delegate: the sheet's own
// keeper first, then each condition through a bus scoped to the ref its loader
// routed on. The view implements dnd5eEvents.EffectScoper, so that scoping is
// what fills the registration list — attribution by construction, and now by
// construction *here*, rather than inside a constructor two modules away
// (rpg-toolkit#985).
//
// Loading strictly is the behaviour change that comes with it: a condition blob
// that will not parse fails the resolution, naming the blob, where the legacy
// path logged and carried on. Resolution is the wrong place to be forgiving —
// it hands back sheets to be persisted, so an effect quietly dropped on the way
// in is an effect deleted on the way out (rpg-toolkit#948).
func attachCharacter(
	ctx context.Context, view *surface, data *character.Data,
) (*character.Character, error) {
	ch, err := character.Load(ctx, data)
	if err != nil {
		return nil, err
	}

	if err := character.Attach(ctx, ch, view); err != nil {
		return nil, err
	}

	return ch, nil
}

// attachMonster is the same two calls, over the composition that knows what a
// whole monster is.
//
// monstertraits.LoadMonster is the pure load — sheet, actions, and the trait
// blobs it was persisted with — and it lives in that package because it is the
// only one that can see both loaders without an import cycle. Calling it rather
// than assembling the pieces here is safer by construction than by test: the
// actions round-trip pin would catch a forgotten LoadMonsterActions, but a
// composition that cannot forget it needs no pin at all, and this caller is the
// one it was written for.
//
// The trait blobs ride on the loaded monster rather than being passed alongside
// it, so the assembly's other old failure — writing a monster back without the
// conditions a skipped call would have restored — is unreachable too. And a
// failed attach is a no-op: the blobs go back, whatever was applied comes off,
// and nothing half-attached survives the error this returns.
func attachMonster(
	ctx context.Context, view *surface, data *monster.Data, roller dice.Roller,
) (*monster.Monster, error) {
	m, err := monstertraits.LoadMonster(ctx, data)
	if err != nil {
		return nil, err
	}

	if err := monstertraits.AttachMonster(ctx, m, view, roller); err != nil {
		return nil, err
	}

	return m, nil
}

// dirtyCharacters snapshots the characters that changed.
//
// Cleanup is never called first (R7): its first statement nils the conditions
// that ToData is about to serialize, so a "tidy" snapshot is a lossy one.
func dirtyCharacters(cast *Participants) []*character.Data {
	var out []*character.Data
	for _, id := range cast.order {
		ch, ok := cast.characters[id]
		if !ok || !ch.IsDirty() {
			continue
		}
		out = append(out, ch.ToData())
	}

	return out
}

func dirtyMonsters(cast *Participants) []*monster.Data {
	var out []*monster.Data
	for _, id := range cast.order {
		m, ok := cast.monsters[id]
		if !ok || !m.IsDirty() {
			continue
		}
		out = append(out, m.ToData())
	}

	return out
}
