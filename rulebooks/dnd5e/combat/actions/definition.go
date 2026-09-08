package actions

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// Definition identifies an action, its optional price, and the profile a
// resolution machine interprets. Exactly one profile must be populated.
//
// The profile is how content chooses a sequence without naming one. A second
// arm is not a second kind of definition: it is a second thing content can
// declare, and the machine that reads it is chosen by which arm is populated
// rather than by what the definition is called.
type Definition struct {
	Ref    core.Ref             `json:"ref"`
	Name   string               `json:"name"`
	Cost   *combat.SpendProfile `json:"cost,omitempty"`
	Attack *AttackProfile       `json:"attack,omitempty"`
	Cast   *CastProfile         `json:"cast,omitempty"`
}

// Validate reports whether the definition has complete identity, a valid
// optional price, and exactly one valid profile.
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
	if populated != 1 {
		return fmt.Errorf("action must populate exactly one profile")
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
		Pools:    cloneMap(in.Pools),
		Requires: cloneMap(in.Requires),
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
