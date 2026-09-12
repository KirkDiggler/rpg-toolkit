// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// CommandedName is what a player is shown wherever this condition appears.
const CommandedName = "Commanded"

// CommandedConditionData is the serializable form of the commanded condition,
// stored by the game server as an opaque JSON blob.
type CommandedConditionData struct {
	Ref          *core.Ref `json:"ref"`
	MemberID     string    `json:"member_id"`
	SourceRef    string    `json:"source_ref"`
	CasterID     string    `json:"caster_id"`
	Word         string    `json:"word"`
	TurnEndsLeft int       `json:"turn_ends_left"`
}

// CommandedCondition is the compulsion Command leaves on a creature that failed
// its save: one word, and the caster that word is measured from.
//
// # It contributes to nothing, and that is the design
//
// Every other condition in this rulebook is read by a roll: rage folds a
// multiplier into damage, Bane subtracts a die, Blade Ward halves a swing. This
// one subscribes to no chain and answers no question a roll asks. Its whole
// readership is the turn: the compelled creature's next turn is driven by the
// engine rather than by whoever usually decides, and the word says how.
//
// The rule that reads it is not here for the reason no rule about turns is here
// — turns belong to the encounter, which is the layer allowed an opinion about
// whose turn it is. What this condition owes that layer is the two facts it
// cannot derive: WHO said the word, because Approach and Flee measure from the
// caster, and WHICH word, because that is the whole of what the spell chose.
//
// # One turn end, counted on the OWNER
//
// "Until the end of the target's next turn" needs no new duration mechanism.
// The condition lands on somebody whose turn has not begun, so the first turn
// end it will ever see with its own SubjectID is the end of that next turn.
// One count is the whole of it — the opposite lean from
// [BladeWardCondition], which is traced during the caster's own turn and needs
// two counts to survive to a swing.
//
// A creature downed before its turn is auto-passed by life state, which still
// announces a turn end, so the compulsion expires unused rather than waiting on
// a turn that will not come.
// # Its fields are unexported, unlike its neighbours here
//
// Every other condition in this package exposes its state, and for most of them
// a half-filled literal is merely wrong. This one has three fields the engine
// cannot derive and will not default — the anchor, the word, and the clock —
// so [NewCommandedCondition] refuses each one that is missing. Exported fields
// would put a second door beside that refusal, through which a compulsion with
// no anchor reaches a driver that would walk a creature at nobody.
type CommandedCondition struct {
	// memberID is the commanded creature.
	memberID string

	// sourceRef is what imposed this, as a ref string — the spell, not the
	// caster.
	sourceRef string

	// casterID is who said the word. The anchor Approach walks toward and
	// Flee walks away from, so it is the creature rather than the spell.
	casterID string

	// word is which of the spell's options was chosen, as the content's own
	// id. Opaque here: this condition does not know the menu, because the
	// menu belongs to the profile that offered it and a second spell with a
	// word of its own must not have to be added here to be obeyed.
	word string

	// turnEndsLeft is how many of the commanded creature's turn ends remain.
	// Counted here rather than derived from a round, for the reason
	// [BladeWardCondition.TurnEndsLeft] is: a TurnEndEvent may carry no round
	// at all, and Round 0 means "unknown" rather than "round zero".
	turnEndsLeft int

	bus             events.EventBus
	subscriptionIDs []string
}

// Ensure CommandedCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*CommandedCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (c *CommandedCondition) Ref() *core.Ref { return refs.Conditions.Commanded() }

// CasterID returns the creature the word is measured from: the anchor Approach
// walks toward and Flee walks away from.
func (c *CommandedCondition) CasterID() string { return c.casterID }

// Word returns which of the spell's options was chosen, as the content's own
// id. The layer that drives the compelled turn is what reads it.
func (c *CommandedCondition) Word() string { return c.word }

// MemberID returns the commanded creature.
func (c *CommandedCondition) MemberID() string { return c.memberID }

// NewCommandedCondition creates the compulsion on one creature for a given
// number of its own turn ends.
//
// Each field is REFUSED rather than defaulted when missing. All three are read
// by the layer that drives the compelled turn, and a default here would be a
// ruling in a constructor: no anchor means a creature walked at nobody, no word
// means a turn the engine would have to pick a rule for, and a spent clock
// means a compulsion that expired before it was applied.
//
// An empty sourceRef falls back to the Command spell, which is the only thing
// in this rulebook that imposes this condition.
func NewCommandedCondition(memberID, sourceRef, casterID, word string, turnEnds int) (*CommandedCondition, error) {
	if casterID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"commanded condition requires the caster id the word is measured from")
	}
	if word == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "commanded condition requires the word that was said")
	}
	if turnEnds <= 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument,
			"commanded condition must last at least one turn end")
	}
	if sourceRef == "" {
		sourceRef = refs.Spells.Command().String()
	}
	return &CommandedCondition{
		memberID:     memberID,
		sourceRef:    sourceRef,
		casterID:     casterID,
		word:         word,
		turnEndsLeft: turnEnds,
	}, nil
}

// IsApplied returns true if this condition is currently applied.
func (c *CommandedCondition) IsApplied() bool { return c.bus != nil }

// Apply subscribes the compulsion to the commanded creature's turn ends and to
// the end of the fight. It joins no damage chain and no roll: nothing here
// changes a number.
func (c *CommandedCondition) Apply(ctx context.Context, bus events.EventBus) error {
	if c.IsApplied() {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "commanded condition already applied")
	}
	c.bus = bus

	turnEnds := dnd5eEvents.TurnEndTopic.On(bus)
	turnSub, err := turnEnds.Subscribe(ctx, c.onTurnEnd)
	if err != nil {
		c.bus = nil
		return rpgerr.Wrap(err, "failed to subscribe to turn end topic")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, turnSub)

	combatEnds := dnd5eEvents.CombatEndTopic.On(bus)
	combatSub, err := combatEnds.Subscribe(ctx, c.onCombatEnd)
	if err != nil {
		_ = c.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to combat end topic")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, combatSub)

	// A compulsion that lasts one turn end should never SEE a long rest, and
	// that is the argument for subscribing rather than against it: the two
	// clocks that normally take it are a turn end and the end of a fight, and
	// a sheet that reaches a rest still carrying one has had neither. Every
	// other spell-delivered condition in this package ends there for the same
	// reason.
	restSub, err := subscribeRemoveOnLongRest(ctx, bus, subscribeRemoveOnLongRestInput{
		Address: ConditionAddressOf(c.memberID, c), Remove: c.Remove,
	})
	if err != nil {
		_ = c.Remove(ctx, bus)
		return rpgerr.Wrap(err, "failed to subscribe to long rest")
	}
	c.subscriptionIDs = append(c.subscriptionIDs, restSub)

	return nil
}

// Remove unsubscribes this condition from all events.
func (c *CommandedCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if c.bus == nil {
		return nil
	}

	total := len(c.subscriptionIDs)
	var errs []error
	for _, subID := range c.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	c.subscriptionIDs = nil
	c.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// onTurnEnd spends one of the commanded creature's own turn ends and ends the
// compulsion when the count runs out. Other members' turns do not own this
// clock — the caster ending their own turn must not take back the word.
func (c *CommandedCondition) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	if event.SubjectID != c.memberID {
		return nil
	}

	c.turnEndsLeft--
	if c.turnEndsLeft > 0 {
		return nil
	}
	return c.end(ctx, "expired")
}

// onCombatEnd ends the compulsion with the fight.
func (c *CommandedCondition) onCombatEnd(ctx context.Context, event dnd5eEvents.CombatEndEvent) error {
	if event.SubjectID != c.memberID {
		return nil
	}
	return c.end(ctx, "combat ended")
}

// end publishes the removal and detaches. The publish comes first so the
// removal is on the bus while this condition still owns the compulsion, which
// is what an activation's effect collector reads.
func (c *CommandedCondition) end(ctx context.Context, reason string) error {
	if c.bus == nil {
		return nil
	}
	bus := c.bus
	if err := dnd5eEvents.ConditionRemovedTopic.On(bus).Publish(ctx, dnd5eEvents.ConditionRemovedEvent{
		MemberID:     c.memberID,
		ConditionRef: refs.Conditions.Commanded().String(),
		Reason:       reason,
	}); err != nil {
		return rpgerr.Wrapf(err, "failed to publish commanded removal for member %s", c.memberID)
	}
	return c.Remove(ctx, bus)
}

// ToJSON converts the condition to JSON for persistence.
func (c *CommandedCondition) ToJSON() (json.RawMessage, error) {
	return json.Marshal(CommandedConditionData{
		Ref:          refs.Conditions.Commanded(),
		MemberID:     c.memberID,
		SourceRef:    c.sourceRef,
		CasterID:     c.casterID,
		Word:         c.word,
		TurnEndsLeft: c.turnEndsLeft,
	})
}

// loadJSON loads commanded condition state from JSON.
//
// It refuses the same two missing fields [NewCommandedCondition] does. The
// loader is the other door onto this type, and a door that admitted state the
// constructor refuses would leave that refusal guarding nothing: a blob with no
// anchor loads clean, attaches clean, and only shows up as a creature walked at
// nobody. The clock is NOT re-checked, because a persisted zero is how a
// compulsion that has already been spent looks on its way to being dropped.
func (c *CommandedCondition) loadJSON(data json.RawMessage) error {
	var stored CommandedConditionData
	if err := json.Unmarshal(data, &stored); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal commanded data")
	}
	if stored.CasterID == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "stored commanded condition has no caster id")
	}
	if stored.Word == "" {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "stored commanded condition has no word")
	}
	c.memberID = stored.MemberID
	c.sourceRef = stored.SourceRef
	c.casterID = stored.CasterID
	c.word = stored.Word
	c.turnEndsLeft = stored.TurnEndsLeft
	return nil
}
