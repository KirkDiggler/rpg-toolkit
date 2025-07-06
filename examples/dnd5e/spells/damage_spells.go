package spells

import (
	"context"
	"fmt"
	"time"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/spells"
)

// Fireball creates a fireball spell.
func Fireball() *spells.SimpleSpell {
	return spells.NewSimpleSpell(spells.SimpleSpellConfig{
		ID:          "fireball",
		Name:        "Fireball",
		Level:       3,
		School:      "evocation",
		CastingTime: 6 * time.Second, // 1 action
		Range:       150,
		Duration:    events.Instantaneous,
		Description: "A bright streak flashes from your pointing finger to a point you choose within range and then blossoms with a low roar into an explosion of flame.",
		Components: spells.CastingComponents{
			Verbal:    true,
			Somatic:   true,
			Material:  true,
			Materials: "a tiny ball of bat guano and sulfur",
		},
		TargetType: spells.TargetPoint,
		AreaOfEffect: &spells.AreaOfEffect{
			Shape: spells.AreaSphere,
			Size:  20,
		},
		Upcastable: true,
		CastFunc: func(ctx spells.CastContext) error {
			// Calculate damage dice
			baseDice := 8
			extraDice := ctx.SlotLevel - 3 // Extra d6 per level above 3rd
			totalDice := baseDice + extraDice

			// Roll damage
			damage := dice.D(6, totalDice).Roll()

			// Get spell from context - the SimpleSpell.Cast method doesn't set this
			// In a real implementation, you'd pass the spell reference another way
			var spell spells.Spell
			if s, ok := ctx.Metadata["spell"].(spells.Spell); ok {
				spell = s
			}

			// Get save DC from metadata
			saveDC := 15 // Default DC
			if dc, ok := ctx.Metadata["save_dc"].(int); ok {
				saveDC = dc
			}

			// Apply to all targets
			for _, target := range ctx.Targets {
				// Publish save event
				saveEvent := &spells.SpellSaveEvent{
					GameEvent: events.GameEvent{
						EventType: spells.EventSpellSave,
					},
					Target:   target,
					Spell:    spell,
					SaveType: "dexterity",
					DC:       saveDC,
				}

				if err := ctx.Bus.Publish(context.TODO(), saveEvent); err != nil {
					return err
				}

				// Check if save succeeded (would be set by save handler)
				finalDamage := damage
				if saveEvent.Success {
					finalDamage = damage / 2
				}

				// Publish damage event
				dmgEvent := &spells.SpellDamageEvent{
					GameEvent: events.GameEvent{
						EventType: spells.EventSpellDamage,
					},
					Source:     ctx.Caster,
					Target:     target,
					Spell:      spell,
					Damage:     finalDamage,
					DamageType: "fire",
				}

				if err := ctx.Bus.Publish(context.TODO(), dmgEvent); err != nil {
					return err
				}
			}

			return nil
		},
	})
}

// MagicMissile creates a magic missile spell.
func MagicMissile() *spells.SimpleSpell {
	return spells.NewSimpleSpell(spells.SimpleSpellConfig{
		ID:          "magic_missile",
		Name:        "Magic Missile",
		Level:       1,
		School:      "evocation",
		CastingTime: 6 * time.Second,
		Range:       120,
		Duration:    events.Instantaneous,
		Description: "You create three glowing darts of magical force.",
		Components: spells.CastingComponents{
			Verbal:  true,
			Somatic: true,
		},
		TargetType: spells.TargetCreature,
		MaxTargets: 3,
		Upcastable: true,
		CastFunc: func(ctx spells.CastContext) error {
			// Calculate number of missiles
			missiles := 3 + (ctx.SlotLevel - 1)

			// Distribute missiles among targets
			for i := 0; i < missiles; i++ {
				target := ctx.Targets[i%len(ctx.Targets)]

				// Each missile does 1d4+1 force damage
				damage := dice.D(4, 1).Roll() + 1

				// Magic missile always hits
				// Get spell from context if available
				var spell spells.Spell
				if s, ok := ctx.Metadata["spell"].(spells.Spell); ok {
					spell = s
				}

				dmgEvent := &spells.SpellDamageEvent{
					GameEvent: events.GameEvent{
						EventType: spells.EventSpellDamage,
					},
					Source:     ctx.Caster,
					Target:     target,
					Spell:      spell,
					Damage:     damage,
					DamageType: "force",
				}

				if err := ctx.Bus.Publish(context.TODO(), dmgEvent); err != nil {
					return err
				}
			}

			return nil
		},
	})
}

// FireBolt creates a fire bolt cantrip.
func FireBolt() *spells.SimpleSpell {
	return spells.NewSimpleSpell(spells.SimpleSpellConfig{
		ID:          "fire_bolt",
		Name:        "Fire Bolt",
		Level:       0, // Cantrip
		School:      "evocation",
		CastingTime: 6 * time.Second,
		Range:       120,
		Duration:    events.Instantaneous,
		Description: "You hurl a mote of fire at a creature or object within range.",
		Components: spells.CastingComponents{
			Verbal:  true,
			Somatic: true,
		},
		TargetType: spells.TargetCreature,
		MaxTargets: 1,
		CastFunc: func(ctx spells.CastContext) error {
			if len(ctx.Targets) == 0 {
				return fmt.Errorf("no target specified")
			}

			// Make spell attack
			// Get spell from context if available
			var spell spells.Spell
			if s, ok := ctx.Metadata["spell"].(spells.Spell); ok {
				spell = s
			}

			attackEvent := &spells.SpellAttackEvent{
				GameEvent: events.GameEvent{
					EventType: spells.EventSpellAttack,
				},
				Attacker: ctx.Caster,
				Target:   ctx.Targets[0],
				Spell:    spell,
			}

			if err := ctx.Bus.Publish(context.TODO(), attackEvent); err != nil {
				return err
			}

			// If hit, deal damage
			if attackEvent.Hit {
				// Cantrip damage scales with level
				casterLevel := 1 // Default level
				if lvl, ok := ctx.Metadata["caster_level"].(int); ok {
					casterLevel = lvl
				}

				damageDice := 1
				if casterLevel >= 5 {
					damageDice = 2
				}
				if casterLevel >= 11 {
					damageDice = 3
				}
				if casterLevel >= 17 {
					damageDice = 4
				}

				damage := dice.D(10, damageDice).Roll()

				dmgEvent := &spells.SpellDamageEvent{
					GameEvent: events.GameEvent{
						EventType: spells.EventSpellDamage,
					},
					Source:     ctx.Caster,
					Target:     ctx.Targets[0],
					Spell:      spell,
					Damage:     damage,
					DamageType: "fire",
				}

				if err := ctx.Bus.Publish(context.TODO(), dmgEvent); err != nil {
					return err
				}
			}

			return nil
		},
	})
}
