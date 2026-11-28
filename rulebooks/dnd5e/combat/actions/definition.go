package actions

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// Definition identifies an action, its optional price, and the profile a
// resolution machine interprets. Exactly one profile must be populated, with
// one exception: a description may carry no profile when it is only its price
// ([Definition.OnlyItsPrice]), which is how Action Surge is said.
//
// The profile is how content chooses a sequence without naming one. A second
// arm is not a second kind of definition: it is a second thing content can
// declare, and the machine that reads it is chosen by which arm is populated
// rather than by what the definition is called.
type Definition struct {
	// Description is rulebook-authored explanatory content. It is prose, not a
	// rule: it is not in a declaration selector's allow-list, so editing it can
	// never change an action's identity.
	Description string               `json:"description,omitempty"`
	Ref         core.Ref             `json:"ref"`
	Name        string               `json:"name"`
	Cost        *combat.SpendProfile `json:"cost,omitempty"`
	Attack      *AttackProfile       `json:"attack,omitempty"`
	Cast        *CastProfile         `json:"cast,omitempty"`

	// Effect is the body of an action that does something to someone without
	// a weapon and without a spell: Second Wind's healing of its user, or a
	// named status put on its user, as Rage's raging.
	Effect *EffectProfile `json:"effect,omitempty"`

	// Sequence is the script arm: several component actions the same actor
	// performs under one declaration, which is the SRD's Multiattack. The
	// components are ordinary definitions on the same actor, so a sequence
	// adds a way to SAY a thing rather than a second way to author an attack.
	Sequence *SequenceProfile `json:"sequence,omitempty"`
}

// OnlyItsPrice reports whether this description is nothing but its price: it
// carries no profile, and its cost gives at least one of a turn's slots. Action
// Surge is the first: spend a use, take one more action. Whoever runs a
// description asks here, so the rule is stated once.
func (d Definition) OnlyItsPrice() bool {
	return d.Attack == nil && d.Cast == nil && d.Sequence == nil && d.Effect == nil &&
		d.Cost != nil && len(d.Cost.GrantSlots) > 0
}

// StatusOnUser returns the status this description puts on its user, and
// whether it puts one: its effect lands on its user and is a status. Whoever
// asks "does she already hold it" asks here for which status, so that is
// stated once.
func (d Definition) StatusOnUser() (core.Ref, bool) {
	if d.Effect == nil || d.Effect.To != EffectSelf || d.Effect.Condition == nil {
		return core.Ref{}, false
	}
	return d.Effect.Condition.Ref, true
}

// Validate reports whether the definition has complete identity, a valid
// optional price, and exactly one valid profile, or no profile when the
// description is only its price ([Definition.OnlyItsPrice]). Two or more
// profiles are always refused. The profile rule runs before the cost's own
// check, and a slot gift counts toward it, so a malformed gift is refused by
// the cost's sentence rather than as a missing profile.
func (d Definition) Validate() error {
	if err := d.Ref.IsValid(); err != nil {
		return fmt.Errorf("action ref is invalid: %w", err)
	}
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("action name must not be empty")
	}
	populated := 0
	if d.Attack != nil {
		populated++
	}
	if d.Cast != nil {
		populated++
	}
	if d.Sequence != nil {
		populated++
	}
	if d.Effect != nil {
		populated++
	}
	if populated > 1 {
		return fmt.Errorf("action must populate exactly one profile")
	}
	if populated == 0 && !d.OnlyItsPrice() {
		return fmt.Errorf("action must populate exactly one profile, or give a turn slot as its price")
	}
	if d.Cost != nil {
		if err := d.Cost.Validate(); err != nil {
			return fmt.Errorf("action cost is invalid: %w", err)
		}
	}
	if d.Attack != nil {
		if err := d.Attack.Validate(); err != nil {
			return fmt.Errorf("attack profile is invalid: %w", err)
		}
	}
	if d.Cast != nil {
		if err := d.Cast.Validate(); err != nil {
			return fmt.Errorf("cast profile is invalid: %w", err)
		}
	}
	if d.Effect != nil {
		if err := d.Effect.Validate(); err != nil {
			return fmt.Errorf("effect profile is invalid: %w", err)
		}
	}
	if d.Sequence != nil {
		if err := d.Sequence.Validate(); err != nil {
			return fmt.Errorf("sequence profile is invalid: %w", err)
		}
		// THE ONE REPERTOIRE QUESTION ANSWERABLE FROM HERE. A step naming
		// this very definition is the cheapest infinite recursion there is,
		// and it needs no other definition to spot. Every other
		// component check waits for [ResolveSequence].
		for index, step := range d.Sequence.Steps {
			if step.Action == d.Ref {
				return fmt.Errorf("sequence step %d names the sequence itself", index)
			}
		}
	}

	return nil
}

// Clone returns a deep copy whose mutable profile, cost, and declaration data
// do not alias the original definition.
func (d Definition) Clone() Definition {
	clone := d
	clone.Cost = CloneSpendProfile(d.Cost)
	if d.Attack != nil {
		attack := d.Attack.Clone()
		clone.Attack = &attack
	}
	if d.Cast != nil {
		cast := d.Cast.Clone()
		clone.Cast = &cast
	}
	if d.Effect != nil {
		effect := d.Effect.Clone()
		clone.Effect = &effect
	}
	if d.Sequence != nil {
		sequence := d.Sequence.Clone()
		clone.Sequence = &sequence
	}
	return clone
}

// CloneSpendProfile returns a deep copy of a compiled action price.
func CloneSpendProfile(in *combat.SpendProfile) *combat.SpendProfile {
	if in == nil {
		return nil
	}

	return &combat.SpendProfile{
		Slots:    cloneMap(in.Slots),
		Capacity: cloneMap(in.Capacity),
		Grants:   cloneMap(in.Grants),
		// GrantSlots is a map like the rest; a copy that shared it would let
		// one definition's edit change another's price.
		GrantSlots: cloneMap(in.GrantSlots),
		Pools:      cloneMap(in.Pools),
		Requires:   cloneMap(in.Requires),
	}
}

func cloneMap[K comparable](in map[K]int) map[K]int {
	if in == nil {
		return nil
	}
	out := make(map[K]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
