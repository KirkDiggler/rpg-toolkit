// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// untrained.go is THE UNTRAINED RULE, AND IT IS ONE FILE ON PURPOSE
// (rpg-project#457 R2, ideas/shenanigans/front-room-goblin.md).
//
// # The rule
//
// A character with no proficiency in the skill a verb rolls takes
// DISADVANTAGE on that check. Against a goblin's DC 9 with +0 Charisma: a
// plain roll is about 60%, disadvantage about 36%, and a proficient +2 about
// 70%. Taking Intimidation at creation now matters at the table.
//
// # It is a divergence from the letter, and it is ruled
//
// The 2014 rules put NO penalty on an untrained ability check: d20 plus the
// ability modifier, and proficiency adds its bonus. Kirk ruled the divergence
// on rpg-project#457 and scoped it to SKILL VERBS — Intimidate, Persuade,
// later Deceive: "the disadvantage is to make taking Intimidation worth
// something". Search and Unlock are not skill verbs and leave
// [CheckInput.Untrained] false; the doc that adds a verb says whether it takes
// the rule.
//
// # Returning to RAW is a DELETION
//
// Kirk: "I am aware I am diverging from RAW. Ideally flipping that to RAW
// would be an easy refactor." So the rule is ONE FUNCTION below, called from
// ONE PLACE ([makeCheckOn]), and going back to the letter is emptying that
// function's body — not hunting for a special case in the modifier math. A
// build that spread this across the sheet, the check and the offer would have
// built it wrong; the sheet answers only "how well is this character trained"
// ([character.Character.SkillProficiency]) and forms no opinion.
//
// The mutation test for that claim is in untrained_test.go: empty the body and
// exactly the scenes that assert the rule fail.
//
// # It reaches the roll the way every other source does
//
// Through the AbilityCheckChain, on the interaction's own bus, as a named
// disadvantage source — so the result's source list says "Untrained imposed
// disadvantage" the way it already says "Raging granted advantage", and the
// subscription is torn down by the surface with everything else.

// untrainedSource puts the untrained rule's disadvantage on this check, when
// it applies — and IS the whole rule.
//
// Four things have to be true, and any one of them missing means this does
// nothing at all:
//
//  1. the caller said this check takes the rule ([CheckInput.Untrained]);
//  2. the applied approach resolved to a SKILL — a bare-ability route
//     (Strength to shove a door) has no training to lack, and the zero
//     [skills.Skill] is how [bestApproach] reports one;
//  3. the character's training in that skill is [shared.NotProficient];
//  4. there is a checker to ask, which there always is by this point.
//
// SUBSCRIBED RATHER THAN PASSED AS A FLAG. [checks.AbilityCheckInput] has a
// HasDisadvantage bool, and using it would record the source as "Input" — a
// roll that tells a player their luck was bad instead of telling them what to
// take next level. The chain is where a named source belongs, and resolution
// is the only lawful owner of this bus (ADR-0038).
func untrainedSource(
	ctx context.Context, surf *surface, ch *character.Character, in *CheckInput, skill skills.Skill,
) error {
	if !in.Untrained || skill == "" || ch == nil {
		return nil
	}
	if ch.SkillProficiency(skill) != shared.NotProficient {
		return nil
	}

	checker := ch.GetID()
	impose := func(
		_ context.Context,
		event *dnd5eEvents.AbilityCheckChainEvent,
		c chain.Chain[*dnd5eEvents.AbilityCheckChainEvent],
	) (chain.Chain[*dnd5eEvents.AbilityCheckChainEvent], error) {
		if event.CheckerID != checker || event.Skill != skill {
			return c, nil
		}
		add := func(
			_ context.Context, e *dnd5eEvents.AbilityCheckChainEvent,
		) (*dnd5eEvents.AbilityCheckChainEvent, error) {
			e.DisadvantageSources = append(e.DisadvantageSources, dnd5eEvents.CheckModifierSource{
				Name:       untrainedName,
				SourceType: untrainedKind,
				SourceRef:  refs.Rules.Untrained(),
				EntityID:   checker,
			})

			return e, nil
		}
		// StageBase, not StageConditions: training is a base fact about the
		// roll, the same rung proficiency and the ability modifier sit on.
		// Nothing carries this and nothing can dispel it.
		if err := c.Add(combat.StageBase, untrainedStage, add); err != nil {
			return c, fmt.Errorf("untrained: %w", err)
		}

		return c, nil
	}

	if _, err := dnd5eEvents.AbilityCheckChain.On(surf).SubscribeWithChain(ctx, impose); err != nil {
		return fmt.Errorf("untrained: %w", err)
	}

	return nil
}

const (
	// untrainedName is what the log calls it. A display name rather than a
	// ref string, because this is what a player reads.
	untrainedName = "Untrained"

	// untrainedKind is the source type. "rule" rather than "condition" or
	// "feature": nobody carries this, nobody granted it, and nothing can
	// dispel it — it is how we play.
	untrainedKind = "rule"

	// untrainedStage names the chain entry, so two of them on one check would
	// collide loudly instead of stacking silently.
	untrainedStage = "untrained_disadvantage"
)
