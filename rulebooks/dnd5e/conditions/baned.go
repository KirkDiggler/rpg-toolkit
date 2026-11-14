// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	generic "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/effects"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const (
	// BanedName is the display name of the named subtractive roll composition.
	BanedName = "Baned"
	// BanedContributionGroup selects the oldest applicable Bane contribution.
	BanedContributionGroup = "bane"
)

var banedDescriptor = namedLinkedDescriptor(namedLinkedDescriptorInput{
	Ref:     refs.Conditions.Baned(),
	Display: Display{Name: BanedName, Detail: "Subtracts 1d4 from attack rolls and saving throws. Multiple Bane effects do not subtract extra dice from the same roll."},
	Parts:   banedParts,
})

func banedParts() ([]effects.PartDeclaration, error) {
	compiled, err := effects.CompileParts(&effects.CompilePartsInput{Parts: []effects.PartSpec{
		{
			Key: "roll",
			Effect: effects.AddToRoll{
				Amount:    effects.SignedDice{Notation: "1d4", Polarity: effects.Negative},
				Rolls:     []contributions.RollKind{contributions.RollKindAttack, contributions.RollKindSavingThrow},
				Selection: generic.SelectionRule{Mode: generic.OldestApplicable, Group: BanedContributionGroup},
			},
			OnTerminal: generic.EndPart,
		},
		{Key: "rest", Effect: effects.EndOn{Beat: effects.BeatRest, Reason: "rest"}, OnTerminal: generic.EndAttachment},
	}})
	if err != nil {
		return nil, err
	}
	return compiled.Parts, nil
}
