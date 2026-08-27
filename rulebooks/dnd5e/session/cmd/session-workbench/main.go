// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Command session-workbench drives a whole session through the SDK, the way a
// game server would.
//
// It exists to be run, not admired. The repositories below are the smallest
// honest implementation the SDK will accept — two maps — which is itself the
// claim being demonstrated: a host that can get and put a blob by key has done
// its entire integration.
//
//	go run ./cmd/session-workbench
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func main() {
	if err := run(os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "workbench:", err)
		os.Exit(1)
	}
}

// run drives the session and writes the transcript.
//
// Narration accumulates in a buffer rather than streaming, so the many
// formatting calls have no errors worth checking and the single write that can
// actually fail is checked exactly once. It also means a caller can assert on
// the whole transcript without a temp file.
func run(w io.Writer) error {
	var buf bytes.Buffer
	if err := drive(&buf); err != nil {
		// Emit whatever was narrated before the failure: on a broken run, the
		// last line printed is the most useful thing on the screen.
		_, _ = w.Write(buf.Bytes())
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// compiledMoveID returns the non-empty Move selector the workbench can echo.
// A blocked Move is reported before the mutating verb is called, preserving the
// declaration's own explanation when Afford supplied one.
func compiledMoveID(declarations []session.Declaration) (string, error) {
	var why string
	for _, declaration := range declarations {
		if declaration.Verb != session.VerbMove {
			continue
		}
		if declaration.ID != "" {
			return declaration.ID, nil
		}
		if declaration.Why != nil && declaration.Why.Text != "" {
			why = declaration.Why.Text
		}
	}
	if why != "" {
		return "", fmt.Errorf("no compiled Move declaration: %s", why)
	}
	return "", fmt.Errorf("no compiled Move declaration")
}

// loadedDice is this host's source of randomness, and it is here because a
// fight now starts by itself: sight starts it, mid-walk, and something must be
// able to say who acts first at that moment.
//
// Note what the host supplies: DICE, not an order. A real server wires its
// seeded or crypto source here and never writes a line of initiative logic —
// the rule that turns a d20 into an order belongs to the rulebook, and the SDK
// routes between the two. The workbench returns a constant because a
// demonstration that printed a different transcript on every run would be
// demonstrating nothing.
type loadedDice struct{}

func (loadedDice) Roll(_ context.Context, _ int) (int, error) { return 10, nil }

// encOrderAsGiven is the same for the authored world this workbench builds
// with the composition directly, before any session exists to own it.
// encEveryoneStanding is the standing capability the authored world is BUILT
// with: nobody is down when the scene is written.
//
// Construction only, like encOrderAsGiven beside it. Once the workbench hands
// the blob to a session, the session supplies the real capability from the
// sheets it holds (rpg-toolkit#1079) and this one is never consulted again.
type encEveryoneStanding struct{}

func (encEveryoneStanding) Standing(_ []encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, nil
}

type encOrderAsGiven struct{}

func (encOrderAsGiven) RollInitiative(m []encounter.MemberID) ([]encounter.MemberID, error) {
	return m, nil
}

// encQuietAnnouncer is this construction's Announcer. It hears the boundaries
// assembling the scene crosses and does nothing with them: there is no rulebook
// attached to a world being built, so a turn boundary means nothing here yet.
type encQuietAnnouncer struct{}

func (encQuietAnnouncer) Announce(context.Context, *encounter.Encounter, []encounter.Boundary) error {
	return nil
}

// encPassDriver is this construction's TurnDriver: every unplayed member
// passes. Matched to session.Pass, the workbench's own answer, for the same
// reason encEveryoneStanding is matched to session's — the scene reads the
// same however it is entered.
type encPassDriver struct{}

func (encPassDriver) Act(encounter.MonsterView) (encounter.TurnIntent, error) {
	return encounter.Pass{}, nil
}

// memSessions is a SessionRepository over a map. Get-by-id and put-by-id is
// the whole interface (S12), which is why this is six lines rather than a
// schema.
type memSessions struct {
	byID map[string]*session.SessionData
}

func (m *memSessions) GetSession(_ context.Context, id string) (*session.SessionData, error) {
	data, ok := m.byID[id]
	if !ok {
		return nil, session.ErrNotFound
	}
	return data, nil
}

func (m *memSessions) SaveSession(_ context.Context, data *session.SessionData) error {
	m.byID[data.ID] = data
	return nil
}

// memEncounters is an EncounterRepository over a map.
type memEncounters struct {
	byID map[string]*encounter.EncounterData
}

func (m *memEncounters) GetEncounter(_ context.Context, id string) (*encounter.EncounterData, error) {
	data, ok := m.byID[id]
	if !ok {
		return nil, session.ErrNotFound
	}
	return data, nil
}

func (m *memEncounters) SaveEncounter(_ context.Context, id string, data *encounter.EncounterData) error {
	m.byID[id] = data
	return nil
}

// memCharacters is a CharacterRepository over a map.
//
// The host owns character storage; this package only ever asks for one by ID
// and hands back data. Nothing here holds a loaded character, because nothing
// can: a character lives for one verb.
type memCharacters struct {
	byID map[string]*character.Data
}

func (m *memCharacters) GetCharacter(_ context.Context, id string) (*character.Data, error) {
	data, ok := m.byID[id]
	if !ok {
		return nil, session.ErrNotFound
	}
	return data, nil
}

func (m *memCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	m.byID[data.ID] = data
	return nil
}

// bobTheDwarf is the stored sheet the workbench loads.
//
// Note what is NOT here: speed. It is derived from race when the character is
// reconstituted, which is why the transcript printing 25 is evidence the load
// really happened rather than evidence a map lookup returned something.
// aliceTheFighter registers alice's own sheet. She had none until
// rpg-toolkit#1169: nothing before Move's own turn-clock pricing ever needed
// to load a member's character for a verb this demo calls on her, so the gap
// was invisible — a fixture that only ever spawned her onto the encounter's
// roster, never into the Characters store Attack and now Move both read.
func aliceTheFighter() *character.Data {
	return &character.Data{
		ID:               "alice",
		PlayerID:         "player-alice",
		Name:             "Alice",
		Level:            3,
		ProficiencyBonus: 2,
		RaceID:           races.Human,
		ClassID:          classes.Fighter,
		HitPoints:        24,
		MaxHitPoints:     28,
		ArmorClass:       16,
	}
}

func bobTheDwarf() *character.Data {
	return &character.Data{
		ID:               "bob",
		PlayerID:         "player-bob",
		Name:             "Bob",
		Level:            3,
		ProficiencyBonus: 2,
		RaceID:           races.Dwarf,
		ClassID:          classes.Fighter,
		HitPoints:        24,
		MaxHitPoints:     28,
		ArmorClass:       16,
	}
}

// printStream shows the fan-out as it happens, one line per addressed event —
// which is what a host would be shipping to each connected client.
type printStream struct{ out *bytes.Buffer }

func (p *printStream) Publish(_ context.Context, events []session.Event) error {
	for _, e := range events {
		var body map[string]any
		_ = json.Unmarshal(e.Payload, &body)
		fmt.Fprintf(p.out, "      → %-6s %-12s seq=%d %v\n", e.Recipient, e.Kind, e.Seq, body["beat"])
	}
	return nil
}

func drive(out *bytes.Buffer) error {
	ctx := context.Background()

	mgr, err := session.NewManager(&session.Config{Dice: loadedDice{},
		Sessions:   &memSessions{byID: map[string]*session.SessionData{}},
		Encounters: &memEncounters{byID: map[string]*encounter.EncounterData{}},
		Characters: &memCharacters{
			byID: map[string]*character.Data{"alice": aliceTheFighter(), "bob": bobTheDwarf()},
		},
		Events: &printStream{out: out},
		// The workbench demonstrates verbs a human drives; nothing in it
		// gives a monster a real behavior yet, so an unplayed member simply
		// passes (rpg-toolkit#1162).
		TurnDriver: session.Pass{},
	})
	if err != nil {
		return err
	}

	world, err := authoredCrypt()
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "== the party enters the crypt ==")
	if _, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "crypt-run", Encounter: "crypt", World: world,
	}); err != nil {
		return err
	}

	joined, err := mgr.Join(ctx, &session.JoinInput{
		Session: "crypt-run", Member: "bob",
		// A cell on the map: authored [1,2], as the map speaks it.
		Position: cellAt(1, 2),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   bob joins at (%v,%v)\n", joined.Member.Position.X, joined.Member.Position.Y)

	// The sheet came back derived, not echoed. Speed is not a field of the
	// stored data at all — a dwarf's 25 comes from reconstituting the
	// character and asking it, which is the whole point of loading here.
	if c := joined.Character; c != nil {
		fmt.Fprintf(out, "   %s, level %d — %d/%d hp, ac %d, speed %d\n",
			c.Name, c.Level, c.HitPoints, c.MaxHitPoints, c.ArmorClass, c.Speed)
	}

	// Bob was LOADED — the host owns his sheet and named him by ID. The
	// skeleton is INSTANTIATED from a ref, because it exists in code and
	// nobody stored it. Two verbs, because the two are genuinely different;
	// the caller never has to say which kind of thing it is.
	//
	// It arrives in the vault, behind the rubble, and that placement is now
	// load-bearing: spawning it into the antechamber in plain view of the party
	// would start a fight on the spot, before anybody had walked a step. That
	// is why SpawnOutput carries Formed at all.
	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "crypt-run", ID: "skel-1",
		Ref: refs.Monsters.Skeleton().String(),
		// Authored [10,5], as the map speaks it. It stands OFF THE ARCH'S
		// LANE: from the threshold the open gate is a corridor of sight
		// down row 1 and this is four rows off it, so the approach sees
		// nothing. Stepping through puts her in the room, where the lane
		// becomes the whole chamber.
		Position: cellAt(10, 5),
	})
	if err != nil {
		return err
	}
	if n := spawned.NPC; n != nil {
		fmt.Fprintf(out, "   %s spawns as %s — %d/%d hp, ac %d, speed %d\n",
			n.Name, n.ID, n.HitPoints, n.MaxHitPoints, n.ArmorClass, n.Speed)
	}

	atlas, err := mgr.Atlas(ctx, &session.AtlasInput{Session: "crypt-run"})
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "\n== the map, in dungeon-absolute space ==")
	blocking := 0
	for _, p := range atlas.Props {
		if p.BlocksLineOfSight {
			blocking++
		}
	}
	fmt.Fprintf(out, "   one %s map: %d cells, %d props (%d of them blocking sight), %d walls\n",
		atlas.Grid, len(atlas.Cells), len(atlas.Props), blocking, len(atlas.Boundaries))
	for _, d := range atlas.Doorways {
		fmt.Fprintf(out, "   %s: (%v,%v) kisses (%v,%v)\n",
			d.Door, d.From.X, d.From.Y, d.To.X, d.To.Y)
	}

	fmt.Fprintln(out, "\n== alice walks to the gate ==")
	walked, err := mgr.Move(ctx, &session.MoveInput{
		Session: "crypt-run", Member: "alice",
		Path: []spatial.Position{cellAt(2, 1), cellAt(3, 1), cellAt(4, 1), cellAt(5, 1)},
	})
	if err != nil {
		return err
	}
	for _, step := range walked.Steps {
		fmt.Fprintf(out, "   alice enters (%v,%v)\n", step.Position.X, step.Position.Y)
	}
	fmt.Fprintf(out, "   delivered %d events\n", walked.Delivery.Events)

	// SHE REACHES THE GATE UNSEEN, and that is a claim about LIGHT rather than
	// about walls. The arch on row 1 is open, and an opening is an opening in
	// both directions — the vault can see out of it exactly as far as somebody
	// can see in — so nothing here hides her except the range of what she
	// carries. At session's four cells the skeleton is out of it until she is
	// through; unbounded, this fight starts halfway down the antechamber and
	// bob joins it from the far room. See sightRangeCells.
	if walked.Formed != nil {
		return fmt.Errorf("the approach should be unseen at torchlight, but a fight started")
	}

	fmt.Fprintln(out, "\n== and through it, into something waiting ==")
	// A doorway is a step. The antechamber's threshold is [5,1] and the
	// vault's is [6,1]: one cell apart on the map, so this is a one-step walk
	// that happens to change chambers.
	crossed, err := mgr.Move(ctx, &session.MoveInput{
		Session: "crypt-run", Member: "alice",
		Path: []spatial.Position{cellAt(6, 1)},
	})
	if err != nil {
		return err
	}
	for _, step := range crossed.Steps {
		fmt.Fprintf(out, "   alice steps through to (%v,%v)\n", step.Position.X, step.Position.Y)
	}

	// The doorway is where the fight starts, and the host is told in the same
	// response that told it about the crossing.
	if crossed.Formed == nil {
		return fmt.Errorf("expected crossing into the vault to start a fight")
	}
	fmt.Fprintf(out, "   a fight starts, in order %v\n", crossed.Formed.Order)
	if len(crossed.Formed.Surprised) > 0 {
		fmt.Fprintf(out, "   caught unaware: %v\n", crossed.Formed.Surprised)
	}

	fmt.Fprintln(out, "\n== alice can still act, on her own turn ==")
	// Alice arrived on the vault's threshold at [6,1]. She steps deeper in —
	// on the turn clock now, a walk spends movement rather than being
	// refused outright (rpg-toolkit#1169), and it is still her turn: she
	// leads the vault fight's own initiative order.
	vaultPath := []spatial.Position{cellAt(7, 1), cellAt(7, 2)}
	afford, err := mgr.Afford(ctx, &session.AffordInput{Session: "crypt-run", Member: "alice"})
	if err != nil {
		return err
	}
	moveID, err := compiledMoveID(afford.Declarations)
	if err != nil {
		return err
	}
	aliceWalk, err := mgr.Move(ctx, &session.MoveInput{
		Session: "crypt-run", Member: "alice", Path: vaultPath, DeclarationID: moveID,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   she walks %d step(s) into the vault, on her own turn\n", len(aliceWalk.Steps))

	// Bob is not in it. A fight is localized to the members who are in contact,
	// so the rest of the party keeps exploring while it runs.
	bobWalk, err := mgr.Move(ctx, &session.MoveInput{
		Session: "crypt-run", Member: "bob", Path: []spatial.Position{cellAt(2, 2)},
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   bob walks on regardless, %d step(s)\n", len(bobWalk.Steps))

	fmt.Fprintln(out, "\n== bob's story ==")
	story, err := mgr.Story(ctx, &session.StoryInput{Session: "crypt-run", Member: "bob"})
	if err != nil {
		return err
	}
	for _, entry := range story {
		var body map[string]any
		_ = json.Unmarshal(entry.Payload, &body)
		fmt.Fprintf(out, "   seq=%-3d %v\n", entry.Seq, body["beat"])
	}

	fmt.Fprintln(out, "\n== the party leaves ==")
	ended, err := mgr.End(ctx, &session.EndInput{Session: "crypt-run", Ending: "withdraw"})
	if err != nil {
		return err
	}
	final := make([]string, 0, len(ended.Outcome.Members))
	for _, m := range ended.Outcome.Members {
		final = append(final, fmt.Sprintf("%s at (%v,%v)", m.ID, m.Position.X, m.Position.Y))
	}
	sort.Strings(final)
	fmt.Fprintf(out, "   ended by %q\n", ended.Outcome.Ending)
	for _, line := range final {
		fmt.Fprintf(out, "   %s\n", line)
	}

	return nil
}

// authoredCrypt is the content a pipeline would have produced: two chambers
// and a gate between them, painted side by side so the gate's two cells are
// adjacent on the map.
//
// Every pair below is an AUTHORED offset [col,row] under pointy-top hexes,
// converted once at construction; the verbs above speak the axial cells
// cellAt makes of them.
func authoredCrypt() (*encounter.EncounterData, error) {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Striker: encounter.RefusingStriker{},
		// Governs THIS construction only, exactly as the sight seam below
		// does: session installs its own announcer the moment it loads this
		// world, and the walk runs on that one. Quiet rather than refusing
		// because a bubble can form while the scene is being assembled.
		Announcer: encQuietAnnouncer{},
		Standing:  encEveryoneStanding{},
		// Governs THIS construction only: once session loads the world it
		// supplies its own sight seam, so the walk below runs on session's
		// answer rather than this one. Matched to it anyway, so the scene
		// reads the same however it is entered.
		Sight: encEveryoneSees{},
		Field: encounter.FieldInput{
			// The space between the chambers is ROCK, which is the ordinary
			// dungeon reading and the one that keeps this scene about the gate:
			// with transparent void, a watcher could see into the vault around
			// the doorway rather than through it (rpg-toolkit#1116).
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{
				chamber("antechamber", 0, 0, 6, 6),
				chamber("vault", 6, 0, 6, 6),
			},
			// The seam between them, open only on row 1 where the gate is.
			// Rooms used to imply this: when each chamber had its own grid,
			// nothing crossed between them except through a declared doorway.
			// On one canvas two chambers side by side are one open space, so
			// without these walls the skeletons see straight into the
			// antechamber and the fight starts before anybody walks anywhere.
			Walls: hexSeam(6, 6, 1),
			// The vault is split by rubble down authored column 8 with one
			// gap at row 3. The wight stands west of it, off the gate's own
			// lane; the skeleton is spawned east of it, past the gap.
			// Crossing the gate puts alice where both of them can be seen,
			// and the fight that starts is the whole room's rather than one
			// skeleton's.
			Props: rubble(
				spatial.Position{X: 8, Y: 0}, spatial.Position{X: 8, Y: 1}, spatial.Position{X: 8, Y: 2},
				spatial.Position{X: 8, Y: 4}, spatial.Position{X: 8, Y: 5},
			),
			Doors: []encounter.DoorInput{{
				ID:    "gate",
				Edges: []encounter.DoorEdge{{From: cellAt(5, 1), To: cellAt(6, 1)}},
				State: encounter.DoorIsOpen(),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			// Three rows below the gate's lane, so the approach down row 1
			// looks past it and the seam wall hides it until she is through.
			{ID: "wight", Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 4}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		return nil, err
	}
	data := enc.ToData()
	return &data, nil
}

// chamber paints a w x h rectangle of authored cells at [col,row] as one
// region, dressed as a crypt and fully lit — this scene is about the gate,
// not about what the chambers look like.
func chamber(id string, col, row, w, h int) encounter.RegionInput {
	cells := make([]spatial.Position, 0, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			cells = append(cells, spatial.Position{X: float64(col + c), Y: float64(row + r)})
		}
	}
	return encounter.RegionInput{
		ID: id, Name: id, Cells: cells, Archetype: "crypt", Lighting: &encounter.Lighting{Intensity: 1},
	}
}

// cellAt is the cell on the map an authored [col,row] pair names — the
// frame every verb above takes and reports.
func cellAt(col, row int) spatial.Position {
	return encounter.HexCellAt(encounter.HexesArePointyTop(), col, row)
}

// rubble is the vault's pile of fallen stone, one prop per cell.
//
// Both answers are stated rather than defaulted (rpg-toolkit#1033): rubble is
// climbed over neither — it stops a walker and it stops a sightline — and
// saying so here is the point, since the same call could describe a coffin
// (walked around, seen over) by changing one word.
func rubble(at ...spatial.Position) []encounter.PropInput {
	blocks := true
	out := make([]encounter.PropInput, 0, len(at))
	for _, cell := range at {
		out = append(out, encounter.PropInput{
			Ref:               "rubble",
			At:                cell,
			BlocksMovement:    &blocks,
			BlocksLineOfSight: &blocks,
		})
	}

	return out
}

// encEveryoneSees gives every member the same sight radius.
type encEveryoneSees struct{}

func (encEveryoneSees) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = sightRadius
	}

	return out, nil
}

// sightRadius is how far anybody in this crypt can see, in cells.
//
// Four, matching session's own answer (sightRangeCells) so this scene behaves
// the same whether it is driven through session or built directly here. Twenty
// feet at five feet to the cell: a torch's bright light.
//
// It is a NUMBER and not a shrug. Unbounded, the open arch on row 1 shows the
// whole vault from halfway down the antechamber, the fight starts before
// anybody reaches the gate, and bob — whose entire purpose in this scene is to
// keep exploring while alice fights — is pulled into it from the far room.
const sightRadius = 4

// hexSeam is the wall between authored column east-1 and column east over
// rows 0..rows-1, with the straight crossing on openRow left open for the
// gate.
//
// EVERY CROSSING, not just the straight one: sight asks for a LANE, not a
// line (spatial's own IsLineOfSightBlocked doc), so a seam sealed only along
// the row lets a viewer several rows off the doorway find an unobstructed
// diagonal through it. Which candidate pairs ARE crossings is the grid's
// answer: a hex has six neighbours and they stagger with the row's parity,
// so each is checked by distance rather than assumed.
func hexSeam(east, rows, openRow int) []encounter.WallInput {
	grid := spatial.NewAxialHexGrid(spatial.AxialHexGridConfig{SpanWidth: 1, SpanHeight: 1})
	out := make([]encounter.WallInput, 0, rows*2)
	for row := 0; row < rows; row++ {
		for _, dr := range []int{-1, 0, 1} {
			to := row + dr
			if to < 0 || to >= rows {
				continue
			}
			if dr == 0 && row == openRow {
				continue // the gate itself
			}
			if !grid.IsAdjacent(cellAt(east-1, row), cellAt(east, to)) {
				continue // not a crossing on this grid
			}
			out = append(out, encounter.WallInput{Boundary: spatial.Boundary{
				From:              spatial.Position{X: float64(east - 1), Y: float64(row)},
				To:                spatial.Position{X: float64(east), Y: float64(to)},
				BlocksMovement:    true,
				BlocksLineOfSight: true,
			}})
		}
	}

	return out
}
