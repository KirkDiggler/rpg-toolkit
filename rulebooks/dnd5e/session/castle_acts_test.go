// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_acts_test.go is E3's castle check (rpg-project#629): the mad king's
// gatehouse played through the session seam, and every recipient's acts
// folded and told.

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
		Events: session.DiscardEvents{},
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
	beats := playTheGatehouse()
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

	// THE RETREAT: the captain's opportunity attack is a reaction under
	// alice's walk's act, and the walk follows it.
	oa := only[session.AttackAct](t, falls.to("alice"))
	assert.Equal(t, session.FamilyReaction, oa.act.Family)
	require.NotNil(t, oa.body.Reaction)
	assert.Equal(t, "Opportunity Attack", oa.body.Reaction.Name)
	retreat := only[session.WalkAct](t, falls.to("alice"))
	assert.Equal(t, oa.act.Act, retreat.act.Act)
	assert.True(t, strings.HasPrefix(retreat.act.Act, "move:"), retreat.act.Act)

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
	for _, b := range playTheGatehouse() {
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
	// time         table_answer gate-1 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-1 walks (-7,9)
	// time         table_answer gate-2 answers time with "toward", rolled 1 of 50
	// move         act 1        gate-2 walks (-2,9) (-1,9)
	// -- alice tells him again --
	// speak        act 1        alice to captain, "stand-aside" (intimidation): rolled 1d20+3 = 8, missed 14
	// speak        state        garrison and party turn hostile
	// speak        clock        a fight forms: alice, captain, gate-1, gate-2
	// speak        table_answer captain answers failure with "fact", rolled 1 of 100
	// speak        perception   alice hears voice
	// -- alice closes on gate-1 --
	// move         act 1        alice walks (-6,11) (-7,11) (-7,10)
	// -- alice cuts gate-1 down --
	// attack       act 1        alice swings Longsword at gate-1: hit, rolled 1d20+5 = 20, beat 13, 4 damage
	// attack       state        gate-1 is down (monster)
	// attack       perception   alice sees [], loses [], sees [gate-1] change
	// -- alice ends her turn --
	// end_turn     clock        alice's turn ends, captain's begins
	// time         table_answer captain answers time with "toward", rolled 1 of 50
	// move         act 1        captain walks (-5,8) (-6,9)
	// time         table_answer captain answers time with "attack", rolled 1 of 50
	// attack       act 1        captain swings Longsword at alice: hit, rolled 1d20+5 = 20, beat 12, 3 damage
	// attack       perception   alice hears clang
	// time         table_answer captain answers time with "toward", rolled 1 of 50
	// pass         act 1        captain stays: already in reach
	// end_turn     clock        captain's turn ends, gate-2's begins
	// time         table_answer gate-2 answers time with "fact", rolled 1 of 200
	// time         perception   alice hears voice
	// end_turn     clock        gate-2's turn ends, alice's begins
	// end_turn     clock        tick 2
	// -- alice falls back past the captain --
	// move         reaction     captain swings Longsword at alice: hit, rolled 1d20+5 = 20, beat 12, 3 damage (Opportunity Attack)
	// move         perception   alice hears clang
	// move         act 1        alice walks (-8,10) (-9,11)
}
