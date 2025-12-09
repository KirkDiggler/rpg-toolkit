// Package monster provides monster/enemy entity types for D&D 5e combat
package monster

import (
	"context"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// Monster represents a hostile creature for combat encounters.
// This is the runtime representation with event bus wiring.
type Monster struct {
	// Identity
	id          string
	name        string
	monsterType string

	// Stats
	hp            int
	maxHP         int
	ac            int
	abilityScores shared.AbilityScores
	speed         SpeedData
	senses        SensesData

	// Actions (typed, ready to use)
	actions []MonsterAction

	// Conditions (wired to bus)
	conditions []dnd5eEvents.ConditionBehavior

	// Proficiencies
	proficiencies map[string]int // skill -> bonus

	// Event bus wiring
	bus             events.EventBus
	subscriptionIDs []string
}

// Config provides initialization values for creating a monster
type Config struct {
	ID            string
	Name          string
	HP            int
	AC            int
	AbilityScores shared.AbilityScores
}

// New creates a new monster with the specified configuration
func New(config Config) *Monster {
	return &Monster{
		id:            config.ID,
		name:          config.Name,
		hp:            config.HP,
		maxHP:         config.HP,
		ac:            config.AC,
		abilityScores: config.AbilityScores,
	}
}

// GetID implements core.Entity
func (m *Monster) GetID() string {
	return m.id
}

// GetType implements core.Entity
func (m *Monster) GetType() core.EntityType {
	return dnd5e.EntityTypeMonster
}

// Name returns the monster's name
func (m *Monster) Name() string {
	return m.name
}

// HP returns current hit points
func (m *Monster) HP() int {
	return m.hp
}

// MaxHP returns maximum hit points
func (m *Monster) MaxHP() int {
	return m.maxHP
}

// AC returns armor class
func (m *Monster) AC() int {
	return m.ac
}

// AbilityScores returns the monster's ability scores
func (m *Monster) AbilityScores() shared.AbilityScores {
	return m.abilityScores
}

// TakeDamage reduces HP (returns actual damage taken)
func (m *Monster) TakeDamage(amount int) int {
	if amount < 0 {
		amount = 0
	}
	previousHP := m.hp
	m.hp -= amount
	if m.hp < 0 {
		m.hp = 0
	}
	return previousHP - m.hp
}

// IsAlive returns true if HP > 0
func (m *Monster) IsAlive() bool {
	return m.hp > 0
}

// NewGoblin creates a standard goblin (CR 1/4, D&D 5e SRD stats)
func NewGoblin(id string) *Monster {
	return New(Config{
		ID:   id,
		Name: "Goblin",
		HP:   7,  // 2d6 average
		AC:   15, // Leather armor + DEX
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,  // -1
			abilities.DEX: 14, // +2
			abilities.CON: 10, // +0
			abilities.INT: 10, // +0
			abilities.WIS: 8,  // -1
			abilities.CHA: 8,  // -1
		},
	})
}

// HPPercent returns current HP as a percentage of max HP
func (m *Monster) HPPercent() int {
	if m.maxHP == 0 {
		return 0
	}
	return (m.hp * 100) / m.maxHP
}

// Speed returns the monster's movement speeds
func (m *Monster) Speed() SpeedData {
	return m.speed
}

// Senses returns the monster's sensory capabilities
func (m *Monster) Senses() SensesData {
	return m.senses
}

// GetConditions returns all active conditions
func (m *Monster) GetConditions() []dnd5eEvents.ConditionBehavior {
	return m.conditions
}

// GetActions returns all monster actions
func (m *Monster) GetActions() []MonsterAction {
	return m.actions
}

// AddAction adds an action to the monster's available actions
func (m *Monster) AddAction(action MonsterAction) {
	m.actions = append(m.actions, action)
}

// LoadFromData creates a Monster from persistent data and wires it to the bus.
// This follows the same pattern as character.LoadFromData.
func LoadFromData(ctx context.Context, d *Data, bus events.EventBus) (*Monster, error) {
	if bus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	// Create the monster with basic data
	m := &Monster{
		id:              d.ID,
		name:            d.Name,
		monsterType:     d.MonsterType,
		hp:              d.HitPoints,
		maxHP:           d.MaxHitPoints,
		ac:              d.ArmorClass,
		abilityScores:   d.AbilityScores,
		speed:           d.Speed,
		senses:          d.Senses,
		bus:             bus,
		subscriptionIDs: make([]string, 0),
		actions:         make([]MonsterAction, 0, len(d.Actions)),
		proficiencies:   make(map[string]int),
	}

	// Load actions (convert from data to runtime)
	for _, actionData := range d.Actions {
		action, err := LoadAction(actionData)
		if err != nil {
			// Skip invalid actions but continue loading
			continue
		}
		m.actions = append(m.actions, action)
	}

	// Load proficiencies
	for _, prof := range d.Proficiencies {
		m.proficiencies[prof.Skill] = prof.Bonus
	}

	// Load and apply conditions
	m.conditions = make([]dnd5eEvents.ConditionBehavior, 0, len(d.Conditions))
	// NOTE: Condition loading would go here if we had conditions to load
	// For now, monsters start with no conditions

	// Subscribe to events
	if err := m.subscribeToEvents(ctx); err != nil {
		return nil, rpgerr.Wrapf(err, "failed to subscribe to events")
	}

	return m, nil
}

// subscribeToEvents subscribes the monster to gameplay events
func (m *Monster) subscribeToEvents(ctx context.Context) error {
	if m.bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "monster has no event bus")
	}

	// Subscribe to damage received
	damageTopic := dnd5eEvents.DamageReceivedTopic.On(m.bus)
	subID, err := damageTopic.Subscribe(ctx, m.onDamageReceived)
	if err != nil {
		return err
	}
	m.subscriptionIDs = append(m.subscriptionIDs, subID)

	// Subscribe to healing received
	healingTopic := dnd5eEvents.HealingReceivedTopic.On(m.bus)
	subID, err = healingTopic.Subscribe(ctx, m.onHealingReceived)
	if err != nil {
		return err
	}
	m.subscriptionIDs = append(m.subscriptionIDs, subID)

	return nil
}

// onDamageReceived handles damage events
func (m *Monster) onDamageReceived(_ context.Context, event dnd5eEvents.DamageReceivedEvent) error {
	if event.TargetID != m.id {
		return nil
	}
	m.TakeDamage(event.Amount)
	return nil
}

// onHealingReceived handles healing events
func (m *Monster) onHealingReceived(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
	if event.TargetID != m.id {
		return nil
	}
	m.hp += event.Amount
	if m.hp > m.maxHP {
		m.hp = m.maxHP
	}
	return nil
}

// TakeTurn executes the monster's turn using utility-based action selection.
// It loops through available actions, picks the best one based on scoring,
// and executes until action economy is exhausted.
func (m *Monster) TakeTurn(ctx context.Context, input *TurnInput) (*TurnResult, error) {
	result := &TurnResult{
		MonsterID: m.id,
		Actions:   make([]ExecutedAction, 0),
		Movement:  make([]Position, 0),
	}

	// Keep selecting actions until resources exhausted
	for m.hasResources(input.ActionEconomy) {
		// Score all valid actions and pick the best
		best := m.selectBestAction(input.ActionEconomy, input.Perception)
		if best == nil {
			break // No valid actions available
		}

		// Select target for this action
		target := m.selectTarget(best, input.Perception)

		// Skip if action requires target but none available
		actionType := best.ActionType()
		requiresTarget := actionType != TypeHeal && actionType != TypeMovement && actionType != TypeStealth
		if target == nil && requiresTarget {
			break // No valid target available
		}

		// Build action input
		actionInput := MonsterActionInput{
			Bus:           input.Bus,
			Perception:    input.Perception,
			Conditions:    m.conditions,
			ActionEconomy: input.ActionEconomy,
			Target:        target,
			Roller:        input.Roller,
		}

		// Execute the action
		err := best.Activate(ctx, m, actionInput)

		// Record the result
		targetID := ""
		if target != nil {
			targetID = target.GetID()
		}
		result.Actions = append(result.Actions, ExecutedAction{
			ActionID:   best.GetID(),
			ActionType: best.ActionType(),
			TargetID:   targetID,
			Success:    err == nil,
		})

		// Consume action economy based on cost
		switch best.Cost() {
		case CostAction:
			_ = input.ActionEconomy.UseAction()
		case CostBonusAction:
			_ = input.ActionEconomy.UseBonusAction()
		case CostReaction:
			_ = input.ActionEconomy.UseReaction()
		}
	}

	return result, nil
}

// hasResources returns true if the monster has any action economy resources left
func (m *Monster) hasResources(economy *combat.ActionEconomy) bool {
	return economy.CanUseAction() || economy.CanUseBonusAction()
}

// selectBestAction scores all actions and returns the one with highest score
func (m *Monster) selectBestAction(economy *combat.ActionEconomy, perception *PerceptionData) MonsterAction {
	var best MonsterAction
	bestScore := -1000

	for _, action := range m.actions {
		// Can we afford this action?
		if !m.canAfford(economy, action.Cost()) {
			continue
		}

		// Score it
		score := action.Score(m, perception)
		if score > bestScore {
			bestScore = score
			best = action
		}
	}

	return best
}

// canAfford checks if the monster can afford an action's cost
func (m *Monster) canAfford(economy *combat.ActionEconomy, cost ActionCost) bool {
	switch cost {
	case CostAction:
		return economy.CanUseAction()
	case CostBonusAction:
		return economy.CanUseBonusAction()
	case CostReaction:
		return economy.CanUseReaction()
	case CostNone:
		return true
	default:
		return false
	}
}

// selectTarget picks an appropriate target based on action type
func (m *Monster) selectTarget(action MonsterAction, perception *PerceptionData) core.Entity {
	switch action.ActionType() {
	case TypeMeleeAttack:
		// Closest enemy for melee
		if len(perception.Enemies) > 0 {
			return perception.Enemies[0].Entity
		}
	case TypeRangedAttack:
		// Closest enemy not adjacent (to avoid disadvantage)
		for _, e := range perception.Enemies {
			if !e.Adjacent {
				return e.Entity
			}
		}
		// Fall back to closest
		if len(perception.Enemies) > 0 {
			return perception.Enemies[0].Entity
		}
	case TypeHeal:
		// Self
		return m
	}
	return nil
}

// ToData converts the monster to its persistent data form
func (m *Monster) ToData() *Data {
	data := &Data{
		ID:            m.id,
		Name:          m.name,
		MonsterType:   m.monsterType,
		HitPoints:     m.hp,
		MaxHitPoints:  m.maxHP,
		ArmorClass:    m.ac,
		AbilityScores: m.abilityScores,
		Speed:         m.speed,
		Senses:        m.senses,
		Actions:       make([]ActionData, 0, len(m.actions)),
		Proficiencies: make([]ProficiencyData, 0, len(m.proficiencies)),
	}

	// Convert proficiencies
	for skill, bonus := range m.proficiencies {
		data.Proficiencies = append(data.Proficiencies, ProficiencyData{
			Skill: skill,
			Bonus: bonus,
		})
	}

	// Sort proficiencies for deterministic output
	sort.Slice(data.Proficiencies, func(i, j int) bool {
		return data.Proficiencies[i].Skill < data.Proficiencies[j].Skill
	})

	return data
}

// Cleanup unsubscribes from all events
func (m *Monster) Cleanup(ctx context.Context) error {
	if m.bus == nil {
		return nil
	}

	for _, subID := range m.subscriptionIDs {
		if err := m.bus.Unsubscribe(ctx, subID); err != nil {
			return rpgerr.Wrapf(err, "failed to unsubscribe")
		}
	}
	m.subscriptionIDs = nil
	return nil
}
