// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// The sides of the courtyard a room can sit on.
const (
	SideNorth = "north"
	SideEast  = "east"
	SideSouth = "south"
	SideWest  = "west"
)

// The room kinds. A kind selects its palette (plan G11); `grounds` is also
// the palette of the courtyard and the two grounds.
const (
	KindHall     = "hall"
	KindService  = "service"
	KindQuarters = "quarters"
	KindLibrary  = "library"
	KindChapel   = "chapel"
	KindPrison   = "prison"
	KindPassage  = "passage"
	KindGrounds  = "grounds"
)

var (
	kinds = map[string]bool{
		KindHall: true, KindService: true, KindQuarters: true, KindLibrary: true,
		KindChapel: true, KindPrison: true, KindPassage: true, KindGrounds: true,
	}
	sides = map[string]bool{SideNorth: true, SideEast: true, SideSouth: true, SideWest: true}

	// idPattern is every id the plan names: it becomes part of region, wall,
	// door, prop and monster ids in the site.
	idPattern = regexp.MustCompile(`^[a-z0-9-]+$`)
)

// Footprint bounds a palette entry is held to: the dialect's own per-prop
// footprint side limits (propDeclarations footprint sides 0.1–12).
const (
	minFootprintSide = 0.1
	maxFootprintSide = 12
)

//go:embed plan/keep.yaml
var embeddedPlan []byte

// Plan is the keep as a floor plan (design Law Plan; README ownership
// table): rooms by side with a footprint at keep scale and a lighting, the
// courtyard, the west and south grounds, the party start and the exits by
// room, doors as room pairs, the wall and door appearance, a prop and monster
// palette per room kind, and the stat blocks the monster palettes name.
//
// IT CARRIES NO CELLS, COORDINATES OR COUNTS. The generator derives them, so
// the plan reads like a floor plan and a layout change is an edit to it.
type Plan struct {
	// Name is the site's name: the scene's and the room's.
	Name string `yaml:"name"`

	// Rooms are the keep's rooms. On the north and south sides they are
	// listed west to east and share one depth; on the west and east sides
	// they are listed north to south and share one width.
	Rooms []Room `yaml:"rooms"`

	// Courtyard is the open middle of the keep. Its size is what the four
	// sides leave.
	Courtyard Area `yaml:"courtyard"`

	// Grounds are the open land west of and south of the keep.
	Grounds Grounds `yaml:"grounds"`

	// PartyStart is the id of the room the party starts in, at its centre.
	PartyStart string `yaml:"partyStart"`

	// Exits are the ways out, each at the middle of its room's outer edge.
	Exits []Exit `yaml:"exits"`

	// Doors are pairs of region ids (rooms, the courtyard, the grounds): one
	// door, bound open, on the wall the two share.
	Doors [][]string `yaml:"doors"`

	// Walls is how every wall and door is drawn (plan G13).
	Walls WallAppearance `yaml:"walls"`

	// Palettes are the props and monsters each room kind draws from, keyed
	// by kind (plan G11, G7). Every kind a room uses needs one, and so does
	// `grounds`, which the courtyard and grounds draw from.
	Palettes map[string]Palette `yaml:"palettes"`

	// Templates are the stat blocks the monster palettes name, carried into
	// the site's root `templates` verbatim so a launch resolves
	// `dnd5e:monsters:<id>` from the file. Every palette monster needs one;
	// a plan with no monsters needs none.
	Templates map[string]dungeonspec.TemplateSpec `yaml:"templates"`
}

// Room is one room of the keep.
type Room struct {
	// ID names the room's region; lowercase letters, digits and dashes.
	ID string `yaml:"id"`
	// Name is the region's display name.
	Name string `yaml:"name"`
	// Kind selects the palette; one of the Kind constants.
	Kind string `yaml:"kind"`
	// Width is the room's west–east extent in hexes at keep scale.
	Width int `yaml:"width"`
	// Depth is the room's north–south extent in hexes at keep scale.
	Depth int `yaml:"depth"`
	// Side is the side of the courtyard the room sits on.
	Side string `yaml:"side"`
	// Lighting is the region's light intensity, 0 to 1 (plan G8).
	Lighting *float64 `yaml:"lighting"`
}

// Area is the courtyard: a region whose size the plan does not state.
type Area struct {
	// ID names the courtyard's region.
	ID string `yaml:"id"`
	// Name is the region's display name.
	Name string `yaml:"name"`
	// Lighting is the region's light intensity, 0 to 1.
	Lighting *float64 `yaml:"lighting"`
}

// Grounds are the open land outside the keep.
type Grounds struct {
	// West runs the keep's height on its west side.
	West WestGrounds `yaml:"west"`
	// South runs under the keep and the west grounds, and absorbs whatever
	// the requested hex count leaves (design R2).
	South SouthGrounds `yaml:"south"`
}

// WestGrounds is the grounds region west of the keep.
type WestGrounds struct {
	// ID names the region.
	ID string `yaml:"id"`
	// Name is the region's display name.
	Name string `yaml:"name"`
	// Width is its west–east extent in hexes at keep scale.
	Width int `yaml:"width"`
	// Lighting is the region's light intensity, 0 to 1.
	Lighting *float64 `yaml:"lighting"`
}

// SouthGrounds is the grounds region south of the keep.
type SouthGrounds struct {
	// ID names the region.
	ID string `yaml:"id"`
	// Name is the region's display name.
	Name string `yaml:"name"`
	// Depth is its north–south extent in hexes at keep scale. At any other
	// scale the depth is whatever makes the floor the requested count.
	Depth int `yaml:"depth"`
	// Lighting is the region's light intensity, 0 to 1.
	Lighting *float64 `yaml:"lighting"`
}

// Exit is one way out of the site.
type Exit struct {
	// ID is the exit's id in the site.
	ID string `yaml:"id"`
	// Room is the room it leaves from, at the middle of its outer edge.
	Room string `yaml:"room"`
}

// WallAppearance is the asset every wall and door is drawn with.
type WallAppearance struct {
	// AssetRef is the wall's repeating piece.
	AssetRef string `yaml:"assetRef"`
	// DoorAssetRef is every door's asset.
	DoorAssetRef string `yaml:"doorAssetRef"`
}

// Palette is what one room kind is furnished and garrisoned from.
type Palette struct {
	// Props are the props the kind draws from. Empty means the kind takes
	// none.
	Props []PropEntry `yaml:"props"`
	// Monsters are full monster refs (`dnd5e:monsters:<id>`). Empty means
	// the kind takes none.
	Monsters []string `yaml:"monsters"`
}

// PropEntry is one prop a palette can place.
type PropEntry struct {
	// AssetRef is the prop's full catalog ref; the web's import check is
	// where an unknown one is refused (README refusals).
	AssetRef string `yaml:"assetRef"`
	// Width is the footprint's extent along the prop's local x, scene units.
	Width float64 `yaml:"width"`
	// Depth is the footprint's extent along the prop's local z, scene units.
	Depth float64 `yaml:"depth"`
	// BlocksMovement marks a blocker. A blocker carries a propDeclaration;
	// an entry that blocks nothing is dressing and carries none.
	BlocksMovement *bool `yaml:"blocksMovement"`
	// BlocksLineOfSight is applied as written (plan G3).
	BlocksLineOfSight *bool `yaml:"blocksLineOfSight"`
}

// DefaultPlan is the embedded keep (`plan/keep.yaml`). It panics if the
// embedded file does not load, which TestLoadPlan_Embedded rules out for
// every build.
func DefaultPlan() *Plan {
	p, err := LoadPlan(bytes.NewReader(embeddedPlan))
	if err != nil {
		panic(fmt.Sprintf("keepgen: the embedded plan does not load: %v", err))
	}
	return p
}

// LoadPlan decodes a plan strictly and judges it. An unknown key anywhere is
// refused by name; so is every missing field, a room kind or side outside
// the known sets, an id that is not lowercase letters, digits and dashes or
// is used twice, a door, exit or party start naming no region, a door pair
// written twice, a palette missing for a kind in use, a palette monster
// whose `dnd5e:monsters:<id>` has no stat block under `templates` (the
// launch resolves it from the site's own templates and nowhere else; a
// rulebook base is named by a template that states only its `base`), and a
// palette entry
// whose footprint is outside 0.1–12 or whose blocking answers are absent.
// Every defect is reported, each prefixed by its path.
//
// Whether the sides close into a keep is [layoutOf]'s judgement, made at
// every size; LoadPlan judges the plan as data.
func LoadPlan(r io.Reader) (*Plan, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	var p Plan
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("keepgen: plan: %w", err)
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// planErrors collects every defect, in the order found.
type planErrors []string

func (e *planErrors) add(path, format string, args ...any) {
	*e = append(*e, path+": "+fmt.Sprintf(format, args...))
}

func (e planErrors) err() error {
	if len(e) == 0 {
		return nil
	}
	return errors.New("keepgen: plan: " + strings.Join(e, "; "))
}

func (p *Plan) validate() error {
	var errs planErrors
	ids := map[string]string{} // id -> path that declared it
	declare := func(path, id string) {
		switch {
		case id == "":
			errs.add(path+".id", "is required")
		case !idPattern.MatchString(id):
			errs.add(path+".id", "%q must be lowercase letters, digits and dashes", id)
		case ids[id] != "":
			errs.add(path+".id", "%q is already %s", id, ids[id])
		default:
			ids[id] = path
		}
	}
	required := func(path, v string) {
		if v == "" {
			errs.add(path, "is required")
		}
	}
	lighting := func(path string, v *float64) {
		switch {
		case v == nil:
			errs.add(path, "is required")
		case !(*v >= 0 && *v <= 1):
			errs.add(path, "must be between 0 and 1")
		}
	}
	positive := func(path string, v int) {
		if v <= 0 {
			errs.add(path, "must be a positive number of hexes")
		}
	}

	required("name", p.Name)
	if len(p.Rooms) == 0 {
		errs.add("rooms", "is required")
	}
	used := map[string]bool{KindGrounds: true}
	for i, room := range p.Rooms {
		path := fmt.Sprintf("rooms[%d]", i)
		declare(path, room.ID)
		required(path+".name", room.Name)
		switch {
		case room.Kind == "":
			errs.add(path+".kind", "is required")
		case !kinds[room.Kind]:
			errs.add(path+".kind", "%q is not a room kind (hall, service, quarters, library, chapel, prison, passage, grounds)", room.Kind)
		default:
			used[room.Kind] = true
		}
		positive(path+".width", room.Width)
		positive(path+".depth", room.Depth)
		switch {
		case room.Side == "":
			errs.add(path+".side", "is required")
		case !sides[room.Side]:
			errs.add(path+".side", "%q is not a side (north, east, south, west)", room.Side)
		}
		lighting(path+".lighting", room.Lighting)
	}
	declare("courtyard", p.Courtyard.ID)
	required("courtyard.name", p.Courtyard.Name)
	lighting("courtyard.lighting", p.Courtyard.Lighting)
	declare("grounds.west", p.Grounds.West.ID)
	required("grounds.west.name", p.Grounds.West.Name)
	positive("grounds.west.width", p.Grounds.West.Width)
	lighting("grounds.west.lighting", p.Grounds.West.Lighting)
	declare("grounds.south", p.Grounds.South.ID)
	required("grounds.south.name", p.Grounds.South.Name)
	positive("grounds.south.depth", p.Grounds.South.Depth)
	lighting("grounds.south.lighting", p.Grounds.South.Lighting)

	isRoom := map[string]bool{}
	for _, room := range p.Rooms {
		isRoom[room.ID] = true
	}
	switch {
	case p.PartyStart == "":
		errs.add("partyStart", "is required")
	case !isRoom[p.PartyStart]:
		errs.add("partyStart", "%q is not a room", p.PartyStart)
	}
	exitIDs := map[string]bool{}
	for i, ex := range p.Exits {
		path := fmt.Sprintf("exits[%d]", i)
		switch {
		case ex.ID == "":
			errs.add(path+".id", "is required")
		case !idPattern.MatchString(ex.ID):
			errs.add(path+".id", "%q must be lowercase letters, digits and dashes", ex.ID)
		case exitIDs[ex.ID]:
			errs.add(path+".id", "%q is already an exit", ex.ID)
		}
		exitIDs[ex.ID] = true
		switch {
		case ex.Room == "":
			errs.add(path+".room", "is required")
		case !isRoom[ex.Room]:
			errs.add(path+".room", "%q is not a room", ex.Room)
		}
	}
	pairs := map[[2]string]bool{}
	for i, door := range p.Doors {
		path := fmt.Sprintf("doors[%d]", i)
		if len(door) != 2 {
			errs.add(path, "must name exactly two regions")
			continue
		}
		known := true
		for j, id := range door {
			if ids[id] == "" {
				errs.add(fmt.Sprintf("%s[%d]", path, j), "%q is not a room, the courtyard or a grounds", id)
				known = false
			}
		}
		if !known {
			continue
		}
		if door[0] == door[1] {
			errs.add(path, "joins %q to itself", door[0])
			continue
		}
		key := [2]string{door[0], door[1]}
		if key[0] > key[1] {
			key[0], key[1] = key[1], key[0]
		}
		if pairs[key] {
			errs.add(path, "%q and %q already have a door", door[0], door[1])
		}
		pairs[key] = true
	}
	required("walls.assetRef", p.Walls.AssetRef)
	required("walls.doorAssetRef", p.Walls.DoorAssetRef)

	declared := map[string]bool{}
	for kind := range p.Palettes {
		declared[kind] = true
	}
	for _, kind := range sortedKeys(declared) {
		if !kinds[kind] {
			errs.add("palettes."+kind, "is not a room kind")
		}
	}
	for _, kind := range sortedKeys(used) {
		pal, ok := p.Palettes[kind]
		if !ok {
			errs.add("palettes."+kind, "is required: a region uses kind %q", kind)
			continue
		}
		for i, e := range pal.Props {
			path := fmt.Sprintf("palettes.%s.props[%d]", kind, i)
			required(path+".assetRef", e.AssetRef)
			for _, side := range []struct {
				name string
				v    float64
			}{{"width", e.Width}, {"depth", e.Depth}} {
				if !(side.v >= minFootprintSide && side.v <= maxFootprintSide) {
					errs.add(path+"."+side.name, "must be between 0.1 and 12")
				}
			}
			if e.BlocksMovement == nil {
				errs.add(path+".blocksMovement", "is required")
			}
			if e.BlocksLineOfSight == nil {
				errs.add(path+".blocksLineOfSight", "is required")
			}
		}
		for i, ref := range pal.Monsters {
			path := fmt.Sprintf("palettes.%s.monsters[%d]", kind, i)
			id := strings.TrimPrefix(ref, monsterRefPrefix)
			switch {
			case !strings.HasPrefix(ref, monsterRefPrefix) || id == "":
				errs.add(path, "%q must be a full ref, %s<id>", ref, monsterRefPrefix)
			case p.Templates[id].Base == "":
				errs.add(path, "%q has no stat block in templates: a launch resolves a monster only from the site's own templates", ref)
			}
		}
	}
	return errs.err()
}

const monsterRefPrefix = "dnd5e:monsters:"

// sortedKeys lists a set's keys in order, so every refusal comes out in the
// same order.
func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
