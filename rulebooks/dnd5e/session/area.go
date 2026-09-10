// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// UnresolvedReason says why a member a footprint caught could not be resolved
// against.
//
// One value, because there is one cause today. A second arrives with whatever
// produces it.
type UnresolvedReason string

// UnresolvedNoSheet is a placed world member — a shopkeeper, a bystander — with
// no character or monster sheet behind it. The blast reached them and nothing in
// this build models what that does.
const UnresolvedNoSheet UnresolvedReason = "no_sheet"

// CaughtMember is somebody an area cast's footprint caught but the engine could
// not resolve against.
//
// REPORTED RATHER THAN DROPPED. "Nobody was standing there" and "somebody was
// standing there and we have nothing to do about it" are different facts, and
// collapsing them is how a missing capability stops being visible. When world
// members are wired to take damage this stops appearing on its own, which is
// what a cost not yet paid should look like.
type CaughtMember struct {
	// Member is who was caught.
	Member string `json:"member"`

	// Kind is what they are, in this seam's own vocabulary — [MemberKind]
	// rather than a bare string, so a caller deciding what to do about a
	// caught member compares against the same constants every other verb
	// hands it.
	Kind MemberKind `json:"kind"`

	// Reason is why the engine could not resolve against them.
	Reason UnresolvedReason `json:"reason"`
}

// areaCaught is one footprint's answer, split by what the engine can do about
// each member.
type areaCaught struct {
	// resolvable are the ids handed to resolution as the cast's recipients.
	resolvable []string

	// unresolved are caught members with nothing to resolve against.
	unresolved []CaughtMember
}

// deriveAreaMembers works out who an area cast catches, by asking the
// composition that owns placement.
//
// THIS SEAM ASKS; IT DOES NOT MEASURE. [encounter.Encounter.MembersWithin]
// answers who is standing in a shape, because placement is the composition's
// and it already answers the same question for an authored region. What happens
// here is the part that is genuinely this seam's: converting the spell's feet
// into the grid's cells, applying the projection the SPELL declared, and
// deciding which of the answer this build can resolve against.
//
// None of that is a rule. There is no die and no threshold the content did not
// state — the radius is the profile's, and the exclusion is the profile's.
func deriveAreaMembers(
	enc *encounter.Encounter, profile *combatActions.CastProfile, casterID string,
	roster []encounter.Member,
) (*areaCaught, error) {
	area := profile.Area
	if area == nil {
		// CastProfile.Validate binds the target rule and the shape together, so
		// this is content that never went through it. Fail closed rather than
		// treat a missing shape as an empty one.
		return nil, fmt.Errorf("%w: area cast declares no area", ErrBadCast)
	}

	// The one refusal that belongs here rather than in the declaration: the
	// rulebook module cannot import the conversion (rpg-toolkit#1625), so a
	// footprint that floors to nothing is caught at the first layer that can
	// measure. A one-to-four-foot radius validates as content and can then only
	// ever reach the caster's own cell.
	cells := encounter.CellsFromFeet(area.Footprint.SizeFeet)
	if cells < 1 {
		return nil, fmt.Errorf("%w: area of %d feet is less than one %d-foot cell",
			ErrBadCast, area.Footprint.SizeFeet, encounter.FeetPerCell)
	}

	var origin encounter.Member
	found := false
	for _, member := range roster {
		if string(member.ID) == casterID {
			origin, found = member, true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("%w: caster %q is not on the roster", ErrBadCast, casterID)
	}

	switch area.Footprint.Origin {
	case combatActions.AreaOriginCaster:
	default:
		return nil, fmt.Errorf("%w: unsupported area origin %q", ErrBadCast, area.Footprint.Origin)
	}

	var caught []encounter.Member
	var err error
	switch area.Footprint.Shape {
	case combatActions.AreaRadius:
		caught, err = enc.MembersWithin(&encounter.MembersWithinInput{
			Origin: origin.Position, RadiusCells: float64(cells),
		})
	default:
		return nil, fmt.Errorf("%w: unsupported area shape %q", ErrBadCast, area.Footprint.Shape)
	}
	if err != nil {
		return nil, fmt.Errorf("area cast: %w", err)
	}

	out := &areaCaught{resolvable: make([]string, 0, len(caught))}
	for _, member := range caught {
		id := string(member.ID)

		// The spell's own projection. The composition answers who is STANDING
		// there and has no idea why it was asked; "each creature other than
		// you" is the content's sentence and is applied here.
		if id == casterID && area.Catches == combatActions.AreaCatchesOthers {
			continue
		}

		// Whatever compileResolutionCast will skip cannot be a recipient, and
		// the two must agree: a member handed to resolution with no participant
		// behind it is refused mid-run, after the door has already charged.
		if member.Kind == encounter.MemberKind(KindWorld) {
			out.unresolved = append(out.unresolved, CaughtMember{
				Member: id, Kind: MemberKind(member.Kind), Reason: UnresolvedNoSheet,
			})
			continue
		}
		out.resolvable = append(out.resolvable, id)
	}
	return out, nil
}

// areaMemberIDs is the recipients to hand resolution, or nil for a cast that
// derived none. Nil in, nil out: a non-area cast has no derived list at all,
// which is a different thing from an area cast that caught nobody, and both
// reach resolution as an empty AreaMembers because neither has recipients to
// name.
func areaMemberIDs(caught *areaCaught) []string {
	if caught == nil {
		return nil
	}
	return caught.resolvable
}

// areaUnresolved is what to report to the caller about members the footprint
// caught and this build could not resolve against. Nil for a cast with no
// footprint, and nil for a footprint that caught nobody it could not handle.
func areaUnresolved(caught *areaCaught) []CaughtMember {
	if caught == nil || len(caught.unresolved) == 0 {
		return nil
	}
	return caught.unresolved
}
