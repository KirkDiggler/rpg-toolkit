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
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
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

	// Actions are inert shared definitions interpreted by resolution.
	actions []combatActions.Definition

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
	keeper          *SheetKeeper

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

// HasShieldEquipped answers FALSE, always, and the constant is the rule rather
// than a stub. Implements combat.Combatant interface.
//
// A monster has no equipment slots. Whatever defence a shield gives one is
// already inside the stat block AC returned above — the author wrote a number,
// not a loadout — so there is nothing here to read and nothing further for a
// rule to add. The features that ask (Unarmored Movement's speed bonus,
// Fighting Style (Protection)'s reaction) are character features, so a monster
// answering false is that question correctly answered, not one deferred.
//
// The day monsters carry real equipment this stops being a constant, and the
// question is already in the right place for that to be the only edit.
func (m *Monster) HasShieldEquipped() bool {
	return false
}

// CanReact reports that nothing on this monster refuses a reaction.
//
// TRUE IS THE ANSWER, not a placeholder for one nobody has written. A monster
// carries no action economy in this rulebook, so there is no reaction slot to
// run out and nothing here that could say no — and false would mean exactly
// that, "my economy refuses." What meters a monster's reaction is the reacting
// condition's own once-per-turn flag, which every reactor has; the slot is the
// additional cost only a sheet can be charged. Implements [combat.Member].
func (m *Monster) CanReact() bool {
	return true
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

// MarkDirty records that something this sheet persists has changed.
//
// The counterpart to MarkClean, for the change no other writer here reports.
// Every other one sets m.dirty inline because it is also the one changing the
// field; a condition stores its turn-scoped memory in its OWN fields, which
// are serialized as part of this sheet, so nothing else notices the change and
// the save is dropped unless something says so.
//
// ITS CALLER IS THIS SHEET'S OWN KEEPER now — onConditionStateChanged, one row
// in the subscription table. It was minted ahead of a caller, for a handle a
// loader was going to pass to a condition so the condition could mark the sheet
// itself; that handle is gone, and what arrived instead is a published fact the
// keeper answers. The problem it was minted for is unchanged: without this, a
// wolf that used its reaction is reloaded having used nothing and the
// once-per-turn rule silently does not exist for monsters.
//
// It stays exported. A sheet must be tellable that it changed by something
// that is not the code changing it, which is what
// TestAMonsterCanBeToldItsPersistedStateChanged pins.
func (m *Monster) MarkDirty() {
	m.dirty = true
}

// AbilityScores returns the monster's ability scores (implements Combatant interface)
func (m *Monster) AbilityScores() shared.AbilityScores {
	return m.abilityScores
}

// ProficiencyBonus returns the monster's proficiency bonus (implements Combatant interface)
func (m *Monster) ProficiencyBonus() int {
	return m.proficiencyBonus
}

// GetSavingThrowModifier returns the monster's modifier for a saving throw.
func (m *Monster) GetSavingThrowModifier(ability abilities.Ability) int {
	return m.abilityScores.Modifier(ability)
}

// PassivePerception returns the monster's passive Perception score, from the
// static Senses field seeded at load time. Implements combat.Combatant.
func (m *Monster) PassivePerception() int {
	return m.senses.PassivePerception
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

// Actions returns deep copies of the monster's shared action definitions.
func (m *Monster) Actions() []combatActions.Definition {
	definitions := make([]combatActions.Definition, len(m.actions))
	for index, definition := range m.actions {
		definitions[index] = definition.Clone()
	}
	return definitions
}

// AddAction validates and stores a deep copy of a shared action definition.
func (m *Monster) AddAction(definition combatActions.Definition) error {
	if err := definition.Validate(); err != nil {
		return rpgerr.Wrap(err, "invalid monster action")
	}
	m.actions = append(m.actions, definition.Clone())
	return nil
}

// AddCondition records a live condition application and marks the sheet dirty.
func (m *Monster) AddCondition(condition dnd5eEvents.ConditionBehavior) {
	m.conditions = append(m.conditions, condition)
	m.dirty = true
}

// AddLoadedCondition records a condition reconstructed from persisted data
// without marking the unchanged sheet dirty.
func (m *Monster) AddLoadedCondition(condition dnd5eEvents.ConditionBehavior) {
	m.conditions = append(m.conditions, condition)
}

// AddTraitData adds raw trait JSON data to the monster.
// This is used by factory functions to store trait data before a bus is available.
// The traits will be serialized to Data.Conditions and applied when LoadFromData
// is called with LoadMonsterConditions.
//
// [Load] carries persisted condition blobs the same way, and
// monstertraits.AttachMonster is what turns either into behaviour — see
// [Monster.TakeUnappliedConditions].
func (m *Monster) AddTraitData(data json.RawMessage) {
	m.traitData = append(m.traitData, data)
}

// LoadFromData creates a Monster from persistent data and wires it to the bus.
//
// It is [Load] followed by the monster's own [SheetKeeper], with one
// difference that is the whole reason both exist: LoadFromData throws away the
// condition blobs in d, because its callers hand the same blobs to
// monstertraits.LoadMonsterConditions themselves and carrying them here too
// would write every condition back twice. Load carries them instead, and
// monstertraits.AttachMonster is what applies them — so the composed path cannot
// lose a monster's conditions by forgetting the trait-attachment call.
func LoadFromData(ctx context.Context, d *Data, bus events.EventBus) (*Monster, error) {
	if bus == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	m, err := loadMonster(d, dropConditions)
	if err != nil {
		return nil, err
	}

	if err := m.SheetKeeper().Apply(ctx, bus); err != nil {
		return nil, rpgerr.Wrapf(err, "failed to subscribe to events")
	}

	return m, nil
}

// onDamageReceived handles damage events
func (m *Monster) onDamageReceived(_ context.Context, event dnd5eEvents.DamageReceivedEvent) error {
	if event.TargetID != m.id {
		return nil
	}
	m.TakeDamage(event.Amount)

	// HP is persisted, and ApplyDamage marks dirty for the same reason: a
	// monster that took damage and did not go dirty is a monster whose new HP
	// never gets written down.
	m.dirty = true

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
	m.dirty = true

	return nil
}

// onConditionApplied applies a live condition delivered to this monster and
// records the persisted sheet change.
func (m *Monster) onConditionApplied(
	ctx context.Context, bus events.EventBus, event dnd5eEvents.ConditionAppliedEvent,
) error {
	if event.Target == nil || event.Target.GetID() != m.id {
		return nil
	}

	// THE ADMISSION DOOR, the twin of the character keeper's — see
	// character.Character.onConditionApplied for why this is the only place
	// either keeper checks. A condition that cannot name itself never joins the
	// list, so onConditionRemoved below can match on Ref() without asking again.
	if event.Condition.Ref() == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument,
			"condition returns a nil Ref, so it could never be matched by a removal")
	}

	if err := event.Condition.Apply(ctx, bus); err != nil {
		_ = event.Condition.Remove(ctx, bus)
		return rpgerr.Wrapf(err, "failed to apply monster condition")
	}
	m.AddCondition(event.Condition)
	return nil
}

// onConditionRemoved drops a condition that ended off this monster's sheet.
//
// The character keeper has had this row since conditions existed; the monster
// keeper did not, so a removal reached a monster's conditions never — latent
// only because no production path removes one yet. It lands here because
// nothing about removal is character-shaped, and a keeper missing a row its
// counterpart has is a gap waiting for its first caller rather than a decision.
//
// It matches on [dnd5eEvents.ConditionBehavior.Ref] rather than round-tripping
// each condition through ToJSON to recover the same string. The character's
// copy of this handler did the round trip until rpg-project#319 Phase 6 —
// it predated conditions being able to name themselves (rpg-toolkit#971) —
// and now asks the same question this one does, including the refusal below.
//
// It asks WITHOUT CHECKING for nil, because onConditionApplied above refused
// anything nameless at the door. That matters more here than it looks:
// [core.Ref.String] has a pointer receiver that dereferences its fields
// unguarded, so a nameless condition reaching this line would PANIC out of a
// bus publish rather than return an error. Admission is what makes the bare
// call safe — protect the construction, and the nil never arrives.
//
// The unapplied trait blobs are deliberately untouched. They are conditions
// that have not been attached yet — monstertraits.AttachMonster drains them
// into m.conditions — so a live removal has nothing to say about them, and a
// removal that arrived before attachment is a shelf that predates this row.
//
// Dirty only when the list actually shrank, for the reason the character's
// does: a removal event reaches every sheet on the bus, and flagging the ones
// it was not about would persist every monster in the fight.
func (m *Monster) onConditionRemoved(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
	if event.MemberID != m.id {
		return nil
	}

	filtered := make([]dnd5eEvents.ConditionBehavior, 0, len(m.conditions))
	for _, condition := range m.conditions {
		if condition.Ref().String() != event.ConditionRef {
			filtered = append(filtered, condition)
		}
	}

	if len(filtered) != len(m.conditions) {
		// ToData serializes conditions, so a sheet that lost one and did not
		// go dirty is a sheet whose removal never gets written down.
		m.dirty = true
	}
	m.conditions = filtered

	return nil
}

// onConditionStateChanged records that a condition hanging on this sheet
// changed its own persisted state.
//
// The same row the character keeper has, doing the same thing, because there
// is nothing character-shaped about it: a monster's conditions serialize into
// its Data exactly as a character's do, and a wolf that spent its reaction is
// reloaded having spent nothing unless the sheet is marked.
//
// This is what [Monster.MarkDirty] was minted ahead of, and it is now the
// monster half of the keeper table rather than a handle a condition holds.
func (m *Monster) onConditionStateChanged(
	_ context.Context, event dnd5eEvents.ConditionStateChangedEvent,
) error {
	if event.MemberID != m.id {
		return nil
	}

	m.dirty = true

	return nil
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
		Actions:          make([]combatActions.Definition, len(m.actions)),
		Proficiencies:    make([]ProficiencyData, 0, len(m.proficiencies)),
	}

	for index, definition := range m.actions {
		data.Actions[index] = definition.Clone()
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
