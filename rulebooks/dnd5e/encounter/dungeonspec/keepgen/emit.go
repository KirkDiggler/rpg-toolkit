// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// emit.go writes the bytes (README: keepgen.Emit): a version-5 site, regions
// as canonical row runs through [dungeonspec.CellRows], everything in a
// fixed order, every number in one spelling, and a header comment that
// records the options and the command line that made it.
//
// THE FORMAT IS FLOW-HEAVY ON PURPOSE: one row run, wall, item or
// declaration per line keeps the geometry under R1's budget at the ceiling
// and diffs one placed thing per line.

const (
	headerCommand = "# keepgen-command: "
	headerOptions = "# keepgen-options: "
)

// emitted is the document and the size of its geometry.
type emitted struct {
	doc           []byte
	geometryBytes int // the room's `walls` and `regions` blocks, as written
}

// num spells a scene coordinate to the ten-thousandth, with no trailing
// zeros and no negative zero.
func num(v float64) string {
	r := math.Round(v*1e4) / 1e4
	if r == 0 {
		return "0"
	}
	return strconv.FormatFloat(r, 'f', -1, 64)
}

// exact spells a value with every digit it has (rotations, densities).
func exact(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }

// quote spells a string as a YAML double-quoted scalar.
func quote(s string) string { return strconv.Quote(s) }

func emitSite(p *Plan, l *layout, s *site, o Options) (emitted, error) {
	var b strings.Builder
	b.WriteString(header(o))
	b.WriteString("version: 5\n")
	fmt.Fprintf(&b, "key: %s\n", o.Key)
	b.WriteString("play: { void: transparent, lighting: bright, standing: centre-covered }\n")
	if len(p.Templates) > 0 {
		var t bytes.Buffer
		enc := yaml.NewEncoder(&t)
		enc.SetIndent(2)
		if err := enc.Encode(p.Templates); err != nil {
			return emitted{}, fmt.Errorf("keepgen: emit templates: %w", err)
		}
		if err := enc.Close(); err != nil {
			return emitted{}, fmt.Errorf("keepgen: emit templates: %w", err)
		}
		b.WriteString("templates:\n")
		for _, line := range strings.Split(strings.TrimRight(t.String(), "\n"), "\n") {
			b.WriteString("  " + line + "\n")
		}
	}
	if len(l.exits) > 0 {
		b.WriteString("exits:\n")
		for _, ex := range l.exits {
			a := ex.at.axial()
			fmt.Fprintf(&b, "  - { id: %s, cell: { q: %d, r: %d } }\n", ex.id, a.Q, a.R)
		}
	}
	name := quote(p.Name)
	b.WriteString("room:\n  version: 3\n  id: room-1\n")
	fmt.Fprintf(&b, "  name: %s\n", name)
	b.WriteString("  coordinateFrame: { horizontalPlane: world-xz, verticalAxis: world-y-up, distanceUnit: world-scene-unit, hexRadius: 1, footprintFrame: owner-local-xz }\n")
	fmt.Fprintf(&b, "  scene:\n    version: 3\n    id: scene-1\n    name: %s\n", name)
	if len(s.props) == 0 {
		b.WriteString("    items: []\n")
	} else {
		b.WriteString("    items:\n")
		for _, pr := range s.props {
			x, z := pr.at.centre()
			fmt.Fprintf(&b, "      - { id: %s, kind: prop, assetRef: %s, label: %s, transform: { x: %s, y: 0, z: %s, rotationY: %s } }\n",
				pr.id, quote(pr.entry.AssetRef), quote(l.regions[pr.region].name+" "+assetName(pr.entry.AssetRef)),
				num(x), num(z), exact(pr.rotation))
		}
	}
	b.WriteString("    groups: []\n    mapLabels: []\n")
	b.WriteString("  room:\n")
	fmt.Fprintf(&b, "    implicitRegionId: %s\n", p.Courtyard.ID)

	geometryStart := b.Len()
	if len(s.walls) == 0 {
		b.WriteString("    walls: []\n")
	} else {
		b.WriteString("    walls:\n")
		for _, w := range s.walls {
			fmt.Fprintf(&b, "      - id: %s\n        label: %s\n", w.id, quote(w.label))
			fmt.Fprintf(&b, "        line: { start: { x: %s, z: %s }, end: { x: %s, z: %s } }\n", num(w.sx), num(w.sz), num(w.ex), num(w.ez))
			fmt.Fprintf(&b, "        appearance: { assetRef: %s, height: %s, thickness: %s, elevation: 0 }\n",
				quote(p.Walls.AssetRef), exact(wallHeight), exact(wallThickness))
			fmt.Fprintf(&b, "        blocker: { blocksMovement: true, blocksLineOfSight: true, footprint: { width: %s, depth: %s, offsetX: 0, offsetZ: 0 } }\n",
				num(w.length), exact(blockerDepth))
			if w.door == nil {
				b.WriteString("        openings: []\n")
				continue
			}
			fmt.Fprintf(&b, "        openings:\n          - { id: %s, position: %s, width: %s, door: { id: %s, assetRef: %s } }\n",
				w.door.openingID, num(w.door.position), exact(doorWidth), w.door.id, quote(p.Walls.DoorAssetRef))
		}
	}
	b.WriteString("    regions:\n")
	for _, g := range l.regions {
		fmt.Fprintf(&b, "      - id: %s\n        name: %s\n        lighting: { intensity: %s }\n        cells:\n",
			g.id, quote(g.name), exact(g.lighting))
		cells := g.cells()
		axial := make([]dungeonspec.RoomCell, len(cells))
		for i, c := range cells {
			axial[i] = c.axial()
		}
		for _, row := range dungeonspec.CellRows(axial) {
			spans := make([]string, len(row.Q))
			for i, q := range row.Q {
				spans[i] = fmt.Sprintf("[%d, %d]", q[0], q[1])
			}
			fmt.Fprintf(&b, "          - { r: %d, q: [%s] }\n", row.R, strings.Join(spans, ", "))
		}
	}
	geometry := b.Len() - geometryStart

	blocking := 0
	for _, pr := range s.props {
		if pr.blocking {
			blocking++
		}
	}
	if blocking == 0 {
		b.WriteString("    propDeclarations: {}\n")
	} else {
		b.WriteString("    propDeclarations:\n")
		for _, pr := range s.props {
			if !pr.blocking {
				continue
			}
			fmt.Fprintf(&b, "      %s: { blocksMovement: true, blocksLineOfSight: %t, footprint: { width: %s, depth: %s, offsetX: 0, offsetZ: 0 } }\n",
				pr.id, *pr.entry.BlocksLineOfSight, exact(pr.entry.Width), exact(pr.entry.Depth))
		}
	}
	b.WriteString("    arrangementDeclarations: {}\n")
	if len(s.monsters) == 0 {
		b.WriteString("    monsterDeclarations: []\n")
	} else {
		b.WriteString("    monsterDeclarations:\n")
		for _, m := range s.monsters {
			a := m.at.axial()
			fmt.Fprintf(&b, "      - { id: %s, ref: %s, startingCell: { location: { q: %d, r: %d } } }\n", m.id, m.ref, a.Q, a.R)
		}
	}
	start := l.start.axial()
	fmt.Fprintf(&b, "    partyStart: { q: %d, r: %d }\n", start.Q, start.R)
	doors := 0
	for _, w := range s.walls {
		if w.door != nil {
			doors++
		}
	}
	if doors == 0 {
		b.WriteString("    doorBindings: {}\n")
	} else {
		b.WriteString("    doorBindings:\n")
		for _, w := range s.walls {
			if w.door != nil {
				fmt.Fprintf(&b, "      %s: {}\n", w.door.id)
			}
		}
	}
	return emitted{doc: []byte(b.String()), geometryBytes: geometry}, nil
}

// assetName is a ref's last part: `dnd5e:props:x:table_01` → `table_01`.
func assetName(ref string) string {
	if i := strings.LastIndexByte(ref, ':'); i >= 0 {
		return ref[i+1:]
	}
	return ref
}

// header is the comment that opens the file: what made it, and the options
// [OptionsFromHeader] reads back.
func header(o Options) string {
	var b strings.Builder
	b.WriteString("# The benchmark keep (rpg-project#740), generated by keepgen\n")
	b.WriteString("# (rpg-toolkit rulebooks/dnd5e/encounter/cmd/keepgen). Measurement content:\n")
	b.WriteString("# regenerate it, never edit it by hand.\n")
	b.WriteString(headerCommand + o.CommandLine + "\n")
	fmt.Fprintf(&b, "%shexes=%d seed=%d props=%s grounds-props=%s monsters=%s key=%s\n",
		headerOptions, o.Hexes, o.Seed, exact(o.PropDensity), exact(o.GroundsDensity), exact(o.MonsterDensity), o.Key)
	return b.String()
}

// OptionsFromHeader reads back the [Options] a generated file was made with,
// from its opening comment, so a shelf copy can be regenerated and compared
// byte for byte (README: keepgen.OptionsFromHeader). It refuses a document
// whose opening comment carries no `keepgen-options` line, or one whose
// values do not parse or omit a field.
func OptionsFromHeader(doc []byte) (Options, error) {
	var o Options
	var sawOptions, sawCommand bool
	for _, line := range strings.Split(string(doc), "\n") {
		if !strings.HasPrefix(line, "#") {
			break
		}
		switch {
		case strings.HasPrefix(line, headerCommand):
			o.CommandLine = strings.TrimPrefix(line, headerCommand)
			sawCommand = true
		case strings.HasPrefix(line, headerOptions):
			seen := map[string]bool{}
			for _, field := range strings.Fields(strings.TrimPrefix(line, headerOptions)) {
				k, v, ok := strings.Cut(field, "=")
				if !ok {
					return Options{}, fmt.Errorf("keepgen: header: %q is not key=value", field)
				}
				var err error
				switch k {
				case "hexes":
					o.Hexes, err = strconv.Atoi(v)
				case "seed":
					o.Seed, err = strconv.ParseInt(v, 10, 64)
				case "props":
					o.PropDensity, err = strconv.ParseFloat(v, 64)
				case "grounds-props":
					o.GroundsDensity, err = strconv.ParseFloat(v, 64)
				case "monsters":
					o.MonsterDensity, err = strconv.ParseFloat(v, 64)
				case "key":
					o.Key = v
				default:
					return Options{}, fmt.Errorf("keepgen: header: unknown option %q", k)
				}
				if err != nil {
					return Options{}, fmt.Errorf("keepgen: header: %s: %w", k, err)
				}
				seen[k] = true
			}
			for _, k := range []string{"hexes", "seed", "props", "grounds-props", "monsters", "key"} {
				if !seen[k] {
					return Options{}, fmt.Errorf("keepgen: header: option %q is missing", k)
				}
			}
			sawOptions = true
		}
	}
	if !sawOptions {
		return Options{}, fmt.Errorf("keepgen: header: no %q line in the opening comment", strings.TrimSpace(headerOptions))
	}
	if !sawCommand {
		return Options{}, fmt.Errorf("keepgen: header: no %q line in the opening comment", strings.TrimSpace(headerCommand))
	}
	return o, nil
}
