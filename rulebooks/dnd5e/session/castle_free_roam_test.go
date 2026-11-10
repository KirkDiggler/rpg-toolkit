// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_free_roam_test.go is rpg-project#785 S1 on the QA castle: the bar on
// the world clock. In free roam the party is offered the verbs a turn offers,
// priced in rounds of the world (R1). A cast that hurts nobody passes a round
// and forms nothing; a swing at the neutral captain lands, the aggression law
// turns the garrison (R2), and the fight forms after the blow with the gate
// posts in it (R3).

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// freeRoamCastle is the castle opened for alice (a fighter) and a cleric at
// the party start, outside the gatehouse.
type freeRoamCastle struct {
	mgr        *session.Manager
	live       *actCapture
	characters *fakeCharacters
	encounters *fakeEncounters
	dice       *castleDice
	sessions   *fakeSessions
	seats      *fakeSeats
}

// manager is a Manager over the castle's own repositories: a second one is
// the host reloading the session from what was saved.
func (c *freeRoamCastle) manager(t *testing.T) *session.Manager {
	t.Helper()
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: c.live, Seats: c.seats, CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: c.dice, TurnDriver: session.Driver(),
		Sessions: c.sessions, Encounters: c.encounters, Characters: c.characters,
	})
	require.NoError(t, err)
	return mgr
}

func openTheCastleForTwo(t *testing.T, d20s ...int) *freeRoamCastle {
	t.Helper()
	alice := armedFighter("alice")
	alice.AbilityScores[abilities.CHA] = 16
	return openTheCastleFor(t, []*character.Data{alice, healingCleric()}, d20s...)
}

// openTheCastleFor is the castle opened for the given party at the party
// start, outside the gatehouse.
func openTheCastleFor(t *testing.T, party []*character.Data, d20s ...int) *freeRoamCastle {
	t.Helper()
	return openTheCastleFrom(t, castleSource(t), party, d20s...)
}

// castleSource is the castle's YAML as the encounter module pinned here
// ships it.
func castleSource(t *testing.T) []byte {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", encounterModule).Output()
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), castleFixture))
	require.NoError(t, err)
	return raw
}

// openTheCastleFrom is [openTheCastleFor] over a given castle source.
func openTheCastleFrom(t *testing.T, raw []byte, party []*character.Data, d20s ...int) *freeRoamCastle {
	t.Helper()
	compiled, err := dungeonspec.Load(raw)
	require.NoError(t, err)

	ids := make([]string, 0, len(party))
	for _, sheet := range party {
		ids = append(ids, sheet.ID)
	}
	c := &freeRoamCastle{live: &actCapture{}, characters: newFakeCharacters(party...), encounters: newFakeEncounters(), dice: &castleDice{d20s: d20s},
		sessions: newFakeSessions(), seats: newFakeSeats()}
	c.mgr = c.manager(t)
	_, err = c.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "sess", Dungeon: &compiled, Party: ids,
	})
	require.NoError(t, err)
	return c
}

// play runs one verb and returns every act it published.
func (c *freeRoamCastle) play(t *testing.T, label string, verb func() error) beat {
	t.Helper()
	c.live.mu.Lock()
	c.live.acts = nil
	c.live.mu.Unlock()
	require.NoError(t, verb(), label)
	c.live.mu.Lock()
	defer c.live.mu.Unlock()
	return beat{label: label, acts: append([]session.Act(nil), c.live.acts...)}
}

func (c *freeRoamCastle) bar(t *testing.T, member string) *session.AffordOutput {
	t.Helper()
	out, err := c.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	require.NoError(t, err)
	return out
}

func (c *freeRoamCastle) row(t *testing.T, member string, verb session.Verb, ref string) session.Declaration {
	t.Helper()
	for _, d := range c.bar(t, member).Declarations {
		if d.Verb != verb {
			continue
		}
		if ref == "" || (d.Spell != nil && d.Spell.Ref == ref) || (d.Ability != nil && d.Ability.Ref == ref) {
			return d
		}
	}
	require.Failf(t, "no row", "%s offered no %s row %s", member, verb, ref)
	return session.Declaration{}
}

// TestAfford_WorldClockOffersTheBar: in free roam on the castle, the bar is
// the turn's bar priced in rounds of the world. Alice is offered Attack and
// her abilities, the cleric her spells, both the social verb; every row is
// SlotNone with no turn-currency shortfall; Move, EndTurn and Death Save are
// not world rows. The turn-clock goldens are pinned by the rest of the suite
// (afford_test.go, the gatehouse route) and are unchanged.
func TestAfford_WorldClockOffersTheBar(t *testing.T) {
	c := openTheCastleForTwo(t, 18)

	for member, want := range map[string][]session.Verb{
		"alice":  {session.VerbAttack, session.VerbActivate, session.VerbSpeak},
		"cleric": {session.VerbAttack, session.VerbCast, session.VerbSpeak},
	} {
		out := c.bar(t, member)
		require.Equal(t, session.ClockWorld, out.Clock, "%s: free roam at the party start", member)
		verbs := declarationVerbs(out.Declarations)
		for _, verb := range want {
			assert.Contains(t, verbs, verb, "%s: %s is on the free-roam bar", member, verb)
		}
		for _, turnOnly := range []session.Verb{session.VerbMove, session.VerbEndTurn, session.VerbDeathSave} {
			assert.NotContains(t, verbs, turnOnly, "%s: %s is a turn's row", member, turnOnly)
		}
		for _, d := range out.Declarations {
			assert.Equal(t, session.SlotNone, d.Slot, "%s %s: no slot on the world clock", member, d.Verb)
			if d.Why != nil {
				assert.NotEqual(t, session.ShortfallNotYourTurn, d.Why.Reason, "%s %s: nothing is greyed for a turn", member, d.Verb)
				assert.NotContains(t, []session.Currency{session.CurrencyAction, session.CurrencyBonus, session.CurrencyReaction},
					d.Why.Currency, "%s %s: no turn currency falls short", member, d.Verb)
			}
			for _, component := range d.Cost {
				assert.Equal(t, session.CurrencyCharges, component.Currency, "%s %s: only the sheet's own charge is priced", member, d.Verb)
			}
		}
	}

	// Speak is the R3 row, unchanged: sheetless, unpriced, one row.
	speak := c.row(t, "alice", session.VerbSpeak, "")
	assert.Equal(t, session.SlotNone, speak.Slot)
	assert.NotNil(t, speak.Information)

	// A cast the cleric can make on herself out here: Guidance, available.
	guidance := c.row(t, "cleric", session.VerbCast, refs.Spells.Guidance().String())
	assert.True(t, guidance.Available, "%+v", guidance.Why)
}

// TestCastleFreeRoamCastAndSwing is the slice's walk as a script: the cleric
// casts Guidance on herself in free roam and a round of the world passes and
// nothing forms; alice walks up to the captain and swings, the blow lands,
// the garrison turns hostile under the swing's act, and the fight forms with
// the captain and both gate posts in it.
func TestCastleFreeRoamCastAndSwing(t *testing.T) {
	ctx := context.Background()
	c := openTheCastleForTwo(t, 18)

	// THE HARMLESS CAST: a round passes, nobody turns, no fight forms.
	guidance := c.row(t, "cleric", session.VerbCast, refs.Spells.Guidance().String())
	require.True(t, guidance.Available, "%+v", guidance.Why)
	cast := c.play(t, "the cleric casts Guidance on herself", func() error {
		_, err := c.mgr.Cast(ctx, &session.CastInput{
			Session: "sess", Member: "cleric", DeclarationID: guidance.ID, Targets: []string{"cleric"},
		})
		return err
	})
	casts := where[session.CastAct](cast.to("cleric"), func(session.CastAct) bool { return true })
	require.NotEmpty(t, casts, "the cast is told to its caster")
	ticks := where[session.ClockEntry](cast.to("cleric"), func(e session.ClockEntry) bool { return e.Mark == session.ClockTick })
	assert.NotEmpty(t, ticks, "a round of the world passes for the cast (R1)")
	assert.Empty(t, where[session.StateChange](cast.to("cleric"), func(s session.StateChange) bool { return s.Change == session.StateStance }),
		"a cast that hurts nobody turns nobody")
	assert.Empty(t, where[session.ClockEntry](cast.to("cleric"), func(e session.ClockEntry) bool { return e.Mark == session.ClockFightFormed }),
		"a cast that hurts nobody forms nothing")
	assert.Nil(t, storedData(t, c.characters, "cleric").ActionEconomy, "the round lent to the cast is not kept")

	// ALICE WALKS UP TO THE CAPTAIN, civil, on the world clock.
	c.play(t, "alice walks up to the captain", func() error {
		enc, err := c.mgr.LoadWorldForTest(ctx, "sess")
		if err != nil {
			return err
		}
		route, err := enc.Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: standingAt(enc, "captain"), Budget: 200})
		if err != nil {
			return err
		}
		_, err = c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: route.Path})
		return err
	})
	attack := c.row(t, "alice", session.VerbAttack, "")
	require.Equal(t, session.SlotNone, attack.Slot)
	captain, ok := castCandidate(t, attack, "captain")
	require.True(t, ok, "the captain is a candidate for the free-roam swing")
	require.True(t, captain.Available, "alice stands beside the captain: %+v", captain.Why)

	// THE FREE SWING: the deed lands on the world clock first (R3), the
	// aggression law turns the garrison under the swing's act (R2), and the
	// fight forms with the gate posts in it.
	swing := c.play(t, "alice swings at the captain", func() error {
		_, err := c.mgr.Attack(ctx, &session.AttackInput{
			Session: "sess", Attacker: "alice", Target: "captain", DeclarationID: attack.ID,
		})
		return err
	})
	blow := only[session.AttackAct](t, swing.to("alice"))
	assert.Equal(t, "captain", blow.body.To.ID)
	assert.Equal(t, session.AttackHit, blow.body.Outcome, "the free blow lands")
	turned := where[session.StateChange](swing.to("alice"), func(s session.StateChange) bool { return s.Change == session.StateStance })
	require.NotEmpty(t, turned, "the garrison turns hostile")
	assert.Equal(t, "hostile", turned[0].body.Stance.Stance)
	formed := where[session.ClockEntry](swing.to("alice"), func(e session.ClockEntry) bool { return e.Mark == session.ClockFightFormed })
	require.NotEmpty(t, formed, "the fight forms after the blow")
	inFight := map[string]bool{}
	for _, p := range formed[0].body.Fight.Order {
		inFight[p.ID] = true
	}
	for _, id := range []string{"alice", "captain", "gate-1", "gate-2"} {
		assert.True(t, inFight[id], "%s is in the fight the swing formed: %v", id, formed[0].body.Fight.Order)
	}
	// THEN ONE WORLD ROUND (R1): the free swing pays the world's price for an
	// action, as a cast does (encounter v0.170.1, rpg-toolkit#2239), after
	// the blow, the turn and the fight it formed.
	swingTicks := where[session.ClockEntry](swing.to("alice"), func(e session.ClockEntry) bool { return e.Mark == session.ClockTick })
	require.Len(t, swingTicks, 1, "the free swing passes one round of the world")
	assert.Equal(t, blow.act.Act, swingTicks[0].act.Act, "the round is the swing's own")
	order := map[string]int{}
	for i, a := range swing.to("alice") {
		switch body := a.Body.(type) {
		case session.AttackAct:
			order["blow"] = i
		case session.StateChange:
			if body.Change == session.StateStance {
				order["stance"] = i
			}
		case session.ClockEntry:
			if body.Mark == session.ClockFightFormed {
				order["formed"] = i
			}
			if body.Mark == session.ClockTick {
				order["tick"] = i
			}
		}
	}
	assert.Less(t, order["blow"], order["stance"], "the blow lands before the garrison turns")
	assert.Less(t, order["stance"], order["formed"], "the garrison turns before the fight forms")
	assert.Less(t, order["formed"], order["tick"], "then the world's round")

	turn, err := c.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	assert.Equal(t, session.ClockTurn, turn.Clock, "the next look is a fight")
	// THE FIGHT'S ENTRANT ECONOMY STANDS. The swing formed the fight in its
	// own verb, and the boundary seated alice with the economy every entrant
	// gets, reaction included: the lent round is dropped, never this.
	entrant := storedData(t, c.characters, "alice").ActionEconomy
	require.NotNil(t, entrant, "alice enters the fight she started with the entrant's economy")
	assert.Equal(t, 1, entrant.ReactionsRemaining, "she holds her reaction before her first turn")

	// A SECOND SWING INSIDE THE FIGHT TICKS NOTHING: the round prices time
	// there, and a turn-clock action paying a world round would cost twice.
	require.Equal(t, "alice", string(turn.Active), "alice acts first in the fight she started")
	second := c.play(t, "alice swings again, inside the fight", func() error {
		_, err := c.mgr.Attack(ctx, &session.AttackInput{
			Session: "sess", Attacker: "alice", Target: "captain",
			DeclarationID: currentAttackID(t, c.mgr, "sess", "alice"),
		})
		return err
	})
	require.NotEmpty(t, where[session.AttackAct](second.to("alice"), func(session.AttackAct) bool { return true }), "the second swing is told")
	assert.Empty(t, where[session.ClockEntry](second.to("alice"), func(e session.ClockEntry) bool { return e.Mark == session.ClockTick }),
		"a swing on the turn clock pays no world round")
}

// storedData is a member's stored sheet.
func storedData(t *testing.T, chars *fakeCharacters, id string) *character.Data {
	t.Helper()
	sheet, err := chars.GetCharacter(context.Background(), id)
	require.NoError(t, err, fmt.Sprintf("load %s", id))
	return sheet
}

// TestCastleFreeRoamThunderwaveShovesBeforeTheFight is rpg-project#831: the
// bard walks up to the gatehouse in free roam and casts Thunderwave at the
// gate guards beside the captain. The cast is hostile, so the garrison turns and the fight forms
// inside the deed (#785 R3), and the shove is part of the deed landing: every
// moved beat the spell caused follows the cast beat and is on the story
// before the fight forms and before any turn of it passes, and a pushed
// guard's own first step leaves from the cell the blast threw it to.
func TestCastleFreeRoamThunderwaveShovesBeforeTheFight(t *testing.T) {
	ctx := context.Background()
	// The d20s in the order they are rolled: the three saves fail (1, 1, 1),
	// so the cube shoves everyone it caught who has room to go (gate-1,
	// posted beneath the shut gate since rpg-project#836, has none); the bard
	// loses initiative (1) and the garrison wins it (20s), so the fight's
	// first turns are played inside the cast, as on the walk; the guards'
	// swings miss (1), so the bard stands and the story stays open.
	c := openTheCastleFor(t, []*character.Data{castingBardWithSpells("bard", spells.Thunderwave)}, 1, 1, 1, 1, 20, 20, 20, 1)

	c.play(t, "the bard walks up to the captain", func() error {
		enc, err := c.mgr.LoadWorldForTest(ctx, "sess")
		if err != nil {
			return err
		}
		route, err := enc.Route(encounter.RouteInput{Mover: "bard", Policy: encounter.MoveToward, Anchor: standingAt(enc, "captain"), Budget: 200})
		if err != nil {
			return err
		}
		_, err = c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "bard", Path: route.Path})
		return err
	})
	turn, err := c.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "bard"})
	require.NoError(t, err)
	require.NotEqual(t, session.ClockTurn, turn.Clock, "the bard casts from free roam")

	before := lastSeq(storyBeats(t, c.mgr, "sess", "bard"))
	enc, err := c.mgr.LoadWorldForTest(ctx, "sess")
	require.NoError(t, err)
	aim := standingAt(enc, "gate-2")
	row := c.row(t, "bard", session.VerbCast, refs.Spells.Thunderwave().String())
	require.True(t, row.Available, "%+v", row.Why)
	_, err = c.mgr.Cast(ctx, &session.CastInput{Session: "sess", Member: "bard", Cell: &aim, DeclarationID: row.ID})
	require.NoError(t, err)

	type movedBeat struct {
		Member   string           `json:"member"`
		Cause    string           `json:"cause"`
		Position spatial.Position `json:"position"`
	}
	thunder := refs.Spells.Thunderwave().String()
	story := storyBeatsAfter(t, c.mgr, "sess", "bard", before)
	castAt, firstPushAt, formedAt, lastPushAt := -1, -1, -1, -1
	var turnsEndedAt []int
	pushedTo := map[string]spatial.Position{}
	firstStep := map[string]spatial.Position{}
	for i, b := range story {
		switch b.Beat {
		case encounter.BeatCast:
			if castAt < 0 {
				castAt = i
			}
		case encounter.BeatFightStarted:
			if formedAt < 0 {
				formedAt = i
			}
		case encounter.BeatTurnEnded:
			turnsEndedAt = append(turnsEndedAt, i)
		case encounter.BeatMoved:
			var m movedBeat
			b.decode(t, &m)
			if m.Cause == thunder {
				if firstPushAt < 0 {
					firstPushAt = i
				}
				lastPushAt = i
				pushedTo[m.Member] = m.Position
				continue
			}
			if _, pushed := pushedTo[m.Member]; pushed && formedAt >= 0 {
				if _, seen := firstStep[m.Member]; !seen {
					firstStep[m.Member] = m.Position
				}
			}
		}
	}

	require.NotEmpty(t, pushedTo, "the failed saves were shoved")
	require.GreaterOrEqual(t, formedAt, 0, "the hostile cast forms the fight")
	require.NotEmpty(t, turnsEndedAt, "the garrison won initiative and took its turns inside the cast")
	require.GreaterOrEqual(t, castAt, 0, "the cast is told")
	assert.Less(t, castAt, firstPushAt, "the thunder, then the slide: the record is the replay")
	assert.Less(t, lastPushAt, formedAt, "every shove lands before the fight forms (#785 R3)")
	assert.Less(t, lastPushAt, turnsEndedAt[0], "every shove lands before any turn of the fight passes")

	// A PUSHED GUARD'S OWN FIRST STEP LEAVES FROM THE CELL THE BLAST THREW IT
	// TO, never from where it stood before the thunder (rpg-project#831).
	require.NotEmpty(t, firstStep, "a pushed guard walked on its first turn")
	for member, step := range firstStep {
		from := pushedTo[member]
		assert.LessOrEqual(t, math.Max(math.Abs(step.X-from.X), math.Abs(step.Y-from.Y)), 1.0,
			"%s's first step %v leaves from its pushed cell %v", member, step, from)
	}
}
