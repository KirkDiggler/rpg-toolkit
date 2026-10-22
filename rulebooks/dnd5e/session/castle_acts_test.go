// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_acts_test.go is E3's castle check (rpg-project#629): the mad king's
// gatehouse played through the session seam, and every recipient's acts
// folded and told. Two scenes, the QA castle's script (rpg-project#639): the
// fight at the gate, which brings nobody, and the troupe, which learns the
// feast from the stranger round the corner, is let through, and opens the
// gate on the far room.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castleFixture is the mad king's castle as the encounter module pins it:
// its own dungeonspec testdata, read from the module this one depends on, so
// there is one castle and not a copy.
const castleFixture = "dungeonspec/testdata/world-builder-v4-castle.yaml"

// castleDice rolls its scripted d20s in order, then the last one forever,
// and the lowest face of any other die.
type castleDice struct {
	mu   sync.Mutex
	d20s []int
}

func (d *castleDice) Roll(_ context.Context, size int) (int, error) {
	if size != 20 {
		return 1, nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	face := d.d20s[0]
	if len(d.d20s) > 1 {
		d.d20s = d.d20s[1:]
	}
	return face, nil
}

// gatehouse is the castle opened for alice at the party start, in the
// gatehouse below the captain and the two gate guards.
type gatehouse struct {
	mgr      *session.Manager
	sessions *fakeSessions
	live     *actCapture
}

// openTheGatehouse launches the castle. It panics rather than failing a
// test, so the Example can tell the same scene.
func openTheGatehouse(d20s ...int) *gatehouse {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", encounterModule).Output()
	if err != nil {
		panic(err)
	}
	raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), castleFixture))
	if err != nil {
		panic(err)
	}
	compiled, err := dungeonspec.Load(raw)
	if err != nil {
		panic(err)
	}
	alice := armedFighter("alice")
	alice.AbilityScores[abilities.CHA] = 16
	g := &gatehouse{sessions: newFakeSessions(), live: &actCapture{}}
	g.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: g.live, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: &castleDice{d20s: d20s}, TurnDriver: session.Driver(),
		Sessions: g.sessions, Encounters: newFakeEncounters(), Characters: newFakeCharacters(alice),
	})
	if err != nil {
		panic(err)
	}
	if _, err := g.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "sess", Dungeon: &compiled, Party: []string{"alice"},
	}); err != nil {
		panic(err)
	}
	return g
}

// panics is a fataler for a scene told outside a test.
type panics struct{}

func (panics) Fatalf(format string, args ...any) { panic(fmt.Sprintf(format, args...)) }

// beat is one verb of the route and every act it published, to everyone.
type beat struct {
	label string
	acts  []session.Act
}

// to is the beat's acts for one recipient.
func (b beat) to(member string) []session.Act {
	var out []session.Act
	for _, a := range b.acts {
		if a.Recipient == member {
			out = append(out, a)
		}
	}
	return out
}

// playTheGatehouse is the gatehouse route through the session seam: alice
// cows the captain with a line (17 against 14) and the world takes its
// round; a second line fails and the garrison turns; she closes on gate-1
// and cuts him down; she ends her turn and the captain and the world answer;
// she falls back past the captain, who takes his opportunity attack.
func playTheGatehouse() []beat {
	ctx := context.Background()
	// The d20s in order: the line that lands, the line that fails, and
	// every roll after.
	g := openTheGatehouse(14, 5, 15)
	var beats []beat
	play := func(label string, verb func() error) {
		g.live.mu.Lock()
		g.live.acts = nil
		g.live.mu.Unlock()
		if err := verb(); err != nil {
			panic(fmt.Sprintf("%s: %v", label, err))
		}
		beats = append(beats, beat{label: label, acts: append([]session.Act(nil), g.live.acts...)})
	}
	standAside := func() error {
		_, err := g.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
		return err
	}
	walkToward := func(anchor func(*encounter.Encounter) spatial.Position, budget int) func() error {
		return func() error {
			enc, err := g.mgr.LoadWorldForTest(ctx, "sess")
			if err != nil {
				return err
			}
			route, err := enc.Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: anchor(enc), Budget: budget})
			if err != nil {
				return err
			}
			_, err = g.mgr.Move(ctx, &session.MoveInput{
				Session: "sess", Member: "alice", Path: route.Path,
				DeclarationID: currentMoveID(panics{}, g.mgr, "sess", "alice"),
			})
			return err
		}
	}

	play("alice tells the captain to stand aside", standAside)
	play("alice tells him again", standAside)
	play("alice closes on gate-1", walkToward(func(enc *encounter.Encounter) spatial.Position {
		return standingAt(enc, "gate-1")
	}, 200))
	play("alice cuts gate-1 down", func() error {
		for i := range g.sessions.byID["sess"].NPCs {
			if g.sessions.byID["sess"].NPCs[i].ID == "gate-1" {
				g.sessions.byID["sess"].NPCs[i].HitPoints = 1
			}
		}
		_, err := g.mgr.Attack(ctx, &session.AttackInput{
			Session: "sess", Attacker: "alice", Target: "gate-1",
			DeclarationID: currentAttackID(panics{}, g.mgr, "sess", "alice"),
		})
		return err
	})
	play("alice ends her turn", func() error {
		_, err := g.mgr.EndTurn(ctx, &session.EndTurnInput{
			Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(panics{}, g.mgr, "sess", "alice"),
		})
		return err
	})
	play("alice falls back past the captain", walkToward(func(*encounter.Encounter) spatial.Position {
		return spatial.Position{X: -10, Y: 12}
	}, 3))
	return beats
}

// The troupe scene's cells, dungeon-absolute: the floor south of the corner
// wall, the cell behind it beside the stranger, and the cell under the gate.
var (
	besideTheCorner = spatial.Position{X: -7, Y: 8}
	theCorner       = spatial.Position{X: -6, Y: 7}
	beneathTheGate  = spatial.Position{X: -3, Y: 7}
)

// castleGate is the gate door's id, as the atlas's doorways name it.
const castleGate = "the-mad-king/gate-wall-door"

// playTheTroupe is the troupe route through the session seam: alice goes
// round the corner and asks the stranger what the bustle is (15 against 5),
// walks up to the captain and passes as the feast's players (15 against 13),
// walks under the gate and opens it, and the far room's two guards arrive and
// walk to the gate, civil.
func playTheTroupe() []beat {
	ctx := context.Background()
	g := openTheGatehouse(12, 12, 15)
	var beats []beat
	play := func(label string, verb func() error) {
		g.live.mu.Lock()
		g.live.acts = nil
		g.live.mu.Unlock()
		if err := verb(); err != nil {
			panic(fmt.Sprintf("%s: %v", label, err))
		}
		beats = append(beats, beat{label: label, acts: append([]session.Act(nil), g.live.acts...)})
	}
	// walk walks alice along a path of cells, each adjacent to the last, on
	// the world clock, where a move carries no declaration.
	walk := func(path []spatial.Position) error {
		_, err := g.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: path})
		return err
	}
	// toward walks alice by the pathfinder's way toward a cell, onto it when
	// it is free.
	toward := func(anchor func(*encounter.Encounter) spatial.Position, onto bool) error {
		enc, err := g.mgr.LoadWorldForTest(ctx, "sess")
		if err != nil {
			return err
		}
		cell := anchor(enc)
		route, err := enc.Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: cell, Budget: 200})
		if err != nil {
			return err
		}
		path := route.Path
		if onto && (len(path) == 0 || path[len(path)-1] != cell) {
			path = append(path, cell)
		}
		return walk(path)
	}
	at := func(cell spatial.Position) func(*encounter.Encounter) spatial.Position {
		return func(*encounter.Encounter) spatial.Position { return cell }
	}
	speak := func(target, option string) func() error {
		return func() error {
			_, err := g.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: target, OptionID: option})
			return err
		}
	}

	play("alice goes round the corner", func() error {
		if err := toward(at(besideTheCorner), true); err != nil {
			return err
		}
		return walk([]spatial.Position{theCorner})
	})
	play("alice asks the stranger what the bustle is", speak("pedlar", "tonight"))
	play("alice walks up to the captain", func() error {
		return toward(func(enc *encounter.Encounter) spatial.Position { return standingAt(enc, "captain") }, false)
	})
	play("alice tells the captain they are the troupe", speak("captain", "troupe"))
	play("alice walks under the gate", func() error { return toward(at(beneathTheGate), true) })
	play("alice opens the gate", func() error {
		_, err := g.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: castleGate})
		return err
	})
	// A round on the world clock is six cells walked: out a cell and back,
	// three times.
	beside := spatial.Position{X: -4, Y: 7}
	play("alice waits at the gate", func() error {
		return walk([]spatial.Position{beside, beneathTheGate, beside, beneathTheGate, beside, beneathTheGate})
	})
	return beats
}

// Each scene is played once per test binary and shared by its test and its
// Example: a scene is the whole castle's world turns through the seam, and
// under -race each play is minutes of the package's budget.
var (
	gatehouseOnce, troupeOnce   sync.Once
	gatehouseBeats, troupeBeats []beat
)

// theGatehouse is the gatehouse route's beats, played once.
func theGatehouse() []beat {
	gatehouseOnce.Do(func() { gatehouseBeats = playTheGatehouse() })
	return gatehouseBeats
}

// theTroupe is the troupe route's beats, played once.
func theTroupe() []beat {
	troupeOnce.Do(func() { troupeBeats = playTheTroupe() })
	return troupeBeats
}

// standingAt is where a member stands now.
func standingAt(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	if err != nil {
		panic(err)
	}
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	panic(fmt.Sprintf("%s is not in the run", id))
}

// TestCastleActsTranscript is E3's castle check: the gatehouse route folds,
// for alice, into acts that each say what happened inside themselves. It
// asserts what each act claims and never how many there are.
func TestCastleActsTranscript(t *testing.T) {
	beats := theGatehouse()
	cowed, recognized, closes, cut, ended, falls := beats[0], beats[1], beats[2], beats[3], beats[4], beats[5]

	// THE LINE THAT LANDS: one speak act, its check and its answer inside.
	line := only[session.SpeakAct](t, cowed.to("alice"))
	require.True(t, strings.HasPrefix(line.act.Act, "speak:"), line.act.Act)
	require.NotNil(t, line.body.Roll)
	assert.Equal(t, 17, line.body.Roll.Total)
	assert.Equal(t, 14, *line.body.Roll.Against)
	assert.True(t, *line.body.Roll.Beaten)
	assert.Equal(t, "1d20+3", line.body.Roll.Expression)
	require.NotNil(t, line.body.Answer, "the answer folds into the line")
	assert.Equal(t, "…go on, then. I saw nothing.", line.body.Answer.Say)

	// THE WORLD'S ROUND: each pick a table answer under a time act, and
	// gate-1's patrol a walk.
	var picks int
	for _, act := range cowed.to("alice") {
		if answer, ok := act.Body.(session.TableAnswer); ok && answer.Key == "time" {
			picks++
			assert.True(t, strings.HasPrefix(act.Act, "time:"), act.Act)
			assert.Equal(t, session.FamilyTableAnswer, act.Family)
		}
	}
	assert.Positive(t, picks, "the world took its round")
	patrol := where[session.WalkAct](cowed.to("alice"), func(w session.WalkAct) bool { return w.Mover.ID == "gate-1" })
	require.NotEmpty(t, patrol, "gate-1 paces the gatehouse")
	assert.True(t, strings.HasPrefix(patrol[0].act.Act, "move:"), patrol[0].act.Act)

	// THE LINE THAT FAILS: the garrison turns and a fight forms, under the
	// line's own act.
	failed := only[session.SpeakAct](t, recognized.to("alice"))
	assert.False(t, *failed.body.Roll.Beaten)
	turned := where[session.StateChange](recognized.to("alice"), func(c session.StateChange) bool { return c.Change == session.StateStance })
	require.NotEmpty(t, turned)
	assert.Equal(t, failed.act.Act, turned[0].act.Act)
	assert.Equal(t, "hostile", turned[0].body.Stance.Stance)
	formed := where[session.ClockEntry](recognized.to("alice"), func(c session.ClockEntry) bool { return c.Mark == session.ClockFightFormed })
	require.NotEmpty(t, formed)
	assert.Equal(t, failed.act.Act, formed[0].act.Act)

	// THE WALK IN, then THE LETHAL SWING: a hit with its roll, and the fall
	// under the swing's act.
	walkIn := only[session.WalkAct](t, closes.to("alice"))
	assert.Equal(t, "alice", walkIn.body.Mover.ID)
	swing := only[session.AttackAct](t, cut.to("alice"))
	assert.Equal(t, session.AttackHit, swing.body.Outcome)
	assert.True(t, *swing.body.Roll.Beaten)
	assert.Positive(t, swing.body.Damage)
	assert.Equal(t, "gate-1", swing.body.To.ID)
	fall := where[session.StateChange](cut.to("alice"), func(c session.StateChange) bool { return c.Change == session.StateDowned })
	require.Len(t, fall, 1)
	assert.Equal(t, swing.act.Act, fall[0].act.Act)
	assert.Equal(t, "gate-1", fall[0].body.Party.ID)
	assert.Equal(t, "monster", fall[0].body.Downed.Kind)

	// THE CAPTAIN'S TURN AND THE WORLD'S: his swing is an act of his own,
	// and the world's clock ticks.
	captains := where[session.AttackAct](ended.to("alice"), func(a session.AttackAct) bool { return a.By.ID == "captain" })
	require.NotEmpty(t, captains)
	assert.Nil(t, captains[0].body.Reaction, "a turn's swing is not a reaction")
	ticks := where[session.ClockEntry](ended.to("alice"), func(c session.ClockEntry) bool { return c.Mark == session.ClockTick })
	assert.NotEmpty(t, ticks)

	// THE FIGHT BRINGS NOBODY (the QA castle's step 6, rpg-project#639): the
	// gate shouts the alarm and alice hears it, and nobody arrives.
	heard := where[session.Perception](ended.to("alice"), func(p session.Perception) bool {
		return p.Sense == session.SenseHearing && p.Heard.Kind == "voice"
	})
	assert.NotEmpty(t, heard, "alice hears the gate shout")
	for _, b := range beats {
		arrived := where[session.ClockEntry](b.to("alice"), func(c session.ClockEntry) bool { return c.Mark == session.ClockArrived })
		assert.Empty(t, arrived, "%s: nobody arrives", b.label)
	}

	// THE RETREAT: the captain's opportunity attack is a reaction under
	// alice's walk's act, and the walk follows it. The walk comes in
	// segments — round the corner wall alice sees new floor mid-walk — and
	// every segment is the one act.
	oa := only[session.AttackAct](t, falls.to("alice"))
	assert.Equal(t, session.FamilyReaction, oa.act.Family)
	require.NotNil(t, oa.body.Reaction)
	assert.Equal(t, "Opportunity Attack", oa.body.Reaction.Name)
	retreat := where[session.WalkAct](falls.to("alice"), func(session.WalkAct) bool { return true })
	require.NotEmpty(t, retreat)
	for _, segment := range retreat {
		assert.Equal(t, oa.act.Act, segment.act.Act, "every segment of the walk is the opportunity attack's act")
		assert.True(t, strings.HasPrefix(segment.act.Act, "move:"), segment.act.Act)
	}

	// EVERY RECIPIENT PERCEIVES ONLY AS ITSELF.
	for _, b := range beats {
		for _, act := range b.acts {
			if p, ok := act.Body.(session.Perception); ok {
				assert.Equal(t, act.Recipient, p.Observer.ID, "%s: a perception is its observer's", b.label)
			}
			assert.Empty(t, act.Undecoded, "%s: %s", b.label, act.Undecoded)
		}
	}
}

// TestCastleTroupeActsTranscript is the QA castle's troupe (rpg-project#639,
// steps 2, 3 and 7): the stranger round the corner gives the feast, the
// captain lets the troupe through, and opening the gate brings the far room,
// civil. It asserts what each act claims and never how many there are.
func TestCastleTroupeActsTranscript(t *testing.T) {
	beats := theTroupe()
	corner, stranger, troupe, gate, wait := beats[0], beats[1], beats[3], beats[5], beats[6]

	// ROUND THE CORNER: alice drops out of the gate's sight.
	lost := map[string]bool{}
	for _, p := range where[session.Perception](corner.to("alice"), func(p session.Perception) bool { return p.Sense == session.SenseSight }) {
		for _, who := range p.body.Sighted.Lost {
			lost[who] = true
		}
	}
	for _, id := range []string{"captain", "gate-1", "gate-2"} {
		assert.True(t, lost[id], "behind the corner alice loses sight of %s", id)
	}

	// THE STRANGER'S LINE: one speak act, its check and the feast inside.
	asked := only[session.SpeakAct](t, stranger.to("alice"))
	assert.Equal(t, "pedlar", asked.body.To.ID)
	assert.Equal(t, "tonight", asked.body.Option)
	require.NotNil(t, asked.body.Roll)
	assert.Equal(t, 5, *asked.body.Roll.Against)
	assert.True(t, *asked.body.Roll.Beaten)
	require.NotNil(t, asked.body.Answer)
	assert.Equal(t, "Feast for the whole court tonight — they’ll want players.", asked.body.Answer.Say)

	// THE TROUPE: the captain's line passes and nothing turns.
	passed := only[session.SpeakAct](t, troupe.to("alice"))
	assert.Equal(t, "troupe", passed.body.Option)
	assert.True(t, *passed.body.Roll.Beaten)
	require.NotNil(t, passed.body.Answer)
	assert.Equal(t, "Through to the hall, then. Don’t dawdle.", passed.body.Answer.Say)
	assert.Empty(t, where[session.StateChange](troupe.to("alice"), func(c session.StateChange) bool { return c.Change == session.StateStance }))

	// THE GATE: alice opens it, and the far room arrives under the door's
	// own act.
	door := only[session.DoorAct](t, gate.to("alice"))
	assert.Equal(t, "alice", door.body.By.ID)
	assert.Equal(t, castleGate, door.body.Door)
	assert.Equal(t, "open", door.body.State)
	arrived := map[string]bool{}
	for _, a := range where[session.ClockEntry](gate.to("alice"), func(c session.ClockEntry) bool { return c.Mark == session.ClockArrived }) {
		arrived[a.body.Arrival.ID] = true
		assert.Equal(t, door.act.Act, a.act.Act, "%s arrives under the door's act", a.body.Arrival.ID)
	}
	assert.True(t, arrived["reserve-1"], "reserve-1 arrives")
	assert.True(t, arrived["reserve-2"], "reserve-2 arrives")

	// THE FAR ROOM WALKS TO THE GATE, CIVIL: both walk, nobody turns and no
	// fight forms.
	for _, id := range []string{"reserve-1", "reserve-2"} {
		walks := where[session.WalkAct](wait.to("alice"), func(w session.WalkAct) bool { return w.Mover.ID == id })
		assert.NotEmpty(t, walks, "%s walks toward the gate", id)
	}
	for _, b := range beats {
		assert.Empty(t, where[session.StateChange](b.to("alice"), func(c session.StateChange) bool { return c.Change == session.StateStance }),
			"%s: nobody turns", b.label)
		assert.Empty(t, where[session.ClockEntry](b.to("alice"), func(c session.ClockEntry) bool { return c.Mark == session.ClockFightFormed }),
			"%s: no fight forms", b.label)
		for _, act := range b.acts {
			assert.Empty(t, act.Undecoded, "%s: %s", b.label, act.Undecoded)
		}
	}
}

// found is one act and its typed body.
type found[B session.ActBody] struct {
	act  session.Act
	body B
}

// where is every act with a B body that keep accepts.
func where[B session.ActBody](acts []session.Act, keep func(B) bool) []found[B] {
	var out []found[B]
	for _, act := range acts {
		if body, ok := act.Body.(B); ok && keep(body) {
			out = append(out, found[B]{act: act, body: body})
		}
	}
	return out
}

// only is the one act with a B body.
func only[B session.ActBody](t *testing.T, acts []session.Act) found[B] {
	t.Helper()
	all := where[B](acts, func(B) bool { return true })
	require.Len(t, all, 1, "exactly one %T", *new(B))
	return all[0]
}

// tell is one act as one line of a transcript: what it is and what it says,
// never its ids, which are minted fresh each run.
func tell(act session.Act) string {
	head := fmt.Sprintf("%-12s %-12s", act.Verb, act.Family)
	if act.Segment > 0 {
		head = fmt.Sprintf("%-12s %-12s", act.Verb, fmt.Sprintf("%s %d", act.Family, act.Segment))
	}
	if act.Undecoded != "" {
		return head + " UNDECODED " + act.Undecoded
	}
	return head + " " + describe(act.Body)
}

func roll(r *session.Roll) string {
	if r == nil {
		return "no roll"
	}
	s := fmt.Sprintf("rolled %d", r.Total)
	if r.Expression != "" {
		s = fmt.Sprintf("rolled %s = %d", r.Expression, r.Total)
	}
	if r.Of > 0 {
		s = fmt.Sprintf("rolled %d of %d", r.Total, r.Of)
	}
	if r.Against != nil {
		verdict := "missed"
		if r.Beaten != nil && *r.Beaten {
			verdict = "beat"
		}
		s += fmt.Sprintf(", %s %d", verdict, *r.Against)
	}
	return s
}

//nolint:gocyclo // one arm per body
func describe(body session.ActBody) string {
	switch b := body.(type) {
	case session.WalkAct:
		cells := make([]string, 0, len(b.Path))
		for _, st := range b.Path {
			c := fmt.Sprintf("(%g,%g)", st.Cell.X, st.Cell.Y)
			if len(st.Doors) > 0 {
				c += " through " + strings.Join(st.Doors, ",")
			}
			cells = append(cells, c)
		}
		return fmt.Sprintf("%s walks %s", b.Mover.ID, strings.Join(cells, " "))
	case session.AttackAct:
		s := fmt.Sprintf("%s swings %s at %s: %s, %s", b.By.ID, b.Attack.Name, b.To.ID, b.Outcome, roll(b.Roll))
		if b.Damage > 0 {
			s += fmt.Sprintf(", %d damage", b.Damage)
		}
		if b.Reaction != nil {
			s += " (" + b.Reaction.Name + ")"
		}
		return s
	case session.SpeakAct:
		s := fmt.Sprintf("%s to %s, %q (%s): %s", b.By.ID, b.To.ID, b.Option, b.Skill, roll(b.Roll))
		if b.Answer != nil {
			s += fmt.Sprintf("; %s answers %q", b.Answer.Who.ID, b.Answer.Say)
		}
		return s
	case session.DoorAct:
		s := fmt.Sprintf("%s's door %s is %s", b.By.ID, b.Door, b.State)
		if b.Roll != nil {
			s += ", " + roll(b.Roll)
		}
		return s
	case session.SneakAct:
		return fmt.Sprintf("%s sneaks past %s: %s", b.By.ID, b.Watcher.ID, roll(b.Roll))
	case session.StayAct:
		return fmt.Sprintf("%s stays: %s", b.Member.ID, b.Why)
	case session.TableAnswer:
		if b.Dealt != nil {
			return fmt.Sprintf("%s is dealt %q, %s", b.Who.ID, b.Dealt.Temper, roll(&b.Dealt.Roll))
		}
		return fmt.Sprintf("%s answers %s with %q, %s", b.Who.ID, b.Key, b.Word, roll(b.Roll))
	case session.StateChange:
		switch b.Change {
		case session.StateDowned:
			return fmt.Sprintf("%s is down (%s)", b.Party.ID, b.Downed.Kind)
		case session.StateExperience:
			return fmt.Sprintf("%s's fall pays %d grant(s)", b.Party.ID, len(b.Experience))
		case session.StateStance:
			return fmt.Sprintf("%s and %s turn %s", b.Stance.Between[0].ID, b.Stance.Between[1].ID, b.Stance.Stance)
		default:
			return fmt.Sprintf("%s: %s", b.Party.ID, b.Change)
		}
	case session.Perception:
		switch b.Sense {
		case session.SenseSight:
			return fmt.Sprintf("%s sees %v, loses %v, sees %v change", b.Observer.ID, b.Sighted.Gained, b.Sighted.Lost, b.Sighted.Changed)
		case session.SenseHearing:
			return fmt.Sprintf("%s hears %s", b.Observer.ID, b.Heard.Kind)
		case session.SenseCells:
			return fmt.Sprintf("%s learns %d region(s)", b.Observer.ID, len(b.Cells.Regions))
		default:
			return fmt.Sprintf("%s perceives by %s", b.Observer.ID, b.Sense)
		}
	case session.ClockEntry:
		switch b.Mark {
		case session.ClockTick:
			return fmt.Sprintf("tick %d", b.Tick)
		case session.ClockTurnEnded:
			return fmt.Sprintf("%s's turn ends, %s's begins", b.Turn.Member.ID, b.Turn.Next.ID)
		case session.ClockFightFormed:
			ids := make([]string, 0, len(b.Fight.Order))
			for _, p := range b.Fight.Order {
				ids = append(ids, p.ID)
			}
			return "a fight forms: " + strings.Join(ids, ", ")
		case session.ClockWindowOpened:
			return fmt.Sprintf("%s's step asks %s", b.Window.Member.ID, b.Window.Windows[0].Audience.ID)
		case session.ClockJoined:
			return b.Member.ID + " joins"
		case session.ClockArrived:
			return fmt.Sprintf("%s arrives (%s)", b.Arrival.ID, b.Arrival.Kind)
		default:
			return string(b.Mark)
		}
	default:
		return fmt.Sprintf("%T", body)
	}
}

// Example_theGatehouseActs is the castle's gatehouse as alice receives it:
// each verb of the route, and the acts it folded for her.
func Example_theGatehouseActs() {
	for _, b := range theGatehouse() {
		fmt.Println("-- " + b.label + " --")
		for _, act := range b.to("alice") {
			fmt.Println(strings.TrimRight(tell(act), " "))
		}
	}
	// Output:
	// -- alice tells the captain to stand aside --
	// speak        act 1        alice to captain, "stand-aside" (intimidation): rolled 1d20+3 = 17, beat 14; captain answers "…go on, then. I saw nothing."
	// speak        perception   alice hears voice
	// speak        clock        tick 1
	// time         table_answer captain answers time with "hold", no roll
	// time         table_answer cook answers time with "hold", rolled 1 of 100
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-1 walks (-6,9) (-5,8)
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-1 stays: standing at its waypoint
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-2 walks (-2,10)
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-2 walks (-2,9) (-2,8)
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-2 stays: standing at its waypoint
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (13,-11) (12,-11) (11,-11) (10,-11) (9,-11) (8,-11)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (7,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (7,-20) (7,-21)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (6,-21) (5,-21)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (5,-20)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (15,-19) (16,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (16,-20) (16,-21)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (15,-21) (14,-21)
	// time         table_answer king answers time with "hold", rolled 1 of 50
	// time         table_answer pedlar answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-1 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-2 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-3 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-4 answers time with "hold", rolled 1 of 100
	// time         table_answer scullion answers time with "hold", rolled 1 of 100
	// time         table_answer sergeant answers time with "hold", rolled 1 of 100
	// time         table_answer turnkey answers time with "hold", rolled 1 of 100
	// -- alice tells him again --
	// speak        act 1        alice to captain, "stand-aside" (intimidation): rolled 1d20+3 = 8, missed 14
	// speak        state        garrison and party turn hostile
	// speak        clock        a fight forms: alice, captain, gate-1, gate-2
	// speak        table_answer captain answers failure with "fact", rolled 1 of 100
	// speak        perception   alice hears voice
	// -- alice closes on gate-1 --
	// move         act 1        alice walks (-5,10) (-5,9)
	// -- alice cuts gate-1 down --
	// attack       act 1        alice swings Longsword at gate-1: hit, rolled 1d20+5 = 20, beat 13, 4 damage
	// attack       state        gate-1 is down (monster)
	// attack       perception   alice sees [], loses [], sees [gate-1] change
	// -- alice ends her turn --
	// end_turn     clock        alice's turn ends, captain's begins
	// time         table_answer captain answers time with "attack", rolled 1 of 50
	// attack       act 1        captain swings Longsword at alice: hit, rolled 1d20+5 = 20, beat 12, 3 damage
	// attack       perception   alice hears clang
	// attack       perception   alice sees [], loses [], sees [captain] change
	// time         table_answer captain answers time with "toward", rolled 1 of 50
	// pass         act 1        captain stays: already in reach
	// end_turn     clock        captain's turn ends, gate-2's begins
	// time         table_answer gate-2 answers time with "fact", rolled 1 of 200
	// time         perception   alice hears voice
	// end_turn     clock        gate-2's turn ends, alice's begins
	// end_turn     clock        tick 2
	// time         table_answer cook answers time with "hold", rolled 1 of 100
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (7,-11) (6,-11) (6,-12)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (4,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (5,-19) (6,-19) (7,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (7,-20) (7,-21)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (14,-20) (13,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (14,-19) (15,-19) (16,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (16,-20)
	// time         table_answer king answers time with "hold", rolled 1 of 50
	// time         table_answer pedlar answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-1 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-2 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-3 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-4 answers time with "hold", rolled 1 of 100
	// time         table_answer scullion answers time with "hold", rolled 1 of 100
	// time         table_answer sergeant answers time with "hold", rolled 1 of 100
	// time         table_answer turnkey answers time with "hold", rolled 1 of 100
	// -- alice falls back past the captain --
	// move         reaction     captain swings Longsword at alice: hit, rolled 1d20+5 = 20, beat 12, 3 damage (Opportunity Attack)
	// move         perception   alice hears clang
	// move         act 1        alice walks (-6,9) (-7,9)
	// move         perception   alice learns 1 region(s)
	// move         act 2        alice walks (-7,9) (-8,10)
}

// Example_theTroupeActs is the castle's troupe as alice receives it: each
// verb of the route, and the acts it folded for her.
func Example_theTroupeActs() {
	for _, b := range theTroupe() {
		fmt.Println("-- " + b.label + " --")
		for _, act := range b.to("alice") {
			fmt.Println(strings.TrimRight(tell(act), " "))
		}
	}
	// Output:
	// -- alice goes round the corner --
	// move         act 1        alice walks (-6,11) (-7,11)
	// move         perception   alice learns 1 region(s)
	// move         act 2        alice walks (-7,11) (-7,10) (-7,9) (-7,8)
	// move         act 1        alice walks (-6,7)
	// move         clock        tick 1
	// time         table_answer captain answers time with "hold", no roll
	// time         table_answer cook answers time with "hold", rolled 1 of 100
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-1 walks (-6,9) (-5,8)
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-1 stays: standing at its waypoint
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-2 walks (-2,10)
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-2 walks (-2,9) (-2,8)
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-2 stays: standing at its waypoint
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (13,-11) (12,-11) (11,-11) (10,-11) (9,-11) (8,-11)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (7,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (7,-20) (7,-21)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (6,-21) (5,-21)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (5,-20)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (15,-19) (16,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (16,-20) (16,-21)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (15,-21) (14,-21)
	// time         table_answer king answers time with "hold", rolled 1 of 50
	// time         table_answer pedlar answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-1 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-2 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-3 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-4 answers time with "hold", rolled 1 of 100
	// time         table_answer scullion answers time with "hold", rolled 1 of 100
	// time         table_answer sergeant answers time with "hold", rolled 1 of 100
	// time         table_answer turnkey answers time with "hold", rolled 1 of 100
	// move         perception   alice sees [], loses [captain gate-1 gate-2], sees [] change
	// -- alice asks the stranger what the bustle is --
	// speak        act 1        alice to pedlar, "tonight" (persuasion): rolled 1d20+3 = 15, beat 5; pedlar answers "Feast for the whole court tonight — they’ll want players."
	// speak        perception   alice hears voice
	// speak        clock        tick 2
	// time         table_answer captain answers time with "hold", no roll
	// time         table_answer cook answers time with "hold", rolled 1 of 100
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-1 stays: standing at its waypoint
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-2 stays: standing at its waypoint
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (7,-11) (6,-11) (6,-12)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (4,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (5,-19) (6,-19) (7,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (7,-20) (7,-21)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (14,-20) (13,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (14,-19) (15,-19) (16,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (16,-20)
	// time         table_answer king answers time with "hold", rolled 1 of 50
	// time         table_answer pedlar answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-1 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-2 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-3 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-4 answers time with "hold", rolled 1 of 100
	// time         table_answer scullion answers time with "hold", rolled 1 of 100
	// time         table_answer sergeant answers time with "hold", rolled 1 of 100
	// time         table_answer turnkey answers time with "hold", rolled 1 of 100
	// -- alice walks up to the captain --
	// move         act 1        alice walks (-7,8)
	// move         perception   alice sees [captain gate-1 gate-2], loses [], sees [] change
	// move         act 2        alice walks (-7,8) (-7,9) (-6,9) (-5,9)
	// -- alice tells the captain they are the troupe --
	// speak        act 1        alice to captain, "troupe" (performance): rolled 1d20+3 = 15, beat 13; captain answers "Through to the hall, then. Don’t dawdle."
	// speak        perception   alice hears voice
	// speak        clock        tick 3
	// time         table_answer captain answers time with "hold", no roll
	// time         table_answer cook answers time with "hold", rolled 1 of 100
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-1 walks (-5,9) (-5,10)
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-1 stays: standing at its waypoint
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-2 walks (-3,9) (-4,10)
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-2 stays: standing at its waypoint
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (6,-13) (6,-14) (6,-15)
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (5,-15) (4,-15) (3,-15)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (6,-21) (5,-21)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (5,-20) (4,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (5,-19) (6,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (16,-21)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (15,-21) (14,-21)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (14,-20) (13,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (14,-19)
	// time         table_answer king answers time with "hold", rolled 1 of 50
	// time         table_answer pedlar answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-1 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-2 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-3 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-4 answers time with "hold", rolled 1 of 100
	// time         table_answer scullion answers time with "hold", rolled 1 of 100
	// time         table_answer sergeant answers time with "hold", rolled 1 of 100
	// time         table_answer turnkey answers time with "hold", rolled 1 of 100
	// -- alice walks under the gate --
	// move         act 1        alice walks (-5,8) (-4,7)
	// move         clock        tick 4
	// time         table_answer captain answers time with "hold", no roll
	// time         table_answer cook answers time with "hold", rolled 1 of 100
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-1 stays: standing at its waypoint
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-2 stays: standing at its waypoint
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (2,-15) (1,-15)
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (0,-14) (-1,-13) (-2,-12)
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (-1,-12)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (7,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (7,-20) (7,-21)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (6,-21) (5,-21)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (5,-20)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (15,-19) (16,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (16,-20) (16,-21)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (15,-21) (14,-21)
	// time         table_answer king answers time with "hold", rolled 1 of 50
	// time         table_answer pedlar answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-1 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-2 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-3 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-4 answers time with "hold", rolled 1 of 100
	// time         table_answer scullion answers time with "hold", rolled 1 of 100
	// time         table_answer sergeant answers time with "hold", rolled 1 of 100
	// time         table_answer turnkey answers time with "hold", rolled 1 of 100
	// move         perception   alice sees [], loses [pedlar], sees [] change
	// move         act 2        alice walks (-4,7) (-3,7)
	// -- alice opens the gate --
	// open_door    act 1        alice's door the-mad-king/gate-wall-door is open
	// open_door    clock        reserve-1 arrives (monster)
	// open_door    table_answer reserve-1 is dealt "coward", rolled 1 of 3
	// open_door    clock        reserve-2 arrives (monster)
	// open_door    table_answer reserve-2 is dealt "coward", rolled 1 of 3
	// open_door    perception   alice learns 1 region(s)
	// open_door    perception   alice sees [reserve-1 reserve-2 sergeant turnkey], loses [], sees [] change
	// -- alice waits at the gate --
	// move         act 1        alice walks (-4,7)
	// move         perception   alice learns 1 region(s)
	// move         perception   alice sees [], loses [reserve-1 reserve-2 sergeant turnkey], sees [] change
	// move         act 2        alice walks (-4,7) (-3,7)
	// move         perception   alice sees [reserve-1 reserve-2 sergeant turnkey], loses [], sees [] change
	// move         act 3        alice walks (-3,7) (-4,7)
	// move         perception   alice sees [], loses [reserve-1 reserve-2 sergeant turnkey], sees [] change
	// move         act 4        alice walks (-4,7) (-3,7)
	// move         perception   alice sees [reserve-1 reserve-2 sergeant turnkey], loses [], sees [] change
	// move         act 5        alice walks (-3,7) (-4,7)
	// move         clock        tick 5
	// time         table_answer captain answers time with "hold", no roll
	// time         table_answer cook answers time with "hold", rolled 1 of 100
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-1 walks (-6,10) (-7,10)
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-1 walks (-6,9) (-5,8)
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-1 stays: standing at its waypoint
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-2 walks (-3,10) (-2,10)
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-2 walks (-2,9) (-2,8)
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// pass         act 1        gate-2 stays: standing at its waypoint
	// time         table_answer hall-1 answers time with "toward", rolled 1 of 50
	// move         act 1        hall-1 walks (0,-12) (1,-12) (2,-12) (3,-12) (4,-12) (5,-12)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (4,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (5,-19) (6,-19) (7,-19)
	// time         table_answer keep-1 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-1 walks (7,-20) (7,-21)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (14,-20) (13,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (14,-19) (15,-19) (16,-19)
	// time         table_answer keep-2 answers time with "toward", rolled 1 of 300
	// move         act 1        keep-2 walks (16,-20)
	// time         table_answer king answers time with "hold", rolled 1 of 50
	// time         table_answer pedlar answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-1 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-2 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-3 answers time with "hold", rolled 1 of 100
	// time         table_answer prisoner-4 answers time with "hold", rolled 1 of 100
	// time         table_answer reserve-1 answers time with "toward", rolled 1 of 50
	// move         act 1        reserve-1 walks (-20,1) (-20,2) (-20,3) (-20,4) (-19,4) (-18,4)
	// time         table_answer reserve-2 answers time with "toward", rolled 1 of 50
	// move         act 1        reserve-2 walks (-19,4) (-18,4) (-17,4) (-16,4) (-15,4) (-14,4)
	// time         table_answer scullion answers time with "hold", rolled 1 of 100
	// time         table_answer sergeant answers time with "hold", rolled 1 of 100
	// time         table_answer turnkey answers time with "hold", rolled 1 of 100
	// move         perception   alice sees [], loses [reserve-1 reserve-2 sergeant turnkey], sees [] change
	// move         act 6        alice walks (-4,7) (-3,7)
	// move         perception   alice sees [reserve-1 reserve-2 sergeant turnkey], loses [], sees [] change
}
