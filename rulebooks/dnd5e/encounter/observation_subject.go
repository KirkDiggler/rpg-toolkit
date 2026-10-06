// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"maps"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
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

// normalizePerceptionSubjects upgrades the old member-only sight namespace
// without observing the live world. Payloads, stamps and currency are unchanged.
// An explicit version avoids guessing whether an old member name was qualified.
// intel types here are its admitted persistence shape, not a second store.
func normalizePerceptionSubjects(data perception.Data, version uint32) (perception.Data, error) {
	if version == perceptionSubjectVersion {
		return data, nil
	}
	if version != 0 {
		return perception.Data{}, fmt.Errorf("perception subject version %d: %w", version, ErrInvalidData)
	}
	out := data
	out.Intel.Holdings = maps.Clone(data.Intel.Holdings)
	clear(out.Intel.Holdings)
	for observer, holdings := range data.Intel.Holdings {
		qualified := maps.Clone(holdings)
		clear(qualified)
		for subject, holding := range holdings {
			id := subject
			if perception.Channel(holding.Channel) == perception.Sight {
				id = intel.Subject(sightMember(core.EntityID(subject)))
			}
			qualified[id] = holding
		}
		out.Intel.Holdings[sightMember(observer)] = qualified
	}
	return out, nil
}
