// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// facts_readings_internal_test.go is THE TWO READINGS BESIDE `Deeds`
// (rpg-toolkit#1883, rpg-project#498).
//
// A `when` on a deed can now say WHOSE deed it is about, and the whole claim of
// that slice is that the facts were never missing — they were discarded. These
// tests hold the projection to it:
//
//   - `AllyDeeds` is a blow to somebody on this creature's side, read through
//     the STANCE GRAPH, so a disposition that changes changes the reading;
//   - `OwnDeeds` is what the creature did, which is the fact a pause is made of;
//   - `Deeds` is untouched, which is what keeps every document written before
//     scopes existed meaning exactly what it meant.
//
// Reading factsFor directly is the same choice bothways_internal_test.go makes
// and for the same reason: a Driver is handed a view and nothing else, so the
// projection is not otherwise reachable from a test — and reading it here is
// reading what the creature's own `time` table will read a moment later.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// readingsEnc builds a room with a WATCHER in `camp`, an ALLY beside it, and
// alice — whose faction the disposition decides. `allyStance` is what the
// disposition declares between the two camps, so a test can change it and ask
// the projection again.
func readingsEnc(t *testing.T, watcherAlliedToParty bool) *Encounter {
	t.Helper()

	const watcherCamp, otherCamp = "watch", "other"
	stance := StanceNeutral
	if watcherAlliedToParty {
		stance = StanceAllied
	}

	enc, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Retention: RetentionUnbounded,
		Field: FieldInput{
			Canvas:  CanvasInput{Void: VoidIsTransparent(), Orientation: HexesArePointyTop()},
			Regions: []RegionInput{rectRegion("yard", 0, 0, 8, 8)},
			Factions: []FactionInput{
				{ID: watcherCamp}, {ID: otherCamp},
			},
			Dispositions: []DispositionInput{
				// A FACTION IS ALWAYS ALLIED WITH ITSELF, so `friend` — the
				// watcher's own camp — needs no declaration (and declaring one
				// is refused by name).
				//
				// The OTHER camp is what the test moves between allied and
				// neutral, which is what proves the reading follows the graph
				// rather than a list frozen when the room was built.
				{Between: [2]FactionID{watcherCamp, otherCamp}, Stance: stance},
			},
		},
		Members: []MemberInput{
			{ID: "watcher", Kind: KindMonster, Faction: watcherCamp, Position: spatial.Position{X: 3, Y: 1}, SpeedFeet: 30, SightFeet: 60},
			{ID: "friend", Kind: KindMonster, Faction: watcherCamp, Position: spatial.Position{X: 4, Y: 1}, SpeedFeet: 30, SightFeet: 60},
			{ID: "stranger", Kind: KindMonster, Faction: otherCamp, Position: spatial.Position{X: 5, Y: 1}, SpeedFeet: 30, SightFeet: 60},
			// NO FACTION: a bystander is neither allied nor hostile to
			// anyone, which is the neutral goblin's own answer and the one
			// reading nothing here should catch.
			{ID: "bystander", Kind: KindMonster, Position: spatial.Position{X: 6, Y: 1}, SpeedFeet: 30, SightFeet: 60},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	return enc
}

// strike lands one hit from `actor` on `target`.
func strike(t *testing.T, enc *Encounter, actor, target string) {
	t.Helper()
	_, err := enc.Record(&RecordInput{
		Kind: OutcomeStruck, Actor: MemberID(actor), Targets: []MemberID{MemberID(target)},
		Values: map[OutcomeValue]int{ValueAmount: 3},
	})
	require.NoError(t, err)
}

func TestABlowToMySideIsNotABlowToMe(t *testing.T) {
	enc := readingsEnc(t, true)
	strike(t, enc, "stranger", "friend")

	facts, err := enc.factsFor("watcher")
	require.NoError(t, err)

	require.Empty(t, facts.Deeds,
		"nothing was done to the watcher: the old reading is untouched")
	require.Len(t, facts.AllyDeeds, 1,
		"the deed was already held — deeds land on witnesses — and this is the reading that stops discarding it")
	require.Equal(t, MemberID("stranger"), facts.AllyDeeds[0].Actor)
}

func TestTheAllyReadingFollowsADispositionThatChanges(t *testing.T) {
	// THE POINT OF ASKING THE GRAPH. The same deed, the same holdings; only
	// the disposition differs. A table that hard-coded an ally list would
	// answer the same both times, and this is the test that fails if the
	// reading is ever frozen instead of asked.
	// THE VICTIM IS IN THE FACTION WHOSE STANCE VARIES — not a campmate, who
	// is allied by definition and would prove nothing about the graph.
	allied := readingsEnc(t, true)
	strike(t, allied, "friend", "stranger")
	alliedVictim, err := allied.factsFor("watcher")
	require.NoError(t, err)
	require.Len(t, alliedVictim.AllyDeeds, 1,
		"with the camps ALLIED, a blow to the stranger is a blow to my side")

	neutralVictim := readingsEnc(t, false)
	strike(t, neutralVictim, "friend", "stranger")
	neutralFacts, err := neutralVictim.factsFor("watcher")
	require.NoError(t, err)
	require.Empty(t, neutralFacts.AllyDeeds,
		"the SAME deed with the camps NEUTRAL: a neutral camp is neither allied nor hostile, so it stops counting")
}

func TestABlowToMeIsNotAnAllyReading(t *testing.T) {
	// The two readings must stay different questions, or `on: ally` would fire
	// for a wound to the creature itself and an author could not tell them
	// apart.
	enc := readingsEnc(t, true)
	strike(t, enc, "stranger", "watcher")

	facts, err := enc.factsFor("watcher")
	require.NoError(t, err)

	require.Len(t, facts.Deeds, 1, "a blow to me is the self reading")
	require.Empty(t, facts.AllyDeeds, "and it is not the ally reading")
}

func TestWhatIDidIsAFactIHold(t *testing.T) {
	// THE PAUSE'S FACT. `{ attacked: { within: 2, as: actor } }` reads this,
	// so "after I strike, stand still" needs no new concept in the grammar.
	enc := readingsEnc(t, true)
	strike(t, enc, "watcher", "friend")

	facts, err := enc.factsFor("watcher")
	require.NoError(t, err)

	require.Len(t, facts.OwnDeeds, 1, "the creature knows what it did")
	require.Equal(t, MemberID("watcher"), facts.OwnDeeds[0].Actor)
	require.Empty(t, facts.Deeds, "doing a thing is not having it done to you")
}

func TestAnUnrelatedBlowAuthorsNothing(t *testing.T) {
	// THE ZERO VALUE TELLS THE TRUTH: two strangers fighting beside the
	// watcher leaves every reading of its own empty, so a table that authors
	// no `on: ally` condition behaves exactly as it did before scopes existed.
	enc := readingsEnc(t, true)
	// A BYSTANDER WITH NO FACTION, struck by somebody else: nobody on the
	// watcher's side was touched and the watcher did nothing.
	strike(t, enc, "stranger", "bystander")

	facts, err := enc.factsFor("watcher")
	require.NoError(t, err)

	require.Empty(t, facts.Deeds)
	require.Empty(t, facts.AllyDeeds)
	require.Empty(t, facts.OwnDeeds)
}

// TestTheScopeSurvivesPersistence is the gap this file found the hard way: the
// compiled condition carries the scope, and the PERSISTED one did not.
//
// A table saved with `on: ally` or `as: actor` would come back reading `Deeds`
// — the scope silently reverting to self — which is the quiet degrade this
// module refuses everywhere else. Nothing in the tests above would have caught
// it, because none of them round-trips through the data shape.
func TestTheScopeSurvivesPersistence(t *testing.T) {
	for _, scope := range []DeedScope{ScopeSelf, ScopeAlly, ScopeActor} {
		entry := Answer{
			Weight: 1,
			When:   &When{Deed: "attacked", Within: 3, Scope: scope},
			Hold:   true,
		}

		// OUT: the entry becomes the persistent shape.
		data := answerDataFrom(entry)
		require.NotNil(t, data.When, "the condition was written")
		require.Equal(t, string(scope), data.When.Scope)

		// BACK: and the persistent shape becomes the entry again.
		back := answerFromData(data)
		require.NotNil(t, back.When, "the condition came back")
		require.Equal(t, scope, back.When.Scope,
			"a scope that did not survive the round trip would read the WRONG deeds")
		require.Equal(t, "attacked", back.When.Deed)
		require.Equal(t, 3, back.When.Within)
	}
}

// TestAnUnauthoredScopeWritesNothing pins the compatibility half: a condition
// with no scope holds the self reading, and writes no `scope` key — so a
// record persisted before scopes existed loads to exactly what it meant.
func TestAnUnauthoredScopeWritesNothing(t *testing.T) {
	data := answerDataFrom(Answer{
		Weight: 1, When: &When{Enemy: EnemySeen}, Hold: true,
	})
	require.NotNil(t, data.When)
	require.Empty(t, data.When.Scope, "the self reading writes no key")

	back := answerFromData(data)
	require.Equal(t, ScopeSelf, back.When.Scope, "and loads back as the self reading")
}

// ---------------------------------------------------------------------------
// The selectors follow the scope (rpg-toolkit#1890 thread 2)
// ---------------------------------------------------------------------------
//
// THE ROW THIS WHOLE SLICE EXISTS FOR. `selectMember` read view.Deeds
// unconditionally, so an ally-scoped vengeance row fired on the ALLY reading
// and then resolved `attacker` off the SELF reading: with nobody having struck
// the creature itself that was a Pass (the row won the roll and nothing
// happened), and with somebody having struck it, it struck the WRONG member.
//
// These go through the real driver, because the defect lived in the pairing of
// two functions and a unit test of either one alone is blind to it — which is
// exactly how it survived the first round.

// vengeanceTable is the natural row for the ally reading: avenge my side.
func vengeanceTable() Table {
	return Table{AnswerTime: {
		{Weight: 100, Attack: &Selector{Word: SelectorAttacker},
			When: &When{Deed: "attacked", Within: 3, Scope: ScopeAlly}},
		{Weight: 1, Hold: true},
	}}
}

// vengeanceView is a creature with a weapon in hand and an enemy in reach.
func vengeanceView(now uint64) MonsterView {
	return MonsterView{
		Self:    "goblin",
		Actions: []ActionView{{Ref: core.Ref{Type: "weapons", ID: "scimitar"}, RangeFeet: 5}},
		// THE VENGEANCE TARGET IS THE ONE IN SIGHT AND IN REACH: `attackIntent`
		// requires the member it swings at to be seen and reachable, so a test
		// that names a deed actor nobody can see would pass for the wrong
		// reason (found while writing these — the first cut did exactly that).
		Seen: []SeenMember{{
			ID: "goblin-slayer", Opposed: true, Standing: true,
			InReach: map[core.Ref]bool{{Type: "weapons", ID: "scimitar"}: true},
		}},
		Budget: TurnBudget{AttacksLeft: 1, MovementFeet: 30},
		Table:  vengeanceTable(),
		At:     now,
		Round:  1,
	}
}

func (s *TableSuite) TestAnAllyVengeanceRowStrikesMyAlliesAttacker() {
	// ONLY the ally reading is filled: nobody has struck the goblin itself.
	view := vengeanceView(10)
	view.AllyDeeds = []HeldDeed{{Kind: DeedAttack, Actor: "goblin-slayer", At: 9}}

	decision, err := TableDriver{Roller: lowestDie{}}.Act(view)
	s.Require().NoError(err)

	attack, ok := decision.Intent.(Attack)
	s.Require().True(ok, "the row fired on the ally reading, so the swing must land — not a silent Pass")
	s.Equal(MemberID("goblin-slayer"), attack.Target,
		"`attacker` must name whoever struck my ALLY, not whoever struck me (nobody did)")
}

func (s *TableSuite) TestAnAllyVengeanceRowDoesNotStrikeItsOwnAttacker() {
	// BOTH readings are filled and they name different members. The scope says
	// which one the selector reads, so the ally row must go for the ally's
	// attacker and never for the goblin's own.
	view := vengeanceView(10)
	view.Deeds = []HeldDeed{{Kind: DeedAttack, Actor: "someone-else", At: 10}}
	view.AllyDeeds = []HeldDeed{{Kind: DeedAttack, Actor: "goblin-slayer", At: 9}}

	decision, err := TableDriver{Roller: lowestDie{}}.Act(view)
	s.Require().NoError(err)

	attack, ok := decision.Intent.(Attack)
	s.Require().True(ok)
	s.Equal(MemberID("goblin-slayer"), attack.Target,
		"the condition and its selector must read the SAME deeds")
	s.NotEqual(MemberID("someone-else"), attack.Target, "which is not the goblin's own attacker")
}

func (s *TableSuite) TestASelfScopedRowStillReadsItsOwnDeeds() {
	// THE COMPATIBILITY HALF: the pre-scope pairing must not move. An unscoped
	// `attacked` row reads the self deeds and strikes the self attacker.
	table := Table{AnswerTime: {
		{Weight: 100, Attack: &Selector{Word: SelectorAttacker},
			When: &When{Deed: "attacked", Within: 3}},
		{Weight: 1, Hold: true},
	}}
	view := vengeanceView(10)
	view.Table = table
	// THE SELF ATTACKER IS THE ONE IN SIGHT AND REACH here, since this row
	// reads the self deeds and swings at whoever is named there.
	view.Seen = []SeenMember{{
		ID: "someone-else", Opposed: true, Standing: true,
		InReach: map[core.Ref]bool{{Type: "weapons", ID: "scimitar"}: true},
	}}
	view.Deeds = []HeldDeed{{Kind: DeedAttack, Actor: "someone-else", At: 10}}
	view.AllyDeeds = []HeldDeed{{Kind: DeedAttack, Actor: "goblin-slayer", At: 9}}

	decision, err := TableDriver{Roller: lowestDie{}}.Act(view)
	s.Require().NoError(err)

	attack, ok := decision.Intent.(Attack)
	s.Require().True(ok)
	s.Equal(MemberID("someone-else"), attack.Target, "an unscoped row still means against me")
}
