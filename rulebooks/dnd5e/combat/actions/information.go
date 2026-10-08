package actions

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

// Information is read-only explanation and assembled base facts. Contextual
// effects remain separate; this value is never execution or selector input.
type Information struct {
	Description string              `json:"description"`
	Details     []InformationDetail `json:"details"`
}

// InformationDetail is one ordered, provider-authored display fact. Labels can
// repeat for distinct damage pools. Its text is not an expression for consumers
// to evaluate and is not permission to act.
type InformationDetail struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// DescribeInput supplies the already-assembled action, not a raw weapon lookup.
// Description is content metadata, and may be empty when the author supplied none.
type DescribeInput struct {
	Definition Definition
}

// DescribeOutput contains the action's own information, without effect folding.
type DescribeOutput struct {
	Information Information
}

// Describe explains a validated definition without modifying it, asking a
// target, rolling, spending or folding effects. Base damage uses the exact
// assembled dice/type and the same ability-inclusion rule execution consumes.
// Nil or malformed input is an error; absent prose is not invented.
func Describe(in *DescribeInput) (*DescribeOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("describe action: input is required")
	}
	if err := in.Definition.Validate(); err != nil {
		return nil, fmt.Errorf("describe action: %w", err)
	}
	info := Information{Description: in.Definition.Description, Details: []InformationDetail{}}
	attack := in.Definition.Attack
	if attack == nil && in.Definition.Cast != nil {
		attack = in.Definition.Cast.Attack
	}
	if attack != nil {
		for _, pool := range attack.Damage {
			value := pool.Dice
			if pool.FlatBonus != 0 {
				value += fmt.Sprintf(" %+d", pool.FlatBonus)
			}
			if attack.Ability != nil && pool.HasProperty(damage.AddsAttackAbilityModifier) &&
				damage.IncludesAbilityModifier(damage.AbilityModifierInput{
					Modifier: attack.Ability.Modifier, OffHand: attack.IsOffHandAttack,
				}) {
				value += fmt.Sprintf(" + %s modifier (%+d)", strings.ToUpper(string(attack.Ability.Ability)), attack.Ability.Modifier)
			}
			info.Details = append(info.Details, InformationDetail{Label: "Base damage", Value: value + " · " + pool.Type.Display()})
		}
		if attack.Delivery.Melee != nil {
			info.Details = append(info.Details, InformationDetail{Label: "Reach", Value: fmt.Sprintf("%d ft", attack.Delivery.Melee.ReachFeet)})
		}
		if attack.Delivery.Ranged != nil {
			info.Details = append(info.Details, InformationDetail{Label: "Range", Value: fmt.Sprintf("%d ft (long %d ft)", attack.Delivery.Ranged.NormalFeet, attack.Delivery.Ranged.LongFeet)})
		}
	}
	return &DescribeOutput{Information: info}, nil
}

// BasicActionKind names the existing non-definition action content. It is not
// the session's verb type and does not grant a host a new executable action.
type BasicActionKind string

const (
	// BasicMove describes choosing a movement path.
	BasicMove BasicActionKind = "move"
	// BasicEndTurn describes finishing the current turn.
	BasicEndTurn BasicActionKind = "end_turn"
	// BasicDeathSave describes the explicit dying-character roll.
	BasicDeathSave BasicActionKind = "death_save"
	// BasicIntimidate describes threatening a creature.
	BasicIntimidate BasicActionKind = "intimidate"
	// BasicPersuade describes trying to change a creature's mind.
	BasicPersuade BasicActionKind = "persuade"
)

// BasicInformationInput selects existing rulebook-owned explanatory content.
type BasicInformationInput struct {
	Kind BasicActionKind
}

// BasicInformation returns the rulebook's explanation of a basic action, with
// no price or availability inference. Unknown kinds return empty information,
// not a made-up description or permission to execute.
func BasicInformation(in BasicInformationInput) Information {
	var description string
	switch in.Kind {
	case BasicMove:
		description = "Move along your chosen path, using the movement available to you."
	case BasicEndTurn:
		description = "Finish your turn and let the next combatant act."
	case BasicDeathSave:
		description = "Roll a death saving throw while dying. Successes help you stabilize; failures bring you closer to death."
	case BasicIntimidate:
		description = "Threaten a creature to influence its response. The outcome depends on the creature and the situation."
	case BasicPersuade:
		description = "Try to change a creature's mind through conversation. The outcome depends on the creature and the situation."
	}
	return Information{Description: description}
}
