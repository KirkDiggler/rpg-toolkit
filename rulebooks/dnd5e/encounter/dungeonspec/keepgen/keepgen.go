// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package keepgen generates the benchmark keep (rpg-project#740): FadedPez's
// keep, read from a plan, written as a version-5 site at any size from keep
// scale to the 30,000-hex ceiling, propped and garrisoned from palettes, and
// proven by the dialect's own loader before a byte is handed back.
//
// The law is rpg-project ideas/benchmark-keep/design.md (R1–R5); the plan is
// its plan.md, tasks T1–T4 and gaps G1–G13. In short:
//
//   - THE PLAN IS DATA ([Plan], embedded as plan/keep.yaml): rooms by side
//     with a footprint at keep scale, doors as room pairs, palettes by room
//     kind. It carries no cells, coordinates or counts.
//   - ONE KEEP, TWO SCALES (R2). Rooms scale by min(1.5, sqrt(hexes /
//     planHexes)); the grounds absorb the remainder exactly. The embedded
//     plan is 10,000 hexes at scale 1 (G1). A wall is one line per shared
//     edge of two regions, with one opening and door per plan door, bound
//     open.
//   - PROPS EXERCISE THE ENGINE (R3). Two in three carry a propDeclaration
//     that blocks movement (and sight, where the palette says so); door
//     thresholds, the party start, exits and monster cells stay clear.
//     Monsters stand from the palette at a density, with no mind or patrol.
//   - PROOF BEFORE WRITE. [Generate] runs [Prove] on its own bytes and
//     returns none if the loader refuses them, the start seats fewer than
//     four, or a region is unreachable through the doors.
//   - EQUAL INPUTS, EQUAL BYTES. Every draw comes from a source seeded per
//     region from [Options.Seed]; the header comment records the options and
//     the command line, and [OptionsFromHeader] reads them back.
//
// The generator reads no asset catalog: the palette carries the refs, and
// the web's import check is where an unknown one is refused.
package keepgen

import (
	"fmt"
	"strings"
)

// Options are every knob (design Defaults). Zero density means none.
type Options struct {
	// Hexes is the floor's exact cell count; at least the plan's own floor.
	Hexes int
	// Seed seeds every placement draw.
	Seed int64
	// PropDensity is props per 100 room hexes; default 12.5 (one per 8).
	PropDensity float64
	// GroundsDensity is props per 100 courtyard and grounds hexes; default
	// 2.5 (one per 40).
	GroundsDensity float64
	// MonsterDensity is monsters per 100 floor hexes; default 1.
	MonsterDensity float64
	// Key is the site's key, which equals its shelf file's name
	// (`bench-keep`, `bench-keep-30k`; plan G6). Lowercase letters, digits
	// and dashes.
	Key string
	// CommandLine is recorded in the header comment, one line.
	CommandLine string
}

// DefaultOptions are the design's defaults at keep scale: 10,000 hexes,
// seed 1, 12.5 props per 100 room hexes, 2.5 per 100 grounds hexes, one
// monster per 100 floor hexes, key bench-keep.
func DefaultOptions() Options {
	return Options{
		Hexes: 10000, Seed: 1, PropDensity: 12.5, GroundsDensity: 2.5, MonsterDensity: 1, Key: "bench-keep",
	}
}

// Report counts what a generated site holds.
type Report struct {
	// Hexes is the floor's cell count.
	Hexes int
	// Regions counts rooms, the courtyard and both grounds.
	Regions int
	// Walls counts wall lines.
	Walls int
	// Doors counts doors, one per plan door.
	Doors int
	// Props counts scene props.
	Props int
	// Blocking counts props with a propDeclaration, every one blocking
	// movement.
	Blocking int
	// Monsters counts declared monsters.
	Monsters int
	// GeometryBytes is the size of the room's `walls` and `regions` blocks
	// as written: what site-format R1's 100 KB budget is measured on.
	GeometryBytes int
	// Scale is the linear factor the rooms were scaled by.
	Scale float64
}

// String is the report as one line, the command's output.
func (r Report) String() string {
	return fmt.Sprintf("hexes=%d scale=%s regions=%d walls=%d doors=%d props=%d blocking=%d monsters=%d geometry=%dB",
		r.Hexes, exact(r.Scale), r.Regions, r.Walls, r.Doors, r.Props, r.Blocking, r.Monsters, r.GeometryBytes)
}

// Generate lays the plan out at o.Hexes, places walls, doors, props and
// monsters, emits the site and proves it (README: Layout → Place → Emit →
// Prove). It returns the bytes and their [Report], or an error and no bytes:
// refused options (hexes below the plan's floor, a negative density, a key
// that is not lowercase letters, digits and dashes, a command line with a
// line break), a plan that does not lay out, or a site [Prove] refuses.
// Equal (p, o) give equal bytes.
func Generate(p *Plan, o Options) ([]byte, Report, error) {
	if p == nil {
		return nil, Report{}, fmt.Errorf("keepgen: no plan")
	}
	if err := o.validate(); err != nil {
		return nil, Report{}, err
	}
	l, err := layoutOf(p, o.Hexes)
	if err != nil {
		return nil, Report{}, err
	}
	s, err := placeAll(p, l, o)
	if err != nil {
		return nil, Report{}, err
	}
	out, err := emitSite(p, l, s, o)
	if err != nil {
		return nil, Report{}, err
	}
	if _, err := Prove(out.doc); err != nil {
		return nil, Report{}, err
	}
	r := Report{
		Hexes: l.floor(), Regions: len(l.regions), Walls: len(s.walls), Props: len(s.props),
		Monsters: len(s.monsters), GeometryBytes: out.geometryBytes, Scale: l.scale,
	}
	for _, w := range s.walls {
		if w.door != nil {
			r.Doors++
		}
	}
	for _, pr := range s.props {
		if pr.blocking {
			r.Blocking++
		}
	}
	return out.doc, r, nil
}

func (o Options) validate() error {
	switch {
	case o.Hexes <= 0:
		return fmt.Errorf("keepgen: hexes must be positive, not %d", o.Hexes)
	case !(o.PropDensity >= 0), !(o.GroundsDensity >= 0), !(o.MonsterDensity >= 0):
		return fmt.Errorf("keepgen: densities must be zero or more (props %v, grounds-props %v, monsters %v)",
			o.PropDensity, o.GroundsDensity, o.MonsterDensity)
	case !idPattern.MatchString(o.Key):
		return fmt.Errorf("keepgen: key %q must be lowercase letters, digits and dashes", o.Key)
	case strings.ContainsAny(o.CommandLine, "\r\n"):
		return fmt.Errorf("keepgen: the command line must be one line")
	}
	return nil
}
