// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dnd5eresolver is the D&D 5e side of the composer's resolver seam.
//
// It is the entire rulebook adapter. The composer hands over an actor, an
// approach and a difficulty; this turns them into a real ability check against
// a real character sheet — real ability modifier, real proficiency bonus, real
// expertise — and hands back success, margin, and a line of transcript.
// Nothing above it learns that a d20 was involved.
//
// It lives beside the scenarios rather than inside one because a resolver is
// not content. Every scenario needs one and none of them should have to import
// another scenario to get it; swapping D&D 5e for another rulebook is a
// different package implementing the same interface, and no scenario changes.
package dnd5eresolver

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/checks"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"

	"github.com/KirkDiggler/rpg-toolkit/examples/world"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

// ErrNoSheet reports an attempt by somebody the resolver has no character for.
var ErrNoSheet = errors.New(
	"nobody has a character sheet by that name — everyone who can attempt something needs one here")

// ErrNoRoller reports a resolver built without dice.
var ErrNoRoller = errors.New("this needs dice — something to roll the checks the rules ask for")

// ErrNoBus reports a resolver built without an event bus.
//
// The bus is not optional here on purpose. Without one, dnd5e's ability-check
// chain never fires, so a condition granting advantage is silently ignored and
// every check quietly comes out a little wrong. That degradation has no error
// to point at, which is exactly why the dependency is refused rather than
// defaulted.
var ErrNoBus = errors.New(
	"this needs an event bus — without one, a condition that would grant advantage is ignored and " +
		"every check comes out quietly wrong")

// Config supplies the resolver's parts.
type Config struct {
	// Sheets maps a world entity to the character who is that entity. Only
	// actors who can attempt things need one.
	Sheets map[journal.EntityID]*character.Character

	// Roller is the dice. Inject a scripted one and the whole camp is
	// reproducible.
	Roller dice.Roller

	// Bus carries dnd5e's ability-check chain.
	Bus events.EventBus
}

// Resolver resolves world attempts with real D&D 5e ability checks.
//
// This is the entire rulebook seam. The kernel hands over an actor, an approach
// and a difficulty; this turns them into [checks.MakeAbilityCheck] against a
// real character sheet — real ability modifier, real proficiency bonus, real
// expertise — and hands back success, margin, and a line of transcript. Nothing
// upstream learns that a d20 was involved.
//
// Swapping D&D 5e for another rulebook is a different implementation of
// [world.Resolver] and nothing else.
type Resolver struct {
	sheets map[journal.EntityID]*character.Character
	roller dice.Roller
	bus    events.EventBus
}

// New returns a resolver over the given sheets.
//
// Returns [ErrNoRoller] or [ErrNoBus].
func New(cfg Config) (*Resolver, error) {
	if cfg.Roller == nil {
		return nil, ErrNoRoller
	}
	if cfg.Bus == nil {
		return nil, ErrNoBus
	}

	sheets := make(map[journal.EntityID]*character.Character, len(cfg.Sheets))
	for id, sheet := range cfg.Sheets {
		sheets[id] = sheet
	}

	return &Resolver{sheets: sheets, roller: cfg.Roller, bus: cfg.Bus}, nil
}

// Resolve makes the ability check.
//
// Returns [ErrNoSheet] for an unknown actor, or an error for an approach D&D 5e
// has no skill for. Both are wiring faults: the attempt could not be judged,
// which is not the same as failing it.
func (r *Resolver) Resolve(ctx context.Context, a world.Attempt) (journal.Outcome, error) {
	sheet, ok := r.sheets[a.Actor]
	if !ok {
		return journal.Outcome{}, fmt.Errorf("%w: %q", ErrNoSheet, a.Actor)
	}

	skill, err := skills.GetByID(string(a.Approach))
	if err != nil {
		return journal.Outcome{}, fmt.Errorf("approach %q: %w", a.Approach, err)
	}

	modifier := sheet.GetSkillModifier(skill)
	result, err := checks.MakeAbilityCheck(ctx, &checks.AbilityCheckInput{
		Roller:    r.roller,
		EventBus:  r.bus,
		CheckerID: string(a.Actor),
		Skill:     skill,
		DC:        a.Difficulty,
		Modifier:  modifier,
	})
	if err != nil {
		return journal.Outcome{}, fmt.Errorf("%s check for %q: %w", skill, a.Actor, err)
	}

	return journal.Outcome{
		Contested: true,
		Succeeded: result.Success,
		Margin:    result.Total - result.DC,
		Detail: fmt.Sprintf("%s: d20(%d)%+d = %d vs DC %d",
			skill, result.Roll, modifier, result.Total, result.DC),
	}, nil
}
