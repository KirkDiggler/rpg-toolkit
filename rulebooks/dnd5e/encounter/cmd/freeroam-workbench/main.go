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
// ghosts at last-seen — which is the intel model made visible: move
// behind the pillar and watch yourself become a memory, or step through
// the passage and watch the crypt itself fade to nothing (T3 — sight
// never crosses a connection). The atlas command makes the OTHER half
// visible: the two rooms placed in one continuous absolute space, the
// passage's kissing pair made concrete (#929 T5).
//
// Run from the module directory:
//
//	go run ./cmd/freeroam-workbench
//
// Commands: move <name> <x> <y> | traverse <name> <connection> | pump |
// view <name> | story <name> | join <name> <x> <y> | exit <name> |
// end withdrew | save <file> | load <file> | atlas | status | help | quit
package main

import (
	"bufio"
	"encoding/json"
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
	return &patrol{route: []spatial.Position{
		{X: 7, Y: 10}, {X: 8, Y: 9}, {X: 7, Y: 8}, {X: 6, Y: 9}, {X: 6, Y: 10},
	}}
}

// dungeonSetup builds the tomb-watch crypt's SetupInput — split out from
// newCrypt (#929 T5 trailing) so main_test.go can construct it directly:
// the demo fixture is now smoke-tested by `go test`, not just by a human
// running the binary. This is exactly what closes the gap the workbench's
// own startup regression exposed (see this file's package doc comment) —
// a future W-law addition that invalidates this fixture now fails CI
// instead of surfacing only when someone launches the workbench by hand.
func dungeonSetup() *encounter.SetupInput {
	return &encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{
					ID: cryptID, Width: cryptSize, Height: cryptSize,
					Occluders: []spatial.Position{{X: 6, Y: 6}, {X: 5, Y: 6}},
				},
				{
					// Anchored immediately east of the crypt (#929): the
					// passage's endpoints, crypt (11,0) and ossuary local
					// (0,0)+Origin(12,0)=(12,0), are Chebyshev-adjacent
					// (W3), and the rooms' absolute footprints — crypt
					// x:[0,11], ossuary x:[12,19] — stay disjoint (W2).
					// Before this Origin existed, both rooms defaulted to
					// (0,0) and W2 rejected the field outright — the
					// workbench crashed on startup (`setup: newencounter:
					// room "crypt" and room "ossuary" overlap at absolute
					// cell (0, 0): no field`) until this fix.
					ID: ossuaryID, Width: ossuaryW, Height: ossuaryH,
					Origin:    spatial.Position{X: 12, Y: 0},
					Occluders: []spatial.Position{{X: 4, Y: 3}},
				},
			},
			Connections: []encounter.ConnectionInput{
				{
					ID: passageID, From: cryptID, To: ossuaryID,
					FromPosition: spatial.Position{X: 11, Y: 0},
					ToPosition:   spatial.Position{X: 0, Y: 0},
				},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: cryptID, Position: spatial.Position{X: 2, Y: 2}},
			{ID: "bella", Kind: encounter.KindPlayer, Room: cryptID, Position: spatial.Position{X: 3, Y: 2}},
			{ID: "goblin", Kind: encounter.KindMonster, Room: cryptID,
				Position: spatial.Position{X: 6, Y: 10}, Decider: goblinPatrol()},
		},
		Endings: []encounter.EndingInput{
			{Key: "stairs", Trigger: encounter.TriggerReachedPosition{
				Room: cryptID, Position: spatial.Position{X: 11, Y: 11}}},
			{Key: "withdrew", Trigger: encounter.TriggerExternal{}},
		},
	}
}

func newCrypt() (*encounter.Encounter, error) {
	return encounter.NewEncounter(dungeonSetup())
}

// roomByID finds a room's data by ID — nil if the field has no such
// room (a defect the caller should treat as "nothing to render").
func roomByID(data encounter.EncounterData, room string) *encounter.RoomData {
	for i := range data.Field.Rooms {
		if data.Field.Rooms[i].ID == room {
			return &data.Field.Rooms[i]
		}
	}
	return nil
}

// worldGrid renders ground truth for ONE room from the aggregate
// snapshot — the HOST's view, which is exactly what a game server
// holds. Sight is room-scoped (T3), so belief grids are room-scoped
// too; world truth follows the same shape for a fair side-by-side.
func worldGrid(data encounter.EncounterData, room string) []string {
	r := roomByID(data, room)
	if r == nil {
		return nil
	}
	grid := blankGrid(r.Width, r.Height)
	for _, o := range r.Occluders {
		set(grid, o.X, o.Y, '#')
	}
	for _, m := range data.Members {
		if m.Room != room {
			continue
		}
		set(grid, m.Position.X, m.Position.Y, initialOf(string(m.ID), true))
	}
	if room == cryptID {
		set(grid, 11, 11, '>')
	}
	return grid
}

// beliefGrid renders one member's intel, scoped to the room they
// currently stand in: what they currently see in capitals, their
// ghosts in lowercase at last-seen (a ghost held from a room the
// member has since left renders nowhere — this grid only shows their
// present room). Their own position comes from world truth (you
// always know where you stand).
func beliefGrid(enc *encounter.Encounter, data encounter.EncounterData, who core.EntityID, room string) ([]string, error) {
	view, err := enc.View(&encounter.ViewInput{Member: who})
	if err != nil {
		return nil, err
	}
	r := roomByID(data, room)
	if r == nil {
		return nil, nil
	}
	grid := blankGrid(r.Width, r.Height)
	for _, o := range r.Occluders {
		set(grid, o.X, o.Y, '#')
	}
	for _, h := range view {
		var p encounter.SightPayload
		if err := json.Unmarshal(h.Payload, &p); err != nil {
			continue
		}
		if p.Room != room {
			continue
		}
		set(grid, p.X, p.Y, initialOf(string(h.Subject), h.Status == intel.Current))
	}
	for _, m := range data.Members {
		if m.ID == who {
			set(grid, m.Position.X, m.Position.Y, '@')
		}
	}
	return grid, nil
}

func blankGrid(w, h int) []string {
	rows := make([]string, h)
	for y := range rows {
		rows[y] = strings.Repeat(".", w)
	}
	return rows
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

// atlasDistanceGrid returns a throwaway grid of the given family, valid
// ONLY as a Distance calculator over ABSOLUTE positions — Distance
// depends solely on the two positions passed to it, never on the grid's
// own bounds (SquareGrid.Distance and AxialHexGrid.Distance's own
// implementations), so any instance of the right family computes the
// correct distance between two dungeon-absolute doorway cells (#929 T5).
func atlasDistanceGrid(family spatial.GridShape) spatial.Grid {
	if family == spatial.GridShapeHex {
		return spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 100000, SpanHeight: 100000})
	}
	return spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 100000, Height: 100000})
}

// printAtlas renders the dungeon in world space (#929 T5): every room
// placed at its absolute footprint, and every doorway's kissing pair —
// the two absolute cells a connection joins, and the distance between
// them (always 1 for anything the composition actually constructed; W3
// guarantees it) — made visible. This is the one property the web
// client renders by: the room boundary is invisible in world space.
func printAtlas(enc *encounter.Encounter) {
	atlas, err := enc.Atlas()
	if err != nil {
		fmt.Println("  atlas:", err)
		return
	}
	fmt.Println("  ATLAS — the dungeon in absolute space")
	for _, r := range atlas.Rooms {
		qMin, qMax, rMin, rMax := r.Cells[0].X, r.Cells[0].X, r.Cells[0].Y, r.Cells[0].Y
		for _, c := range r.Cells {
			qMin, qMax = min(qMin, c.X), max(qMax, c.X)
			rMin, rMax = min(rMin, c.Y), max(rMax, c.Y)
		}
		fmt.Printf("    room %-10s origin (%g,%g)  %dx%d  absolute x:[%g,%g] y:[%g,%g]\n",
			r.ID, r.Origin.X, r.Origin.Y, r.Width, r.Height, qMin, qMax, rMin, rMax)
	}
	for _, d := range atlas.Doorways {
		var family spatial.GridShape
		for _, r := range atlas.Rooms {
			if r.ID == d.From {
				family = r.Grid
			}
		}
		dist := atlasDistanceGrid(family).Distance(d.FromCell, d.ToCell)
		fmt.Printf("    doorway %-10s %s(%g,%g) absolute -- %s(%g,%g) absolute  distance %g — they kiss\n",
			d.Connection, d.From, d.FromCell.X, d.FromCell.Y, d.To, d.ToCell.X, d.ToCell.Y, dist)
	}
}

const legend = `  @ you   A/B/C capitals: seen NOW   a/b/g lowercase: ghost at last-seen
  # pillar/sarcophagus   > the stairs down (reach them and the encounter closes)
  connection "passage": crypt (11,0) <-> ossuary (0,0) — stand on the
  threshold and traverse; the view flips to whichever room you're in`

const commands = `  move <name> <x> <y>   walk (the world holds still — you pump it)
  traverse <name> <connection>   cross a connection from its threshold
  pump                  a tick passes: the goblin patrols, sights refresh
  view <name>           world truth beside <name>'s beliefs
  story <name>          the record, as <name> is allowed to hear it
  join <name> <x> <y>   a late player joins the ambient
  exit <name>           a player heads back to town (carry-forward prints)
  end withdrew          the party calls the delve off (External ending)
  save <file> / load <file>   the ONE aggregate blob, round-tripped
  atlas                 the dungeon in absolute space — rooms placed by
                        origin, every doorway's kissing pair made visible
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
		case "move":
			if len(args) != 4 {
				fmt.Println("  move <name> <x> <y>")
				continue
			}
			x, y := num(args[2]), num(args[3])
			out, err := enc.Move(&encounter.MoveInput{Member: core.EntityID(args[1]), To: spatial.Position{X: x, Y: y}})
			if err != nil {
				fmt.Println(" ", err)
				continue
			}
			showView(enc, core.EntityID(args[1]))
			if out.Outcome != nil {
				printStatus(enc)
			}
		case "traverse":
			if len(args) != 3 {
				fmt.Println("  traverse <name> <connection>")
				continue
			}
			out, err := enc.Traverse(&encounter.TraverseInput{
				Member: core.EntityID(args[1]), Connection: args[2],
			})
			if err != nil {
				fmt.Println(" ", err)
				continue
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
				fmt.Printf("; %s prowls (%g,%g)->(%g,%g)", mv.Member, mv.From.X, mv.From.Y, mv.To.X, mv.To.Y)
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
			_, err := enc.Join(&encounter.JoinInput{Member: encounter.MemberInput{
				ID: core.EntityID(args[1]), Kind: encounter.KindPlayer, Room: cryptID,
				Position: spatial.Position{X: num(args[2]), Y: num(args[3])},
			}})
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
			loaded, err := encounter.LoadEncounter(data, map[encounter.MemberID]encounter.Decider{
				"goblin": goblinPatrol(),
			})
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
	room := ""
	for _, m := range data.Members {
		if m.ID == who {
			room = m.Room
		}
	}
	if room == "" {
		fmt.Println(" ", who, "is not a member")
		return
	}
	belief, err := beliefGrid(enc, data, who, room)
	if err != nil {
		fmt.Println(" ", err)
		return
	}
	title := fmt.Sprintf("WORLD TRUTH (%s)", room)
	printSideBySide(worldGrid(data, room), belief, title, fmt.Sprintf("%s BELIEVES", strings.ToUpper(string(who))))
}

func num(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
