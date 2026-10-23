// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// heading.go is THE WAY AN OBSERVER SAW A SUBJECT GO (rpg-project#679 R10):
// [SightTestimony.Heading], derived per observer from that observer's own
// previous testimony, because testimony is per observer (#673 R8) and a step
// nobody watched is not a thing anybody saw.
//
// ONE PAYLOAD PER SUBJECT STILL, WHEREVER IT CAN BE (rpg-toolkit#1691). A
// sighting's bytes are the subject's own facts plus, now, a heading that can
// differ by observer. So observers that would land the same headings share one
// perception pass — on a turn where nobody moved, or where every watcher saw
// the same step, that is the one pass and the one encode per member it always
// was — and only an observer whose history differs gets a pass of its own,
// with only the differing subjects encoded again.

// headingPass is one perception pass of a refresh: the observers it lands on
// and the presences they see.
type headingPass struct {
	observers []core.EntityID
	presences []perception.Presence
}

// headingPasses splits one refresh's observers by the headings they would
// write, and builds each group's presences: the shared base payload for a
// subject with no heading, and that subject's testimony re-encoded with the
// heading otherwise (each distinct (subject, heading) encoded once).
//
// presences is the base pass, in which every placed member at seenAs[id] was
// encoded with no heading; positions is where each placed member stands now.
func (e *Encounter) headingPasses(
	observers []core.EntityID, presences []perception.Presence,
	positions map[MemberID]spatial.Position, seenAs map[MemberID]int,
) ([]headingPass, error) {
	type variant struct {
		subject MemberID
		heading float64
	}
	encoded := map[variant][]byte{}
	groups := map[string]*headingPass{}
	var order []string
	for _, observer := range observers {
		headings, err := e.headingsSeenBy(observer, positions)
		if err != nil {
			return nil, err
		}
		subjects := make([]MemberID, 0, len(headings))
		for id := range headings {
			subjects = append(subjects, id)
		}
		sort.Slice(subjects, func(i, j int) bool { return subjects[i] < subjects[j] })
		var key strings.Builder
		for _, id := range subjects {
			key.WriteString(string(id) + "=" + strconv.FormatFloat(headings[id], 'g', -1, 64) + ";")
		}
		if g, ok := groups[key.String()]; ok {
			g.observers = append(g.observers, observer)
			continue
		}
		g := &headingPass{observers: []core.EntityID{observer}, presences: presences}
		if len(subjects) > 0 {
			g.presences = append([]perception.Presence(nil), presences...)
			for _, id := range subjects {
				v := variant{subject: id, heading: headings[id]}
				payload, ok := encoded[v]
				if !ok {
					base := g.presences[seenAs[id]]
					testimony, decoded := DecodeSightTestimony(base.Payload)
					if !decoded {
						return nil, fmt.Errorf("heading of %q: %w", id, ErrInvalidData)
					}
					h := v.heading
					testimony.Heading = &h
					var err error
					if payload, err = encodeSighting(testimony); err != nil {
						return nil, fmt.Errorf("encode sight testimony: %w", err)
					}
					encoded[v] = payload
				}
				g.presences[seenAs[id]].Payload = payload
			}
		}
		groups[key.String()] = g
		order = append(order, key.String())
	}

	out := make([]headingPass, 0, len(order))
	for _, k := range order {
		out = append(out, *groups[k])
	}

	return out, nil
}

// headingsSeenBy is the heading observer would hold for every placed member
// it already holds placed sight testimony of, were it to see that member
// where it stands now: the bearing from the cell its testimony places them
// on to positions[subject], or the testimony's own heading when that is the
// same cell. A member it holds nothing placed on is absent — nil, never seen
// going anywhere. Whether observer sees the member this pass is perception's
// question; a heading for a member it does not see is never landed.
func (e *Encounter) headingsSeenBy(
	observer core.EntityID, positions map[MemberID]spatial.Position,
) (map[MemberID]float64, error) {
	holdings, err := e.intelLog.Held(observer)
	if err != nil {
		return nil, fmt.Errorf("headings held by %q: %w", observer, err)
	}
	out := map[MemberID]float64{}
	for _, h := range holdings {
		if h.Channel != perception.Sight {
			continue
		}
		subject, ok := subjectID(h.Subject, memberSubjectKind)
		if !ok {
			continue
		}
		now, placed := positions[MemberID(subject)]
		if !placed {
			continue
		}
		before, ok := DecodeSightTestimony(h.Payload)
		if !ok || before.State == LocationUnknown {
			continue
		}
		if before.Position == now {
			if before.Heading != nil {
				out[MemberID(subject)] = *before.Heading
			}
			continue
		}
		if degrees, ok := bearing(e.field.orientation, before.Position, now); ok {
			out[MemberID(subject)] = degrees
		}
	}

	return out, nil
}
