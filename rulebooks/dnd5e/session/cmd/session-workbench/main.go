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
	"errors"
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

// encPassDriver is this construction's TurnDriver: every unplayed member
// passes. Matched to session.Pass, the workbench's own answer, for the same
// reason encEveryoneStanding is matched to session's — the scene reads the
// same however it is entered.
type encPassDriver struct{}

func (encPassDriver) Act(encounter.MemberID) (encounter.TurnOutcome, error) {
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
		Characters: &memCharacters{byID: map[string]*character.Data{"bob": bobTheDwarf()}},
		Events:     &printStream{out: out},
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
		// A cell on the map. The antechamber is anchored at the origin, so
		// this one reads the same either way; the vault below does not.
		Position: spatial.Position{X: 1, Y: 2},
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
		// The vault is anchored at (6,0), so its local (1,1) is (7,1) on the
		// map — and the map is what this seam speaks. It stands OFF THE
		// ARCH'S LANE: from the threshold the open gate is a corridor of
		// sight down row 1 and this is three rows off it, so the approach
		// sees nothing. Stepping through puts her in the room, where the
		// lane becomes the whole chamber.
		Position: spatial.Position{X: 10, Y: 5},
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
			d.Connection, d.From.X, d.From.Y, d.To.X, d.To.Y)
	}

	fmt.Fprintln(out, "\n== alice walks to the gate ==")
	walked, err := mgr.Move(ctx, &session.MoveInput{
		Session: "crypt-run", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 1}, {X: 5, Y: 1}},
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
	// A doorway is a step. The antechamber's threshold is (5,1) and the
	// vault's is (6,1): one cell apart on the map, so this is a one-step walk
	// that happens to change rooms.
	crossed, err := mgr.Move(ctx, &session.MoveInput{
		Session: "crypt-run", Member: "alice",
		Path: []spatial.Position{{X: 6, Y: 1}},
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

	fmt.Fprintln(out, "\n== and she cannot simply walk away ==")
	// Cells on the map: alice arrived at the vault's (0,1) local, which is
	// (6,1) with the vault anchored at (6,0). She tries to step deeper in.
	vaultPath := []spatial.Position{{X: 7, Y: 1}, {X: 7, Y: 2}}
	_, err = mgr.Move(ctx, &session.MoveInput{
		Session: "crypt-run", Member: "alice", Path: vaultPath,
	})
	fmt.Fprintf(out, "   free roam refused for a fight member: %v\n",
		errors.Is(err, session.ErrInBubble))

	// Bob is not in it. A fight is localized to the members who are in contact,
	// so the rest of the party keeps exploring while it runs.
	bobWalk, err := mgr.Move(ctx, &session.MoveInput{
		Session: "crypt-run", Member: "bob", Path: []spatial.Position{{X: 2, Y: 2}},
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

// authoredCrypt is the content a pipeline would have produced: two rooms and a
// gate between them, anchored so the doorway's endpoints are adjacent in
// absolute space.
func authoredCrypt() (*encounter.EncounterData, error) {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
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
			Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque()},
			Rooms: []encounter.RoomInput{
				{ID: "antechamber", Width: 6, Height: 6,
					// The seam with the vault, open only on row 1 where the gate
					// is. Rooms used to imply this: when each chamber had its own
					// grid, nothing crossed between them except through a declared
					// doorway. On one canvas (rpg-toolkit#1127) two chambers side
					// by side are one open space, so without these walls the
					// skeletons see straight into the antechamber and the fight
					// starts before anybody walks anywhere.
					Boundaries: squareSeam(6, 6, 1),
				},
				// The vault is split by rubble with one sightline through it at
				// y=3. Crossing the gate puts alice where both of them can be
				// seen — sight is a LANE now (spatial v0.9.1), so she looks
				// past the rubble's corner rather than dead down its file, and
				// the fight that starts is the whole room's rather than one
				// skeleton's.
				{ID: "vault", Width: 6, Height: 6, Origin: spatial.Position{X: 6, Y: 0}, Props: rubble(
					spatial.Position{X: 2, Y: 0}, spatial.Position{X: 2, Y: 1}, spatial.Position{X: 2, Y: 2},
					spatial.Position{X: 2, Y: 4}, spatial.Position{X: 2, Y: 5},
				)},
			},
			Connections: []encounter.ConnectionInput{{
				ID: "gate", From: "antechamber", To: "vault",
				FromPosition: spatial.Position{X: 5, Y: 1},
				ToPosition:   spatial.Position{X: 0, Y: 1},
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Room: "antechamber", Position: spatial.Position{X: 1, Y: 1}},
			{ID: "wight", Kind: encounter.KindMonster, Room: "vault", Position: spatial.Position{X: 4, Y: 3}},
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

// squareSeam is the wall between two side-by-side square chambers, with one row
// left open for the doorway. Room-local to the WEST chamber, where column
// width-1 is its last and column width is the east chamber's first.
func squareSeam(width, rows, openRow int) []spatial.Boundary {
	out := make([]spatial.Boundary, 0, rows)
	for row := 0; row < rows; row++ {
		if row == openRow {
			continue // the gate itself
		}
		out = append(out, spatial.Boundary{
			From:              spatial.Position{X: float64(width - 1), Y: float64(row)},
			To:                spatial.Position{X: float64(width), Y: float64(row)},
			BlocksMovement:    true,
			BlocksLineOfSight: true,
		})
	}

	return out
}
