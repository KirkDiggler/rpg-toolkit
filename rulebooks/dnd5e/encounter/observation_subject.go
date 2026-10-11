// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// These qualify identity, not sensory channels. A creature, prop and door may
// share an authored name without overwriting each other's testimony. Observers
// use the member identity too, so perception's self-exclusion remains correct.
const (
	memberSubjectKind        perception.Channel = "member"
	propSubjectKind          perception.Channel = "prop"
	doorSubjectKind          perception.Channel = "door"
	perceptionSubjectVersion                    = 1
)

func sightMember(id MemberID) core.EntityID { return perception.Qualify(memberSubjectKind, id) }
func sightProp(id PropID) core.EntityID {
	return perception.Qualify(propSubjectKind, core.EntityID(id))
}
func sightDoor(id DoorID) core.EntityID {
	return perception.Qualify(doorSubjectKind, core.EntityID(id))
}

func subjectID(subject core.EntityID, kind perception.Channel) (core.EntityID, bool) {
	prefix := string(perception.Qualify(kind, ""))
	id, ok := strings.CutPrefix(string(subject), prefix)
	return core.EntityID(id), ok && id != ""
}

// memberIntel preserves the existing creature/deed read contract. Prop and door
// sightings have their own typed projections from this same perception store.
func (e *Encounter) memberIntel(observer MemberID) ([]perception.Holding, error) {
	held, err := e.intelLog.Held(sightMember(observer))
	if err != nil {
		return nil, err
	}
	out := make([]perception.Holding, 0, len(held))
	for _, h := range held {
		if h.Channel == perception.Sight {
			id, member := subjectID(h.Subject, memberSubjectKind)
			if !member {
				continue
			}
			h.Subject = id
		}
		out = append(out, h)
	}
	return out, nil
}

// memberReportStore keeps stage's actor/target vocabulary unchanged while
// binding reports to the same qualified observers used by the sight pass.
type memberReportStore struct{ encounter *Encounter }

func (s memberReportStore) Held(observer core.EntityID) ([]perception.Holding, error) {
	return s.encounter.memberIntel(observer)
}

func (s memberReportStore) Report(in perception.ReportInput) (*perception.ReportOutput, error) {
	in.Observer = sightMember(in.Observer)
	return s.encounter.intelLog.Report(in)
}

// memberPerceptionDelta keeps creature IDs on the existing public delta while
// the private store qualifies the identity of everything it observes. A first
// contact's payload is the delivered form ([deliveredSightPayload]): the delta
// is handed out, and seen conditions do not leave with it.
func memberPerceptionDelta(in *perception.Delta) (*IntelDelta, error) {
	if in == nil {
		return nil, nil
	}
	out := &IntelDelta{}
	for _, presence := range in.FirstContact {
		if id, member := subjectID(presence.ID, memberSubjectKind); member {
			payload, err := deliveredSightPayload(presence.Payload)
			if err != nil {
				return nil, fmt.Errorf("first contact with %q: %w", id, err)
			}
			out.FirstContact = append(out.FirstContact, perception.Presence{ID: id, Payload: payload})
		} else {
			out.KnowledgeChanged = true
		}
	}
	for _, pair := range []struct {
		from       []core.EntityID
		to         *[]core.EntityID
		transition bool
	}{
		{in.Refreshed, &out.Refreshed, false}, {in.Faded, &out.Faded, true},
		{in.Changed, &out.Changed, true}, {in.Reacquired, &out.Reacquired, true},
	} {
		for _, subject := range pair.from {
			if id, member := subjectID(subject, memberSubjectKind); member {
				*pair.to = append(*pair.to, id)
			} else if pair.transition {
				out.KnowledgeChanged = true
			}
		}
	}
	return out, nil
}

// requirePerceptionSubjects refuses a blob whose subject namespace is not the
// one this build writes. Version zero was the old member-only sight namespace;
// nothing upgrades it in place, because an upgrade guessed which names were
// already qualified and a guess is not testimony. A blob that carries it is
// refused with [ErrInvalidData].
func requirePerceptionSubjects(version uint32) error {
	if version != perceptionSubjectVersion {
		return fmt.Errorf("perception subject version %d, want %d: %w", version, perceptionSubjectVersion, ErrInvalidData)
	}
	return nil
}
