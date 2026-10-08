package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// Who is in a runtime area is this module's answer, asked at the moment it
// matters and never stored (rpg-project#539, "Who is in an area"). No sheet,
// condition or pass holds a copy, so there is nothing to reconcile: a member
// is in an area exactly when [Encounter.inSightArea] says so.
//
// ONE MEMBERSHIP FUNCTION, TWO CALLERS. A step asks it of the cell a member
// left and the cell it reached; an area change asks it of every member's
// placement ([Encounter.placementOf], through [Encounter.Members]) against the
// area set before and after. Both hand the answers to one diff,
// [Encounter.sightAreaTransitions], so a step and an area change cannot
// disagree about who entered, who left and who saw the area end.
//
// The labels are content's: the area carries its MembershipRef, Name and
// SourceID as the spell declared them, and this module interprets geometry
// only (C1).

type sightAreaTransition struct {
	payload  []byte
	audience []MemberID
}

// inSightArea is THE membership answer: whether a point lies inside a runtime
// area, by the same hex distance and feet-to-cell conversion sight reach uses.
func (e *Encounter) inSightArea(area SightArea, point spatial.Position) bool {
	return areaHolds(area, point, e.canvas.GetGrid())
}

// areaHolds is the one point-in-area test: membership, the footprint a member
// sees from inside ([Encounter.SightAreasFor]) and a sight lane's crossing
// ([areaCrosses]) all answer through it.
func areaHolds(area SightArea, point spatial.Position, grid spatial.Grid) bool {
	if area.RadiusFeet <= 0 {
		return false
	}
	return grid.Distance(area.Center, point) <= float64(area.RadiusFeet)/float64(FeetPerCell)
}

// sightAreaTransitions diffs one member's membership of every labelled area
// between two moments: standing at `from` under the `before` area set, and at
// `to` under `after`. A step passes one area set and two cells; an area change
// passes one cell and two area sets.
//
// The reason a membership ends is told from the area set, not the member: a
// member who walked out "left area"; a member whose area went away, or whose
// area's label changed under them, saw the "area ended".
func (e *Encounter) sightAreaTransitions(
	member MemberID, from, to spatial.Position, before, after map[string]SightArea,
) ([]sightAreaTransition, error) {
	ids := make([]string, 0, len(before)+len(after))
	for id := range before {
		ids = append(ids, id)
	}
	for id := range after {
		if _, ok := before[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)

	out := make([]sightAreaTransition, 0)
	for _, id := range ids {
		was, hadWas := before[id]
		now, hasNow := after[id]
		in := hadWas && was.MembershipRef != "" && e.inSightArea(was, from)
		is := hasNow && now.MembershipRef != "" && e.inSightArea(now, to)
		same := hadWas && hasNow &&
			was.MembershipRef == now.MembershipRef && was.MembershipSourceID == now.MembershipSourceID
		if in && (!is || !same) {
			reason := "left area"
			if !same {
				reason = "area ended"
			}
			transition, err := e.makeSightAreaTransition(member, was, false, reason, from, to, before, after)
			if err != nil {
				return nil, err
			}
			out = append(out, transition)
		}
		if is && (!in || !same) {
			transition, err := e.makeSightAreaTransition(member, now, true, "", from, to, before, after)
			if err != nil {
				return nil, err
			}
			out = append(out, transition)
		}
	}
	return out, nil
}

// areaChangeTransitions is an area change's half: every member, at its one
// shared placement, diffed across the area set before and after the change.
// Computed before the change is applied, so a refusal leaves both the area
// set and the story untouched.
func (e *Encounter) areaChangeTransitions(before, after map[string]SightArea) ([]sightAreaTransition, error) {
	members, err := e.Members()
	if err != nil {
		return nil, fmt.Errorf("area membership: %w", err)
	}
	out := make([]sightAreaTransition, 0)
	for _, m := range members {
		transitions, err := e.sightAreaTransitions(m.ID, m.Position, m.Position, before, after)
		if err != nil {
			return nil, fmt.Errorf("area membership: %w", err)
		}
		out = append(out, transitions...)
	}
	return out, nil
}

// appendSightAreaTransitions writes membership beats in order. A beat is
// written when its change is applied and never held in memory, so a save, a
// step or an exit can neither lose nor reorder it.
func (e *Encounter) appendSightAreaTransitions(transitions []sightAreaTransition) error {
	for _, transition := range transitions {
		if err := e.appendSightAreaTransition(transition); err != nil {
			return fmt.Errorf("area membership: %w", err)
		}
	}
	return nil
}

// appendStepAreaTransitions is a step's half, called once per executed step
// after its moved beat. Comparing the two cells of each step, under one area
// set, records even a walk that enters and leaves an area before the host
// saves.
func (e *Encounter) appendStepAreaTransitions(member MemberID, from, to spatial.Position) error {
	transitions, err := e.sightAreaTransitions(member, from, to, e.sightAreas, e.sightAreas)
	if err != nil {
		return err
	}
	return e.appendSightAreaTransitions(transitions)
}

func (e *Encounter) makeSightAreaTransition(member MemberID, area SightArea, entered bool, reason string, from, to spatial.Position, before, after map[string]SightArea) (sightAreaTransition, error) {
	kind := ResultConditionRemoved
	if entered {
		kind = ResultConditionApplied
	}
	result, err := e.prepareActivationResult("area membership", 0, ActivationResult{
		Kind: kind, Address: &ConditionAddress{MemberID: member, ConditionKey: ConditionKey{ConditionRef: area.MembershipRef, SourceID: area.MembershipSourceID}}, Name: area.MembershipName, Reason: reason,
	})
	if err != nil {
		return sightAreaTransition{}, err
	}
	payload, err := json.Marshal(activationResultPayload{Beat: BeatActivationResult, Actor: member, Result: result})
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
