package damage

import (
	"regexp"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
)

var pureDiceNotation = regexp.MustCompile(`^[1-9][0-9]*d[1-9][0-9]*(\+[1-9][0-9]*d[1-9][0-9]*)*$`)

// Damage describes one independently rolled damage pool.
type Damage struct {
	Dice       string     `json:"dice"`
	Type       Type       `json:"type"`
	FlatBonus  int        `json:"flat_bonus,omitempty"`
	Properties []Property `json:"properties,omitempty"`
	Save       *SaveSpec  `json:"save,omitempty"`
}

// DamageSpec describes all of the damage pools produced by an attack or effect.
type DamageSpec struct {
	Pools []Damage `json:"pools"`
}

// Property modifies how a damage pool is resolved.
type Property string

const (
	// PropertyCritEligible permits a pool's dice to be doubled on a critical hit.
	PropertyCritEligible Property = "crit_eligible"
)

// SaveSpec describes the saving throw associated with a damage pool.
type SaveSpec struct {
	Ability abilities.Ability `json:"ability"`
	DC      int               `json:"dc"`
	Effect  SaveEffect        `json:"effect"`
}

// SaveEffect describes the outcome when a saving throw succeeds.
type SaveEffect string

const (
	// SaveEffectHalf deals half damage on a successful saving throw.
	SaveEffectHalf SaveEffect = "half"
)

// Validate verifies the damage pools and their static metadata without resolving them.
func (s *DamageSpec) Validate() error {
	if s == nil || len(s.Pools) == 0 {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "damage pools are required")
	}

	for _, pool := range s.Pools {
		if _, err := dice.ParseNotation(pool.Dice); err != nil {
			return rpgerr.Wrap(err, "invalid damage dice")
		}
		if !pureDiceNotation.MatchString(pool.Dice) {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "damage dice cannot include a static modifier; use flat_bonus")
		}
		if pool.Type == None {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "damage type cannot be none")
		}
		if _, ok := All[string(pool.Type)]; !ok {
			return rpgerr.New(rpgerr.CodeInvalidArgument, "unknown damage type")
		}
		for _, property := range pool.Properties {
			if property != PropertyCritEligible {
				return rpgerr.New(rpgerr.CodeInvalidArgument, "unknown damage property")
			}
		}
		if pool.Save != nil {
			if err := pool.Save.Validate(); err != nil {
				return err
			}
		}
	}

	return nil
}

// Validate verifies save metadata without resolving the save.
func (s *SaveSpec) Validate() error {
	if s == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "save spec is required")
	}
	if _, ok := abilities.All[string(s.Ability)]; !ok {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "unknown save ability")
	}
	if s.DC <= 0 {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "save DC must be positive")
	}
	if s.Effect != SaveEffectHalf {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "unknown save effect")
	}
	return nil
}
