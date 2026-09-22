package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type sightAreaTransition struct {
	payload  []byte
	audience []MemberID
}

// QueueSightAreaTransitions captures membership changes caused by replacing
// areas. The labels are supplied by the rulebook; this package interprets only
// geometry. Nothing is appended until the causal outcome has been recorded.
func (e *Encounter) QueueSightAreaTransitions(before []SightAreaData) error {
	if err := validateSightAreasData(before); err != nil {
		return err
	}
	old := sightAreasFromData(before)
	now := e.sightAreas
	ids := make([]string, 0, len(old)+len(now))
	for id := range old {
		ids = append(ids, id)
	}
	for id := range now {
		if _, ok := old[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	members, err := e.Members()
	if err != nil {
		return err
	}
	pending := make([]sightAreaTransition, 0)
	for _, id := range ids {
		oa, hadOld := old[id]
		na, hasNew := now[id]
		for _, m := range members {
			was := hadOld && oa.MembershipRef != "" && sightAreaIncludes(oa, m.Position, e.canvas.GetGrid())
			is := hasNew && na.MembershipRef != "" && sightAreaIncludes(na, m.Position, e.canvas.GetGrid())
			same := oa.MembershipRef == na.MembershipRef && oa.MembershipSourceID == na.MembershipSourceID
			if was && (!is || !same) {
				reason := "left area"
				if !hasNew || !same {
					reason = "area ended"
				}
				transition, err := e.makeSightAreaTransition(m.ID, oa, false, reason, m.Position, m.Position, old, now)
				if err != nil {
					return err
				}
				pending = append(pending, transition)
			}
			if is && (!was || !same) {
				transition, err := e.makeSightAreaTransition(m.ID, na, true, "", m.Position, m.Position, old, now)
				if err != nil {
					return err
				}
				pending = append(pending, transition)
			}
		}
	}
	e.pendingSightAreaTransitions = append(e.pendingSightAreaTransitions, pending...)
	return nil
}

func sightAreaIncludes(area SightArea, point spatial.Position, grid spatial.Grid) bool {
	return SightAreaContains(SightAreaData{Center: PositionData{X: area.Center.X, Y: area.Center.Y}, RadiusFeet: area.RadiusFeet}, point, grid)
}

// Movement calls this once per executed step, after its moved beat. Comparing
// endpoints avoids a second persisted membership truth and records even a walk
// that enters and leaves the same area before the host saves.
func (e *Encounter) appendSightAreaMovementTransitions(member MemberID, from, to spatial.Position) error {
	ids := make([]string, 0, len(e.sightAreas))
	for id := range e.sightAreas {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	transitions := make([]sightAreaTransition, 0)
	for _, id := range ids {
		area := e.sightAreas[id]
		if area.MembershipRef == "" {
			continue
		}
		was := sightAreaIncludes(area, from, e.canvas.GetGrid())
		is := sightAreaIncludes(area, to, e.canvas.GetGrid())
		if was == is {
			continue
		}
		reason := ""
		if !is {
			reason = "left area"
		}
		transition, err := e.makeSightAreaTransition(member, area, is, reason, from, to, e.sightAreas, e.sightAreas)
		if err != nil {
			return err
		}
		transitions = append(transitions, transition)
	}
	for _, transition := range transitions {
		if err := e.appendSightAreaTransition(transition); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encounter) makeSightAreaTransition(member MemberID, area SightArea, entered bool, reason string, from, to spatial.Position, before, after map[string]SightArea) (sightAreaTransition, error) {
	kind := ResultConditionRemoved
	if entered {
		kind = ResultConditionApplied
	}
	result, err := e.prepareActivationResult("area membership", 0, ActivationResult{
		Kind: kind, Address: &ConditionAddress{MemberID: member, ConditionRef: area.MembershipRef, SourceID: area.MembershipSourceID}, Name: area.MembershipName, Reason: reason,
	})
	if err != nil {
		return sightAreaTransition{}, err
	}
	payload, err := json.Marshal(activationResultPayload{Beat: "activation-result", Actor: member, Result: result})
	if err != nil {
		return sightAreaTransition{}, err
	}
	ranges, err := e.sightNow()
	if err != nil {
		return sightAreaTransition{}, err
	}
	positions := make(map[MemberID]spatial.Position, len(e.members))
	for _, id := range e.rosterIDs() {
		if p, ok := e.canvas.GetEntityPosition(string(id)); ok {
			positions[id] = p
		}
	}
	audience := make([]MemberID, 0)
	for _, observer := range e.rosterIDs() {
		if observer == member {
			audience = append(audience, observer)
			continue
		}
		positions[member] = from
		beforeReach := sightReach{positions: positions, cells: ranges, canvas: e.canvas, areas: before}
		visibleBefore := beforeReach.Reaches("sight", observer, member)
		positions[member] = to
		afterReach := sightReach{positions: positions, cells: ranges, canvas: e.canvas, areas: after}
		if visibleBefore || afterReach.Reaches("sight", observer, member) {
			audience = append(audience, observer)
		}
	}
	return sightAreaTransition{payload: payload, audience: audience}, nil
}

func (e *Encounter) appendSightAreaTransition(transition sightAreaTransition) error {
	_, err := e.appendBeat(&record.AppendInput{At: uint64(e.clock.ToData().HighWater), Audience: transition.audience, Tags: map[string]string{"tag": "outcome"}, Payload: transition.payload})
	return err
}

// FlushSightAreaTransitions appends captured results without activating an
// ability, charging an action, or running a participation pass.
func (e *Encounter) FlushSightAreaTransitions() error {
	for len(e.pendingSightAreaTransitions) > 0 {
		if err := e.appendSightAreaTransition(e.pendingSightAreaTransitions[0]); err != nil {
			return fmt.Errorf("area membership: %w", err)
		}
		e.pendingSightAreaTransitions = e.pendingSightAreaTransitions[1:]
	}
	return nil
}
