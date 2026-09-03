package character

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combatabilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// GetActionEconomy returns the current action economy data, or nil if not in combat.
//
// The economy comes back live, not copied — the same shape as GetResource, and
// the same warning. Reading it is free; writing through it moves persisted
// state without the sheet noticing, and the write is then dropped by the next
// write-back. Spend through the methods on this file, which mark the sheet
// dirty (#1087).
func (c *Character) GetActionEconomy() *ActionEconomyData {
	return c.actionEconomy
}

// InCombat returns true if the character is currently in combat.
// Combat is indicated by the action economy being initialized (non-nil).
func (c *Character) InCombat() bool {
	return c.actionEconomy != nil
}

// economyChanged records that a write landed on c.actionEconomy.
//
// The economy is persisted — ToData writes it to Data.ActionEconomy and Load
// restores it — but serializing is not saving: resolution keeps only the
// sheets that report IsDirty() and session writes back only those. An economy
// mutation that skips this is a spend that serializes perfectly and is then
// discarded (#1087).
//
// Each writer calls this for itself rather than trusting a caller to. The
// restoreActionType rollback is the one exception because it undoes a spend
// that already marked. The load path seeds the economy from
// stored data, so the sheet already matches storage.
func (c *Character) economyChanged() {
	c.dirty = true
}

// ExitCombat clears the action economy entirely, removing combat state.
// Call this when the encounter ends, not between turns.
func (c *Character) ExitCombat(_ context.Context, _ *ExitCombatInput) (*ExitCombatOutput, error) {
	// Guarded so that leaving combat twice is not a second change: an economy
	// that was already absent has nothing to write.
	if c.actionEconomy != nil {
		c.actionEconomy = nil
		c.economyChanged()
	}

	return &ExitCombatOutput{}, nil
}

// StartTurn initializes the action economy for a new turn.
// Sets 1 action, 1 bonus action, 1 reaction, and movement from input speed.
// Returns the available abilities for this turn.
//
// A call with no input is refused rather than dereferenced. The two turn verbs
// answer the same way about the same mistake: [Character.RefreshForTurn] will
// not reseed turn zero at zero speed either.
func (c *Character) StartTurn(_ context.Context, input *StartTurnInput) (*StartTurnOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "no turn to start")
	}

	c.seedTurn(input.TurnNumber, input.Speed)

	return &StartTurnOutput{
		Abilities: c.buildAvailableAbilities(),
	}, nil
}

// seedTurn replaces the action economy with a full one for the given turn.
//
// Everything a turn grants is here and nothing else is: the three slots, the
// movement the caller states (conditions modify speed, and that arithmetic
// belongs above this), and an empty bank — capacity granted last turn is not
// this turn's to spend. Shared by the turn-start verb and the freshness helper
// so the two cannot drift into disagreeing about what a fresh turn looks like.
func (c *Character) seedTurn(turnNumber, speed int) {
	granted := make(map[GrantedActionKey]int)
	if combat.ParticipationFor(c.lifeState()).NeedsDeathSave {
		granted[GrantedDeathSaves] = 1
	}

	c.actionEconomy = &ActionEconomyData{
		TurnNumber:            turnNumber,
		ActionsRemaining:      1,
		BonusActionsRemaining: 1,
		ReactionsRemaining:    1,
		MovementRemaining:     speed,
		Granted:               granted,
	}
	c.economyChanged()
}

// RefreshForTurn fills a stale bank, so that the economy is observably full
// when the character's turn begins.
//
// The stored TurnNumber is what makes this answerable without anybody
// remembering: an economy left over from turn 3, asked about turn 4, is stale
// and gets reseeded; asked about turn 3 again it is untouched, so a second
// swing cannot refill what the first one spent. That is the whole rule, and it
// is why this is safe to call at every ask rather than exactly once.
//
// Materialised at the first ask rather than pushed at the turn boundary: the
// sheet may not have been loaded when the turn changed, and a bank that is
// only correct if something remembered to announce the boundary is a bank that
// is eventually wrong.
//
// A sheet that is not in combat has no turn to be stale and is left alone —
// combat starts somewhere else, and inventing an economy here would put a
// character in a fight nobody put them in.
//
// Nothing in this rulebook calls it yet. The door that pays an action's cost
// is what will (rpg-toolkit#1091).
func (c *Character) RefreshForTurn(
	_ context.Context, input *RefreshForTurnInput,
) (*RefreshForTurnOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "no turn to refresh for")
	}
	if c.actionEconomy == nil {
		return &RefreshForTurnOutput{}, nil
	}

	// Any turn number that is not the stored one is stale, including one that
	// went backwards. A bank left empty because a number moved the wrong way is
	// a character who cannot act on their own turn, which is a worse failure
	// than a bank refilled once too often.
	if c.actionEconomy.TurnNumber == input.TurnNumber {
		return &RefreshForTurnOutput{}, nil
	}

	c.seedTurn(input.TurnNumber, input.Speed)

	return &RefreshForTurnOutput{Reseeded: true}, nil
}

// EndTurn spends out what the turn owned and leaves the reaction alone. The
// character remains in combat (actionEconomy stays non-nil).
//
// # The reaction is the exception, and it is not an oversight
//
// Action, bonus action, movement and granted capacity are TURN-SCOPED: you do
// not carry an action into somebody else's turn, so ending yours empties them.
// The reaction is the one resource whose whole purpose is the gap BETWEEN your
// turns — an opportunity attack happens on another creature's turn by
// definition. 2014 RAW refreshes it at the START of each of your turns, so it
// must survive the end of the previous one.
//
// This method used to zero it too, and was safe only because nothing called it
// (rpg-project#316). Wiring it into a turn boundary would have disarmed every
// reactor for exactly the window the reaction governs, silently: the OA
// condition asks CanReact, would find an empty purse, and decline — which looks
// identical to choosing not to react. Kirk ruled it a bug rather than a hazard
// to be guarded, so the guard is gone and the behaviour is correct instead.
func (c *Character) EndTurn(_ context.Context, _ *EndTurnInput) (*EndTurnOutput, error) {
	if c.actionEconomy != nil {
		c.actionEconomy.ActionsRemaining = 0
		c.actionEconomy.BonusActionsRemaining = 0
		c.actionEconomy.MovementRemaining = 0
		c.actionEconomy.Granted = make(map[GrantedActionKey]int)
		c.economyChanged()
	}
	return &EndTurnOutput{}, nil
}

// AvailableAbilities returns the list of abilities the character can potentially use.
// Returns an empty slice if not in combat.
func (c *Character) AvailableAbilities() []AvailableAbility {
	if !c.InCombat() {
		return []AvailableAbility{}
	}
	return c.buildAvailableAbilities()
}

// ActivateAbility activates a combat ability or feature by ref.
// Routes to the appropriate handler based on whether the ref matches a combat ability or feature.
// Returns success=false with an error message if activation fails.
//
// # A call with nothing in it is refused, not answered
//
// "Not in combat" and "unknown ability" are ANSWERS: a question was asked and
// the sheet has a reply. A nil input, or one naming no ability, is not a
// question — the caller has a bug, and handing it back a well-formed
// Success:false would report that bug as a barbarian who cannot rage. So it is
// an error by name at the door, the same door [Character.StartTurn] and
// [Character.RefreshForTurn] keep over the same state (rpg-toolkit#1093).
//
// The refusal PRECEDES the combat check, and that ordering is the fix rather
// than incidental to it. Before this, a nil input from a character out of
// combat came back "not in combat" — true about the world, false about what
// went wrong — and the same nil from a character IN combat panicked. One
// mistake, two behaviours, neither of them the truth.
func (c *Character) ActivateAbility(_ context.Context, input *ActivateAbilityInput) (*ActivateAbilityOutput, error) {
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "no ability to activate")
	}
	if input.AbilityRef == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "no ability ref to activate")
	}

	if !c.InCombat() {
		return &ActivateAbilityOutput{
			Success: false,
			Error:   "not in combat",
		}, nil
	}

	// Try combat abilities first
	for _, ca := range c.combatAbilities {
		if ca.Ref().ID == input.AbilityRef.ID {
			return c.activateCombatAbility(ca, input)
		}
	}

	// Try features
	for _, f := range c.features {
		if f.Ref().ID == input.AbilityRef.ID {
			return c.activateFeature(f, input)
		}
	}

	return &ActivateAbilityOutput{
		Success: false,
		Error:   "unknown ability",
	}, nil
}

// GrantCapacity grants a specified amount of capacity for a given key.
// Used by external systems to grant additional capacity (e.g., Action Surge granting extra attacks).
func (c *Character) GrantCapacity(key GrantedActionKey, amount int) {
	if c.actionEconomy == nil {
		return
	}
	if c.actionEconomy.Granted == nil {
		c.actionEconomy.Granted = make(map[GrantedActionKey]int)
	}
	c.actionEconomy.Granted[key] += amount
	c.economyChanged()
}

// HasGranted returns whether the character has any remaining capacity for the given key.
func (c *Character) HasGranted(key GrantedActionKey) bool {
	if c.actionEconomy == nil || c.actionEconomy.Granted == nil {
		return false
	}
	return c.actionEconomy.Granted[key] > 0
}

// --- Available builders ---

// targetKindForRef returns the UI target prompt a menu entry needs, keyed off
// the action/ability ref. This is toolkit-authored rules knowledge (the toolkit
// owns "what does this action target") — the game server never computes it.
// An unknown ref returns TargetKindUnspecified so a new ref surfaces as a
// visible defect rather than silently defaulting.
//
// # It answers for features too, and until now it did not
//
// The table was written when only combat abilities reached a menu, so every
// FEATURE fell to the default and came back Unspecified. That defaulting did
// exactly what it was designed to do — it stayed silent while nothing read the
// answer, and it surfaced the moment something did (rpg-project#300). Rage and
// Second Wind are here because they are the level-1 features a player can
// activate; the rest of the module's features are deliberately absent rather
// than guessed, and arrive with the slice that makes each of them reachable.
func targetKindForRef(ref *core.Ref) TargetKind {
	if ref == nil {
		return TargetKindUnspecified
	}
	switch ref.ID {
	// Attack-shaped abilities target one entity.
	case refs.CombatAbilities.Attack().ID,
		refs.CombatAbilities.Help().ID:
		return TargetKindSingleEntity
	// Self-affecting abilities (grant a condition on the actor).
	case refs.CombatAbilities.Dodge().ID,
		refs.CombatAbilities.Disengage().ID,
		refs.CombatAbilities.Hide().ID:
		return TargetKindSelf
	// Deliberately untargeted (fire without a prompt).
	case refs.CombatAbilities.Dash().ID:
		return TargetKindNone
	// Features that act on the character who used them. Self rather than None:
	// both mean "do not prompt", and only Self also says whose sheet the effect
	// lands on — which is the half a caller other than a prompt cares about.
	case refs.Features.Rage().ID,
		refs.Features.SecondWind().ID:
		return TargetKindSelf
	default:
		return TargetKindUnspecified
	}
}

// buildAvailableAbilities builds the list of available abilities from combat abilities and features.
func (c *Character) buildAvailableAbilities() []AvailableAbility {
	result := make([]AvailableAbility, 0, len(c.combatAbilities)+len(c.features))

	// Combat abilities
	for _, ca := range c.combatAbilities {
		canUse := c.canUseAbilityByActionType(ca.ActionType())
		reason := c.actionTypeExhaustedReason(ca.ActionType())

		result = append(result, AvailableAbility{
			Ref:         ca.Ref(),
			Name:        ca.Name(),
			ActionType:  ca.ActionType(),
			EconomySlot: economySlotForActionType(ca.ActionType()),
			TargetKind:  targetKindForRef(ca.Ref()),
			CanUse:      canUse,
			Reason:      c.actionReason(canUse, reason),
		})
	}

	// Features
	for _, f := range c.features {
		canUse := c.canUseAbilityByActionType(f.ActionType())
		reason := c.actionTypeExhaustedReason(f.ActionType())

		// Check feature-specific resource availability
		if canUse {
			if err := f.CanActivate(context.Background(), c, features.FeatureInput{}); err != nil {
				canUse = false
				reason = err.Error()
			}
		}

		current, max := c.featureResourceInfo(f)

		result = append(result, AvailableAbility{
			Ref:             f.Ref(),
			Name:            f.Name(),
			ActionType:      f.ActionType(),
			EconomySlot:     economySlotForActionType(f.ActionType()),
			TargetKind:      targetKindForRef(f.Ref()),
			CanUse:          canUse,
			Reason:          c.actionReason(canUse, reason),
			ResourceCurrent: current,
			ResourceMax:     max,
		})
	}

	return result
}

// --- Activate helpers ---

// activateCombatAbility uses the bridge pattern to activate a combat ability.
// Converts ActionEconomyData to toolkit ActionEconomy, calls the ability, then syncs back.
func (c *Character) activateCombatAbility(
	ca combatabilities.CombatAbility, activateInput *ActivateAbilityInput,
) (*ActivateAbilityOutput, error) {
	ae := c.toToolkitActionEconomy()

	ctx := context.Background()
	input := combatabilities.CombatAbilityInput{
		Bus:                        c.bus,
		ActionEconomy:              ae,
		Speed:                      c.GetSpeed(),
		ExtraAttacks:               c.GetExtraAttacksCount(),
		Target:                     activateInput.Target,
		ObserverPassivePerceptions: activateInput.ObserverPassivePerceptions,
	}

	if err := ca.CanActivate(ctx, c, input); err != nil {
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     err.Error(),
			Abilities: c.buildAvailableAbilities(),
		}, nil
	}
	if err := ca.Activate(ctx, c, input); err != nil {
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     err.Error(),
			Abilities: c.buildAvailableAbilities(),
		}, nil
	}

	c.fromToolkitActionEconomy(ae)
	return &ActivateAbilityOutput{
		Success:         true,
		GrantedCapacity: c.describeGrantedCapacity(ca),
		Abilities:       c.buildAvailableAbilities(),
	}, nil
}

// activateFeature directly manages action economy for feature activation.
// Features manage their own resources while Character manages slot consumption.
func (c *Character) activateFeature(f features.Feature, _ *ActivateAbilityInput) (*ActivateAbilityOutput, error) {
	if !c.canUseAbilityByActionType(f.ActionType()) {
		reason := c.actionTypeExhaustedReason(f.ActionType())
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     reason,
			Abilities: c.buildAvailableAbilities(),
		}, nil
	}

	ctx := context.Background()
	if err := f.CanActivate(ctx, c, features.FeatureInput{}); err != nil {
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     err.Error(),
			Abilities: c.buildAvailableAbilities(),
		}, nil
	}

	c.consumeActionType(f.ActionType())
	if err := f.Activate(ctx, c, features.FeatureInput{Bus: c.bus}); err != nil {
		c.restoreActionType(f.ActionType())
		return &ActivateAbilityOutput{
			Success:   false,
			Error:     err.Error(),
			Abilities: c.buildAvailableAbilities(),
		}, nil
	}

	return &ActivateAbilityOutput{
		Success:   true,
		Abilities: c.buildAvailableAbilities(),
	}, nil
}

// --- Bridge methods ---

// toToolkitActionEconomy converts ActionEconomyData to the toolkit's combat.ActionEconomy.
// This bridges our serializable data with the toolkit's combat ability system.
//
// It only reads: the value it returns is a detached copy that the ability
// mutates, and fromToolkitActionEconomy is what puts the result back on the
// sheet. Nothing here dirties anything.
func (c *Character) toToolkitActionEconomy() *combat.ActionEconomy {
	ae := &combat.ActionEconomy{
		ActionsRemaining:      c.actionEconomy.ActionsRemaining,
		BonusActionsRemaining: c.actionEconomy.BonusActionsRemaining,
		ReactionsRemaining:    c.actionEconomy.ReactionsRemaining,
		MovementRemaining:     c.actionEconomy.MovementRemaining,
	}

	// Map granted capacity
	if attacks, ok := c.actionEconomy.Granted[GrantedAttacks]; ok {
		ae.AttacksRemaining = attacks
	}
	if offHand, ok := c.actionEconomy.Granted[GrantedOffHandStrikes]; ok {
		ae.OffHandAttacksRemaining = offHand
	}
	if martialArts, ok := c.actionEconomy.Granted[GrantedMartialArtsBonus]; ok {
		ae.MartialArtsBonusAttacksRemaining = martialArts
	}
	if flurry, ok := c.actionEconomy.Granted[GrantedFlurryStrikes]; ok {
		ae.FlurryStrikesRemaining = flurry
	}
	if deathSaves, ok := c.actionEconomy.Granted[GrantedDeathSaves]; ok {
		ae.DeathSavesRemaining = deathSaves
	}

	return ae
}

// fromToolkitActionEconomy syncs the toolkit's combat.ActionEconomy back to ActionEconomyData.
// Called after a combat ability modifies the toolkit ActionEconomy.
//
// This is where a whole ability activation lands on the sheet at once — the
// slot it spent and the capacity it granted — so it is where that activation
// becomes something to save.
func (c *Character) fromToolkitActionEconomy(ae *combat.ActionEconomy) {
	c.actionEconomy.ActionsRemaining = ae.ActionsRemaining
	c.actionEconomy.BonusActionsRemaining = ae.BonusActionsRemaining
	c.actionEconomy.ReactionsRemaining = ae.ReactionsRemaining
	c.actionEconomy.MovementRemaining = ae.MovementRemaining

	// Sync granted capacity back.
	if ae.AttacksRemaining > 0 {
		c.actionEconomy.Granted[GrantedAttacks] = ae.AttacksRemaining
	}
	if ae.OffHandAttacksRemaining > 0 {
		c.actionEconomy.Granted[GrantedOffHandStrikes] = ae.OffHandAttacksRemaining
	}
	if ae.MartialArtsBonusAttacksRemaining > 0 {
		c.actionEconomy.Granted[GrantedMartialArtsBonus] = ae.MartialArtsBonusAttacksRemaining
	}
	if ae.FlurryStrikesRemaining > 0 {
		c.actionEconomy.Granted[GrantedFlurryStrikes] = ae.FlurryStrikesRemaining
	}
	if ae.DeathSavesRemaining > 0 {
		c.actionEconomy.Granted[GrantedDeathSaves] = ae.DeathSavesRemaining
	} else {
		delete(c.actionEconomy.Granted, GrantedDeathSaves)
	}

	c.economyChanged()
}

// --- Helper methods ---

// canUseAbilityByActionType checks if the character has the action economy resource
// for the given action type.
func (c *Character) canUseAbilityByActionType(actionType coreCombat.ActionType) bool {
	switch actionType {
	case coreCombat.ActionStandard:
		return c.actionEconomy.ActionsRemaining > 0
	case coreCombat.ActionBonus:
		return c.actionEconomy.BonusActionsRemaining > 0
	case coreCombat.ActionReaction:
		return c.actionEconomy.ReactionsRemaining > 0
	case coreCombat.ActionFree:
		return true
	default:
		return false
	}
}

// actionTypeExhaustedReason returns a human-readable reason for why an action type is exhausted.
func (c *Character) actionTypeExhaustedReason(actionType coreCombat.ActionType) string {
	switch actionType {
	case coreCombat.ActionStandard:
		return "no action remaining"
	case coreCombat.ActionBonus:
		return "no bonus action remaining"
	case coreCombat.ActionReaction:
		return "no reaction remaining"
	default:
		return ""
	}
}

// actionReason returns the reason string only if canUse is false.
func (c *Character) actionReason(canUse bool, reason string) string {
	if canUse {
		return ""
	}
	return reason
}

// consumeActionType decrements the appropriate action economy counter.
func (c *Character) consumeActionType(actionType coreCombat.ActionType) {
	switch actionType {
	case coreCombat.ActionStandard:
		c.actionEconomy.ActionsRemaining--
	case coreCombat.ActionBonus:
		c.actionEconomy.BonusActionsRemaining--
	case coreCombat.ActionReaction:
		c.actionEconomy.ReactionsRemaining--
	default:
		// Nothing to spend, so nothing to save. A free action costs no slot by
		// definition; any OTHER type never reaches here at all, because
		// activateFeature gates on canUseAbilityByActionType, which recognises
		// exactly the four types above and refuses the rest.
		//
		// The two free features that ship say the same thing from the other
		// side: Reckless Attack's change to the sheet is a condition, and the
		// condition site marks it, while Action Surge cannot reach its spend
		// through this path at all — its CanActivate demands an ActionEconomy
		// that activateFeature does not supply.
		return
	}

	c.economyChanged()
}

// restoreActionType increments the appropriate action economy counter (rollback).
//
// It does not need to mark the sheet dirty — the consumeActionType it undoes
// already did — and deliberately does not mark it clean again. A feature whose
// Activate failed may have moved its own persisted state before it errored, and
// one redundant write costs less than one lost spend.
func (c *Character) restoreActionType(actionType coreCombat.ActionType) {
	switch actionType {
	case coreCombat.ActionStandard:
		c.actionEconomy.ActionsRemaining++
	case coreCombat.ActionBonus:
		c.actionEconomy.BonusActionsRemaining++
	case coreCombat.ActionReaction:
		c.actionEconomy.ReactionsRemaining++
	}
}

// describeGrantedCapacity returns a human-readable description of what was granted
// by activating a combat ability.
func (c *Character) describeGrantedCapacity(ca combatabilities.CombatAbility) string {
	switch ca.Ref().ID {
	case refs.CombatAbilities.Attack().ID:
		attacks := c.actionEconomy.Granted[GrantedAttacks]
		if attacks == 1 {
			return "1 attack"
		}
		return fmt.Sprintf("%d attacks", attacks)
	case refs.CombatAbilities.Dash().ID:
		return fmt.Sprintf("%dft movement", c.GetSpeed())
	case refs.CombatAbilities.Dodge().ID:
		return "dodging until next turn"
	case refs.CombatAbilities.Disengage().ID:
		return "disengaging until next turn"
	default:
		return ""
	}
}

// featureResourceInfo returns the current and max resource values for a feature.
// Maps feature refs to their corresponding resource keys on the character.
func (c *Character) featureResourceInfo(f features.Feature) (current, max int) {
	ref := f.Ref()
	if ref == nil {
		return 0, 0
	}

	var key *core.Ref

	switch ref.ID {
	case refs.Features.Rage().ID:
		r := c.GetResource(resources.RageCharges)
		return r.Current(), r.Maximum()
	case refs.Features.SecondWind().ID:
		// SecondWind manages its own resource internally
		return 0, 0
	case refs.Features.FlurryOfBlows().ID, refs.Features.PatientDefense().ID, refs.Features.StepOfTheWind().ID:
		r := c.GetResource(resources.Ki)
		return r.Current(), r.Maximum()
	}

	_ = key
	return 0, 0
}
