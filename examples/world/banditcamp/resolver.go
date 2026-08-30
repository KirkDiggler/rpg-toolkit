// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package banditcamp

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
var ErrNoSheet = errors.New("banditcamp: no character sheet for actor")

// ErrNoRoller reports a resolver built without dice.
var ErrNoRoller = errors.New("banditcamp: roller is required")

// ErrNoBus reports a resolver built without an event bus.
//
// The bus is not optional here on purpose. Without one, dnd5e's ability-check
// chain never fires, so a condition granting advantage is silently ignored and
// every check quietly comes out a little wrong. That degradation has no error
// to point at, which is exactly why the dependency is refused rather than
// defaulted.
var ErrNoBus = errors.New("banditcamp: event bus is required")

// CheckResolverConfig supplies the resolver's parts.
type CheckResolverConfig struct {
	// Sheets maps a world entity to the character who is that entity. Only
	// actors who can attempt things need one.
	Sheets map[journal.EntityID]*character.Character

	// Roller is the dice. Inject a scripted one and the whole camp is
	// reproducible.
	Roller dice.Roller

	// Bus carries dnd5e's ability-check chain.
	Bus events.EventBus
}

// CheckResolver resolves world attempts with real D&D 5e ability checks.
//
// This is the entire rulebook seam. The kernel hands over an actor, an approach
// and a difficulty; this turns them into [checks.MakeAbilityCheck] against a
// real character sheet — real ability modifier, real proficiency bonus, real
// expertise — and hands back success, margin, and a line of transcript. Nothing
// upstream learns that a d20 was involved.
//
// Swapping D&D 5e for another rulebook is a different implementation of
// [world.Resolver] and nothing else.
type CheckResolver struct {
	sheets map[journal.EntityID]*character.Character
	roller dice.Roller
	bus    events.EventBus
}

// NewCheckResolver returns a resolver over the given sheets.
//
// Returns [ErrNoRoller] or [ErrNoBus].
func NewCheckResolver(cfg CheckResolverConfig) (*CheckResolver, error) {
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

	return &CheckResolver{sheets: sheets, roller: cfg.Roller, bus: cfg.Bus}, nil
}

// Resolve makes the ability check.
//
// Returns [ErrNoSheet] for an unknown actor, or an error for an approach D&D 5e
// has no skill for. Both are wiring faults: the attempt could not be judged,
// which is not the same as failing it.
func (r *CheckResolver) Resolve(ctx context.Context, a world.Attempt) (journal.Outcome, error) {
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
