// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/customization"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// This file is the converter layer, and its isolation is deliberate.
//
// Encapsulation is not free: keeping inner types off the exported surface (S2)
// means every value the composition hands back must be rewritten into a shape
// this package owns. That is the trade — hosts never recompile when an inner
// module changes, and we maintain a translation layer forever. Keeping it in
// one file makes the cost visible rather than smeared through the verbs, and
// gives it a single place to be tested.
//
// The failure mode of a hand-written converter is not a crash. It is silently
// dropping a field when an inner type grows, and shipping a projection that
// looks complete. convert_test.go closes that: every inner field must be
// carried across or explicitly justified as omitted.

// projectLayout maps the composition's authoring frame onto the wire's render
// word (rpg-toolkit#1140).
//
// The default arm is unreachable: encounter.Orientation is SEALED by an
// unexported method, so the only kinds that can exist are the two named
// here, and a hex field must declare one of them. It still refuses to guess
// rather than invent a layout: a guess would turn an impossible state into a
// wrong picture, which is strictly worse than an obviously absent one.
func projectLayout(o encounter.Orientation) HexLayout {
	if o == nil {
		return ""
	}
	switch o.Kind() {
	case encounter.OrientationPointyTop:
		return HexLayoutPointyTop
	case encounter.OrientationFlatTop:
		return HexLayoutFlatTop
	default:
		return ""
	}
}

// projectAtlas copies the composition's map across, field for field.
//
// It used to be a reshape: the composition answered room by room — a
// footprint per room, props and walls grouped under the room that declared
// them, a doorway naming the two rooms it joined — and this was where that
// stopped being anybody else's problem, by enumerating the footprints into
// absolute cells and sorting every list into one coordinate order. As of
// rpg-project#256 the composition's field IS flat: a region is a named set
// of absolute cells, and props, walls and doorways are field-level facts
// sorted in the composition's own order. There is nothing left to flatten,
// and — the symmetric-bug lesson — nothing to convert: every cell arriving
// here is already axial, and the ONE place a cell becomes axial is the
// composition's construction, not this seam.
//
// What remains is the translation S2 exists for: the composition's types
// become this package's, so a host never recompiles when the inner module
// changes shape. convert_test.go holds this honest field by field.
func projectAtlas(in encounter.Atlas) Atlas {
	out := Atlas{
		// Hex is the field's only family as of rpg-project#256; the
		// composition's Grid() has no other answer to give.
		Grid:       GridHex,
		Layout:     projectLayout(in.Orientation),
		Cells:      append([]spatial.Position(nil), in.Cells...),
		Props:      make([]AtlasProp, 0, len(in.Props)),
		Boundaries: make([]AtlasBoundary, 0, len(in.Boundaries)),
		Doorways:   make([]AtlasDoorway, 0, len(in.Doorways)),
		Regions:    make([]AtlasRegion, 0, len(in.Regions)),
		Segments:   make([]AtlasSegment, 0, len(in.Segments)),
		Sealed:     append([]spatial.Position(nil), in.Sealed...),
	}

	for _, seg := range in.Segments {
		out.Segments = append(out.Segments, AtlasSegment{
			From:   AxialPointF{Q: seg.From.Q, R: seg.From.R},
			To:     AxialPointF{Q: seg.To.Q, R: seg.To.R},
			Height: seg.Height,
		})
	}

	for _, prop := range in.Props {
		out.Props = append(out.Props, AtlasProp{
			Ref:               prop.Ref,
			At:                prop.At,
			BlocksMovement:    prop.BlocksMovement,
			BlocksLineOfSight: prop.BlocksLineOfSight,
			Facing:            prop.Facing,
			Offset:            prop.Offset,
		})
	}

	for _, b := range in.Boundaries {
		out.Boundaries = append(out.Boundaries, AtlasBoundary{
			From:              b.From,
			To:                b.To,
			BlocksMovement:    b.BlocksMovement,
			BlocksLineOfSight: b.BlocksLineOfSight,
			Height:            b.Height,
		})
	}

	for _, d := range in.Doorways {
		out.Doorways = append(out.Doorways, AtlasDoorway{
			Door: string(d.Door),
			From: d.From,
			To:   d.To,
		})
	}

	for _, r := range in.Regions {
		out.Regions = append(out.Regions, AtlasRegion{
			ID:        string(r.ID),
			Name:      r.Name,
			Cells:     append([]spatial.Position(nil), r.Cells...),
			Archetype: r.Archetype,
			Lighting:  Lighting{Intensity: r.Lighting.Intensity},
		})
	}

	return out
}

func projectStatus(in *encounter.Status) *Status {
	if in == nil {
		return nil
	}
	out := &Status{Open: in.Open}
	if in.Outcome == nil {
		return out
	}

	outcome := &Outcome{
		Ending:  in.Outcome.Ending,
		At:      in.Outcome.At,
		Members: make([]MemberOutcome, 0, len(in.Outcome.Members)),
	}
	for _, m := range in.Outcome.Members {
		outcome.Members = append(outcome.Members, projectMemberOutcome(m))
	}
	out.Outcome = outcome
	return out
}

func projectSightings(
	in []intel.Holding, names map[string]string, kinds map[string]MemberKind, down map[string]bool,
) []Sighting {
	out := make([]Sighting, 0, len(in))
	for _, h := range in {
		subject := string(h.Subject)
		via := make([]string, 0, len(h.CurrentVia))
		for _, c := range h.CurrentVia {
			via = append(via, string(c))
		}
		out = append(out, Sighting{
			Subject:       subject,
			Name:          names[subject],
			Kind:          kinds[subject],
			Seen:          projectSeen(h.Channel, h.Payload, down[subject]),
			LocationState: projectLocationState(h.Channel, h.Payload),
			Payload:       append([]byte(nil), h.Payload...),
			Channel:       string(h.Channel),
			At:            h.At,
			CurrentVia:    via,
			Status:        string(h.Status),
		})
	}
	return out
}

// projectSeen copies the sight channel's typed knowledge into Seen (ADR-0041,
// rpg-toolkit#1137). It is nil for every channel but sight, and nil if a
// sight-channel payload somehow fails to decode — an impossible state today
// (the composition is the only writer of sight payloads) that a wrong
// Position would be worse than admitting to.
//
// The decode itself happens in encounter.DecodeLocationPayload, not here: this
// package never calls encoding/json on a payload. h.Channel is intel's own
// provenance field — a holding's last accepted testimony — so a held memory
// (CurrentVia empty) still carries the channel and payload that produced it.
// Known testimony gets Seen; explicit unknown testimony gets LocationState
// without a stale coordinate.
//
// downed is the caller's own batched Standing() answer for this subject —
// asked once per verb over the whole roster (turn.go's own pattern), never
// once per sighting, and passed in rather than looked up here so this stays
// a pure projection.
func projectSeen(channel intel.Channel, payload []byte, downed bool) *Seen {
	if channel != intel.Sight {
		return nil
	}
	location, ok := encounter.DecodeLocationPayload(payload)
	if !ok || location.State != encounter.LocationKnown {
		return nil
	}
	standing := StandingUp
	if downed {
		standing = StandingDowned
	}
	return &Seen{Position: location.Position, Standing: standing}
}

func projectLocationState(channel intel.Channel, payload []byte) LocationState {
	if channel != intel.Sight {
		return ""
	}
	location, ok := encounter.DecodeLocationPayload(payload)
	if !ok {
		return ""
	}
	return LocationState(location.State)
}

func projectMember(in encounter.Member) Member {
	return Member{
		ID:       string(in.ID),
		Kind:     MemberKind(in.Kind),
		Name:     in.Name,
		Position: in.Position,
	}
}

func projectRosterCharacter(member encounter.Member, data *character.Data, loaded *character.Character) PublicMember {
	return PublicMember{
		ID:            string(member.ID),
		Kind:          KindPlayer,
		Name:          loaded.GetName(),
		ClassRef:      string(data.ClassID),
		RaceRef:       string(data.RaceID),
		Customization: projectCustomization(loaded.Appearance()),
	}
}

func indexRosterNPCs(npcs []monster.Data) (map[string]*monster.Data, error) {
	indexed := make(map[string]*monster.Data, len(npcs))
	for i := range npcs {
		stored := &npcs[i]
		if stored.ID == "" || stored.Name == "" || stored.Ref == nil {
			return nil, fmt.Errorf("roster: npc at index %d has corrupt identity: %w", i, ErrBadNPC)
		}
		if _, ok := indexed[stored.ID]; ok {
			return nil, fmt.Errorf("roster: duplicate npc %q: %w", stored.ID, ErrBadNPC)
		}
		if err := stored.Ref.IsValid(); err != nil {
			return nil, fmt.Errorf("roster: npc %q has corrupt ref: %w: %v", stored.ID, ErrBadNPC, err)
		}
		indexed[stored.ID] = stored
	}
	return indexed, nil
}

func projectRosterMonster(member encounter.Member, npcs map[string]*monster.Data) (PublicMember, error) {
	stored, ok := npcs[string(member.ID)]
	if !ok {
		return PublicMember{}, fmt.Errorf("roster: monster %q: %w", member.ID, ErrNoSheet)
	}

	return PublicMember{
		ID:            string(member.ID),
		Kind:          KindMonster,
		Name:          stored.Name,
		MonsterRef:    stored.Ref.String(),
		Customization: Customization{},
	}, nil
}

func projectCustomization(in *customization.Appearance) Customization {
	if in == nil {
		return Customization{}
	}
	return Customization{
		Hair:   projectHairCustomization(in.Hair),
		Outfit: projectOutfitCustomization(in.Outfit),
	}
}

func projectHairCustomization(in *customization.HairCustomization) *HairCustomization {
	if in == nil {
		return nil
	}
	out := &HairCustomization{
		Scalp:      projectStyleSelection(in.Scalp),
		FacialHair: projectStyleSelection(in.FacialHair),
		ColorSRGB:  cloneUint32(in.ColorSRGB),
		Roughness:  cloneFloat32(in.Roughness),
	}
	return out
}

func projectOutfitCustomization(in *customization.OutfitCustomization) *OutfitCustomization {
	if in == nil {
		return nil
	}
	return &OutfitCustomization{
		PrimaryColorSRGB:   cloneUint32(in.PrimaryColorSRGB),
		SecondaryColorSRGB: cloneUint32(in.SecondaryColorSRGB),
	}
}

func projectStyleSelection(in *customization.StyleSelection) *StyleSelection {
	if in == nil {
		return nil
	}
	return &StyleSelection{Kind: StyleSelectionKind(in.Kind), StyleRef: in.StyleRef}
}

func cloneUint32(in *uint32) *uint32 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

func cloneFloat32(in *float32) *float32 {
	if in == nil {
		return nil
	}
	out := *in
	return &out
}

// onMap is gone (rpg-toolkit#1059), and its absence is the point.
//
// It projected a composition room-local cell onto the one map this seam
// speaks, and what was left for it to do had been shrinking for three slices:
// placement reads came back absolute at encounter v0.10.0, outcomes at v0.13.0
// (rpg-toolkit#1068), and the last room-local cells anywhere near this package
// were the ones the walk handed down to the composition's Move and read back
// out of it. The step verb takes and reports absolute cells, so there is
// nothing left to project.
//
// Its error path is why deleting it beats keeping it: an unprojectable cell
// returned the cell UNPROJECTED, on the grounds that a wrong answer a test can
// see beats a panic in a host. That fallback measurably masked a real bug —
// after #1068, projectMemberOutcome would have anchored every outcome twice,
// and every fixture hid it because their rooms sit far enough out that an
// absolute cell is never also a legal local one, so the second projection was
// refused and the refusal was swallowed (#1053, PR #1072's evidence). A helper
// with no callers cannot swallow anything.

// projectMemberOutcome carries the outcome's cell across unchanged.
//
// It used to project one: an outcome was the last shape the composition still
// reported room-local, so the seam re-asked for an absolute cell through
// Absolute. Encounter v0.13.0 reports the outcome on the dungeon map itself
// (rpg-toolkit#1068), and the same anchoring applied twice would put every
// member off by their room's origin — in the one report a host reads after
// there is nothing left to check it against.
func projectMemberOutcome(in encounter.MemberOutcome) MemberOutcome {
	return MemberOutcome{
		ID:       string(in.ID),
		Position: in.Position,
	}
}

// projectOutcome converts a bare outcome. projectStatus has its own inline
// copy of this walk because it must also handle the nil-Status case; this one
// serves the verbs that return an outcome directly.
func projectOutcome(in *encounter.Outcome) *Outcome {
	if in == nil {
		return nil
	}
	out := &Outcome{
		Ending:  in.Ending,
		At:      in.At,
		Members: make([]MemberOutcome, 0, len(in.Members)),
	}
	for _, m := range in.Members {
		out.Members = append(out.Members, projectMemberOutcome(m))
	}
	return out
}

// projectDiscoveries converts the per-observer perception deltas a verb
// produced.
//
// Observers with a nil delta are skipped rather than given an empty entry: a
// present key means "something changed for this observer", and manufacturing
// empty entries for everyone who happened to be in the encounter would make
// the map's size meaningless to a caller deciding whom to notify.
func projectDiscoveries(in map[encounter.MemberID]*encounter.IntelDelta, down map[string]bool) map[string]Discovery {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]Discovery, len(in))
	for id, delta := range in {
		if delta == nil {
			continue
		}
		out[string(id)] = projectDiscovery(delta, down)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func projectDiscovery(in *encounter.IntelDelta, down map[string]bool) Discovery {
	out := Discovery{}
	for _, r := range in.FirstContact {
		out.FirstContact = append(out.FirstContact, Report{
			Subject: string(r.Subject),
			Seen:    projectReportSeen(r.Payload, down[string(r.Subject)]),
			Payload: append([]byte(nil), r.Payload...),
		})
	}
	for _, s := range in.Refreshed {
		out.Refreshed = append(out.Refreshed, string(s))
	}
	for _, s := range in.Faded {
		out.Faded = append(out.Faded, string(s))
	}
	return out
}

// projectIntelCorrections converts encounter-owned correction deltas into a
// deterministic session-owned list. Observer and subject are the only facts
// exposed; corrected payloads never cross this seam.
func projectIntelCorrections(in map[encounter.MemberID]*encounter.IntelDelta) []IntelCorrection {
	if len(in) == 0 {
		return nil
	}
	out := make([]IntelCorrection, 0)
	for observer, delta := range in {
		if delta == nil {
			continue
		}
		for _, subject := range delta.Corrected {
			out = append(out, IntelCorrection{Observer: string(observer), Subject: string(subject)})
		}
	}
	return sortIntelCorrections(out)
}

func sortIntelCorrections(in []IntelCorrection) []IntelCorrection {
	if len(in) == 0 {
		return nil
	}
	out := append([]IntelCorrection(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Observer == out[j].Observer {
			return out[i].Subject < out[j].Subject
		}
		return out[i].Observer < out[j].Observer
	})
	return out
}

// projectReportSeen decodes first-contact's Seen the same way projectSeen
// does, but cannot gate on channel the way projectSeen does: intel.Report
// carries no Channel of its own — a SurveilOutput is scoped to the one
// Channel its Surveil call used, but that channel is not threaded back onto
// each Report inside it. So this is decode-and-see rather than a channel
// check.
//
// That is equivalent to projectSeen's guard ONLY as long as sight is the only
// channel any composition surveils with — true today, since rebuildPercepts
// (encounter.go, refreshSight) is the sole Surveil call site in this
// codebase and always passes intel.Sight. The day a second channel starts
// calling Surveil, an undecodable payload here stops meaning "not sight" and
// starts meaning "channel this SDK has not typed yet OR truly bad bytes" —
// indistinguishable from here.
// TestProjectReportSeenCannotDistinguishSightFromALookalikePayload
// (seen_internal_test.go) documents the risk rather than closing it: closing
// it needs SurveilOutput (or the percept it is built from) to carry its own
// channel, which is a play/intel change outside this PR's scope.
func projectReportSeen(payload []byte, downed bool) *Seen {
	pos, ok := encounter.DecodeSightPayload(payload)
	if !ok {
		return nil
	}
	standing := StandingUp
	if downed {
		standing = StandingDowned
	}
	return &Seen{Position: pos, Standing: standing}
}
