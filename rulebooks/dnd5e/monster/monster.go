// Package monster provides monster/enemy entity types for D&D 5e combat
package monster

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Monster represents a hostile creature for combat encounters.
// This is the runtime representation with event bus wiring.
type Monster struct {
	// Identity
	id   string
	name string
	ref  *core.Ref // Type reference (e.g., refs.Monsters.Skeleton())

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

	// Trait data (unapplied - for serialization before bus is available)
	// This is populated by factory functions and serialized to Data.Conditions.
	// When LoadFromData is called, these get applied via LoadMonsterConditions.
	traitData []json.RawMessage

	// Proficiencies
	proficiencyBonus int            // Base proficiency bonus (CR-based)
	proficiencies    map[string]int // skill -> bonus

	// AI behavior
	targeting TargetingStrategy

	// Event bus wiring
	bus             events.EventBus
	subscriptionIDs []string

	// Dirty tracking for persistence
	dirty bool
}

// Config provides initialization values for creating a monster
type Config struct {
	ID               string
	Name             string
	Ref              *core.Ref // Type reference (e.g., refs.Monsters.Skeleton())
	HP               int
	AC               int
	AbilityScores    shared.AbilityScores
	ProficiencyBonus int // CR-based proficiency bonus (default 2 if not set)
}

// New creates a new monster with the specified configuration
func New(config Config) *Monster {
	profBonus := config.ProficiencyBonus
	if profBonus == 0 {
		profBonus = 2 // Default for low CR monsters
	}
	return &Monster{
		id:               config.ID,
		name:             config.Name,
		ref:              config.Ref,
		hp:               config.HP,
		maxHP:            config.HP,
		ac:               config.AC,
		abilityScores:    config.AbilityScores,
		proficiencyBonus: profBonus,
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

// Ref returns the monster's type reference (e.g., refs.Monsters.Skeleton())
func (m *Monster) Ref() *core.Ref {
	return m.ref
}

// HP returns current hit points
func (m *Monster) HP() int {
	return m.hp
}

// MaxHP returns maximum hit points
func (m *Monster) MaxHP() int {
	return m.maxHP
}

// GetHitPoints returns current HP.
// Implements combat.Combatant interface.
func (m *Monster) GetHitPoints() int {
	return m.hp
}

// GetMaxHitPoints returns maximum HP.
// Implements combat.Combatant interface.
func (m *Monster) GetMaxHitPoints() int {
	return m.maxHP
}

// ApplyDamage reduces the monster's HP by the damage amount(s).
// HP cannot go below 0. Returns the result of the damage application.
//
// This method directly mutates the monster's HP. The caller is responsible
// for persisting the updated monster state.
//
// Implements combat.Combatant interface.
//
//nolint:revive // ctx is unused but kept for interface consistency and future use
func (m *Monster) ApplyDamage(_ context.Context, input *combat.ApplyDamageInput) *combat.ApplyDamageResult {
	if input == nil {
		return &combat.ApplyDamageResult{
			CurrentHP:  m.hp,
			PreviousHP: m.hp,
		}
	}

	previousHP := m.hp
	totalDamage := 0

	// Sum all damage instances
	for _, instance := range input.Instances {
		totalDamage += instance.Amount
	}

	// Apply damage (minimum HP is 0)
	m.hp -= totalDamage
	if m.hp < 0 {
		m.hp = 0
	}

	m.dirty = true // Mark dirty when HP changes

	return &combat.ApplyDamageResult{
		TotalDamage:   totalDamage,
		CurrentHP:     m.hp,
		DroppedToZero: m.hp == 0 && previousHP > 0,
		PreviousHP:    previousHP,
	}
}

// AC returns armor class
func (m *Monster) AC() int {
	return m.ac
}

// IsDirty returns true if the monster has been modified since last save.
// Implements combat.Combatant interface.
func (m *Monster) IsDirty() bool {
	return m.dirty
}

// MarkClean marks the monster as saved (not dirty).
// Implements combat.Combatant interface.
func (m *Monster) MarkClean() {
	m.dirty = false
}

// AbilityScores returns the monster's ability scores (implements Combatant interface)
func (m *Monster) AbilityScores() shared.AbilityScores {
	return m.abilityScores
}

// ProficiencyBonus returns the monster's proficiency bonus (implements Combatant interface)
func (m *Monster) ProficiencyBonus() int {
	return m.proficiencyBonus
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
	m := New(Config{
		ID:   id,
		Name: "Goblin",
		Ref:  refs.Monsters.Goblin(),
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

	// Add default goblin actions (SRD stats)
	m.AddAction(NewScimitarAction(ScimitarConfig{
		AttackBonus: 4,       // +2 DEX + 2 proficiency
		DamageDice:  "1d6+2", // 1d6 + DEX
	}))

	// Set default goblin speed
	m.speed = SpeedData{Walk: 30}

	return m
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

// SetSpeed sets the monster's movement speeds
func (m *Monster) SetSpeed(speed SpeedData) {
	m.speed = speed
}

// Senses returns the monster's sensory capabilities
func (m *Monster) Senses() SensesData {
	return m.senses
}

// GetConditions returns all active conditions
func (m *Monster) GetConditions() []dnd5eEvents.ConditionBehavior {
	return m.conditions
}

// Actions returns all monster actions
func (m *Monster) Actions() []MonsterAction {
	return m.actions
}

// AddAction adds an action to the monster's available actions
func (m *Monster) AddAction(action MonsterAction) {
	m.actions = append(m.actions, action)
}

// AddCondition adds a condition/trait to the monster's active conditions.
// The condition should already be applied to the event bus before adding.
func (m *Monster) AddCondition(condition dnd5eEvents.ConditionBehavior) {
	m.conditions = append(m.conditions, condition)
}

// AddTraitData adds raw trait JSON data to the monster.
// This is used by factory functions to store trait data before a bus is available.
// The traits will be serialized to Data.Conditions and applied when LoadFromData
// is called with LoadMonsterConditions.
func (m *Monster) AddTraitData(data json.RawMessage) {
	m.traitData = append(m.traitData, data)
}

// LoadFromData creates a Monster from persistent data and wires it to the bus.
// This follows the same pattern as character.LoadFromData.
func LoadFromData(ctx context.Context, d *Data, bus events.EventBus) (*Monster, error) {
	if bus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	// Handle proficiency bonus - default to 2 if not set
	profBonus := d.ProficiencyBonus
	if profBonus == 0 {
		profBonus = 2
	}

	// Create the monster with basic data
	m := &Monster{
		id:               d.ID,
		name:             d.Name,
		ref:              d.Ref,
		hp:               d.HitPoints,
		maxHP:            d.MaxHitPoints,
		ac:               d.ArmorClass,
		abilityScores:    d.AbilityScores,
		proficiencyBonus: profBonus,
		speed:            d.Speed,
		senses:           d.Senses,
		targeting:        d.Targeting,
		bus:              bus,
		subscriptionIDs:  make([]string, 0),
		actions:          make([]MonsterAction, 0, len(d.Actions)),
		proficiencies:    make(map[string]int),
	}

	// Actions must be loaded by the caller to avoid import cycles.
	// The monster package cannot import monster/actions because actions imports monster.
	// Use LoadMonsterActions helper to load actions after creating the monster.
	// Example:
	//   monster, err := LoadFromData(ctx, data, bus)
	//   if err := LoadMonsterActions(monster, data.Actions); err != nil {
	//       // handle error
	//   }

	// Load proficiencies
	for _, prof := range d.Proficiencies {
		m.proficiencies[prof.Skill] = prof.Bonus
	}

	// Conditions must be loaded by the caller to avoid import cycles.
	// The monster package cannot import monstertraits because traits need monster types.
	// Use LoadMonsterConditions helper to load conditions after creating the monster.
	// Example:
	//   monster, err := LoadFromData(ctx, data, bus)
	//   if err := monstertraits.LoadMonsterConditions(ctx, monster, data.Conditions, bus, roller); err != nil {
	//       // handle error
	//   }
	m.conditions = make([]dnd5eEvents.ConditionBehavior, 0, len(d.Conditions))

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
		Movement:  make([]spatial.CubeCoordinate, 0),
	}

	// Move toward closest enemy if not adjacent
	m.moveTowardEnemy(input, result)

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

		// Check if action can be activated (target in range, etc.)
		if err := best.CanActivate(ctx, m, actionInput); err != nil {
			// Action can't be used right now - try to find another or end turn
			break
		}

		// Execute the action
		err := best.Activate(ctx, m, actionInput)

		// Record the result (only if we actually attempted it)
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

// selectTarget picks an appropriate target based on action type and targeting strategy
func (m *Monster) selectTarget(action MonsterAction, perception *PerceptionData) core.Entity {
	switch action.ActionType() {
	case TypeMeleeAttack:
		return m.selectTargetByStrategy(perception.Enemies)
	case TypeRangedAttack:
		// For ranged attacks, prefer non-adjacent enemies (to avoid disadvantage)
		nonAdjacent := make([]PerceivedEntity, 0)
		for _, e := range perception.Enemies {
			if !e.Adjacent {
				nonAdjacent = append(nonAdjacent, e)
			}
		}
		// If we have non-adjacent enemies, pick from those
		if len(nonAdjacent) > 0 {
			return m.selectTargetByStrategy(nonAdjacent)
		}
		// Otherwise fall back to all enemies (including adjacent)
		return m.selectTargetByStrategy(perception.Enemies)
	case TypeHeal:
		// Self
		return m
	}
	return nil
}

// selectTargetByStrategy applies the monster's targeting strategy to select from available enemies
func (m *Monster) selectTargetByStrategy(enemies []PerceivedEntity) core.Entity {
	if len(enemies) == 0 {
		return nil
	}

	switch m.targeting {
	case TargetLowestHP:
		// Find enemy with lowest HP
		lowestIdx := 0
		lowestHP := enemies[0].HP
		for i, e := range enemies {
			if e.HP < lowestHP {
				lowestHP = e.HP
				lowestIdx = i
			}
		}
		return enemies[lowestIdx].Entity

	case TargetLowestAC:
		// Find enemy with lowest AC
		lowestIdx := 0
		lowestAC := enemies[0].AC
		for i, e := range enemies {
			if e.AC < lowestAC {
				lowestAC = e.AC
				lowestIdx = i
			}
		}
		return enemies[lowestIdx].Entity

	case TargetClosest:
		fallthrough
	default:
		// Default behavior: pick closest (first in list, as Enemies is sorted by distance)
		return enemies[0].Entity
	}
}

// moveTowardEnemy moves the monster toward the closest enemy if not already adjacent.
// Uses A* pathfinding to navigate around obstacles.
// Updates perception data to reflect new position after movement.
func (m *Monster) moveTowardEnemy(input *TurnInput, result *TurnResult) {
	if input.Perception == nil || len(input.Perception.Enemies) == 0 {
		return
	}

	closest := input.Perception.ClosestEnemy()
	if closest == nil || closest.Adjacent {
		// Already adjacent or no enemy - no movement needed
		return
	}

	// Calculate how far we can move (use input speed, fall back to monster's speed)
	// input.Speed is already in hexes, but m.speed.Walk is in feet (5 feet per hex)
	speed := input.Speed
	if speed == 0 {
		speed = m.speed.Walk / 5 // Convert feet to hexes
	}
	if speed == 0 {
		return // Can't move
	}

	// Build blocked hex map for pathfinding
	blocked := make(map[spatial.CubeCoordinate]bool)
	for _, hex := range input.Perception.BlockedHexes {
		blocked[hex] = true
	}

	// Find path using A*
	pathFinder := NewSimplePathFinder()
	path := pathFinder.FindPath(input.Perception.MyPosition, closest.Position, blocked)

	if len(path) == 0 {
		return // No valid path - stay put
	}

	// Calculate how many hexes to move (stop 1 hex short to stay adjacent)
	hexesToMove := len(path) - 1 // Stop adjacent to target
	if hexesToMove <= 0 {
		return // Already close enough
	}
	if hexesToMove > speed {
		hexesToMove = speed
	}

	// Build movement path (include start position, then each hex moved to)
	current := input.Perception.MyPosition
	movementPath := []spatial.CubeCoordinate{current}
	for i := 0; i < hexesToMove; i++ {
		current = path[i]
		movementPath = append(movementPath, current)
	}

	// Record full path (every hex crossed)
	result.Movement = movementPath

	// Update perception with new position
	input.Perception.MyPosition = current

	// Recalculate distances and adjacency for enemies
	for i := range input.Perception.Enemies {
		enemy := &input.Perception.Enemies[i]
		enemy.Distance = current.Distance(enemy.Position)
		enemy.Adjacent = enemy.Distance == 1
	}

	// Recalculate distances and adjacency for allies
	for i := range input.Perception.Allies {
		ally := &input.Perception.Allies[i]
		ally.Distance = current.Distance(ally.Position)
		ally.Adjacent = ally.Distance == 1
	}
}

// ToData converts the monster to its persistent data form
func (m *Monster) ToData() *Data {
	data := &Data{
		ID:               m.id,
		Name:             m.name,
		Ref:              m.ref,
		HitPoints:        m.hp,
		MaxHitPoints:     m.maxHP,
		ArmorClass:       m.ac,
		AbilityScores:    m.abilityScores,
		ProficiencyBonus: m.proficiencyBonus,
		Speed:            m.speed,
		Senses:           m.senses,
		Targeting:        m.targeting,
		Actions:          make([]ActionData, 0, len(m.actions)),
		Proficiencies:    make([]ProficiencyData, 0, len(m.proficiencies)),
	}

	// Convert actions
	for _, action := range m.actions {
		data.Actions = append(data.Actions, action.ToData())
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

	// Convert conditions to persisted JSON
	// Include both applied conditions and unapplied trait data
	totalConditions := len(m.conditions) + len(m.traitData)
	data.Conditions = make([]json.RawMessage, 0, totalConditions)

	// First, add applied conditions
	for _, condition := range m.conditions {
		condJSON, err := condition.ToJSON()
		if err != nil {
			// Skip conditions that can't be serialized
			continue
		}
		data.Conditions = append(data.Conditions, condJSON)
	}

	// Then, add unapplied trait data (from factory functions)
	data.Conditions = append(data.Conditions, m.traitData...)

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
