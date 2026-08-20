// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"strings"
)

// Version is the one dialect this build speaks.
//
// A single accepted value rather than a floor, deliberately. "Version 2 or
// later is fine" is a promise about files nobody has written yet, and the
// standing precedent for a shape this build does not know is to refuse it by
// name rather than read it hopefully (rpg-toolkit#1053/#1068).
const Version = 1

// The author-facing vocabularies. Each is closed, and an unrecognised word is
// refused rather than mapped to the nearest one — for [encounter.Void]'s
// reason: a word this build has never heard of is a dialect it does not speak,
// and picking the closest answer would author a dungeon the host did not.
var (
	voids        = map[string]bool{"opaque": true, "transparent": true}
	orientations = map[string]bool{"pointy": true, "flat": true}

	// The targeting words are carried, not interpreted — but a TYPO is still
	// worth catching, because "lowest-helth" would otherwise ride all the way
	// through the compiler and be rejected (or worse, ignored) by a rulebook
	// the author never sees. Checking the spelling of a word whose meaning
	// this package does not know is exactly as far as it may go.
	targetings = map[string]bool{"closest": true, "lowest-health": true, "lowest-ac": true}
)

// The ref type segments this compiler can route. A ref's type decides what a
// placement BECOMES, which is why an unknown one is refused: there is no
// default kind of thing to be.
const (
	typeProps    = "props"
	typeMonsters = "monsters"
)

// Validate reports whether a decoded spec is a dungeon, in the author's own
// vocabulary.
//
// # What it checks, and what it deliberately does not
//
// Everything here is true BEFORE geometry: no check below depends on the grid
// family, the orientation, or how chambers are laid out on a canvas. That is
// the seam — this function is about whether the file describes a dungeon, and
// the compiler is about what that dungeon becomes.
//
// The one place it reaches toward geometry is the doorway rule, and it is worth
// saying why that is not a leak. A connector's opening lands on the seam row,
// so a placement there would sit in the doorway. The composition already
// refuses that (validateConnectionInputs, "from-position on prop") — but it
// refuses it in the COMPOSITION's vocabulary, about an absolute cell the author
// never wrote, after a compile they cannot see. Said here, it is about the line
// they did write.
//
// # It stops at the first defect
//
// Not a list, on purpose. A spec with a malformed ref usually has three, and an
// author fixing them one at a time is served better by a message about one line
// than a wall about ten — and a wall invites skimming, which is how the fourth
// one ships.
func Validate(spec *Spec) error {
	if spec == nil {
		return fmt.Errorf("no dungeon spec: %w", ErrBadSpec)
	}

	if spec.Version != Version {
		return fmt.Errorf(
			"dungeon spec version %d, which this build does not speak (it speaks %d): %w",
			spec.Version, Version, ErrBadSpec)
	}
	if spec.Key == "" {
		return fmt.Errorf("the dungeon has no key: %w", ErrBadSpec)
	}
	if spec.Height < 1 {
		return fmt.Errorf("the dungeon's height is %d, and a chamber must be at least one cell tall: %w",
			spec.Height, ErrBadSpec)
	}
	if !voids[spec.Void] {
		return fmt.Errorf(
			"the dungeon does not say what its void is (void: %q; opaque or transparent, and there is no default): %w",
			spec.Void, ErrBadSpec)
	}
	if !orientations[spec.Orientation] {
		return fmt.Errorf(
			"the dungeon does not say which way its hexes point "+
				"(orientation: %q; pointy or flat, and every `at` in the file depends on it): %w",
			spec.Orientation, ErrBadSpec)
	}
	if len(spec.Rooms) == 0 {
		return fmt.Errorf("the dungeon has no rooms: %w", ErrBadSpec)
	}

	index, err := validateRooms(spec)
	if err != nil {
		return err
	}

	return validateConnectors(spec, index)
}

// validateRooms checks the chamber list and returns each chamber's position in
// it, which validateConnectors needs to tell neighbours from strangers.
func validateRooms(spec *Spec) (map[string]int, error) {
	index := make(map[string]int, len(spec.Rooms))

	for i, room := range spec.Rooms {
		if room.ID == "" {
			return nil, fmt.Errorf("rooms[%d] has no id: %w", i, ErrBadSpec)
		}
		if _, seen := index[room.ID]; seen {
			return nil, fmt.Errorf("duplicate room %q: %w", room.ID, ErrBadSpec)
		}
		index[room.ID] = i

		if room.Width < 1 {
			return nil, fmt.Errorf("room %q has width %d, and a chamber must be at least one cell wide: %w",
				room.ID, room.Width, ErrBadSpec)
		}

		if err := validatePlacements(spec, room); err != nil {
			return nil, err
		}
	}

	return index, nil
}

// validatePlacements checks everything standing in one chamber, boss included.
//
// The boss is checked through the SAME rules as everything else rather than a
// parallel set: it is a monster in a cell, and the only thing that makes it
// special is that the author named it separately. A second implementation of
// "is this cell inside the room" is how the two answers eventually disagree.
func validatePlacements(spec *Spec, room RoomSpec) error {
	occupied := make(map[[2]int]string, len(room.Place)+1)

	check := func(what, ref string, at [2]int, targeting *string, blocksMovement, blocksLoS *bool) error {
		kind, err := refKind(ref)
		if err != nil {
			return fmt.Errorf("room %q: %s: %w", room.ID, what, err)
		}
		if what == "boss" && kind != typeMonsters {
			return fmt.Errorf("room %q: boss %q is not a monster: %w", room.ID, ref, ErrBadSpec)
		}

		if at[0] < 0 || at[0] >= room.Width || at[1] < 0 || at[1] >= spec.Height {
			return fmt.Errorf(
				"room %q: %s %q at [%d,%d], which is outside the chamber (%dx%d): %w",
				room.ID, what, ref, at[0], at[1], room.Width, spec.Height, ErrBadSpec)
		}
		if doorwayRow(spec.Height) == at[1] && standsInASeam(spec, room, at[0]) {
			return fmt.Errorf(
				"room %q: %s %q at [%d,%d] stands in a doorway: %w",
				room.ID, what, ref, at[0], at[1], ErrBadSpec)
		}
		if other, taken := occupied[at]; taken {
			return fmt.Errorf("room %q: %q and %q are on the same cell [%d,%d]: %w",
				room.ID, other, ref, at[0], at[1], ErrBadSpec)
		}
		occupied[at] = ref

		if targeting != nil {
			if kind != typeMonsters {
				return fmt.Errorf("room %q: %q is not a monster and cannot have targeting: %w",
					room.ID, ref, ErrBadSpec)
			}
			if !targetings[*targeting] {
				return fmt.Errorf("room %q: %q declares targeting %q, which is not a word this build knows: %w",
					room.ID, ref, *targeting, ErrBadSpec)
			}
		}
		if kind != typeProps {
			if blocksMovement != nil {
				return fmt.Errorf("room %q: %q is not a prop and cannot declare blocks_movement: %w",
					room.ID, ref, ErrBadSpec)
			}
			if blocksLoS != nil {
				return fmt.Errorf("room %q: %q is not a prop and cannot declare blocks_los: %w",
					room.ID, ref, ErrBadSpec)
			}
		}

		return nil
	}

	for _, p := range room.Place {
		if err := check("place", p.Ref, p.At, p.Targeting, p.BlocksMovement, p.BlocksLoS); err != nil {
			return err
		}
	}
	if room.Boss != nil {
		if err := check("boss", room.Boss.Ref, room.Boss.At, room.Boss.Targeting, nil, nil); err != nil {
			return err
		}
	}

	return nil
}

// validateConnectors checks the openings.
func validateConnectors(spec *Spec, index map[string]int) error {
	seams := make(map[[2]string]bool, len(spec.Connectors))

	for i, c := range spec.Connectors {
		from, ok := index[c.From]
		if !ok {
			return fmt.Errorf("connectors[%d] joins room %q, which is not declared: %w", i, c.From, ErrBadSpec)
		}
		to, ok := index[c.To]
		if !ok {
			return fmt.Errorf("connectors[%d] joins room %q, which is not declared: %w", i, c.To, ErrBadSpec)
		}
		if from == to {
			return fmt.Errorf("connectors[%d] joins room %q to itself: %w", i, c.From, ErrBadSpec)
		}
		// Chambers are laid out in declaration order, so a connector between
		// rooms that are not adjacent in that order names a seam no two
		// chambers share.
		if from != to-1 && to != from-1 {
			return fmt.Errorf(
				"connectors[%d] joins %q and %q, which are not next to each other in the room list: %w",
				i, c.From, c.To, ErrBadSpec)
		}

		seam := [2]string{c.From, c.To}
		if from > to {
			seam = [2]string{c.To, c.From}
		}
		if seams[seam] {
			return fmt.Errorf("connectors[%d] declares the seam between %q and %q twice: %w",
				i, seam[0], seam[1], ErrBadSpec)
		}
		seams[seam] = true

		if c.Locked == nil {
			continue
		}
		if c.Locked.DC < 1 {
			return fmt.Errorf("connectors[%d] is locked with dc %d, and a lock with nothing to beat is not a lock: %w",
				i, c.Locked.DC, ErrBadSpec)
		}
		if c.Locked.Ability == "" {
			return fmt.Errorf("connectors[%d] is locked with no ability to check: %w", i, ErrBadSpec)
		}
	}

	return nil
}

// refKind returns a ref's type segment, which is what routes a placement.
//
// Parsed here rather than through the rulebook's own ref parser for the reason
// this package exists: importing one would break design law C1. The check is
// deliberately shallow — three non-empty segments — because "is this a ref that
// resolves to real content" is a question only the layer that owns content can
// answer, and pretending to answer it here would be a second, weaker opinion
// beside the real one.
func refKind(ref string) (string, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", fmt.Errorf("ref %q is not module:type:id: %w", ref, ErrBadSpec)
	}

	switch parts[1] {
	case typeProps, typeMonsters:
		return parts[1], nil
	default:
		return "", fmt.Errorf("ref %q names type %q, which this compiler cannot place: %w",
			ref, parts[1], ErrBadSpec)
	}
}

// doorwayRow is the row every connector's opening sits on.
//
// height/2, which is the old stack's rule (dungeonspec's own validate reserves
// the same row) and, more to the point, is where the reference tomb's author
// already believes it is: the shipping file places things on rows 1, 2, 3, 5
// and 6 of an 8-tall dungeon, and never on row 4.
func doorwayRow(height int) int { return height / 2 }

// standsInASeam reports whether a column is the one a connector's opening
// actually lands in — the only column where the doorway rule can bite.
//
// TWO NARROWINGS, and both matter. A chamber's INTERIOR is its own business:
// something standing on the doorway ROW in the middle of a hall is in nobody's
// way, and refusing it would be the old stack's rule imported without the
// geometry that made it necessary — over there a wall column was carved for the
// door, so the whole row was spoken for, and here nothing is.
//
// And an edge column only counts if a connector opens THROUGH it. The tomb's
// entrance has a seam on its east side and open rock to its west, so a brazier
// against its west wall on row 4 is a brazier against a wall. Asking "does this
// chamber have any connector at all" would have refused it, which is the kind
// of over-refusal that teaches authors to distrust the compiler.
func standsInASeam(spec *Spec, room RoomSpec, col int) bool {
	west, east := col == 0, col == room.Width-1
	if !west && !east {
		return false
	}

	// The chambers are laid out in declaration order, so the neighbour a
	// connector must name is the one on that side.
	var index int
	for i, r := range spec.Rooms {
		if r.ID == room.ID {
			index = i
			break
		}
	}

	for _, c := range spec.Connectors {
		other := ""
		switch room.ID {
		case c.From:
			other = c.To
		case c.To:
			other = c.From
		default:
			continue
		}
		for i, r := range spec.Rooms {
			if r.ID != other {
				continue
			}
			if (west && i == index-1) || (east && i == index+1) {
				return true
			}
		}
	}

	return false
}
