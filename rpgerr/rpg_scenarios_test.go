package rpgerr_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

type RPGScenariosTestSuite struct {
	suite.Suite
}

func TestRPGScenariosSuite(t *testing.T) {
	suite.Run(t, new(RPGScenariosTestSuite))
}

// TestMeleeAttackOutOfRange shows how context accumulates through an attack attempt
func (s *RPGScenariosTestSuite) TestMeleeAttackOutOfRange() {
	// Combat system level
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("encounter_id", "enc-001"),
		rpgerr.Meta("round", 3),
		rpgerr.Meta("turn", "fighter"),
	)

	// Attack action level
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("action_type", "attack"),
		rpgerr.Meta("attacker_id", "fighter-001"),
		rpgerr.Meta("target_id", "goblin-002"),
	)

	// Range validation level
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("attacker_position", "5,5"),
		rpgerr.Meta("target_position", "15,15"),
		rpgerr.Meta("weapon", "shortsword"),
		rpgerr.Meta("weapon_reach", 5),
		rpgerr.Meta("calculated_distance", 14.14),
	)

	// Create the error with full context
	err := rpgerr.OutOfRangeCtx(ctx, "melee attack")

	// Verify the error tells the complete story
	meta := rpgerr.GetMeta(err)
	s.Equal("enc-001", meta["encounter_id"])
	s.Equal(3, meta["round"])
	s.Equal("fighter", meta["turn"])
	s.Equal("shortsword", meta["weapon"])
	s.Equal(14.14, meta["calculated_distance"])
	s.Equal(5, meta["weapon_reach"])

	// The error message plus metadata tells us exactly why the attack failed
	s.Contains(err.Error(), "melee attack out of range")
}

// TestSpellcastingWithoutSlots shows resource exhaustion with full context
func (s *RPGScenariosTestSuite) TestSpellcastingWithoutSlots() {
	// Game session level
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("session_id", "session-456"),
		rpgerr.Meta("campaign", "lost_mines"),
	)

	// Character state level
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("character_id", "wizard-001"),
		rpgerr.Meta("character_level", 5),
		rpgerr.Meta("character_class", "wizard"),
	)

	// Spellcasting attempt level
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("spell", "fireball"),
		rpgerr.Meta("spell_level", 3),
		rpgerr.Meta("attempted_slot_level", 3),
		rpgerr.Meta("slots_remaining", map[string]int{
			"1st": 4,
			"2nd": 3,
			"3rd": 0, // No 3rd level slots
			"4th": 0,
		}),
	)

	err := rpgerr.ResourceExhaustedCtx(ctx, "spell slots")

	meta := rpgerr.GetMeta(err)
	slots := meta["slots_remaining"].(map[string]int)
	s.Equal(0, slots["3rd"])
	s.Equal("fireball", meta["spell"])
	s.Equal(3, meta["spell_level"])
}

// TestConcentrationConflict shows conflicting game states
func (s *RPGScenariosTestSuite) TestConcentrationConflict() {
	ctx := context.Background()

	// Current state
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("character_id", "cleric-001"),
		rpgerr.Meta("current_concentration", "bless"),
		rpgerr.Meta("concentration_duration", "3 rounds"),
		rpgerr.Meta("concentration_targets", []string{"fighter-001", "rogue-001"}),
	)

	// Attempted action
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("attempted_spell", "hold_person"),
		rpgerr.Meta("requires_concentration", true),
		rpgerr.Meta("target", "orc-001"),
	)

	err := rpgerr.ConflictingStateCtx(ctx, "already concentrating on bless")

	meta := rpgerr.GetMeta(err)
	s.Equal("bless", meta["current_concentration"])
	s.Equal("hold_person", meta["attempted_spell"])
	s.True(meta["requires_concentration"].(bool))
}

// TestNestedPipelineAttackFlow shows deep nesting with context accumulation
func (s *RPGScenariosTestSuite) TestNestedPipelineAttackFlow() {
	// Level 1: Attack Pipeline
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("pipeline", "AttackPipeline"),
		rpgerr.Meta("attacker", "barbarian-001"),
		rpgerr.Meta("target", "dragon-001"),
		rpgerr.Meta("weapon", "greataxe"),
	)

	// Level 2: Hit Calculation
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("pipeline", "HitCalculation"),
		rpgerr.Meta("attack_roll", 18),
		rpgerr.Meta("attack_bonus", 7),
		rpgerr.Meta("total_attack", 25),
		rpgerr.Meta("target_ac", 19),
		rpgerr.Meta("hit", true),
	)

	// Level 3: Damage Pipeline
	damageCtx := rpgerr.WithMetadata(ctx,
		rpgerr.Meta("pipeline", "DamagePipeline"),
		rpgerr.Meta("base_damage", "1d12"),
		rpgerr.Meta("damage_roll", 8),
		rpgerr.Meta("strength_bonus", 4),
		rpgerr.Meta("rage_bonus", 2),
	)

	// Level 4: Damage Reduction
	reductionCtx := rpgerr.WithMetadata(damageCtx,
		rpgerr.Meta("pipeline", "DamageReduction"),
		rpgerr.Meta("damage_type", "slashing"),
		rpgerr.Meta("target_immunities", []string{"poison", "psychic"}),
		rpgerr.Meta("target_resistances", []string{"slashing", "piercing", "bludgeoning"}),
	)

	// Dragon has resistance to non-magical weapons
	err := rpgerr.NewCtx(reductionCtx, rpgerr.CodeBlocked,
		"damage reduced by resistance to non-magical slashing")

	// Add call stack to show the execution path
	err.CallStack = []string{
		"AttackPipeline",
		"HitCalculation",
		"DamagePipeline",
		"DamageReduction",
	}

	meta := rpgerr.GetMeta(err)
	s.Equal("barbarian-001", meta["attacker"])
	s.Equal("dragon-001", meta["target"])
	s.Equal("greataxe", meta["weapon"])
	s.Equal(true, meta["hit"])
	s.Equal("slashing", meta["damage_type"])

	resistances := meta["target_resistances"].([]string)
	s.Contains(resistances, "slashing")

	stack := rpgerr.GetCallStack(err)
	s.Len(stack, 4)
	s.Equal("DamageReduction", stack[3])
}

// TestActionEconomyViolation shows timing restrictions with context
func (s *RPGScenariosTestSuite) TestActionEconomyViolation() {
	ctx := context.Background()

	// Turn tracking
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("round", 2),
		rpgerr.Meta("current_turn", "rogue-001"),
		rpgerr.Meta("phase", "action"),
	)

	// Character's action economy state
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("character_id", "rogue-001"),
		rpgerr.Meta("action_used", true),
		rpgerr.Meta("bonus_action_used", false),
		rpgerr.Meta("movement_used", 15),
		rpgerr.Meta("movement_total", 30),
		rpgerr.Meta("reaction_used", false),
	)

	// Attempted action
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("attempted_action", "attack"),
		rpgerr.Meta("action_type", "action"),
		rpgerr.Meta("previous_action", "dash"),
	)

	err := rpgerr.TimingRestrictionCtx(ctx, "action already used this turn")

	meta := rpgerr.GetMeta(err)
	s.True(meta["action_used"].(bool))
	s.Equal("attack", meta["attempted_action"])
	s.Equal("dash", meta["previous_action"])
}

// TestPrerequisiteChain shows multiple prerequisite failures
func (s *RPGScenariosTestSuite) TestPrerequisiteChain() {
	ctx := context.Background()

	// Character attempting the action
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("character_id", "fighter-001"),
		rpgerr.Meta("character_level", 3),
		rpgerr.Meta("character_class", "fighter"),
		rpgerr.Meta("subclass", "none"), // Haven't chosen archetype yet
	)

	// Ability being attempted
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("ability", "action_surge"),
		rpgerr.Meta("ability_level_required", 2),
		rpgerr.Meta("ability_uses_remaining", 0),
		rpgerr.Meta("ability_recharge", "short_rest"),
		rpgerr.Meta("last_rest", "long_rest_2_encounters_ago"),
	)

	err := rpgerr.ResourceExhaustedCtx(ctx, "action surge uses")

	meta := rpgerr.GetMeta(err)
	s.Equal(0, meta["ability_uses_remaining"])
	s.Equal("short_rest", meta["ability_recharge"])
	s.Equal(3, meta["character_level"]) // Has the level requirement
}

// TestImmunityContext shows immunity with full context
func (s *RPGScenariosTestSuite) TestImmunityContext() {
	ctx := context.Background()

	// Spell being cast
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("spell", "charm_person"),
		rpgerr.Meta("spell_school", "enchantment"),
		rpgerr.Meta("save_dc", 15),
		rpgerr.Meta("caster", "bard-001"),
	)

	// Target information
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("target", "undead-skeleton-001"),
		rpgerr.Meta("target_type", "undead"),
		rpgerr.Meta("target_immunities", []string{
			"poison",
			"exhaustion",
			"charm",
			"frightened",
		}),
	)

	err := rpgerr.ImmuneCtx(ctx, "charm effects (undead immunity)")

	meta := rpgerr.GetMeta(err)
	s.Equal("charm_person", meta["spell"])
	s.Equal("undead", meta["target_type"])

	immunities := meta["target_immunities"].([]string)
	s.Contains(immunities, "charm")
}

// TestInterruptionChain shows how counterspell interrupts a spell
func (s *RPGScenariosTestSuite) TestInterruptionChain() {
	// Original spell cast
	ctx := context.Background()
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("pipeline", "SpellCastPipeline"),
		rpgerr.Meta("caster", "wizard-001"),
		rpgerr.Meta("spell", "disintegrate"),
		rpgerr.Meta("spell_level", 6),
		rpgerr.Meta("target", "fighter-001"),
		rpgerr.Meta("phase", "casting"),
	)

	// Reaction triggered
	ctx = rpgerr.WithMetadata(ctx,
		rpgerr.Meta("interrupt_pipeline", "CounterspellPipeline"),
		rpgerr.Meta("interruptor", "wizard-002"),
		rpgerr.Meta("counterspell_level", 6),
		rpgerr.Meta("automatic_success", true), // Same level = auto success
		rpgerr.Meta("reaction_used", true),
	)

	err := rpgerr.InterruptedCtx(ctx, "counterspell")
	err.CallStack = []string{
		"SpellCastPipeline.Begin",
		"SpellCastPipeline.DeclareTarget",
		"ReactionWindow.Open",
		"CounterspellPipeline.Trigger",
		"CounterspellPipeline.Resolve",
		"SpellCastPipeline.Cancelled",
	}

	meta := rpgerr.GetMeta(err)
	s.Equal("disintegrate", meta["spell"])
	s.Equal("wizard-002", meta["interruptor"])
	s.True(meta["automatic_success"].(bool))

	stack := rpgerr.GetCallStack(err)
	s.Contains(stack, "ReactionWindow.Open")
	s.Contains(stack, "SpellCastPipeline.Cancelled")
}
