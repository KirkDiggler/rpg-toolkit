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
	// BlessedName is the display name of the named additive roll composition.
	BlessedName = "Blessed"
	// BlessedContributionGroup selects the oldest applicable Bless contribution.
	BlessedContributionGroup = "bless"
)

var blessedDescriptor = namedLinkedDescriptor(namedLinkedDescriptorInput{
	Ref:     refs.Conditions.Blessed(),
	Display: Display{Name: BlessedName, Detail: "Adds 1d4 to attack rolls and saving throws. Multiple Bless effects do not add extra dice to the same roll."},
	Parts:   blessedParts,
})

func blessedParts() ([]effects.PartDeclaration, error) {
	compiled, err := effects.CompileParts(&effects.CompilePartsInput{Parts: []effects.PartSpec{
		{
			Key: "roll",
			Effect: effects.AddToRoll{
				Amount:    effects.SignedDice{Notation: "1d4", Polarity: effects.Positive},
				Rolls:     []contributions.RollKind{contributions.RollKindAttack, contributions.RollKindSavingThrow},
				Selection: generic.SelectionRule{Mode: generic.OldestApplicable, Group: BlessedContributionGroup},
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
