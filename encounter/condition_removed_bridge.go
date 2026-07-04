package encounter

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// subscribeConditionRemovedBridge installs the encounter's ONE permanent
// subscription to the rulebook-level dnd5eEvents.ConditionRemovedTopic on
// e.bus (rpg-toolkit#734).
//
// Unlike every other bridge in this package (subscribeAttacks,
// subscribeConditions in npc.go; installTriggerBuffer in combat_phased.go;
// the capture-window applyActivatedConditions itself relies on), those are
// all TRANSIENT capture windows: install a temporary subscriber right before
// one specific verb call, collect whatever fires synchronously during that
// call, then unsubscribe. That pattern does not work here — a condition can
// self-remove at ANY future turn start (seedActorTurn, unrelated to whoever
// originally applied it), or on a healing action targeting a downed ally (a
// completely different verb call, possibly turns later). There is no single
// verb call to wrap a transient subscriber around, so this subscription must
// outlive any one verb call. It is installed once, right after e.bus is
// constructed, and lives for the Encounter's lifetime — called from both New
// and LoadFromData so there is exactly one implementation of the
// subscribe+handler logic. This is the first permanent (non-transient-capture)
// bus subscription in the encounter package.
//
// Subscribe on a freshly constructed dnd5e EventBus never errors (see
// events/bus.go's simpleEventBus.Subscribe, which unconditionally returns a
// nil error) — the error is intentionally discarded here because New() has
// no error return to propagate it through, and LoadFromData already fails a
// caller on the (unrelated) hydration error path, not this one.
func (e *Encounter) subscribeConditionRemovedBridge(ctx context.Context) {
	_, _ = dnd5eEvents.ConditionRemovedTopic.On(e.bus).Subscribe(ctx,
		func(_ context.Context, evt dnd5eEvents.ConditionRemovedEvent) error {
			return e.bridgeConditionRemoved(evt)
		})
}

// bridgeConditionRemoved translates one rulebook-level
// dnd5eEvents.ConditionRemovedEvent into a broker-facing
// events.ConditionRemovedEvent (StatusRemoved), mirroring
// applyActivatedConditions' audience-projection template
// (activate_feature.go) exactly: LoS from each viewer's position to the
// TARGET's position — never the causer's, since removal (like application)
// is something that happens TO the target.
//
// Format consistency (rpg-toolkit#734): the rulebook event's ConditionRef
// arrives as a fully namespaced ref string (e.g. "dnd5e:conditions:dodging"
// — see conditions/dodging.go's onTurnStart), but the broker
// ConditionAppliedEvent this pairs with on the wire carries the short
// ConditionType keyword ("dodging" — activate_feature.go's
// `condRef := string(cond.Type)`). Without normalizing, StatusApplied and
// StatusRemoved for the SAME condition would carry the ref in two different
// string formats and a client matching them up by ref would silently fail to
// correlate a removal with its earlier applied event.
// normalizeConditionRef parses the full ref and keeps just the ID segment so
// both sides match.
func (e *Encounter) bridgeConditionRemoved(evt dnd5eEvents.ConditionRemovedEvent) error {
	targetID := encountercore.EntityID(evt.CharacterID)
	condRef := normalizeConditionRef(evt.ConditionRef)

	targetPos, haveTargetPos := e.findEntityPosition(targetID)

	perPlayer := make(map[encountercore.PlayerID]events.ConditionRemovedSlice)
	for viewerID, viewer := range e.data.Players {
		if !haveTargetPos || !e.viewerCanSee(viewer, targetPos) {
			continue
		}
		perPlayer[viewerID] = events.ConditionRemovedSlice{Visible: true}
	}

	if err := e.broker.Publish(events.NewConditionRemovedEvent(
		e.data.ID, e.nextSeq(), targetID, condRef, evt.Reason, perPlayer,
	)); err != nil {
		return fmt.Errorf("publish condition removed: %w", err)
	}
	return nil
}

// normalizeConditionRef reduces a fully namespaced condition ref string
// (e.g. "dnd5e:conditions:dodging") to just its ID segment ("dodging") — the
// same short keyword form ConditionAppliedEvent.ConditionRef already carries
// on the wire (see activate_feature.go's applyActivatedConditions). Falls
// back to the raw input if it does not parse as a namespaced ref, so an
// already-short or unexpected value is still published rather than dropped.
func normalizeConditionRef(ref string) string {
	parsed, err := core.ParseString(ref)
	if err != nil {
		return ref
	}
	return parsed.ID
}
