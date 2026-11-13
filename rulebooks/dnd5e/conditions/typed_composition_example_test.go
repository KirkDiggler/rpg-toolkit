// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"fmt"

	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
)

// This is the complete effect definition. Source identity, payment, recipients
// and delivery belong to the source/host below, not inside this composition.
func typedRagingRing() (*conditions.CompiledDefinitions, error) {
	bonus := 2
	benefits, err := conditions.RagingBenefits(&conditions.RagingBenefitsInput{
		DamageBonus: conditions.RagingDamageBonus{Fixed: &bonus},
	})
	if err != nil {
		return nil, err
	}
	compiled, err := conditions.CompileComposition(&conditions.CompileCompositionInput{
		ID: "raging-ring",
		Spec: conditions.CompositionSpec{
			Name: "Maintained rage", Coordinator: conditions.DefinitionUser,
			Benefits: benefits.Parts,
			Maintenance: []effects.PartSpec{
				{Key: "concentration", Effect: effects.Concentration{}, OnTerminal: generic.EndApplication},
				{Key: "empty", Effect: effects.UntilEmpty{}, OnTerminal: generic.EndApplication},
				{Key: "rest", Effect: effects.EndOn{Beat: effects.BeatRest, Reason: "rest"}, OnTerminal: generic.EndApplication},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return compiled.Definitions, nil
}

func typedRingTranscript() (result string, err error) {
	definitions, err := typedRagingRing()
	if err != nil {
		return "", err
	}
	world, err := newDefinitionWorld()
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := world.close(); err == nil {
			err = closeErr
		}
	}()
	source := newFixtureSource("ring", "A", "raging-ring")
	grant, err := useDefinition(context.Background(), definitionUseInput{Source: source, RecipientID: "B", ApplicationID: "ring-use", Definitions: definitions, Participants: bindingParticipants(world.sheets)})
	if err != nil {
		return "", err
	}
	if err := world.apply(grant.Prepared); err != nil {
		return "", err
	}
	for range 10 {
		if err := world.turnEnd("B"); err != nil {
			return "", err
		}
	}
	if err := world.reload(); err != nil {
		return "", err
	}
	owner, err := world.coordinator("ring-use")
	if err != nil {
		return "", err
	}
	outgoing, err := world.damageBonus("B", "C", abilities.STR, true, true)
	if err != nil {
		return "", err
	}
	incoming, err := world.incoming("B", damage.Slashing, 9)
	if err != nil {
		return "", err
	}
	result = fmt.Sprintf("idle recipient after ten turn ends and reload: maintainer=%s damage=%d physical=%d uses=%d\n", owner, totalFixtureDamage(outgoing), incoming.Types[0].Taken, source.Budget.Current)
	taken, err := world.hit("A", damage.Fire, 6)
	if err != nil {
		return "", err
	}
	if len(taken.FollowUps) != 1 {
		return "", fmt.Errorf("expected one real concentration follow-up")
	}
	failure := taken.FollowUps[0].OnFailure
	// The fixture host chooses failure; this is not a native save-window roll.
	if err := world.remove(failure.Owner, failure.Owner.SourceID, failure.Reason); err != nil {
		return "", err
	}
	if err := world.binding.CheckComplete(); err != nil {
		return "", err
	}
	result += fmt.Sprintf("test-selected concentration failure: grant-ended=%t uses=%d", len(world.holder("B").GetConditions()) == 0, source.Budget.Current)
	return result, nil
}

func Example_typedRagingRing() {
	transcript, err := typedRingTranscript()
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(transcript)
	// Output:
	// idle recipient after ten turn ends and reload: maintainer=A damage=12 physical=4 uses=0
	// test-selected concentration failure: grant-ended=true uses=0
}
