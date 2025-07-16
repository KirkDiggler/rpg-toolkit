package proficiency

import (
	"context"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

// SkillProficiency implements skill proficiency for D&D 5e
type SkillProficiency struct {
	*proficiency.SimpleProficiency
	skill     Skill
	level     int
	expertise bool // Expertise doubles proficiency bonus
}

// NewSkillProficiency creates a skill proficiency that adds bonuses to ability checks
func NewSkillProficiency(owner core.Entity, skill Skill, source string, level int) *SkillProficiency {
	sp := &SkillProficiency{
		skill: skill,
		level: level,
	}

	// Create the underlying simple proficiency with custom handlers
	sp.SimpleProficiency = proficiency.NewSimpleProficiency(proficiency.SimpleProficiencyConfig{
		ID:      fmt.Sprintf("%s-skill-prof-%s", owner.GetID(), skill),
		Type:    "proficiency.skill",
		Owner:   owner,
		Subject: string(skill),
		Source:  source,
		ApplyFunc: func(p *proficiency.SimpleProficiency, bus events.EventBus) error {
			// Subscribe to ability check events
			p.Subscribe(bus, events.EventOnAbilityCheck, 100, sp.handleAbilityCheck)
			return nil
		},
	})

	return sp
}

// SetExpertise marks this skill as having expertise (double proficiency)
func (sp *SkillProficiency) SetExpertise(expertise bool) {
	sp.expertise = expertise
}

// handleAbilityCheck adds proficiency bonus to relevant skill checks
func (sp *SkillProficiency) handleAbilityCheck(ctx context.Context, e events.Event) error {
	// Only apply to our owner
	if e.Source() == nil || e.Source().GetID() != sp.Owner().GetID() {
		return nil
	}

	// Get the skill being checked
	skill, ok := e.Context().GetString("skill")
	if !ok {
		return nil
	}

	// Check if this is our skill
	if !strings.EqualFold(skill, string(sp.skill)) {
		return nil
	}

	// Calculate proficiency bonus
	profBonus := GetProficiencyBonus(sp.level)
	if sp.expertise {
		profBonus *= 2 // Expertise doubles the bonus
	}

	// Add proficiency bonus modifier
	modifierName := "skill-proficiency"
	if sp.expertise {
		modifierName = "skill-expertise"
	}

	e.Context().AddModifier(events.NewModifier(
		modifierName,
		events.ModifierSaveBonus, // Skills use same modifier type as saves
		events.NewRawValue(profBonus, fmt.Sprintf("%s proficiency", sp.skill)),
		50, // Apply after ability modifier
	))

	return nil
}
