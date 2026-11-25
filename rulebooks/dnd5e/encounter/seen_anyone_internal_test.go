// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// seen_anyone_internal_test.go pins SEEN FIRES FOR ANYONE WHO COMES INTO SIGHT
// AND A STEP READS ITS STANCE TO THEM (rpg-project#1123, slice
// rpg-project#1125): a creature whose state listens for `seen` is woken by
// each member who comes into its sight, an enemy or not, and the `stance`
// word asks how it stands to the one the beat names. Internal, because the
// landed beats and the intel store are the composition's own.
//
// HOW THE TESTS MAKE SOMEONE COME INTO SIGHT: [saSight] answers the guard a
// range the test sets (0 at first: he sees nobody), and a sight refresh
// ([Encounter.refreshSight]) after the range is raised is what lands `seen`.
// The refresh lands the beats and does not match them: the world's own pass
// (spendWorldAction, or a Speak/Step, which runs it) steps the machines.

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	saRaiders = FactionID("raiders")
	saStance  = machine.FactName("stance")
)

// saSight is everyone seeing the whole map but the guard, whose range the
// test sets.
type saSight struct{ guard int }

// Sight answers the guard's range and the whole map for the rest.
func (s *saSight) Sight(members []MemberID) (map[MemberID]int, error) {
	out, _ := everyoneSeesTheWholeMap{}.Sight(members)
	if _, ok := out[soGuard]; ok {
		out[soGuard] = s.guard
	}

	return out, nil
}

func saFact(name machine.FactName, value any) *machine.Guard {
	return &machine.Guard{Fact: &machine.FactLeaf{Name: name, Op: machine.OpEq, Value: value}}
}

// saHall is the posted hall with a guard running states from `calm`, blind at
// first, beside alice, the members given (neutral to the guard unless they
// are in raiders, which is hostile to him) and a raiders faction.
func saHall(t *testing.T, states map[string]machine.State, members ...MemberInput) (*Encounter, *SetupInput, *saSight) {
	t.Helper()
	g := soGuardIn(t, nil)
	g.Behaviors.Instances[0].Instance.Recipe = *recipe(t, "calm", states)
	sight := &saSight{}
	in := postedSetup(sight, append([]MemberInput{g}, members...)...)
	in.Field.Factions = append(in.Field.Factions, FactionInput{ID: saRaiders})
	in.Field.Dispositions = append(in.Field.Dispositions,
		DispositionInput{Between: [2]FactionID{postedGarrison, saRaiders}, Stance: StanceHostile})
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	return enc, in, sight
}

// saSees raises the guard's sight and refreshes it, so whoever is in range
// comes into view, and returns the `seen` beats landed on the guard, as the
// ones they name. The world has not moved on.
func saSees(t *testing.T, enc *Encounter, sight *saSight) []MemberID {
	t.Helper()
	beats, _ := saSeesGained(t, enc, sight)

	return beats
}

// saSeesGained is [saSees] and also the members the guard gained in the
// refresh's own answer, in id order, whether or not a beat was landed for them.
func saSeesGained(t *testing.T, enc *Encounter, sight *saSight) (beats, gained []MemberID) {
	t.Helper()
	before := len(enc.landed)
	sight.guard = 1_000_000
	deltas, _, err := enc.refreshSight(enc.rosterIDs())
	require.NoError(t, err)
	for _, b := range enc.landed[before:] {
		if b.Beat == beatSeen && b.Member == soGuard {
			beats = append(beats, b.Subject)
		}
	}
	if d := deltas[soGuard]; d != nil {
		for _, p := range d.FirstContact {
			gained = append(gained, MemberID(p.ID))
		}
		for _, id := range d.Reacquired {
			gained = append(gained, MemberID(id))
		}
	}
	sort.Slice(gained, func(i, j int) bool { return gained[i] < gained[j] })

	return beats, gained
}

// saWake lets the world move on: its own pass when no fight has formed, and
// the landed beats matched when one has (a hostile in sight forms a fight, and
// a fight is a clock of its own that the world's pass does not run).
func saWake(t *testing.T, enc *Encounter) {
	t.Helper()
	bubble, err := enc.bubbleFor("alice")
	if err != nil && !errors.Is(err, errNotInBubble) {
		require.NoError(t, err)
	}
	if bubble != nil {
		require.NoError(t, enc.matchLanded())

		return
	}
	require.NoError(t, enc.spendWorldAction("alice"))
}

func saPerson(id MemberID, x float64) MemberInput {
	return MemberInput{ID: id, Kind: KindPlayer, Position: spatial.Position{X: x, Y: 2}}
}

func saRaider(id MemberID, x float64) MemberInput {
	return MemberInput{ID: id, Kind: KindMonster, Faction: saRaiders, Position: spatial.Position{X: x, Y: 3}}
}

// saCallsOut is a guard who calls out on seeing the one he barred.
func saCallsOut() map[string]machine.State {
	return map[string]machine.State{
		"calm":      {Transitions: []machine.Transition{{On: beatSeen, When: saFact(MachineNotedAs(string(nvTopic)), "barred"), To: "calls-out", Reason: "known on sight"}}},
		"calls-out": {},
	}
}

func TestSeenAnyoneKnownOnSight(t *testing.T) {
	enc, _, sight := saHall(t, saCallsOut(), saPerson("zed", 4))
	tbHold(t, enc, "barred", "zed")
	require.Equal(t, "calm", stateOf(enc, soGuard), "precondition: he noted zed barred and has not seen him")

	seen := saSees(t, enc, sight)
	require.Contains(t, seen, MemberID("zed"), "zed is not his enemy and came into his sight: a seen beat names him")
	require.NoError(t, enc.spendWorldAction("alice"))

	require.Equal(t, "calls-out", stateOf(enc, soGuard), "barred, and known on sight")
}

func TestSeenAnyoneAStrangerDoesNotMoveHim(t *testing.T) {
	enc, _, sight := saHall(t, saCallsOut(), saPerson("bob", 4))
	tbHold(t, enc, "barred", "zed")

	beats, gained := saSeesGained(t, enc, sight)
	require.Contains(t, gained, MemberID("bob"), "bob came into sight: he is among those the guard gained")
	require.NotContains(t, beats, MemberID("bob"), "he noted nothing about bob, so no beat was landed for him (rpg-project#1126)")
	require.NoError(t, enc.spendWorldAction("alice"))

	require.Equal(t, "calm", stateOf(enc, soGuard), "he noted nothing about bob: he has not stepped")
}

func TestSeenAnyoneNotHiddenBehindAFriend(t *testing.T) {
	enc, _, sight := saHall(t, saCallsOut(), saPerson("bob", 4), saPerson("zed", 5))
	tbHold(t, enc, "barred", "zed")

	beats, gained := saSeesGained(t, enc, sight)
	require.Equal(t, []MemberID{"alice", "bob", "zed"}, gained, "all three came into sight, in id order")
	require.Equal(t, []MemberID{"zed"}, beats,
		"his step requires barred (rpg-project#1126): the one beat names zed, last in id order, behind alice and bob")
	require.NoError(t, enc.spendWorldAction("alice"))

	require.Equal(t, "calls-out", stateOf(enc, soGuard), "the one he barred is not hidden behind the others")
}

func TestSeenAnyoneAnOpenStepHearsOfEachOneBehindAFriend(t *testing.T) {
	states := map[string]machine.State{
		"calm":    {Transitions: []machine.Transition{{On: beatSeen, To: "curious", Reason: "anyone"}}},
		"curious": {},
	}
	enc, _, sight := saHall(t, states, saPerson("bob", 4), saPerson("zed", 5))

	require.Equal(t, []MemberID{"alice", "bob", "zed"}, saSees(t, enc, sight),
		"a step that requires nothing: one seen beat for each who came into sight, in id order")
}

func TestSeenAnyoneANonEnemyIsSeen(t *testing.T) {
	open := map[string]machine.State{
		"calm":    {Transitions: []machine.Transition{{On: beatSeen, To: "curious", Reason: "anyone"}}},
		"curious": {},
	}
	enc, _, sight := saHall(t, open, saPerson("bob", 4))
	require.Equal(t, StanceNeutral, enc.stanceOf(postedGarrison, FactionParty), "precondition: the party is neutral to him")

	require.Equal(t, []MemberID{"alice", "bob"}, saSees(t, enc, sight), "a neutral who comes into sight lands a seen beat naming them")
}

func TestSeenAnyoneEnemiesOnlyByTheWord(t *testing.T) {
	states := map[string]machine.State{
		"calm":  {Transitions: []machine.Transition{{On: beatSeen, When: saFact(saStance, string(StanceHostile)), To: "alarm", Reason: "an enemy"}}},
		"alarm": {},
	}
	t.Run("a neutral is no alarm", func(t *testing.T) {
		enc, _, sight := saHall(t, states, saPerson("bob", 4))
		beats, gained := saSeesGained(t, enc, sight)
		require.Contains(t, gained, MemberID("bob"), "the neutral came into his sight")
		require.Empty(t, beats, "his step requires an enemy (rpg-project#1126): no beat was landed for a neutral")
		require.NoError(t, enc.spendWorldAction("alice"))
		require.Equal(t, "calm", stateOf(enc, soGuard))
	})
	t.Run("a hostile is", func(t *testing.T) {
		enc, _, sight := saHall(t, states, saRaider("rex", 4))
		require.Contains(t, saSees(t, enc, sight), MemberID("rex"))
		saWake(t, enc)
		require.Equal(t, "alarm", stateOf(enc, soGuard))
	})
}

func TestSeenAnyoneTheWordsOtherAnswers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		member MemberInput
		stance Stance
	}{
		{"neutral", saPerson("bob", 4), StanceNeutral},
		{"none for a member in no faction", MemberInput{ID: "drifter", Kind: KindWorld, Position: spatial.Position{X: 4, Y: 3}}, StanceNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			states := map[string]machine.State{
				"calm": {Transitions: []machine.Transition{
					{On: beatSeen, When: &machine.Guard{All: []machine.Guard{
						*saFact(saStance, string(tc.stance)), *saFact(MachineNotedAs(string(nvTopic)), "marked")}}, To: "answered", Reason: "asked"},
				}},
				"answered": {},
			}
			enc, _, sight := saHall(t, states, tc.member)
			tbHold(t, enc, "marked", tc.member.ID)
			require.Contains(t, saSees(t, enc, sight), tc.member.ID)
			require.NoError(t, enc.spendWorldAction("alice"))
			require.Equal(t, "answered", stateOf(enc, soGuard), "stance read %s about the one named", tc.stance)
		})
	}
}

func TestSeenAnyoneNotOfStanceAppliesWhenTheBeatNamesNobody(t *testing.T) {
	states := map[string]machine.State{
		"calm":   {Transitions: []machine.Transition{{On: beatHeard, When: &machine.Guard{Not: saFact(saStance, string(StanceHostile))}, To: "uneasy", Reason: "a voice"}}},
		"uneasy": {},
	}
	enc, _, _ := saHall(t, states)
	require.NoError(t, enc.makeSound(noiseAttack, "alice", uint64(enc.clock.ToData().HighWater)))
	require.NoError(t, enc.spendWorldAction("alice"))

	require.Equal(t, "uneasy", stateOf(enc, soGuard),
		"a voice heard and not seen names nobody: stance answers known, so the not of it fires")
}

func TestSeenAnyoneTheRefusals(t *testing.T) {
	on := GuardWordsRefusal(beatTime, saFact(saStance, string(StanceHostile)))
	require.Equal(t, "`stance` asks about the one this beat is about, and `time` is about nobody: "+
		"it is read on heard, seen, topic, barred and approached", on)

	angry := GuardWordsRefusal(beatSeen, saFact(saStance, "angry"))
	require.Equal(t, "`stance` is hostile, neutral, allied or none, and this names \"angry\"", angry)

	require.Empty(t, GuardWordsRefusal(beatSeen, saFact(saStance, string(StanceAllied))))
	require.Empty(t, GuardWordsRefusal(beatHeard, saFact(saStance, string(StanceNone))))
}

func TestSeenAnyoneTheWordsTablePublishesIt(t *testing.T) {
	seen, ok := BeatWordOf("seen")
	require.True(t, ok)
	require.Equal(t, "sees someone", seen.Label)
	require.Equal(t, "when someone comes into its sight", seen.Short)
	require.Equal(t, "when someone comes into {whose} sight", seen.Long)
	require.Equal(t, "the one {who} saw", seen.About)

	stance := publishedCondition(t, "stance")
	require.True(t, stance.Fact)
	require.Equal(t, ArgKindString, stance.Kind)
	require.Equal(t, []string{"hostile", "neutral", "allied", "none"}, stance.Values)
	require.Equal(t, "is {value} to it", stance.Label)
	require.Equal(t, "is not {value} to it", stance.LabelNot)
	require.Equal(t, "is its enemy", stance.ValueLabels["hostile"])
	require.Equal(t, "is neutral to it", stance.ValueLabels["neutral"])
	require.Equal(t, "is its ally", stance.ValueLabels["allied"])
	require.Equal(t, "is on no side", stance.ValueLabels["none"])
	require.Equal(t, "{about} {value}", stance.Long)
	require.Equal(t, []string{"heard", "seen", "topic", "barred", "approached"}, stance.allowedOn())
	require.False(t, strings.Contains(strings.Join(stance.On, ","), "time"))
}

func TestSeenAnyoneSavedBetweenTheSightingAndTheWake(t *testing.T) {
	states := map[string]machine.State{
		"calm":    {Transitions: []machine.Transition{{On: beatSeen, When: saFact(saStance, string(StanceNeutral)), To: "greeted", Reason: "a neutral"}}},
		"greeted": {},
	}
	enc, in, sight := saHall(t, states, saPerson("bob", 4))
	require.NotEmpty(t, saSees(t, enc, sight))
	require.NotEmpty(t, enc.landed, "precondition: the beat is landed and waits for the world")

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	var data EncounterData
	require.NoError(t, json.Unmarshal(raw, &data))
	loaded, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: in.Capabilities})
	require.NoError(t, err)
	require.Equal(t, "calm", stateOf(loaded, soGuard), "precondition: he has not stepped")

	require.NoError(t, loaded.spendWorldAction("alice"))
	require.Equal(t, "greeted", stateOf(loaded, soGuard), "the saved seen beat still names a neutral, and woke him")
}
