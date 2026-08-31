package character

import (
	"context"
	"encoding/json"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/stretchr/testify/suite"
)

// ActionEconomyTestSuite tests the action economy types and persistence
type ActionEconomyTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

// SetupTest runs before each test function
func (s *ActionEconomyTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

// TestActionEconomyTestSuite runs the test suite
func TestActionEconomyTestSuite(t *testing.T) {
	suite.Run(t, new(ActionEconomyTestSuite))
}

// --- Test helper functions ---

// createTestFighterCharacter creates a Fighter with standard combat abilities and SecondWind.
func createTestFighterCharacter(t *testing.T, bus events.EventBus) *Character {
	t.Helper()

	// Load SecondWind feature from JSON
	swData := json.RawMessage(`{"ref":{"module":"dnd5e","type":"features","id":"second_wind"},"id":"second-wind-1","name":"Second Wind","level":3,"character_id":"fighter-1","uses":1,"max_uses":1}`)
	sw, err := features.LoadJSON(swData)
	if err != nil {
		t.Fatalf("failed to load second wind: %v", err)
	}

	return &Character{
		id:               "fighter-1",
		name:             "Test Fighter",
		level:            3,
		proficiencyBonus: 2,
		classID:          classes.Fighter,
		raceID:           races.Human,
		abilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		hitPoints:    28,
		maxHitPoints: 28,
		armorClass:   18,
		skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		savingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
		resources:    make(map[coreResources.ResourceKey]*combat.RecoverableResource),
		combatAbilities: []combatabilities.CombatAbility{
			combatabilities.NewAttack("attack-1"),
			combatabilities.NewDash("dash-1"),
			combatabilities.NewDodge("dodge-1"),
			combatabilities.NewDisengage("disengage-1"),
		},
		features: []features.Feature{sw},
		bus:      bus,
	}
}

// createTestBarbarianCharacter creates a Barbarian with Rage feature and rage charges.
// createTestMonkCharacter creates a Monk with appropriate class ID.
// createTWFCharacter creates a character with two light weapons equipped.
// --- Persistence tests (existing) ---

func (s *ActionEconomyTestSuite) TestInCombat_NilActionEconomy() {
	char := &Character{}
	s.False(char.InCombat())
}

func (s *ActionEconomyTestSuite) TestInCombat_WithActionEconomy() {
	char := &Character{
		actionEconomy: &ActionEconomyData{
			ActionsRemaining:      1,
			BonusActionsRemaining: 1,
			ReactionsRemaining:    1,
		},
	}
	s.True(char.InCombat())
}

func (s *ActionEconomyTestSuite) TestExitCombat() {
	char := &Character{
		actionEconomy: &ActionEconomyData{
			ActionsRemaining: 1,
		},
	}

	_, err := char.ExitCombat(s.ctx, &ExitCombatInput{})
	s.Require().NoError(err)
	s.False(char.InCombat())
}

func (s *ActionEconomyTestSuite) TestToData_NilActionEconomyOmitted() {
	char := &Character{
		id:           "test-char",
		name:         "Test",
		level:        1,
		skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		savingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
	}
	char.actionEconomy = nil

	data := char.ToData()
	s.Nil(data.ActionEconomy)

	// Verify it marshals without the field
	bytes, err := json.Marshal(data)
	s.Require().NoError(err)
	s.NotContains(string(bytes), "action_economy")
}

func (s *ActionEconomyTestSuite) TestToData_IncludesActionEconomy() {
	char := &Character{
		id:           "test-char",
		name:         "Test",
		level:        1,
		skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		savingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
	}
	char.actionEconomy = &ActionEconomyData{
		ActionsRemaining:      1,
		BonusActionsRemaining: 0,
		ReactionsRemaining:    1,
		MovementRemaining:     15,
		Granted: map[GrantedActionKey]int{
			GrantedAttacks: 1,
		},
	}

	data := char.ToData()
	s.Require().NotNil(data.ActionEconomy)
	s.Equal(1, data.ActionEconomy.ActionsRemaining)
	s.Equal(0, data.ActionEconomy.BonusActionsRemaining)
	s.Equal(15, data.ActionEconomy.MovementRemaining)
	s.Equal(1, data.ActionEconomy.Granted[GrantedAttacks])
}

func (s *ActionEconomyTestSuite) TestLoadFromData_RoundTrip() {
	// Create minimal valid Data with action economy
	data := &Data{
		ID:               "test-char",
		PlayerID:         "player-1",
		Name:             "Test Fighter",
		Level:            5,
		ProficiencyBonus: 3,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    44,
		MaxHitPoints: 44,
		ArmorClass:   18,
		Skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		SavingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
		ActionEconomy: &ActionEconomyData{
			ActionsRemaining:      0,
			BonusActionsRemaining: 1,
			ReactionsRemaining:    1,
			MovementRemaining:     15,
			Granted: map[GrantedActionKey]int{
				GrantedAttacks: 2,
			},
		},
	}

	// Load from data
	loaded, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)

	// Verify action economy was restored
	s.True(loaded.InCombat())

	// Round-trip through ToData
	roundTripped := loaded.ToData()
	s.Require().NotNil(roundTripped.ActionEconomy)
	s.Equal(0, roundTripped.ActionEconomy.ActionsRemaining)
	s.Equal(1, roundTripped.ActionEconomy.BonusActionsRemaining)
	s.Equal(1, roundTripped.ActionEconomy.ReactionsRemaining)
	s.Equal(15, roundTripped.ActionEconomy.MovementRemaining)
	s.Equal(2, roundTripped.ActionEconomy.Granted[GrantedAttacks])
}

// TestSeededEconomy_RoundTrip_ActivateAbility_NoNilMapPanic is the #706
// regression: StartTurn seeds an EMPTY Granted map, which json:"granted,omitempty"
// drops from the serialized JSON, so after a full JSON round-trip the loaded
// character's Granted is nil. Activating an ability then writes into Granted via
// fromToolkitActionEconomy and would panic ("assignment to entry in nil map")
// without the LoadFromData re-init. Reproduces the api seeding path (#598) — and
// the toolkit's own EndTurn→persist→reload — by marshaling to JSON and back, not
// just passing the struct.
func (s *ActionEconomyTestSuite) TestSeededEconomy_RoundTrip_ActivateAbility_NoNilMapPanic() {
	char := createTestFighterCharacter(s.T(), s.bus)

	// Seed the turn economy — this sets Granted to an empty map.
	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)
	s.Require().NotNil(char.actionEconomy)
	s.Require().Empty(char.actionEconomy.Granted, "StartTurn seeds an empty Granted map")

	// Serialize → JSON → back, faithfully reproducing the omitempty drop the
	// host (rpg-api) hits when it persists and reloads the character.
	data := char.ToData()
	raw, err := json.Marshal(data)
	s.Require().NoError(err)

	// Assert the omitempty drop precisely on the action_economy object (a
	// substring scan of the whole blob could false-fail on an unrelated
	// "granted" key in some other serialized field).
	var envelope struct {
		ActionEconomy map[string]json.RawMessage `json:"action_economy"`
	}
	s.Require().NoError(json.Unmarshal(raw, &envelope))
	s.Require().NotNil(envelope.ActionEconomy, "action_economy is present in the JSON")
	s.NotContains(envelope.ActionEconomy, "granted",
		"empty Granted is omitted from the action_economy JSON (omitempty)")

	var reloadedData Data
	s.Require().NoError(json.Unmarshal(raw, &reloadedData))
	s.Nil(reloadedData.ActionEconomy.Granted, "Granted comes back nil after the omitempty round-trip")

	loaded, err := LoadFromData(s.ctx, &reloadedData, events.NewEventBus())
	s.Require().NoError(err)

	// The fix: the loaded economy's Granted is a fresh writable map, not nil.
	s.Require().NotNil(loaded.GetActionEconomy())
	s.NotNil(loaded.GetActionEconomy().Granted, "LoadFromData must re-init a writable Granted map (#706)")

	// Activating the Attack ability writes into Granted (grants attacks). This
	// is the line that panicked on the nil map before the fix.
	s.Require().NotPanics(func() {
		out, actErr := loaded.ActivateAbility(s.ctx, &ActivateAbilityInput{
			AbilityRef: refs.CombatAbilities.Attack(),
		})
		s.Require().NoError(actErr)
		s.True(out.Success, "Attack should activate after a seeded-economy round-trip")
	})

	// The granted capacity was recorded (proves Granted is live, not just non-nil).
	s.Positive(loaded.GetActionEconomy().Granted[GrantedAttacks],
		"activating Attack must grant attack capacity into the reloaded economy")
}

func (s *ActionEconomyTestSuite) TestLoadFromData_NilActionEconomy() {
	// Create minimal valid Data without action economy
	data := &Data{
		ID:               "test-char",
		PlayerID:         "player-1",
		Name:             "Test Fighter",
		Level:            5,
		ProficiencyBonus: 3,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    44,
		MaxHitPoints: 44,
		ArmorClass:   18,
		Skills:       make(map[skills.Skill]shared.ProficiencyLevel),
		SavingThrows: make(map[abilities.Ability]shared.ProficiencyLevel),
	}

	// Load from data
	loaded, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.Require().NotNil(loaded)

	// Verify not in combat
	s.False(loaded.InCombat())
}

// --- Turn lifecycle ---

func (s *ActionEconomyTestSuite) TestStartTurn() {
	char := createTestFighterCharacter(s.T(), s.bus)

	output, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Verify action economy initialized
	s.True(char.InCombat())
	s.Equal(1, char.actionEconomy.ActionsRemaining)
	s.Equal(1, char.actionEconomy.BonusActionsRemaining)
	s.Equal(1, char.actionEconomy.ReactionsRemaining)
	s.Equal(30, char.actionEconomy.MovementRemaining)

	// Verify abilities returned
	s.NotEmpty(output.Abilities)

}

func (s *ActionEconomyTestSuite) TestStartTurn_ResetsFromPreviousTurn() {
	char := createTestFighterCharacter(s.T(), s.bus)

	// Start first turn and consume resources
	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)
	char.actionEconomy.ActionsRemaining = 0
	char.actionEconomy.BonusActionsRemaining = 0
	char.actionEconomy.Granted[GrantedAttacks] = 2

	// Start second turn
	output, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Verify fresh resources
	s.Equal(1, char.actionEconomy.ActionsRemaining)
	s.Equal(1, char.actionEconomy.BonusActionsRemaining)
	s.Equal(1, char.actionEconomy.ReactionsRemaining)
	s.Equal(30, char.actionEconomy.MovementRemaining)
	s.Equal(0, char.actionEconomy.Granted[GrantedAttacks], "granted should be cleared")
	s.NotEmpty(output.Abilities)
}

func (s *ActionEconomyTestSuite) TestEndTurn_ResetsButStaysInCombat() {
	char := createTestFighterCharacter(s.T(), s.bus)

	// Start turn
	_, err := char.StartTurn(s.ctx, &StartTurnInput{Speed: 30})
	s.Require().NoError(err)

	// Grant some capacity
	char.actionEconomy.Granted[GrantedAttacks] = 1

	// End turn
	_, err = char.EndTurn(s.ctx, &EndTurnInput{})
	s.Require().NoError(err)

	// Verify resources zeroed
	s.Equal(0, char.actionEconomy.ActionsRemaining)
	s.Equal(0, char.actionEconomy.BonusActionsRemaining)
	s.Equal(0, char.actionEconomy.ReactionsRemaining)
	s.Equal(0, char.actionEconomy.MovementRemaining)
	s.Equal(0, char.actionEconomy.Granted[GrantedAttacks], "granted should be cleared")

	// But still in combat
	s.True(char.InCombat())
}

// --- rpg-toolkit#1093: a call with nothing in it is refused, not answered ---

// The panic case. In combat, the ref lookup dereferenced the input directly.
func (s *ActionEconomyTestSuite) TestActivatingWithoutInputIsRefused() {
	char := createTestFighterCharacter(s.T(), s.bus)
	_, err := char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	markSaved(char)

	out, err := char.ActivateAbility(s.ctx, nil)

	s.Require().Error(err)
	s.Nil(out)
	// A refusal spends nothing: the action is still there to take once the
	// caller passes an ability.
	s.Equal(1, char.GetActionEconomy().ActionsRemaining)
	s.False(char.IsDirty())
}

// THE ORDERING TEST, and the reason the nil check sits above the combat check
// rather than below it. Out of combat, a nil input never reached the
// dereference — it came back Success:false, "not in combat", which is a true
// statement about the world and a false one about what the caller did wrong.
// A test that only ever passed nil to a character IN combat cannot tell the
// two orderings apart, because both panic-free versions look identical there.
func (s *ActionEconomyTestSuite) TestActivatingWithoutInputIsRefusedEvenOutOfCombat() {
	char := createTestFighterCharacter(s.T(), s.bus)
	s.Require().False(char.InCombat())

	out, err := char.ActivateAbility(s.ctx, nil)

	s.Require().Error(err)
	s.Nil(out)
	s.NotContains(err.Error(), "not in combat")
}

// The other half of the same mistake: an input that exists but names nothing.
// It dereferences one field further in, so it survives a fix that only guards
// the outer pointer.
func (s *ActionEconomyTestSuite) TestActivatingWithoutARefIsRefused() {
	char := createTestFighterCharacter(s.T(), s.bus)
	_, err := char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)
	markSaved(char)

	out, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{})

	s.Require().Error(err)
	s.Nil(out)
	s.Equal(1, char.GetActionEconomy().ActionsRemaining)
	s.False(char.IsDirty())
}

// The refusals are refusals and not a new way to fail an ordinary call: an
// ability this character does not have is still an ANSWER, not an error.
func (s *ActionEconomyTestSuite) TestAnUnknownAbilityIsStillAnAnswer() {
	char := createTestFighterCharacter(s.T(), s.bus)
	_, err := char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	out, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{
		AbilityRef: refs.Features.Rage(),
	})

	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.False(out.Success)
	s.Equal("unknown ability", out.Error)
}

func (s *ActionEconomyTestSuite) TestTwoWeaponsDoNotSynthesizeAnActivateAbility() {
	char, err := LoadFromData(s.ctx, &Data{
		ID:               "two-weapon-fighter",
		PlayerID:         "player-1",
		Name:             "Two Weapons",
		Level:            1,
		ClassID:          classes.Fighter,
		RaceID:           races.Human,
		ProficiencyBonus: 2,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:    12,
		MaxHitPoints: 12,
		ArmorClass:   14,
		Inventory: []InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Scimitar), Quantity: 1},
		},
		EquipmentSlots: EquipmentSlots{
			SlotMainHand: string(weapons.Shortsword),
			SlotOffHand:  string(weapons.Scimitar),
		},
	}, s.bus)
	s.Require().NoError(err)
	_, err = char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	for _, ability := range char.AvailableAbilities() {
		s.NotEqual(refs.CombatAbilities.OffHandAttack().String(), ability.Ref.String())
	}
}

// --- rpg-project#300: the target table has to answer for features too ---

// The behavioural half: a fighter's Second Wind reaches a caller with a target
// kind, not with the blank that means "the producer forgot to say".
func (s *ActionEconomyTestSuite) TestAFeatureCarriesItsTargetKind() {
	char := createTestFighterCharacter(s.T(), s.bus)
	_, err := char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	// One call, held. AvailableAbilities builds a fresh slice every time, so
	// indexing a second call would take a pointer into a different slice than
	// the one being ranged over — harmless here and a trap the moment the
	// builder does anything order- or state-dependent.
	abilities := char.AvailableAbilities()

	var secondWind *AvailableAbility
	for i := range abilities {
		if abilities[i].Ref != nil && abilities[i].Ref.ID == refs.Features.SecondWind().ID {
			secondWind = &abilities[i]
		}
	}

	s.Require().NotNil(secondWind, "the fighter should offer Second Wind")
	s.Equal(TargetKindSelf, secondWind.TargetKind)
}

// The table half. Rage has no helper on this suite, and the point being pinned
// is the table rather than the barbarian, so it is asked directly.
func (s *ActionEconomyTestSuite) TestTheTargetTableKnowsTheLevelOneFeatures() {
	s.Equal(TargetKindSelf, targetKindForRef(refs.Features.Rage()))
	s.Equal(TargetKindSelf, targetKindForRef(refs.Features.SecondWind()))
}

// AND THE DEFAULT IS STILL A DEFAULT. Teaching the table two refs must not
// turn its safety net into a guess: a feature nobody has ruled on yet still
// comes back Unspecified, which is what makes the next gap visible the way
// this one was.
func (s *ActionEconomyTestSuite) TestAnUnruledFeatureStillSurfacesAsUnspecified() {
	s.Equal(TargetKindUnspecified, targetKindForRef(refs.Features.ActionSurge()))
	s.Equal(TargetKindUnspecified, targetKindForRef(refs.Features.FlurryOfBlows()))
	s.Equal(TargetKindUnspecified, targetKindForRef(nil))
}
