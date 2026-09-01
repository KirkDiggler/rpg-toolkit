// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// effectPolicy decides what a load does with a persisted blob it cannot make
// sense of. It is a property of how a sheet was loaded, and it travels with the
// sheet: a sheet loaded leniently attaches leniently, because the two halves of
// one loader must not disagree about what a failure means.
type effectPolicy int

const (
	// strictEffects refuses the whole load and names what failed. This is the
	// zero value, so anything not loaded from persisted data — a finalized
	// draft, for instance — is strict without having to say so.
	strictEffects effectPolicy = iota

	// lenientEffects drops what it cannot read and carries on, which is what
	// [LoadFromData] has always done.
	lenientEffects
)

// loadedEffect pairs a condition's behaviour with the ref its loader routed on.
//
// The pairing is only knowable here. A [dnd5eEvents.ConditionBehavior] cannot
// name itself (rpg-toolkit#971), so the moment the load routes a blob's ref to
// a loader is the one moment the (ref, behaviour) pair exists — and per-effect
// attribution downstream is built entirely out of that pair.
type loadedEffect struct {
	ref      core.Ref
	behavior dnd5eEvents.ConditionBehavior
}

// Load turns persisted data into a character sheet, and does nothing else.
//
// No event bus, no subscriptions, no effect applied — a loaded sheet is inert
// until [Attach] puts it on a bus. Conditions are still parsed into behaviours
// and kept on the sheet, together with the ref their loader routed on, so that
// [Attach] can attribute each one later and so that ToData writes back what
// the sheet was loaded with. Load(d).ToData() is the data it was handed.
//
// Load is strict where [LoadFromData] is forgiving: a persisted blob it cannot
// make sense of — a condition, a feature, an unknown inventory item, or an
// inventory row with a nonpositive quantity — fails the load, and the error
// names the blob. Invalid character-owned resource bounds
// (negative current/maximum or current above maximum) fail before a resource is
// constructed. LoadFromData drops that blob or malformed resource and carries on,
// which is the behaviour its callers have today and keep. The divergence is
// deliberate. A loader that continues past what it cannot read hands back a
// sheet that is quietly missing something, and the next save persists the
// loss; that is the shape of rpg-toolkit#948, and refusing is the only way the
// round trip can be an actual guarantee.
//
// Two fields of [Data] do not survive a round trip through any loader:
// BackgroundID and CreatedAt have nowhere to live on the sheet, and ToData
// does not write them. That is a pre-existing gap in the sheet rather than in
// this loader — pinned by TestKnownRoundTripGaps so it cannot be mistaken for
// a guarantee — and closing it means changing ToData, which belongs in its own
// change.
//
// ctx is unused: a pure load has nothing to cancel and reads no clock. It is
// part of the signature so that the pure and legacy loaders read the same at a
// call site, and so that adding cancellable work here is never a breaking
// change.
func Load(_ context.Context, d *Data) (*Character, error) {
	if d == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "character data is required")
	}

	return loadSheet(d, strictEffects)
}

// Attach puts a loaded sheet on an event bus: the sheet's own keeper and
// attachable features first, then every condition the load parsed. Features
// and conditions each receive the bus scoped to their own ref.
//
// This is the whole attach loop, in one call, on the dnd5e side of the seam.
// Ordering is fixed rather than incidental: the sheet's own hooks are attached
// before any effect's, so two attaches over identical data grant identical
// registrations in identical order (ADR-0038 R4), and so an effect's hooks are
// never interleaved with the sheet's in a registration list.
//
// Each attachable feature and condition goes on through
// [dnd5eEvents.BusForEffect]. Features name themselves through Feature.Ref;
// conditions use the ref the load captured. A plain bus returns itself and
// nothing changes; a bus that keeps a registration list gets to record which
// effect made which subscription. The sheet keeper's own hooks use the bus
// itself, so the character's own machinery is never laundered into an effect's
// name.
//
// Attaching also parks the bus on the sheet, for the verb methods that still
// read it (MakeSavingThrow, EffectiveAC, the rests).
// That is the keeper's doing, not this function's, because a character built
// by Draft.Finalize gets there without ever passing through here; see
// SheetKeeper.subscribeSelf.
//
// **A failed strict Attach is a no-op.** Every effect that did go on comes back
// off, newest first; the keeper is removed; the sheet gets back the bus it was
// holding; and the conditions the load parsed are still pending, still paired
// with their refs. So the bus is left carrying nothing this call put there, the
// sheet is unchanged (ToData writes exactly what it wrote before), and the
// caller can attach again — to this bus or another one. Half-attached and
// reported as failed is the worst of the three states: the leak has an alibi
// and the retry is impossible.
//
// The lenient path is unchanged: it is LoadFromData's, and dropping what will
// not apply is the behaviour its callers have.
func Attach(ctx context.Context, c *Character, bus events.EventBus) error {
	if c == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "character is required")
	}
	if bus == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
	}

	previousBus := c.bus

	if err := c.SheetKeeper().Apply(ctx, bus); err != nil {
		// A failed Apply already put itself back; there is nothing else to undo.
		return err
	}

	// Drained, not read: a second Attach must not put the same conditions on a
	// second bus, and the sheet keeps them in c.conditions either way. A strict
	// failure below puts them back.
	pending := c.pendingEffects
	c.pendingEffects = nil

	// Applied ALONGSIDE the pending effects and never mixed into them. A
	// rollback must return the sheet to what Attach found, and what it found
	// did not include these — so unattach below is handed `pending`, not
	// `applying`.
	carried := c.freeReactionsToCarry()
	applying := make([]loadedEffect, 0, len(pending)+len(carried))
	applying = append(applying, pending...)
	applying = append(applying, carried...)

	attached := make([]attachedEffect, 0, len(applying))

	for _, effect := range applying {
		effectBus := dnd5eEvents.BusForEffect(bus, effect.ref)

		if err := effect.behavior.Apply(ctx, effectBus); err != nil {
			// Undo whatever the failed Apply managed to subscribe.
			_ = effect.behavior.Remove(ctx, effectBus)

			if c.policy == strictEffects {
				c.unattach(ctx, bus, attached, pending, previousBus)

				return rpgerr.Wrapf(err, "failed to apply condition %s", effect.ref.String())
			}

			// Lenient: the legacy path drops a condition it could not apply.
			warnDropped(c.id, "condition", effect.ref, err, slog.String("phase", "apply"))
			c.dropCondition(effect.behavior)

			continue
		}

		attached = append(attached, attachedEffect{effect: effect, bus: effectBus})
	}

	return nil
}

// attachedEffect is an effect this attach put on a bus, with the bus it was put
// on. Remembered rather than recomputed: asking an EffectScoper for a scope is
// how it learns an effect exists, and a rollback that asked again would leave a
// registration ledger describing an attach that did not happen.
type attachedEffect struct {
	effect loadedEffect
	bus    events.EventBus
}

// unattach returns the sheet to the state [Attach] found it in: every effect
// that went on comes back off newest first, the keeper is removed, the pending
// effects are restored whole, and the sheet gets back the bus it was holding.
func (c *Character) unattach(
	ctx context.Context,
	bus events.EventBus,
	attached []attachedEffect,
	pending []loadedEffect,
	previousBus events.EventBus,
) {
	// Newest first, the order resolution tears down in (ADR-0038 R5).
	for i := len(attached) - 1; i >= 0; i-- {
		_ = attached[i].effect.behavior.Remove(ctx, attached[i].bus)
	}

	_ = c.SheetKeeper().Remove(ctx, bus)

	// All of them, including the one that failed: nothing is applied now, so
	// nothing has been attached, so everything is still waiting to be.
	c.pendingEffects = pending
	c.bus = previousBus
}

// loadSheet builds the sheet both loaders hand back. Everything that reads
// [Data] and writes the character lives here; everything that touches a bus
// lives in [Attach] and [SheetKeeper]. The split is the point of this file:
// what a character *is* no longer depends on there being a bus to load it onto.
func loadSheet(d *Data, policy effectPolicy) (*Character, error) {
	char := &Character{
		id:                  d.ID,
		playerID:            d.PlayerID,
		name:                d.Name,
		level:               d.Level,
		proficiencyBonus:    d.ProficiencyBonus,
		raceID:              d.RaceID,
		subraceID:           d.SubraceID,
		classID:             d.ClassID,
		subclassID:          d.SubclassID,
		abilityScores:       d.AbilityScores,
		hitPoints:           d.HitPoints,
		maxHitPoints:        d.MaxHitPoints,
		armorClass:          d.ArmorClass,
		deathSaveState:      d.DeathSaveState,
		skills:              d.Skills,
		savingThrows:        d.SavingThrows,
		languages:           d.Languages,
		armorProficiencies:  d.ArmorProficiencies,
		weaponProficiencies: d.WeaponProficiencies,
		toolProficiencies:   d.ToolProficiencies,
		equipmentSlots:      d.EquipmentSlots,
		// Round-trip fix (#659): SpellSlots and ClassResources are written
		// by ToData (character.go ~954-957) via maps.Clone, but were not
		// being read back here. A finalized character round-tripping through
		// Data lost its spell slots and class resources, breaking any
		// consumer that gates on them — most visibly Wave 2.11d's
		// applyReactionConditions.hasFirstLevelSpellSlot check in rpg-api
		// that decides whether to Apply()  the Shield reaction.
		// maps.Clone(nil) safely returns nil; consumers (hasFirstLevelSpellSlot,
		// etc.) already handle the nil-map case.
		spellSlots:      maps.Clone(d.SpellSlots),
		classResources:  maps.Clone(d.ClassResources),
		subscriptionIDs: make([]string, 0),
		policy:          policy,
	}

	// Deep-copy action economy state to avoid aliasing mutable Granted map.
	// Granted is tagged json:"granted,omitempty", so a freshly-StartTurn-seeded
	// EMPTY map is omitted from the serialized JSON and comes back nil after a
	// round-trip. fromToolkitActionEconomy writes into Granted unconditionally,
	// so a nil map there panics ("assignment to entry in nil map") on the next
	// ability activation. Always re-init: clone when non-nil, else make a fresh
	// empty map so the loaded economy is immediately writable (#706).
	if d.ActionEconomy != nil {
		aeCopy := *d.ActionEconomy
		if d.ActionEconomy.Granted != nil {
			aeCopy.Granted = maps.Clone(d.ActionEconomy.Granted)
		} else {
			aeCopy.Granted = make(map[GrantedActionKey]int)
		}
		char.actionEconomy = &aeCopy
	}

	// Get hit dice from class data
	if classData := classes.GetData(d.ClassID); classData != nil {
		char.hitDice = classData.HitDice
	}

	inventory, err := loadInventory(d.Inventory, char.id, policy)
	if err != nil {
		return nil, err
	}
	char.inventory = inventory

	loadedFeatures, err := loadFeatures(d.Features, char.id, policy)
	if err != nil {
		return nil, err
	}
	char.features = loadedFeatures

	effects, err := loadEffects(d.Conditions, char.id, policy)
	if err != nil {
		return nil, err
	}
	char.pendingEffects = effects
	char.conditions = make([]dnd5eEvents.ConditionBehavior, 0, len(effects))
	for _, effect := range effects {
		char.conditions = append(char.conditions, effect.behavior)
	}

	loadedResources, err := loadResources(d.Resources, char.id, policy)
	if err != nil {
		return nil, err
	}
	char.resources = loadedResources

	// Re-register standard combat abilities (not persisted, always available)
	initStandardCombatAbilities(char)

	return char, nil
}

// loadInventory validates persisted quantities and resolves item IDs against
// the equipment catalog. A nonpositive quantity or an ID the catalog does not
// know is a lost item: ToData writes back only what resolved, so the lenient
// path's skip is how an item disappears from a character between two saves.
func loadInventory(items []InventoryItemData, characterID string, policy effectPolicy) ([]InventoryItem, error) {
	inventory := make([]InventoryItem, 0, len(items))

	for i, itemData := range items {
		if itemData.Quantity <= 0 {
			err := rpgerr.NewfWithOpts(rpgerr.CodeInvalidArgument, []rpgerr.Option{
				rpgerr.WithMeta("item_id", itemData.ID),
				rpgerr.WithMeta("index", i),
				rpgerr.WithMeta("quantity", itemData.Quantity),
			}, "inventory item %d (%q) has nonpositive persisted quantity %d", i, itemData.ID, itemData.Quantity)
			if policy == strictEffects {
				return nil, err
			}

			warnDropped(characterID, "inventory item", core.Ref{}, err,
				slog.Int("index", i),
				slog.String("item", itemData.ID),
				slog.Int("quantity", itemData.Quantity))

			continue
		}

		equip, err := equipment.GetByID(itemData.ID)
		if err != nil {
			if policy == strictEffects {
				return nil, rpgerr.Wrapf(err, "inventory item %d (%q) is not in the equipment catalog", i, itemData.ID)
			}

			warnDropped(characterID, "inventory item", core.Ref{}, err,
				slog.Int("index", i), slog.String("item", itemData.ID))

			continue
		}

		inventory = append(inventory, InventoryItem{
			Equipment: equip,
			Quantity:  itemData.Quantity,
		})
	}

	return inventory, nil
}

// loadFeatures reconstitutes persisted features by routing each blob's ref to
// its loader. A blob from another module has no loader here; the lenient path
// skips it silently, and skipping it is what drops it from the next ToData.
func loadFeatures(raw []json.RawMessage, characterID string, policy effectPolicy) ([]features.Feature, error) {
	loaded := make([]features.Feature, 0, len(raw))

	for i, rawFeature := range raw {
		// Peek at the ref to check module
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		if err := json.Unmarshal(rawFeature, &peek); err != nil {
			if policy == strictEffects {
				return nil, rpgerr.Wrapf(err, "failed to read the ref of feature %d: %s", i, rawFeature)
			}

			// No ref to report: the blob would not parse far enough to have one.
			warnDropped(characterID, "feature", core.Ref{}, err, slog.Int("index", i))

			continue
		}

		// Only dnd5e features have a loader here; another module's feature
		// would need a module registry that does not exist yet.
		if peek.Ref.Module != refs.Module {
			if policy == strictEffects {
				return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
					"feature %d has no loader here: module %q, blob %s", i, peek.Ref.Module, rawFeature)
			}

			// Not silently, any more. This build owns no loader for another
			// module's content; the day a module registry exists it routes here.
			warnDropped(characterID, "feature", peek.Ref, errNoModuleLoader,
				slog.Int("index", i), slog.String("module", string(peek.Ref.Module)))

			continue
		}

		feature, err := features.LoadJSON(rawFeature)
		if err != nil {
			if policy == strictEffects {
				return nil, rpgerr.Wrapf(err, "failed to load feature %d: %s", i, rawFeature)
			}

			warnDropped(characterID, "feature", peek.Ref, err, slog.Int("index", i))

			continue
		}

		loaded = append(loaded, feature)
	}

	return loaded, nil
}

// loadEffects reconstitutes persisted conditions, keeping each behaviour with
// the ref its loader routed on. Nothing is applied here — that is [Attach]'s
// job, and the ref is what lets it attribute the subscriptions each condition
// then makes.
func loadEffects(raw []json.RawMessage, characterID string, policy effectPolicy) ([]loadedEffect, error) {
	effects := make([]loadedEffect, 0, len(raw))

	for i, rawCondition := range raw {
		condition, err := conditions.LoadJSON(rawCondition)
		if err != nil {
			if policy == strictEffects {
				return nil, rpgerr.Wrapf(err, "failed to load condition %d: %s", i, rawCondition)
			}

			// The ref is peeked rather than assumed, and comes back zero when
			// the blob is too broken to carry one — which warnDropped then
			// omits rather than fabricating.
			warnDropped(characterID, "condition", peekEffectRef(rawCondition), err, slog.Int("index", i))

			continue
		}

		effects = append(effects, loadedEffect{
			// Peeked rather than asked for: the ref conditions.LoadJSON just
			// routed on is the only name this behaviour will ever have.
			ref:      peekEffectRef(rawCondition),
			behavior: condition,
		})
	}

	return effects, nil
}

// loadResources rebuilds the character's recoverable resources at their
// persisted values. Strict loads reject malformed bounds before construction;
// lenient loads drop those entries rather than allowing a constructor or failed
// Use call to normalize them into valid-looking counts. Valid resources stay
// inert: Character.LongRest and Character.ShortRest recover these
// character-owned pools directly rather than subscribing them to RestTopic.
func loadResources(
	persisted map[coreResources.ResourceKey]RecoverableResourceData,
	characterID string,
	policy effectPolicy,
) (map[coreResources.ResourceKey]*combat.RecoverableResource, error) {
	loaded := make(map[coreResources.ResourceKey]*combat.RecoverableResource, len(persisted))

	for key, resData := range persisted {
		if err := validatePersistedResource(key, resData); err != nil {
			if policy == strictEffects {
				return nil, err
			}
			// The lenient loader keeps its forgiving policy by dropping the
			// malformed entry. It must not feed bad bounds to constructors whose
			// clamping/failed Use path would turn them into valid-looking counts.
			warnDropped(characterID, "resource", core.Ref{}, err, slog.String("resource", string(key)))

			continue
		}

		resource := combat.NewRecoverableResource(combat.RecoverableResourceConfig{
			ID:          string(key),
			Maximum:     resData.Maximum,
			CharacterID: characterID,
			ResetType:   resData.ResetType,
		})

		if resData.Current != resData.Maximum {
			deficit := resData.Maximum - resData.Current
			if err := resource.Use(deficit); err != nil {
				// Bounds were validated above, so reaching this is an internal
				// disagreement between validation and the resource primitive.
				return nil, rpgerr.Wrapf(err, "failed to restore resource %s", key)
			}
		}

		loaded[key] = resource
	}

	return loaded, nil
}

func validatePersistedResource(
	key coreResources.ResourceKey, data RecoverableResourceData,
) error {
	if data.Current < 0 || data.Maximum < 0 {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"resource %s has negative persisted bounds: current=%d maximum=%d",
			key, data.Current, data.Maximum)
	}
	if data.Current > data.Maximum {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"resource %s persisted current %d exceeds maximum %d",
			key, data.Current, data.Maximum)
	}
	return nil
}

// peekEffectRef reads the ref a persisted effect routes on, which is the same
// field its loader routes on. It returns the zero Ref for a blob that has none
// rather than an error: an effect that loaded is applied either way, and the
// only thing a missing ref costs is attribution.
func peekEffectRef(raw json.RawMessage) core.Ref {
	var peek struct {
		Ref core.Ref `json:"ref"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		return core.Ref{}
	}

	return peek.Ref
}

// freeReactionsToCarry names the reactions every combatant has, the
// way a class grant gives it a class feature — as part of what this creature
// IS, rather than seated by whatever happens to be running.
//
// Kirk ruled the shape 2026-08-28 (rpg-project#316): "when we initialize
// monsters they should have the condition on them like a character grant... and
// only dirty if they fire the OA."
//
// # Why not a real Grant.Conditions entry
//
// Because an opportunity attack is not a class feature. Every melee combatant
// has one, monsters included, which is why it is correctly absent from all
// twelve grant lists — and a grant is applied by Draft.Finalize at CREATION, so
// every character made before today would have been permanently unable to take
// one. Added here instead, it reaches every sheet on its next load with no
// backfill and no migration.
//
// # Why it must not mark the sheet dirty
//
// Because gaining it changed nothing a player did. resolution states the
// invariant in its own test — "a participant nothing happened to must not be
// written back (R3 says pass everyone in; it does not say charge for
// everyone)" — and an earlier draft of this slice seated the condition through
// ConditionAppliedEvent, which marks dirty unconditionally, and failed 21 tests
// across six suites for exactly that reason. Appending to the pending effects
// is the load path, and the load path is silent by construction.
//
// The condition still marks the sheet dirty when it SPENDS its meter, which is
// the only moment anything worth persisting has happened.
func (c *Character) freeReactionsToCarry() []loadedEffect {
	var carried []loadedEffect
	for _, ref := range freeReactionRefs {
		if carriesRef(c.conditions, ref) {
			continue
		}
		carried = append(carried, loadedEffect{
			ref:      *ref,
			behavior: conditions.NewOpportunityAttackCondition(c.id),
		})
	}

	return carried
}

// freeReactionRefs are the reactions a combatant carries by existing. ONE
// ENTRY, and the list is the rule rather than an optimisation of it: a COSTED
// reaction (Shield burns a spell slot, Uncanny Dodge burns a class feature) is
// not had by existing and does not belong here.
var freeReactionRefs = []*core.Ref{
	refs.Conditions.OpportunityAttack(),
}

// carriesRef reports whether the sheet already holds this ref — asked of
// c.conditions rather than of the pending list, because pendingEffects is
// DRAINED by Attach and a second Attach would otherwise stack a second copy on
// top of the one the first put there.
func carriesRef(carried []dnd5eEvents.ConditionBehavior, ref *core.Ref) bool {
	for _, condition := range carried {
		if got := condition.Ref(); got != nil && got.String() == ref.String() {
			return true
		}
	}

	return false
}

// errNoModuleLoader names the one lenient drop that is not a failure to read
// anything — the blob parsed, and this build simply owns no loader for the
// module it names.
var errNoModuleLoader = errors.New("no loader here for another module's content")

// warnDropped is the one place a lenient load says out loud what it threw away.
//
// A lenient load exists so a character carrying a blob this build cannot read
// still enters play: refusing would put one unreadable condition between a
// player and the game. But dropping in SILENCE is how a monk fought at base AC
// for the life of a character with nobody able to see why, so the drop is loud.
// Lenient must not mean invisible.
//
// A warning log is the whole of it, deliberately. Getting this data out
// cleanly — a report on the loader, riding an entry's output, reaching the
// combat log, carrying an error code when this is a real game — is a NAMED
// SHELF in the game-context design, carved when the structure exists rather
// than guessed at now. Until then these lines mark every site that shelf will
// serve, which is worth more than a shape invented early.
//
// THE REF IS OMITTED when the blob would not parse far enough to have one, and
// its absence is the fact: a blob too broken to name itself is a different and
// worse thing than one naming something unknown. A placeholder would read like
// a ref that exists.
//
// This is the toolkit's first deliberate log line. slog to the default logger
// is the standard library's own convention and adds no API, no config field and
// no seam to unpick later; a host that wants these somewhere else calls
// slog.SetDefault. If a different norm is wanted, this is the place it changes.
func warnDropped(characterID, kind string, ref core.Ref, reason error, extra ...slog.Attr) {
	attrs := []any{
		slog.String("character", characterID),
		slog.String("dropped", kind),
		slog.Any("reason", reason),
	}
	if ref != (core.Ref{}) {
		attrs = append(attrs, slog.String("ref", ref.String()))
	}
	for _, attr := range extra {
		attrs = append(attrs, attr)
	}

	slog.Warn("dnd5e/character: lenient load dropped a persisted entry", attrs...)
}
