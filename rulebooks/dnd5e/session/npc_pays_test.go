// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// This file is in the INTERNAL package: Tests 1 to 4 need openForWrite and the
// strike seam, which are unexported. Test 5 plays the manager's real verbs
// (Launch, Afford, Move, EndTurn) from the same package, on the same fixtures
// the other internal tests build (rpg-project#1198).

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// npcPaysFixture is one fighter beside one monster, with a write scope open.
type npcPaysFixture struct {
	mgr        *Manager
	scope      *writeScope
	roller     *scriptedDice
	characters *strikeCharacters
	sessions   *strikeSessions
}

func newNPCPaysFixture(t *testing.T, id, ref string) *npcPaysFixture {
	t.Helper()
	ctx := context.Background()
	f := &npcPaysFixture{
		sessions:   &strikeSessions{byID: map[string]*SessionData{}},
		characters: &strikeCharacters{byID: map[string]*character.Data{"fighter": strikeFixtureFighter("fighter")}},
		roller:     &scriptedDice{rolls: []int{17, 4, 17, 4, 17, 4, 17, 4}},
	}
	encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	mgr, err := NewManager(&Config{Record: DiscardRecord{}, Acts: DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: f.roller, TurnDriver: Pass{},
		Sessions: f.sessions, Encounters: encounters, Characters: f.characters,
	})
	require.NoError(t, err)
	f.mgr = mgr
	launchScene(t, mgr, scene{
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 12, 6)}},
		Party:    []sceneSeat{seatAt("fighter", 0, 0)},
		Monsters: []dungeonspec.MonsterPlacement{monsterAt(id, ref, 1, 0)},
		Endings:  []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
	})
	f.roller.next = 0 // isolate the verb: formation already spent dice.
	f.scope, err = mgr.openForWrite(ctx, "sess")
	require.NoError(t, err)
	return f
}

// sheet is the monster's stored sheet as the scope holds it now.
func (f *npcPaysFixture) sheet(t *testing.T, id string) *monster.Data {
	t.Helper()
	data, found := npcSheet(f.scope.data, id)
	require.True(t, found)
	return data
}

func (f *npcPaysFixture) sheetJSON(t *testing.T, id string) string {
	t.Helper()
	raw, err := json.Marshal(f.sheet(t, id))
	require.NoError(t, err)
	return string(raw)
}

func (f *npcPaysFixture) storyLen(t *testing.T) int {
	t.Helper()
	story, err := f.scope.enc.Story(&encounter.StoryInput{Audience: "fighter"})
	require.NoError(t, err)
	return len(story)
}

func (f *npcPaysFixture) strike(t *testing.T, id string) error {
	t.Helper()
	action := refs.Weapons.Shortsword()
	return (strikerSeam{m: f.mgr, scope: f.scope}).Strike(context.Background(), f.scope.enc, encounter.MemberID(id), "fighter", *action)
}

func requireSlots(t *testing.T, sheet *monster.Data, action, bonus, reaction int) {
	t.Helper()
	require.Equal(t, action, sheet.SlotsLeft(coreCombat.ActionStandard), "action left")
	require.Equal(t, bonus, sheet.SlotsLeft(coreCombat.ActionBonus), "bonus action left")
	require.Equal(t, reaction, sheet.SlotsLeft(coreCombat.ActionReaction), "reaction left")
}

// Test 1: a swing the stat block states no price for is priced by the rules at
// one action and paid from the stored sheet.
func TestASwingSpendsTheActionOnTheStoredSheet(t *testing.T) {
	f := newNPCPaysFixture(t, "skel-1", refs.Monsters.Skeleton().String())
	requireSlots(t, f.sheet(t, "skel-1"), 1, 1, 1)
	before := f.storyLen(t)

	require.NoError(t, f.strike(t, "skel-1"))

	requireSlots(t, f.sheet(t, "skel-1"), 0, 1, 1)
	require.Greater(t, f.storyLen(t), before, "the swing is in the story")
}

// Test 2: a second swing before its turn start is refused (ErrCannotAfford,
// after translation) and changes nothing.
func TestASecondSwingBeforeItsTurnStartIsRefusedAndChangesNothing(t *testing.T) {
	ctx := context.Background()
	f := newNPCPaysFixture(t, "skel-1", refs.Monsters.Skeleton().String())
	require.NoError(t, f.strike(t, "skel-1"))
	sheetAfterFirst := f.sheetJSON(t, "skel-1")
	storyAfterFirst := f.storyLen(t)
	rolled := f.roller.next
	fighter, err := f.mgr.sheetsFor(nil).load(ctx, "participant", "fighter")
	require.NoError(t, err)

	err = f.strike(t, "skel-1")

	require.ErrorIs(t, err, ErrCannotAfford)
	require.Equal(t, rolled, f.roller.next, "refused before any die rolls")
	require.Equal(t, storyAfterFirst, f.storyLen(t), "no new beat")
	require.JSONEq(t, sheetAfterFirst, f.sheetJSON(t, "skel-1"), "the sheet is as the first swing left it")
	after, err := f.mgr.sheetsFor(nil).load(ctx, "participant", "fighter")
	require.NoError(t, err)
	require.Equal(t, fighter.HitPoints, after.HitPoints)
}

func (f *npcPaysFixture) disengage(id string) error {
	return (strikerSeam{m: f.mgr, scope: f.scope}).Disengage(context.Background(), f.scope.enc, encounter.MemberID(id))
}

func (f *npcPaysFixture) disengaging(t *testing.T, id string) int {
	t.Helper()
	held, err := conditions.HeldAddresses(id, f.sheet(t, id).Conditions)
	require.NoError(t, err)
	count := 0
	for _, address := range held {
		if address.ConditionRef == refs.Conditions.Disengaging().String() {
			count++
		}
	}
	return count
}

// Test 3: a Disengage spends the bonus action on the stored sheet, seats the
// one Disengaging condition and tells nothing.
func TestADisengageSpendsTheBonusActionOnTheStoredSheet(t *testing.T) {
	f := newNPCPaysFixture(t, "goblin", refs.Monsters.Goblin().String())
	before := f.storyLen(t)

	require.NoError(t, f.disengage("goblin"))

	requireSlots(t, f.sheet(t, "goblin"), 1, 0, 1)
	require.Equal(t, 1, f.disengaging(t, "goblin"))
	require.Equal(t, before, f.storyLen(t), "the seam tells nothing")
}

// Test 4: a second Disengage in the same turn is refused; still one condition.
func TestASecondDisengageInOneTurnIsRefused(t *testing.T) {
	f := newNPCPaysFixture(t, "goblin", refs.Monsters.Goblin().String())
	require.NoError(t, f.disengage("goblin"))
	sheetAfterFirst := f.sheetJSON(t, "goblin")

	err := f.disengage("goblin")

	require.ErrorIs(t, err, ErrCannotAfford)
	require.Equal(t, 1, f.disengaging(t, "goblin"))
	requireSlots(t, f.sheet(t, "goblin"), 1, 0, 1)
	require.JSONEq(t, sheetAfterFirst, f.sheetJSON(t, "goblin"))
}

// walkerEachTurn walks the next path in its list on a turn's first ask, and
// passes on every ask after a walk within that turn. An ask that follows a
// Pass starts the next turn.
type walkerEachTurn struct {
	paths [][]spatial.Position
	step  int
}

func (w *walkerEachTurn) Act(MonsterView) (TurnIntent, error) {
	defer func() { w.step++ }()
	if w.step%2 == 0 && w.step/2 < len(w.paths) {
		return Move{Path: w.paths[w.step/2]}, nil
	}
	return Pass{}, nil
}

// goblinEconomy is what the goblin's stored sheet holds right now.
func goblinEconomy(t *testing.T, sessions *strikeSessions) *monster.Data {
	t.Helper()
	stored, err := sessions.GetSession(context.Background(), "sess")
	require.NoError(t, err)
	data, found := npcSheet(stored, "gob-1")
	require.True(t, found)
	return data
}

func declarationOf(t *testing.T, mgr *Manager, member string, verb Verb) Declaration {
	t.Helper()
	out, err := mgr.Afford(context.Background(), &AffordInput{Session: "sess", Member: member})
	require.NoError(t, err)
	for _, declaration := range out.Declarations {
		if declaration.Verb == verb {
			return declaration
		}
	}
	require.FailNowf(t, "no declaration", "afford returned no %s for %s", verb, member)
	return Declaration{}
}

// Test 5: played over two rounds through the manager's real verbs, on the
// scene mover_test.go's Nimble goblin tests play (a fighter beside a goblin
// with Nimble Escape; the goblin walks out of her reach), with a driver that
// walks every turn. Round one: the goblin disengages and walks, and its sheet
// shows the bonus action spent. Round two: the fighter closes in again, and at
// the goblin's turn it disengages and walks a SECOND time, which can only be so
// if its turn start gave the turn back.
func TestAnNPCsTurnComesBackAndTheFightPlaysAsItDid(t *testing.T) {
	ctx := context.Background()
	hex := func(col int) spatial.Position { return encounter.HexCellAt(encounter.HexesArePointyTop(), col, 0) }
	sessions := &strikeSessions{byID: map[string]*SessionData{}}
	characters := &strikeCharacters{byID: map[string]*character.Data{"fighter": strikeFixtureFighter("fighter")}}
	encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	driver := &walkerEachTurn{paths: [][]spatial.Position{
		{hex(3), hex(4)}, // round one: out of the fighter's reach
		{hex(5), hex(6)}, // round two: out of it again, once she has closed in
	}}
	mgr, err := NewManager(&Config{Record: DiscardRecord{}, Acts: DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: &scriptedDice{rolls: []int{20, 20, 20, 20, 20, 20, 20, 20, 20, 20, 20, 20, 1, 1, 1, 1}}, TurnDriver: driver,
		Sessions: sessions, Encounters: encounters, Characters: characters,
	})
	require.NoError(t, err)

	out := launchScene(t, mgr, scene{
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 12, 6)}},
		Party:    []sceneSeat{seatAt("fighter", 1, 0)},
		Monsters: []dungeonspec.MonsterPlacement{monsterAt("gob-1", refs.Monsters.Goblin().String(), 2, 0)},
		Endings:  []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
	})
	require.NotEmpty(t, out.Formed, "adjacent and in sight starts the fight")
	fighter := characters.byID["fighter"]
	fighter.ActionEconomy = &character.ActionEconomyData{
		TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1, ReactionsRemaining: 1, MovementRemaining: 30,
	}

	disengagements := func() int {
		enc, err := mgr.LoadWorldForTest(ctx, "sess")
		require.NoError(t, err)
		story, err := enc.Story(&encounter.StoryInput{Audience: "fighter"})
		require.NoError(t, err)
		n := 0
		for _, entry := range story {
			var fields map[string]any
			require.NoError(t, json.Unmarshal(entry.Payload, &fields))
			if fields["beat"] == encounter.BeatActivated {
				n++
			}
		}
		return n
	}

	_, err = mgr.EndTurn(ctx, &EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: declarationOf(t, mgr, "fighter", VerbEndTurn).ID})
	require.NoError(t, err)

	// Round one's goblin turn has run.
	require.Equal(t, 1, disengagements(), "round one: the goblin disengaged and walked")
	where, err := mgr.Where(ctx, &WhereInput{Session: "sess", Member: "gob-1"})
	require.NoError(t, err)
	require.Equal(t, hex(4), where.Position)
	spent := goblinEconomy(t, sessions)
	require.NotNil(t, spent.Economy, "the sheet shows what the goblin spent")
	requireSlots(t, spent, 1, 0, 1)

	// Round two: the fighter closes in and ends her turn.
	turn, err := mgr.Turn(ctx, &TurnInput{Session: "sess", Member: "fighter"})
	require.NoError(t, err)
	require.Equal(t, "fighter", turn.Active)
	_, err = mgr.Move(ctx, &MoveInput{Session: "sess", Member: "fighter", DeclarationID: declarationOf(t, mgr, "fighter", VerbMove).ID,
		Path: []spatial.Position{hex(2), hex(3)}})
	require.NoError(t, err)
	_, err = mgr.EndTurn(ctx, &EndTurnInput{Session: "sess", Member: "fighter", DeclarationID: declarationOf(t, mgr, "fighter", VerbEndTurn).ID})
	require.NoError(t, err)

	require.Equal(t, 2, disengagements(), "round two: the goblin disengaged again, so its turn start gave the turn back")
	where, err = mgr.Where(ctx, &WhereInput{Session: "sess", Member: "gob-1"})
	require.NoError(t, err)
	require.Equal(t, hex(6), where.Position)
	requireSlots(t, goblinEconomy(t, sessions), 1, 0, 1)
}
