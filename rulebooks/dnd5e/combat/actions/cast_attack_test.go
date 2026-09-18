package actions

import (
	"encoding/json"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCastAttackContract(t *testing.T) {
	p := CastProfile{RangeFeet: 120, Target: CastTargetOneCreature, MinTargets: 1, MaxTargets: 1,
		Attack: &AttackProfile{Category: AttackCategorySpell, Delivery: AttackDelivery{Ranged: &RangedDelivery{NormalFeet: 120}}, OnHit: []ConditionApplication{{Ref: *refs.Conditions.GuidingBolt(), Parameters: json.RawMessage(`{"source_id":"caster"}`)}}}}
	require.NoError(t, p.Validate())
	copy := p.Clone()
	copy.Attack.Delivery.Ranged.NormalFeet = 30
	copy.Attack.OnHit[0].Parameters[0] = ' '
	require.Equal(t, 120, p.Attack.Delivery.Ranged.NormalFeet)
	require.Equal(t, byte('{'), p.Attack.OnHit[0].Parameters[0])
	p.Stabilize = true
	require.Error(t, p.Validate())
	p.Stabilize = false
	p.Attack.Category = AttackCategoryWeapon
	require.Error(t, p.Validate())
}
