// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type standingOnly struct{}

func (standingOnly) Standing([]encounter.MemberID) ([]encounter.MemberID, error) { return nil, nil }

type scriptedParticipation struct {
	members       map[encounter.MemberID]encounter.MemberParticipation
	partyDefeated bool
	assessment    *encounter.ParticipationAssessment
	returnNil     bool
	err           error
	questions     [][]encounter.MemberID
}

func (s *scriptedParticipation) Standing([]encounter.MemberID) ([]encounter.MemberID, error) {
	return nil, errors.New("legacy Standing must not be consulted")
}

func (s *scriptedParticipation) Assess(members []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	s.questions = append(s.questions, append([]encounter.MemberID(nil), members...))
	if s.err != nil {
		return nil, s.err
	}
	if s.returnNil {
		return nil, nil
	}
	if s.assessment != nil {
		return s.assessment, nil
	}

	out := &encounter.ParticipationAssessment{PartyDefeated: s.partyDefeated}
	for _, id := range members {
		member, ok := s.members[id]
		if !ok {
			member = encounter.MemberParticipation{
				Member: id, Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait,
			}
		}
		member.Member = id
		out.Members = append(out.Members, member)
	}
	return out, nil
}

func participationSetup(capability encounter.Standing, members ...encounter.MemberInput) *encounter.SetupInput {
	return &encounter.SetupInput{
		Initiative: orderAsGiven{},
		Standing:   capability,
		Sight:      everyoneSeesTheWholeMap{},
		TurnDriver: passDriver{},
		Striker:    passStriker{},
		Announcer:  quietAnnouncer{},
		Retention:  encounter.RetentionUnbounded,
		Field: encounter.FieldInput{
			Canvas: openAir(),
			Regions: []encounter.RegionInput{
				rectRegion("participation-yard", 0, 0, 12, 12),
			},
		},
		Members: members,
		Endings: []encounter.EndingInput{
			{Key: "withdrawn", Trigger: encounter.TriggerExternal{}},
		},
	}
}

func participationTrio(t *testing.T, capability encounter.Standing) *encounter.Encounter {
	t.Helper()
	enc, err := encounter.NewEncounter(participationSetup(capability,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 2}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
	))
	require.NoError(t, err)
	return enc
}

func clockState(t *testing.T, enc *encounter.Encounter, member encounter.MemberID) *encounter.ClockOfOutput {
	t.Helper()
	out, err := enc.ClockOf(&encounter.ClockOfInput{Member: member})
	require.NoError(t, err)
	return out
}

func storyBeats(t *testing.T, enc *encounter.Encounter, member encounter.MemberID) []map[string]any {
	t.Helper()
	entries, err := enc.Story(&encounter.StoryInput{Audience: member})
	require.NoError(t, err)
	beats := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		var beat map[string]any
		require.NoError(t, json.Unmarshal(entry.Payload, &beat))
		beats = append(beats, beat)
	}
	return beats
}

func TestParticipationIsRequiredWithoutChangingTheStandingFieldShape(t *testing.T) {
	input := participationSetup(standingOnly{},
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
	)
	_, err := encounter.NewEncounter(input)
	require.ErrorIs(t, err, encounter.ErrNoParticipation)

	capability := &scriptedParticipation{}
	built, err := encounter.NewEncounter(participationSetup(capability,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
	))
	require.NoError(t, err)

	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: built.ToData(), Initiative: orderAsGiven{}, Standing: standingOnly{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	require.ErrorIs(t, err, encounter.ErrNoParticipation)
}

func TestParticipationQuestionAndAnswerContract(t *testing.T) {
	t.Run("stable complete question exactly once per pass", func(t *testing.T) {
		capability := &scriptedParticipation{}
		_, err := encounter.NewEncounter(participationSetup(capability,
			encounter.MemberInput{ID: "zara", Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			encounter.MemberInput{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 2}},
		))
		require.NoError(t, err)
		require.Equal(t, [][]encounter.MemberID{{"alice", "zara"}}, capability.questions)
	})

	for _, tc := range []struct {
		name       string
		assessment *encounter.ParticipationAssessment
		sentinel   error
	}{
		{name: "nil assessment", assessment: nil, sentinel: encounter.ErrInvalidData},
		{name: "missing member", assessment: &encounter.ParticipationAssessment{}, sentinel: encounter.ErrInvalidData},
		{name: "duplicate member", assessment: &encounter.ParticipationAssessment{Members: []encounter.MemberParticipation{
			{Member: alice, Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait},
			{Member: alice, Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait},
		}}, sentinel: encounter.ErrInvalidData},
		{name: "foreign member", assessment: &encounter.ParticipationAssessment{Members: []encounter.MemberParticipation{
			{Member: "ghost", Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait},
		}}, sentinel: encounter.ErrNotMember},
		{name: "unknown turn", assessment: &encounter.ParticipationAssessment{Members: []encounter.MemberParticipation{
			{Member: alice, Contact: true, Conscious: true, Turn: encounter.TurnParticipation("delay")},
		}}, sentinel: encounter.ErrInvalidData},
	} {
		t.Run(tc.name, func(t *testing.T) {
			capability := &scriptedParticipation{assessment: tc.assessment, returnNil: tc.name == "nil assessment"}
			_, err := encounter.NewEncounter(participationSetup(capability,
				encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			))
			require.ErrorIs(t, err, tc.sentinel)
		})
	}
}

func TestContactRatherThanDownDecidesEncounterSides(t *testing.T) {
	t.Run("down member can still be supplied on a contact side", func(t *testing.T) {
		capability := &scriptedParticipation{members: map[encounter.MemberID]encounter.MemberParticipation{
			goblin: {Down: true, Contact: true, Turn: encounter.TurnParticipationWait},
		}}
		enc := participationTrio(t, capability)
		require.Equal(t, encounter.ClockTurn, clockState(t, enc, goblin).Kind)
	})

	t.Run("not-down member can be absent from contact", func(t *testing.T) {
		capability := &scriptedParticipation{members: map[encounter.MemberID]encounter.MemberParticipation{
			goblin: {Turn: encounter.TurnParticipationWait},
		}}
		enc := participationTrio(t, capability)
		require.Equal(t, encounter.ClockWorld, clockState(t, enc, goblin).Kind)
		require.Equal(t, encounter.ClockWorld, clockState(t, enc, alice).Kind)
	})
}

func TestDyingRetainsItsExactInitiativeSlotAndCanBecomeActive(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := participationTrio(t, capability)

	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		bob: {Down: true, Turn: encounter.TurnParticipationWait},
	}
	_, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err)
	require.Equal(t, []encounter.MemberID{alice, bob, goblin}, clockState(t, enc, alice).Order)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	state := clockState(t, enc, bob)
	require.Equal(t, bob, state.Active)
	require.Equal(t, []encounter.MemberID{alice, bob, goblin}, state.Order)
}

func TestStabilizedAutoPassesInPlaceAndOrdinaryWaitReturnsWithoutReinsertion(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := participationTrio(t, capability)

	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		bob: {Down: true, Turn: encounter.TurnParticipationAutoPass},
	}
	_, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err)
	require.Equal(t, []encounter.MemberID{alice, bob, goblin}, clockState(t, enc, alice).Order)

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	state := clockState(t, enc, alice)
	require.Equal(t, alice, state.Active, "bob auto-passes and the unplayed goblin is driven normally")
	require.Equal(t, []encounter.MemberID{alice, bob, goblin}, state.Order, "auto-pass retains the slot")

	capability.members[bob] = encounter.MemberParticipation{Turn: encounter.TurnParticipationWait}
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	state = clockState(t, enc, bob)
	require.Equal(t, bob, state.Active)
	require.Equal(t, []encounter.MemberID{alice, bob, goblin}, state.Order, "the same slot becomes controlled")
}

func TestConsecutiveStabilizedMembersAdvanceIteratively(t *testing.T) {
	capability := &scriptedParticipation{}
	enc, err := encounter.NewEncounter(participationSetup(capability,
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
		encounter.MemberInput{ID: bob, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 2}},
		encounter.MemberInput{ID: "carl", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
	))
	require.NoError(t, err)
	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		bob:    {Down: true, Turn: encounter.TurnParticipationAutoPass},
		"carl": {Down: true, Turn: encounter.TurnParticipationAutoPass},
	}

	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.Equal(t, alice, clockState(t, enc, alice).Active)
	require.Equal(t, []encounter.MemberID{alice, bob, "carl", goblin}, clockState(t, enc, alice).Order)
}

func TestRemoveLeavesInitiativeButKeepsMapAndRoster(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := participationTrio(t, capability)
	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		bob: {Down: true, Turn: encounter.TurnParticipationRemove},
	}

	_, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err)
	require.Equal(t, []encounter.MemberID{alice, goblin}, clockState(t, enc, alice).Order)
	require.Equal(t, encounter.ClockWorld, clockState(t, enc, bob).Kind)
	members, err := enc.Members()
	require.NoError(t, err)
	require.Len(t, members, 3)
	require.Equal(t, cellAt(1, 2), members[1].Position)
}

func TestRemovingTheActiveMemberAdvancesExactlyOnce(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := participationTrio(t, capability)
	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.Equal(t, bob, clockState(t, enc, bob).Active)

	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		bob: {Down: true, Turn: encounter.TurnParticipationRemove},
	}
	_, err = enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err)
	require.Equal(t, alice, clockState(t, enc, alice).Active)

	endedByGoblin := 0
	for _, beat := range storyBeats(t, enc, alice) {
		if beat["beat"] == "turn-ended" && beat["member"] == "goblin" {
			endedByGoblin++
		}
	}
	require.Equal(t, 1, endedByGoblin)
}

func TestSuppliedPartyDefeatClosesAfterItsCausalBeats(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := participationTrio(t, capability)
	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		alice: {Down: true, Turn: encounter.TurnParticipationWait},
		bob:   {Down: true, Turn: encounter.TurnParticipationWait},
	}
	capability.partyDefeated = true

	_, err := enc.Record(&encounter.RecordInput{
		Kind:  encounter.OutcomeDeathSave,
		Actor: alice,
		DeathSave: &encounter.DeathSaveDetail{
			Roll: 9, Outcome: "dead", FailuresAdded: 1, Failures: 3,
			FailuresRemaining: 0, Dead: true, Continuation: "already_advanced", PresentationID: "death-save",
		},
	})
	require.NoError(t, err)

	status, err := enc.Status()
	require.NoError(t, err)
	require.False(t, status.Open)
	require.Equal(t, "party_defeated", status.Outcome.Ending)

	beats := storyBeats(t, enc, alice)
	require.GreaterOrEqual(t, len(beats), 4)
	require.Equal(t, "death_save", beats[len(beats)-4]["beat"])
	require.Equal(t, "down", beats[len(beats)-3]["beat"])
	require.Equal(t, "down", beats[len(beats)-2]["beat"])
	require.Equal(t, "ended", beats[len(beats)-1]["beat"])

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Initiative: orderAsGiven{}, Standing: capability,
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	require.NoError(t, err)
	reloadedStatus, err := reloaded.Status()
	require.NoError(t, err)
	require.Equal(t, "party_defeated", reloadedStatus.Outcome.Ending)
}

func TestPartyDefeatedFalseWithDyingAndConsciousAlliesKeepsFightOpen(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := participationTrio(t, capability)
	capability.members = map[encounter.MemberID]encounter.MemberParticipation{
		alice: {Down: true, Turn: encounter.TurnParticipationWait},
		bob:   {Contact: true, Conscious: true, Turn: encounter.TurnParticipationWait},
	}

	_, err := enc.Pump(&encounter.PumpInput{})
	require.NoError(t, err)
	status, err := enc.Status()
	require.NoError(t, err)
	require.True(t, status.Open)
	require.Equal(t, encounter.ClockTurn, clockState(t, enc, bob).Kind)
	require.Equal(t, []encounter.MemberID{alice, bob, goblin}, clockState(t, enc, bob).Order)
}

func TestDeathSaveDetailRoundTripsEveryPrimitiveAndRejectsMismatches(t *testing.T) {
	capability := &scriptedParticipation{}
	enc := participationTrio(t, capability)
	detail := &encounter.DeathSaveDetail{
		Roll: 20, Outcome: "recovered", SuccessesAdded: 1, FailuresAdded: 2,
		Successes: 3, Failures: 2, SuccessesNeeded: 0, FailuresRemaining: 1,
		Stabilized: true, Dead: true, Recovered: true, HPRestored: 1,
		Continuation: "keep_turn", PresentationID: "natural-20",
	}
	_, err := enc.Record(&encounter.RecordInput{Kind: encounter.OutcomeDeathSave, Actor: alice, DeathSave: detail})
	require.NoError(t, err)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Initiative: orderAsGiven{}, Standing: capability,
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	require.NoError(t, err)
	beats := storyBeats(t, reloaded, alice)
	raw, err := json.Marshal(beats[len(beats)-1]["death_save"])
	require.NoError(t, err)
	var got encounter.DeathSaveDetail
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, *detail, got)
	require.Equal(t, []string{
		"Roll", "Outcome", "SuccessesAdded", "FailuresAdded", "Successes", "Failures",
		"SuccessesNeeded", "FailuresRemaining", "Stabilized", "Dead", "Recovered",
		"HPRestored", "Continuation", "PresentationID",
	}, structFieldNames(encounter.DeathSaveDetail{}), "closed detail has no caller prose field")

	_, err = participationTrio(t, &scriptedParticipation{}).Record(&encounter.RecordInput{
		Kind: encounter.OutcomeDeathSave, Actor: alice,
	})
	require.ErrorIs(t, err, encounter.ErrInvalidData)

	_, err = participationTrio(t, &scriptedParticipation{}).Record(&encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, DeathSave: detail,
	})
	require.ErrorIs(t, err, encounter.ErrInvalidData)
}

func TestNextStorySeqIsAReadAndEqualsTheNextSuccessfulRecord(t *testing.T) {
	enc := participationTrio(t, &scriptedParticipation{})
	before := storyBeats(t, enc, alice)
	next, err := enc.NextStorySeq()
	require.NoError(t, err)
	require.Equal(t, before, storyBeats(t, enc, alice), "reading the sequence does not append")

	_, err = enc.Record(&encounter.RecordInput{Kind: encounter.OutcomeDeathSave, Actor: alice})
	require.ErrorIs(t, err, encounter.ErrInvalidData)
	afterRejected, err := enc.NextStorySeq()
	require.NoError(t, err)
	require.Equal(t, next, afterRejected, "a rejected append does not reserve a sequence")

	out, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeDeathSave, Actor: alice,
		DeathSave: &encounter.DeathSaveDetail{Roll: 10, Outcome: "success", SuccessesAdded: 1,
			Successes: 1, SuccessesNeeded: 2, FailuresRemaining: 3,
			Continuation: "end_turn", PresentationID: "death-save"},
	})
	require.NoError(t, err)
	require.Equal(t, next, out.Seq)

	after, err := enc.NextStorySeq()
	require.NoError(t, err)
	require.Equal(t, next+1, after)
}
