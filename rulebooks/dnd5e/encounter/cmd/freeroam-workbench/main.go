// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Command freeroam-workbench drives the free-roam encounter in a
// terminal — the pre-UI loop (the dungeonspec-workbench tradition): no
// server, no rpg-api, no Discord activity involved. It sets up the
// tomb-watch crypt (two players, a patrolling goblin, a pillar), a
// passage in its unclaimed corner leading to a second room — the
// ossuary, anchored immediately east of the crypt (#929, W2/W3) — and
// hands you the verbs. Every action renders the WORLD TRUTH beside each
// asked-for player's BELIEFS, scoped to whichever room that player
// currently stands in — capitals are current sightings, lowercase are
// ghosts at last-seen — which is the intel model made visible: step
// behind the pillar and watch yourself become a memory.
//
// SIGHT NOW CROSSES THE PASSAGE, and watching it is the point of running
// this after rpg-toolkit#1106: the two chambers compile into one canvas, so
// standing at the passage's mouth shows you the ossuary rather than nothing.
// Neither chamber walls its shared edge in this fixture, so the seam is open
// along its whole length — author a boundary on the crypt's east column to
// see a wall behave like a wall.
//
// Run from the module directory:
//
//	go run ./cmd/freeroam-workbench
//
// Commands: step <name> <x> <y> | pump | view <name> | story <name> |
// join <name> <x> <y> | exit <name> | end withdrew | save <file> |
// load <file> | atlas | status | help | quit
//
// Every coordinate is DUNGEON-ABSOLUTE — the crypt is anchored at (0,0) so its
// cells read the same either way, but the ossuary's start at x=12.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

const (
	cryptID   = "crypt"
	cryptSize = 12
	ossuaryID = "ossuary"
	ossuaryW  = 8
	ossuaryH  = 6
	passageID = "passage"
)

// patrol is the goblin's route brain: a fixed loop of waypoints.
type patrol struct {
	route []spatial.Position
	step  int
}

// Decide returns the next waypoint regardless of what the goblin
// believes — a deliberately dumb decider; the point of the workbench is
// watching the courier loop, not clever monsters.
func (p *patrol) Decide(_ encounter.Snapshot) (encounter.Intent, error) {
	target := p.route[p.step%len(p.route)]
	p.step++
	return encounter.IntentMoveTo{To: target}, nil
}

func goblinPatrol() *patrol {
	// Waypoints are authored [col,row] pairs; a decider speaks absolute
	// axial cells, so each is converted once here.
	return &patrol{route: []spatial.Position{
		cellAt(7, 10), cellAt(8, 9), cellAt(7, 8), cellAt(6, 9), cellAt(6, 10),
	}}
}

// dungeonSetup builds the tomb-watch crypt's SetupInput — split out from
// newCrypt (#929 T5 trailing) so main_test.go can construct it directly:
// the demo fixture is now smoke-tested by `go test`, not just by a human
// running the binary. This is exactly what closes the gap the workbench's
// own startup regression exposed (the ossuary room's Origin comment
// below, and TestDungeonSetupConstructs' own doc comment in
// main_test.go, both tell the story) — a future W-law addition that
// invalidates this fixture now fails CI instead of surfacing only when
// someone launches the workbench by hand.
// solidPillar is a chunk of rubble the workbench draws as '#': solid to walk
// into and solid to look through, which is what its two-cell runs were always
// meant to be. Before rpg-toolkit#1128 a room's contents could not say either,
// and this pane's '#' was drawing a thing you could stand inside.
func solidPillar(x, y float64) encounter.PropInput {
	blocks := true
	return encounter.PropInput{
		Ref:               "workbench:props:rubble",
		At:                spatial.Position{X: x, Y: y},
		BlocksMovement:    &blocks,
		BlocksLineOfSight: &blocks,
	}
}

func dungeonSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Sight: torchAndDarkvision{}, Standing: rollAllStanding{}, Initiative: rollOrderAsGiven{},
		TurnDriver: encounter.PassDriver{}, Striker: noAttacksExpected{}, Announcer: nobodyIsListening{},
		Field: encounter.FieldInput{
			// You cannot see across the space the crypt's two regions do not
			// cover — the fiction is the mountain they were cut from, and the
			// declaration is the effect (rpg-toolkit#1116). There IS such
			// space: the ossuary is half the crypt's height, so the canvas's
			// north-east corner belongs to no region. Every [col,row] below is
			// pointy-top offset, the frame a designer draws in; the
			// composition converts once (rpg-project#256).
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: layout},
			Regions: []encounter.RegionInput{
				rectRegion(cryptID, 0, 0, cryptSize, cryptSize),
				// Painted immediately east of the crypt: columns 12..19.
				rectRegion(ossuaryID, cryptSize, 0, ossuaryW, ossuaryH),
			},
			Props: []encounter.PropInput{
				solidPillar(6, 6), solidPillar(5, 6),
				solidPillar(cryptSize+4, 3),
			},
			// The seam between the two regions is walled except for the
			// passage on row 0 — a wall is an edge somebody draws now, and
			// the open door standing in the gap is what a step names.
			Walls: seamWall(cryptSize-1, cryptSize, 0, ossuaryH),
			Doors: []encounter.DoorInput{{
				ID:    passageID,
				Edges: []encounter.DoorEdge{{From: cellAt(cryptSize-1, 0), To: cellAt(cryptSize, 0)}},
				State: encounter.DoorIsOpen(),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
			{ID: "bella", Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 2}},
			{ID: "goblin", Kind: encounter.KindMonster,
				Position: spatial.Position{X: 6, Y: 10}, Decider: goblinPatrol()},
		},
		Endings: []encounter.EndingInput{
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Position: spatial.Position{X: 11, Y: 11}}},
			{Key: "withdrew", Trigger: encounter.TriggerExternal{}},
		},
	}
}

// layout is the one orientation this workbench draws in.
var layout = encounter.HexesArePointyTop()

// cellAt is the absolute axial cell an authored [col,row] pair names under
// the workbench's layout — the same conversion the composition runs.
func cellAt(col, row int) spatial.Position { return encounter.HexCellAt(layout, col, row) }

// offsetOf is cellAt run backwards, for drawing: the [col,row] an absolute
// cell sits at on a pointy-top screen.
func offsetOf(cell spatial.Position) (col, row int) {
	o := spatial.AxialToCube(cell).ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
	return int(o.X), int(o.Y)
}

// rectRegion paints a w x h rectangle of authored cells with its top-left at
// [col,row]: the shape a terminal can draw.
func rectRegion(id string, col, row, w, h int) encounter.RegionInput {
	cells := make([]spatial.Position, 0, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			cells = append(cells, spatial.Position{X: float64(col + c), Y: float64(row + r)})
		}
	}
	return encounter.RegionInput{ID: id, Name: id, Cells: cells, Archetype: "crypt", Lighting: &encounter.Lighting{Intensity: 1}}
}

// seamWall is every crossing between column west and west+1 over rows
// [0,rows), minus the straight crossing on the gap row — asked of the grid,
// since a hex's crossings stagger and a parity table would be a second
// answer to which cells touch.
func seamWall(west, east, gap, rows int) []encounter.WallInput {
	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1e6, SpanHeight: 1e6})
	var out []encounter.WallInput
	for row := 0; row < rows; row++ {
		for _, dr := range []int{-1, 0, 1} {
			to := row + dr
			if to < 0 || to >= rows || (dr == 0 && row == gap) {
				continue
			}
			if grid.Distance(cellAt(west, row), cellAt(east, to)) != 1 {
				continue
			}
			out = append(out, encounter.WallInput{Boundary: spatial.Boundary{
				From: spatial.Position{X: float64(west), Y: float64(row)}, To: spatial.Position{X: float64(east), Y: float64(to)},
				BlocksMovement: true, BlocksLineOfSight: true,
			}})
		}
	}
	return out
}

func newCrypt() (*encounter.Encounter, error) {
	return encounter.NewEncounter(dungeonSetup())
}

// pane is one region's authored bounding box, which is what a terminal
// can draw: the columns and rows its cells span, and the cells themselves.
type pane struct {
	colMin, rowMin, width, height int
	floor                         map[[2]int]bool
}

// paneOf reads a region's cells off the persisted field. Nil if the field
// has no such region (a defect the caller should treat as "nothing to
// render").
func paneOf(data encounter.EncounterData, region string) *pane {
	for _, r := range data.Field.Regions {
		if r.ID != region || len(r.Cells) == 0 {
			continue
		}
		p := &pane{colMin: int(r.Cells[0].X), rowMin: int(r.Cells[0].Y), floor: map[[2]int]bool{}}
		colMax, rowMax := p.colMin, p.rowMin
		for _, c := range r.Cells {
			col, row := int(c.X), int(c.Y)
			p.floor[[2]int{col, row}] = true
			p.colMin, colMax = min(p.colMin, col), max(colMax, col)
			p.rowMin, rowMax = min(p.rowMin, row), max(rowMax, row)
		}
		p.width, p.height = colMax-p.colMin+1, rowMax-p.rowMin+1
		return p
	}
	return nil
}

// blank is the pane with its floor drawn as '.' and everything else as ' '.
func (p *pane) blank() []string {
	rows := make([]string, p.height)
	for r := range rows {
		row := make([]byte, p.width)
		for c := range row {
			row[c] = ' '
			if p.floor[[2]int{p.colMin + c, p.rowMin + r}] {
				row[c] = '.'
			}
		}
		rows[r] = string(row)
	}
	return rows
}

// mark draws one absolute cell on the pane, or nothing if it is off it.
func (p *pane) mark(grid []string, cell spatial.Position, ch byte) {
	col, row := offsetOf(cell)
	set(grid, float64(col-p.colMin), float64(row-p.rowMin), ch)
}

// props draws the field's props that fall on this pane.
func (p *pane) props(grid []string, data encounter.EncounterData) {
	for _, o := range data.Field.Props {
		set(grid, o.At.X-float64(p.colMin), o.At.Y-float64(p.rowMin), '#')
	}
}

// worldGrid renders ground truth for ONE region — the HOST's view, which is
// exactly what a game server holds. The panes stay region-scoped because a
// terminal is small, not because sight is: since rpg-toolkit#1106 a belief
// grid may hold sightings in the region next door, which this pane drops.
func worldGrid(data encounter.EncounterData, members []encounter.Member, region string) []string {
	p := paneOf(data, region)
	if p == nil {
		return nil
	}
	grid := p.blank()
	p.props(grid, data)
	for _, m := range members {
		if m.Region != region {
			continue
		}
		p.mark(grid, m.Position, initialOf(string(m.ID), true))
	}
	if region == cryptID {
		set(grid, 11, 11, '>')
	}
	return grid
}

// beliefGrid renders one member's intel, scoped to the region they
// currently stand in: what they currently see in capitals, their ghosts in
// lowercase at last-seen (a ghost held from a region the member has since
// left renders nowhere — this grid only shows their present region). Their
// own position comes from world truth (you always know where you stand).
func beliefGrid(
	enc *encounter.Encounter, data encounter.EncounterData, members []encounter.Member,
	who core.EntityID, region string,
) ([]string, error) {
	view, err := enc.View(&encounter.ViewInput{Member: who})
	if err != nil {
		return nil, err
	}
	p := paneOf(data, region)
	if p == nil {
		return nil, nil
	}
	grid := p.blank()
	p.props(grid, data)
	for _, h := range view {
		var sp encounter.SightPayload
		if err := json.Unmarshal(h.Payload, &sp); err != nil {
			continue
		}
		// Sight payloads are dungeon-absolute; mark drops anything off
		// this pane.
		p.mark(grid, spatial.Position{X: sp.X, Y: sp.Y}, initialOf(string(h.Subject), h.Status == intel.Current))
	}
	for _, m := range members {
		if m.ID == who {
			p.mark(grid, m.Position, '@')
		}
	}
	return grid, nil
}

func set(grid []string, x, y float64, ch byte) {
	xi, yi := int(x), int(y)
	if yi < 0 || yi >= len(grid) || xi < 0 || xi >= len(grid[yi]) {
		return
	}
	row := []byte(grid[yi])
	row[xi] = ch
	grid[yi] = string(row)
}

func initialOf(name string, current bool) byte {
	if name == "" {
		return '?'
	}
	c := name[0]
	if current {
		return byte(strings.ToUpper(string(c))[0])
	}
	return byte(strings.ToLower(string(c))[0])
}

func printSideBySide(left, right []string, leftTitle, rightTitle string) {
	width := cryptSize
	if len(left) > 0 {
		width = len(left[0])
	}
	fmt.Printf("  %-*s   %s\n", width, leftTitle, rightTitle)
	for i := range left {
		fmt.Printf("  %s   %s\n", left[i], right[i])
	}
}

func printStory(enc *encounter.Encounter, who core.EntityID) {
	story, err := enc.Story(&encounter.StoryInput{Audience: who})
	if err != nil {
		fmt.Println("  story:", err)
		return
	}
	for _, e := range story {
		var beat map[string]any
		_ = json.Unmarshal(e.Payload, &beat)
		fmt.Printf("  %3d  %v\n", e.Seq, beat)
	}
}

func printStatus(enc *encounter.Encounter) {
	st, err := enc.Status()
	if err != nil {
		fmt.Println("  status:", err)
		return
	}
	if st.Open {
		fmt.Println("  open — the delve continues")
		return
	}
	fmt.Printf("  CLOSED — ending %q; final positions:\n", st.Outcome.Ending)
	for _, m := range st.Outcome.Members {
		fmt.Printf("    %s at (%g,%g)\n", m.ID, m.Position.X, m.Position.Y)
	}
}

// printAtlas renders the dungeon in world space: every region's cells as a
// count and an authored extent, every door's crossing as the two absolute
// cells it joins and the distance between them (always 1 for anything the
// composition constructed; an edge joins adjacent cells by law).
func printAtlas(enc *encounter.Encounter) {
	atlas, err := enc.Atlas()
	if err != nil {
		fmt.Println("  atlas:", err)
		return
	}
	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 100000, SpanHeight: 100000})
	fmt.Printf("  ATLAS — the dungeon in absolute space (%d floor cells, %s)\n", len(atlas.Cells), atlas.Orientation.Kind())
	for _, r := range atlas.Regions {
		colMin, colMax, rowMin, rowMax := regionExtent(r)
		fmt.Printf("    region %-10s %3d cells  %-8s lit %.2f  columns %d..%d rows %d..%d\n",
			r.ID, len(r.Cells), r.Archetype, r.Lighting.Intensity, colMin, colMax, rowMin, rowMax)
	}
	for _, d := range atlas.Doorways {
		fmt.Printf("    doorway %-10s (%g,%g) -- (%g,%g)  distance %g — they kiss\n",
			d.Door, d.From.X, d.From.Y, d.To.X, d.To.Y, grid.Distance(d.From, d.To))
	}
}

// regionExtent is the authored columns and rows a region's cells span — read
// back off the absolute cells, since the Atlas speaks axial.
func regionExtent(r encounter.AtlasRegion) (colMin, colMax, rowMin, rowMax int) {
	for i, c := range r.Cells {
		col, row := offsetOf(c)
		if i == 0 {
			colMin, colMax, rowMin, rowMax = col, col, row, row
			continue
		}
		colMin, colMax = min(colMin, col), max(colMax, col)
		rowMin, rowMax = min(rowMin, row), max(rowMax, row)
	}
	return colMin, colMax, rowMin, rowMax
}

const legend = `  @ you   A/B/C capitals: seen NOW   a/b/g lowercase: ghost at last-seen
  # pillar/sarcophagus   > the stairs down (reach them and the encounter closes)
  doorway "passage": [11,0] <-> [12,0] on the map — step from one to the
  other like any other cell; the view flips to whichever region you land in
  every <x> <y> you type is a column and row as drawn; the world speaks axial`

const commands = `  step <name> <x> <y>   walk one cell (column, row as drawn; the world holds
                        still — you pump it)
  pump                  a tick passes: the goblin patrols, sights refresh
  view <name>           world truth beside <name>'s beliefs
  story <name>          the record, as <name> is allowed to hear it
  join <name> <x> <y>   a late player joins the ambient
  exit <name>           a player heads back to town (carry-forward prints)
  end withdrew          the party calls the delve off (External ending)
  save <file> / load <file>   the ONE aggregate blob, round-tripped
  atlas                 the dungeon in absolute space — each region's anchor
                        and span, every doorway's kissing pair made visible
  status | help | quit`

func main() {
	enc, err := newCrypt()
	if err != nil {
		fmt.Fprintln(os.Stderr, "setup:", err)
		os.Exit(1)
	}
	fmt.Println("THE TOMB WATCH — free-roam encounter workbench")
	fmt.Println(legend)
	fmt.Println()
	fmt.Println(commands)
	fmt.Println()
	showView(enc, "alice")

	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !sc.Scan() {
			return
		}
		args := strings.Fields(sc.Text())
		if len(args) == 0 {
			continue
		}
		switch args[0] {
		case "quit", "q":
			return
		case "help":
			fmt.Println(commands)
		case "status":
			printStatus(enc)
		case "atlas":
			printAtlas(enc)
		case "view":
			if len(args) == 2 {
				showView(enc, core.EntityID(args[1]))
			}
		case "story":
			if len(args) == 2 {
				printStory(enc, core.EntityID(args[1]))
			}
		case "step":
			if len(args) != 4 {
				fmt.Println("  step <name> <x> <y>   (column, row as drawn)")
				continue
			}
			x, y := num(args[2]), num(args[3])
			out, err := enc.Step(&encounter.StepInput{Member: core.EntityID(args[1]), To: cellAt(int(x), int(y))})
			if err != nil {
				fmt.Println(" ", err)
				continue
			}
			for _, d := range out.Doors {
				fmt.Printf("  through %s (%s)\n", d.ID, d.State)
			}
			showView(enc, core.EntityID(args[1]))
			if out.Outcome != nil {
				printStatus(enc)
			}
		case "pump":
			out, err := enc.Pump(&encounter.PumpInput{})
			if err != nil {
				fmt.Println(" ", err)
				continue
			}
			fmt.Printf("  tick %d", out.Tick)
			for _, mv := range out.MonsterMoves {
				// Absolute cells, same as the atlas above prints — a prowl in
				// the ossuary (anchored at (12,0)) reads on the same map as
				// one in the crypt (rpg-toolkit#1062).
				fmt.Printf("; %s prowls (%g,%g)->(%g,%g) on the map", mv.Member, mv.From.X, mv.From.Y, mv.To.X, mv.To.Y)
			}
			fmt.Println()
			if out.Outcome != nil {
				printStatus(enc)
			}
		case "join":
			if len(args) != 4 {
				fmt.Println("  join <name> <x> <y>")
				continue
			}
			_, err := enc.Join(&encounter.JoinInput{
				Member: core.EntityID(args[1]), Kind: encounter.KindPlayer,
				Cell: cellAt(int(num(args[2])), int(num(args[3]))),
			})
			if err != nil {
				fmt.Println(" ", err)
				continue
			}
			showView(enc, core.EntityID(args[1]))
		case "exit":
			if len(args) != 2 {
				fmt.Println("  exit <name>")
				continue
			}
			out, err := enc.Exit(&encounter.ExitInput{Member: core.EntityID(args[1])})
			if err != nil {
				fmt.Println(" ", err)
				continue
			}
			fmt.Printf("  %s departs from (%g,%g) carrying %d held beliefs\n",
				args[1], out.Outcome.Position.X, out.Outcome.Position.Y, len(out.Carry))
			if out.Closed != nil {
				printStatus(enc)
			}
		case "end":
			if len(args) != 2 {
				fmt.Println("  end withdrew")
				continue
			}
			if _, err := enc.End(&encounter.EndInput{Ending: args[1]}); err != nil {
				fmt.Println(" ", err)
				continue
			}
			printStatus(enc)
		case "save":
			if len(args) != 2 {
				fmt.Println("  save <file>")
				continue
			}
			bs, err := json.MarshalIndent(enc.ToData(), "", "  ")
			if err != nil {
				fmt.Println(" ", err)
				continue
			}
			if err := os.WriteFile(args[1], bs, 0o644); err != nil {
				fmt.Println(" ", err)
				continue
			}
			fmt.Printf("  saved: one aggregate blob, %d bytes — this is what rpg-api would store\n", len(bs))
		case "load":
			if len(args) != 2 {
				fmt.Println("  load <file>")
				continue
			}
			bs, err := os.ReadFile(args[1])
			if err != nil {
				fmt.Println(" ", err)
				continue
			}
			var data encounter.EncounterData
			if err := json.Unmarshal(bs, &data); err != nil {
				fmt.Println(" ", err)
				continue
			}
			loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
				Sight: torchAndDarkvision{}, Standing: rollAllStanding{}, Initiative: rollOrderAsGiven{}, TurnDriver: encounter.PassDriver{}, Striker: noAttacksExpected{}, Announcer: nobodyIsListening{}, Data: data, Deciders: map[encounter.MemberID]encounter.Decider{
					"goblin": goblinPatrol(),
				}})
			if err != nil {
				fmt.Println(" ", err)
				continue
			}
			enc = loaded
			fmt.Println("  reloaded — beliefs traveled as state; the patrol re-attached as behavior")
			showView(enc, "alice")
		default:
			fmt.Println("  ? try: help")
		}
	}
}

func showView(enc *encounter.Encounter, who core.EntityID) {
	data := enc.ToData()
	members, err := enc.Members()
	if err != nil {
		fmt.Println(" ", err)
		return
	}
	room := ""
	for _, m := range members {
		if m.ID == who {
			room = m.Region
		}
	}
	if room == "" {
		fmt.Println(" ", who, "is not a member")
		return
	}
	belief, err := beliefGrid(enc, data, members, who, room)
	if err != nil {
		fmt.Println(" ", err)
		return
	}
	title := fmt.Sprintf("WORLD TRUTH (%s)", room)
	printSideBySide(worldGrid(data, members, room), belief, title, fmt.Sprintf("%s BELIEVES", strings.ToUpper(string(who))))
}

func num(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// rollOrderAsGiven is the workbench's initiative roller: it keeps the order it
// is handed. The workbench is a deterministic demo, so a shuffle would make
// its transcript differ run to run for no gain.
type rollOrderAsGiven struct{}

func (rollOrderAsGiven) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return members, nil
}

// rollAllStanding is the workbench's Standing capability: nobody is ever down.
// The workbench drives free roam and sight, not damage — nothing in it can take
// a member to zero — so the honest answer is that everyone is on their feet,
// said out loud because the capability is required.
type rollAllStanding struct{}

func (rollAllStanding) Standing([]encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

// nobodyIsListening is the workbench's Announcer capability. The workbench
// drives clocks — turns end here — so unlike noAttacksExpected this really is
// called. It succeeds silently: there is no rulebook attached to this scene,
// so there is nothing a turn boundary could mean to anything in it.
type nobodyIsListening struct{}

func (nobodyIsListening) Announce(
	context.Context, *encounter.Encounter, []encounter.Boundary,
) error {
	return nil
}

// noAttacksExpected is the workbench's Striker capability. The workbench
// demonstrates free roam and sight, not combat — its TurnDriver is
// [encounter.PassDriver], which never returns an Attack intent — so this is
// never actually called; it exists only because the capability is required
// (rpg-toolkit#1033, rpg-project#254) and says so honestly rather than
// returning a fabricated hit.
type noAttacksExpected struct{}

func (noAttacksExpected) Strike(context.Context, *encounter.Encounter, encounter.MemberID, encounter.MemberID, core.Ref) error {
	return errors.New("workbench: no driver here ever attacks")
}

// torchAndDarkvision is the workbench's Sight capability, and it is the one
// place in this module where the capability answers something a rulebook would
// actually say (rpg-toolkit#1111).
//
// The party carries a torch: 40 feet of light, bright then dim, which is 8
// cells. The goblin has darkvision: 60 feet, which is 12. Nothing about the
// composition knows either of those sentences — it asks, and gets two numbers
// — which is exactly the promise the capability makes. Swap this type for one
// that reads a character sheet and the workbench gets a real light model with
// no other line changing.
//
// The asymmetry is the demo. Walk the party far enough down the crypt and the
// goblin sees them from a distance they cannot see back from: the bubble forms
// spotted, and they enter it surprised.
type torchAndDarkvision struct{}

func (torchAndDarkvision) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	reach := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		if id == "goblin" {
			reach[id] = 12 // 60 feet of darkvision

			continue
		}
		reach[id] = 8 // 40 feet of torchlight
	}

	return reach, nil
}
