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

	mgr, err := session.NewManager(&session.Config{
		Sessions:   &memSessions{byID: map[string]*session.SessionData{}},
		Encounters: &memEncounters{byID: map[string]*encounter.EncounterData{}},
		Characters: &memCharacters{byID: map[string]*character.Data{"bob": bobTheDwarf()}},
		Events:     &printStream{out: out},
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
		Room: "antechamber", Position: spatial.Position{X: 1, Y: 2},
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   bob joins the %s\n", joined.Member.Room)

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
	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "crypt-run", ID: "skel-1",
		Ref:  refs.Monsters.Skeleton().String(),
		Room: "antechamber", Position: spatial.Position{X: 4, Y: 2},
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
	for _, room := range atlas.Rooms {
		fmt.Fprintf(out, "   %-12s %dx%d %-6s origin (%v,%v)  %d cells\n",
			room.ID, room.Width, room.Height, room.Grid,
			room.Origin.X, room.Origin.Y, len(room.Cells))
	}
	for _, d := range atlas.Doorways {
		fmt.Fprintf(out, "   %s: %s (%v,%v) kisses %s (%v,%v)\n",
			d.Connection, d.From, d.FromCell.X, d.FromCell.Y,
			d.To, d.ToCell.X, d.ToCell.Y)
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

	fmt.Fprintln(out, "\n== and through it ==")
	crossed, err := mgr.Traverse(ctx, &session.TraverseInput{
		Session: "crypt-run", Member: "alice", Connection: "gate",
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "   %s (%v,%v) → %s (%v,%v)\n",
		crossed.FromRoom, crossed.From.X, crossed.From.Y,
		crossed.ToRoom, crossed.To.X, crossed.To.Y)

	fmt.Fprintln(out, "\n== something moves in the dark ==")
	vaultPath := []spatial.Position{{X: 1, Y: 1}, {X: 1, Y: 2}, {X: 1, Y: 3}, {X: 1, Y: 4}}
	probe, err := mgr.Move(ctx, &session.MoveInput{
		Session: "crypt-run", Member: "alice", Path: vaultPath,
	})
	if err != nil {
		return err
	}
	for _, step := range probe.Steps {
		fmt.Fprintf(out, "   alice enters (%v,%v)\n", step.Position.X, step.Position.Y)
	}
	if probe.Pending == nil {
		return fmt.Errorf("expected the vault to interrupt the walk, got %d/%d steps",
			len(probe.Steps), len(vaultPath))
	}
	fmt.Fprintf(out, "   the world freezes after %d/%d: %s sees %v\n",
		len(probe.Steps), len(vaultPath), probe.Pending.Audience, probe.Pending.Prompt.Sighted)

	// Everything that would change the world is refused now, and says why.
	if _, err := mgr.End(ctx, &session.EndInput{
		Session: "crypt-run", Ending: "withdraw",
	}); err != nil {
		fmt.Fprintf(out, "   end refused while frozen: %v\n", err)
	}

	resumed, err := mgr.Answer(ctx, &session.AnswerInput{
		Session: "crypt-run", Window: probe.Pending.Window,
		Member: "alice", Option: string(session.OptionContinue),
	})
	if err != nil {
		return err
	}
	for _, step := range resumed.Steps {
		fmt.Fprintf(out, "   alice presses on to (%v,%v)\n", step.Position.X, step.Position.Y)
	}

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
		final = append(final, fmt.Sprintf("%s in %s at (%v,%v)", m.ID, m.Room, m.Position.X, m.Position.Y))
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
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Rooms: []encounter.RoomInput{
				{ID: "antechamber", Width: 6, Height: 6},
				// The vault is split by rubble with one sightline through it at
				// y=3, so what waits at (4,3) is invisible until someone lines
				// up with the gap.
				{ID: "vault", Width: 6, Height: 6, Origin: spatial.Position{X: 6, Y: 0}, Occluders: []spatial.Position{
					{X: 2, Y: 0}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 2, Y: 4}, {X: 2, Y: 5},
				}},
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
