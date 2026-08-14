// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	monsterActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
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
		return fmt.Errorf("%w: %q is both a character and a monster", ErrBadParticipant, p.ID())
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

	// Roller is used to reconstitute effects that need one — a monster's Undead
	// Fortitude, for instance, which rolls when it is triggered rather than
	// when it is loaded. Nil takes the default roller. It is not the machine's
	// roller: a machine that rolls carries its own.
	Roller dice.Roller
}

// Validate reports whether this input describes a resolvable interaction.
func (in *Input) Validate() error {
	if in == nil {
		return ErrNilInput
	}

	if in.Machine == nil {
		return ErrNoMachine
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

	return nil
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
	if roller == nil {
		roller = dice.NewRoller()
	}

	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:     in.World,
		Deciders: in.Deciders,
	})
	if err != nil {
		return nil, fmt.Errorf("resolution: load world: %w", err)
	}

	cast, err := attachAll(ctx, surf, in.Participants, roller)
	if err != nil {
		// Tear down whatever did attach before giving up: a half-attached bus
		// is about to be garbage either way, but leaving revocation to the
		// error path's silence is how leaks become normal.
		_ = surf.teardown(ctx)
		return nil, err
	}

	outcome, runErr := drive(ctx, surf, in.Machine, cast)

	// R5: revoke everything granted, whether or not the machine succeeded.
	tearErr := surf.teardown(ctx)

	if runErr != nil {
		return nil, fmt.Errorf("resolution: %w", runErr)
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
			ch, err := character.LoadFromData(ctx, p.Character, view)
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

// attachMonster performs the three-call assembly the monster package documents.
//
// All three calls, not one: monster.LoadFromData deliberately loads neither
// actions nor conditions (both would be import cycles), and ToData serializes
// both. Skipping either call therefore does not merely leave a monster
// underpowered — it writes back a monster that has silently lost them.
func attachMonster(
	ctx context.Context, view *surface, data *monster.Data, roller dice.Roller,
) (*monster.Monster, error) {
	m, err := monster.LoadFromData(ctx, data, view)
	if err != nil {
		return nil, err
	}

	if err := monsterActions.LoadMonsterActions(m, data.Actions); err != nil {
		return nil, err
	}

	if err := monstertraits.LoadMonsterConditions(ctx, m, data.Conditions, view, roller); err != nil {
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
