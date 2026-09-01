// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

var joinRestPool = coreResources.ResourceKey("join-rest-pool")

// copyingCharacters models the ownership boundary a repository actually has:
// both reads and successful writes cross a serialization boundary. A fake that
// returns or stores the caller's pointer could make an omitted SaveCharacter
// look durable and would make the first-admission tests meaningless.
type copyingCharacters struct {
	byID map[string]*character.Data

	saves        int
	saveAttempts int
	saveErr      error
}

func newCopyingCharacters(t *testing.T, records ...*character.Data) *copyingCharacters {
	t.Helper()
	out := &copyingCharacters{byID: make(map[string]*character.Data)}
	for _, record := range records {
		out.seed(t, record)
	}
	return out
}

func (r *copyingCharacters) seed(t *testing.T, data *character.Data) {
	t.Helper()
	copied, err := copyOf(data)
	if err != nil {
		t.Fatalf("copy character %q into repository: %v", data.ID, err)
	}
	r.byID[data.ID] = copied
}

func (r *copyingCharacters) stored(t *testing.T, id string) *character.Data {
	t.Helper()
	data, ok := r.byID[id]
	if !ok {
		t.Fatalf("character repository does not hold %q", id)
	}
	copied, err := copyOf(data)
	if err != nil {
		t.Fatalf("copy stored character %q: %v", id, err)
	}
	return copied
}

func (r *copyingCharacters) GetCharacter(_ context.Context, id string) (*character.Data, error) {
	data, ok := r.byID[id]
	if !ok {
		return nil, session.ErrNotFound
	}
	return copyOf(data)
}

func (r *copyingCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	r.saveAttempts++
	if r.saveErr != nil {
		return r.saveErr
	}
	copied, err := copyOf(data)
	if err != nil {
		return err
	}
	r.byID[data.ID] = copied
	r.saves++
	return nil
}

// failingReadCharacters makes a placement/discovery consult fail after Join's
// own record fetch has succeeded. It pins that the rested record is not saved
// merely because rest and projection succeeded; every local check must pass.
type failingReadCharacters struct {
	inner  *copyingCharacters
	failID string
	err    error
}

func (r failingReadCharacters) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	if id == r.failID {
		return nil, r.err
	}
	return r.inner.GetCharacter(ctx, id)
}

func (r failingReadCharacters) SaveCharacter(ctx context.Context, data *character.Data) error {
	return r.inner.SaveCharacter(ctx, data)
}

// JoinLongRestTestSuite covers the first-admission rest as a persistence
// operation. Every positive assertion reads the copying repository again; a
// returned JoinOutput alone cannot prove the character write happened.
type JoinLongRestTestSuite struct {
	suite.Suite

	ctx        context.Context
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *copyingCharacters
	mgr        *session.Manager
}

func (s *JoinLongRestTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.characters = newCopyingCharacters(s.T(),
		dwarfCharacter("alice"), spentJoinFighter(s.T(), "bob"), spentJoinFighter(s.T(), "carol"))
	s.mgr = s.manager(s.encounters, s.characters)

	_, err := s.mgr.StartSession(s.ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: hexWorld(s.T()),
	})
	s.Require().NoError(err)
}

func (s *JoinLongRestTestSuite) manager(
	encounters session.EncounterRepository, characters session.CharacterRepository,
) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Events: session.DiscardEvents{},
		Sessions: s.sessions, Encounters: encounters, Characters: characters,
	})
	s.Require().NoError(err)
	return mgr
}

func (s *JoinLongRestTestSuite) TestFirstAdmissionRestsPersistsAndProjectsTheCompleteOutcome() {
	out, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Require().NotNil(out.Character)
	s.Equal(36, out.Character.HitPoints, "JoinOutput is projected from the rested record")
	s.Equal(36, out.Character.MaxHitPoints)
	s.Equal([]string{"character:bob", "encounter:world", "session:sess"}, out.Saved.Written)
	s.Empty(out.Saved.Failed)
	s.Equal(1, s.characters.saves, "first admission performs one durable character save")

	s.assertCompleteRest(s.characters.stored(s.T(), "bob"))
}

func (s *JoinLongRestTestSuite) TestDuplicateCurrentJoinDoesNotRestOrSave() {
	_, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().NoError(err)

	// A strict LongRest would reject this effect. The ordinary projection is
	// deliberately lenient, so ErrNoMember proves encounter.Join — not rest —
	// refused the duplicate.
	spent := spentJoinFighter(s.T(), "bob")
	spent.Conditions = append(spent.Conditions, malformedRage("bob"))
	s.characters.seed(s.T(), spent)
	beforeSaves := s.characters.saves
	beforeAttempts := s.characters.saveAttempts
	beforeEncounterSaves := s.encounters.saves

	out, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(3, 2),
	})
	s.Require().ErrorIs(err, session.ErrNoMember)
	s.Nil(out)
	s.Equal(beforeSaves, s.characters.saves)
	s.Equal(beforeAttempts, s.characters.saveAttempts, "a duplicate never attempts a character save")
	s.Equal(beforeEncounterSaves, s.encounters.saves)
	s.Equal(7, s.characters.stored(s.T(), "bob").HitPoints, "the deliberately re-spent record remains spent")
}

func (s *JoinLongRestTestSuite) TestExitThenRejoinUsesPersistedEverMembersAndDoesNotRestOrSave() {
	_, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().NoError(err)
	_, err = s.mgr.Exit(s.ctx, &session.ExitInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)

	spent := spentJoinFighter(s.T(), "bob")
	spent.Conditions = append(spent.Conditions, malformedRage("bob"))
	s.characters.seed(s.T(), spent)
	beforeSaves := s.characters.saves
	beforeAttempts := s.characters.saveAttempts

	out, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(3, 2),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Character)
	s.Equal(7, out.Character.HitPoints, "rejoin projects the original, deliberately re-spent record")
	s.NotContains(out.Saved.Written, "character:bob")
	s.Equal(beforeSaves, s.characters.saves)
	s.Equal(beforeAttempts, s.characters.saveAttempts)
	s.Equal(7, s.characters.stored(s.T(), "bob").HitPoints)
}

func (s *JoinLongRestTestSuite) TestGenuinelyNewLateMemberRests() {
	_, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().NoError(err)
	beforeSaves := s.characters.saves

	out, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "carol", Position: hexCell(3, 2),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Character)
	s.Equal(36, out.Character.HitPoints)
	s.Contains(out.Saved.Written, "character:carol")
	s.Equal(beforeSaves+1, s.characters.saves)
	s.assertCompleteRest(s.characters.stored(s.T(), "carol"))
}

func (s *JoinLongRestTestSuite) TestMalformedFirstAdmissionWritesNothing() {
	malformed := spentJoinFighter(s.T(), "bob")
	malformed.Conditions = append(malformed.Conditions, malformedRage("bob"))
	s.characters.seed(s.T(), malformed)
	beforeWorld := s.storedWorldJSON()
	beforeEncounterSaves := s.encounters.saves

	out, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadCharacter)
	s.Nil(out)
	s.Zero(s.characters.saveAttempts)
	s.Equal(beforeEncounterSaves, s.encounters.saves)
	s.JSONEq(beforeWorld, s.storedWorldJSON())
	s.Equal(7, s.characters.stored(s.T(), "bob").HitPoints)
}

func (s *JoinLongRestTestSuite) TestBadProjectionLeavesEarlyRestDurableAndReported() {
	broken := spentJoinFighter(s.T(), "bob")
	broken.EquipmentSlots = character.EquipmentSlots{character.SlotMainHand: armor.ChainMail}
	s.characters.seed(s.T(), broken)
	beforeWorld := s.storedWorldJSON()
	beforeEncounterSaves := s.encounters.saves

	out, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadAttack)
	s.assertWrittenOnly(err, "character:bob")
	s.Nil(out)
	s.Equal(1, s.characters.saves)
	s.Equal(beforeEncounterSaves, s.encounters.saves)
	s.JSONEq(beforeWorld, s.storedWorldJSON())
	s.assertCompleteRest(s.characters.stored(s.T(), "bob"))
}

func (s *JoinLongRestTestSuite) TestBadPlacementLeavesEarlyRestDurableAndReported() {
	beforeWorld := s.storedWorldJSON()
	beforeEncounterSaves := s.encounters.saves

	out, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(100, 100),
	})
	s.Require().ErrorIs(err, session.ErrBadPosition)
	s.assertWrittenOnly(err, "character:bob")
	s.Nil(out)
	s.Equal(1, s.characters.saves)
	s.Equal(beforeEncounterSaves, s.encounters.saves)
	s.JSONEq(beforeWorld, s.storedWorldJSON())
	s.assertCompleteRest(s.characters.stored(s.T(), "bob"))
}

func (s *JoinLongRestTestSuite) TestDiscoveryFailureLeavesEarlyRestDurableAndReported() {
	brokenReads := failingReadCharacters{inner: s.characters, failID: "alice", err: errBroken}
	mgr := s.manager(s.encounters, brokenReads)
	beforeWorld := s.storedWorldJSON()
	beforeEncounterSaves := s.encounters.saves

	out, err := mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().ErrorIs(err, errBroken)
	s.assertWrittenOnly(err, "character:bob")
	s.Nil(out)
	s.Equal(1, s.characters.saves)
	s.Equal(beforeEncounterSaves, s.encounters.saves)
	s.JSONEq(beforeWorld, s.storedWorldJSON())
	s.assertCompleteRest(s.characters.stored(s.T(), "bob"))
}

func (s *JoinLongRestTestSuite) TestCorruptStreamAfterEarlyRestReportsWriteAndSavesNoEncounter() {
	data, err := s.sessions.GetSession(s.ctx, "sess")
	s.Require().NoError(err)
	data.Streams = map[string]session.StreamCursor{
		"alice": {UpTo: 1, Count: 0},
	}
	s.Require().NoError(s.sessions.SaveSession(s.ctx, data))
	beforeWorld := s.storedWorldJSON()
	beforeEncounterSaves := s.encounters.saves

	out, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrInvalidWorld)
	s.assertWrittenOnly(err, "character:bob")
	s.Nil(out)
	s.Equal(1, s.characters.saves)
	s.Equal(beforeEncounterSaves, s.encounters.saves)
	s.JSONEq(beforeWorld, s.storedWorldJSON())
	s.assertCompleteRest(s.characters.stored(s.T(), "bob"))
}

func (s *JoinLongRestTestSuite) TestCharacterSaveFailureLeavesEncounterUnchangedAndReportsFailure() {
	s.characters.saveErr = errBroken
	beforeWorld := s.storedWorldJSON()
	beforeEncounterSaves := s.encounters.saves

	out, err := s.mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrSaveFailed)
	s.ErrorIs(err, errBroken)
	s.Nil(out)
	var saveErr *session.SaveError
	s.Require().True(errors.As(err, &saveErr))
	s.Equal(session.SaveReport{Failed: []string{"character:bob"}}, saveErr.Report)
	s.Equal(1, s.characters.saveAttempts)
	s.Zero(s.characters.saves)
	s.Equal(beforeEncounterSaves, s.encounters.saves)
	s.JSONEq(beforeWorld, s.storedWorldJSON())
	s.Equal(7, s.characters.stored(s.T(), "bob").HitPoints)
}

func (s *JoinLongRestTestSuite) TestEncounterSaveFailureLeavesRestedCharacterDurableAndReportsPartialWrite() {
	failing := &failingEncounters{fakeEncounters: s.encounters, saveErr: errBroken}
	mgr := s.manager(failing, s.characters)
	beforeWorld := s.storedWorldJSON()
	beforeEncounterSaves := s.encounters.saves

	out, err := mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrSaveFailed)
	s.ErrorIs(err, errBroken)
	s.Nil(out)
	var saveErr *session.SaveError
	s.Require().True(errors.As(err, &saveErr))
	s.Equal(session.SaveReport{
		Written: []string{"character:bob"},
		Failed:  []string{"encounter:world"},
	}, saveErr.Report)
	s.True(saveErr.Report.Partial())
	s.Equal(1, s.characters.saves)
	s.Equal(beforeEncounterSaves, s.encounters.saves)
	s.JSONEq(beforeWorld, s.storedWorldJSON(), "the failed placement never persisted")
	s.assertCompleteRest(s.characters.stored(s.T(), "bob"))
}

func (s *JoinLongRestTestSuite) TestSessionSaveFailureReportsEarlyRestAndPersistedEncounter() {
	failing := &failingSessions{fakeSessions: s.sessions, saveErr: errBroken}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Events: session.DiscardEvents{},
		Sessions: failing, Encounters: s.encounters, Characters: s.characters,
	})
	s.Require().NoError(err)
	beforeEncounterSaves := s.encounters.saves

	out, err := mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(2, 2),
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrSaveFailed)
	s.ErrorIs(err, errBroken)
	s.Nil(out)
	var saveErr *session.SaveError
	s.Require().True(errors.As(err, &saveErr))
	s.Equal(session.SaveReport{
		Written: []string{"character:bob", "encounter:world"},
		Failed:  []string{"session:sess"},
	}, saveErr.Report)
	s.Equal(beforeEncounterSaves+1, s.encounters.saves)
	s.assertCompleteRest(s.characters.stored(s.T(), "bob"))

	// The encounter half really landed even though its session cursor did not.
	_, err = s.mgr.View(s.ctx, &session.ViewInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
}

func (s *JoinLongRestTestSuite) TestZeroHPFirstAdmissionIsStandingAndCannotFireMemberDownEnding() {
	sessions := newFakeSessions()
	encounters := newFakeEncounters()
	zero := spentJoinFighter(s.T(), "bob")
	zero.HitPoints = 0
	characters := newCopyingCharacters(s.T(), zero)
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Events: session.DiscardEvents{},
		Sessions: sessions, Encounters: encounters, Characters: characters,
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(s.ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: memberDownJoinWorld(s.T()),
	})
	s.Require().NoError(err)

	out, err := mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(1, 1),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Nil(out.Outcome, "rested repository truth must not fire the member-down ending")
	status, err := mgr.Status(s.ctx, &session.StatusInput{Session: "sess"})
	s.Require().NoError(err)
	s.True(status.Open)
	s.Nil(status.Outcome)
	s.Equal(36, characters.stored(s.T(), "bob").HitPoints)
}

func (s *JoinLongRestTestSuite) TestPlacementDrivenStrikeReadsRestedTruthAndIsNotOverwritten() {
	sessions := newFakeSessions()
	encounters := newFakeEncounters()
	characters := newCopyingCharacters(s.T(), spentJoinFighter(s.T(), "bob"))
	// Initiative asks alphabetically: bob rolls 1, skel-1 rolls 20. The
	// skeleton then hits on 15; the remaining fixed rolls drive its damage.
	dice := &sequenceDice{rolls: []int{1, 20, 15, 4, 4, 4, 4, 4}}
	mgr, err := session.NewManager(&session.Config{
		Dice: dice, TurnDriver: session.Behavior(), Events: session.DiscardEvents{},
		Sessions: sessions, Encounters: encounters, Characters: characters,
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(s.ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: tombRoom(6, 6),
	})
	s.Require().NoError(err)
	spawned, err := mgr.Spawn(s.ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(), Position: hexCell(1, 0),
	})
	s.Require().NoError(err)
	s.Nil(spawned.Formed, "the skeleton alone cannot form a fight")

	joined, err := mgr.Join(s.ctx, &session.JoinInput{
		Session: "sess", Member: "bob", Position: hexCell(0, 0),
	})
	s.Require().NoError(err)
	s.Require().NotNil(joined.Formed, "the arrival must trigger the driven monster turn")
	s.Equal([]string{"skel-1", "bob"}, joined.Formed.Order)
	stored := characters.stored(s.T(), "bob")
	s.Positive(stored.HitPoints, "the driven strike starts from full rested truth, not the old seven HP")
	s.Less(stored.HitPoints, stored.MaxHitPoints,
		"the driven damage survives; no stale rest save may overwrite it")
	s.Equal(2, characters.saves, "the early rest and later driven damage are both durable")
}

func (s *JoinLongRestTestSuite) assertWrittenOnly(err error, identities ...string) {
	s.T().Helper()
	s.ErrorIs(err, session.ErrSaveFailed)
	var saveErr *session.SaveError
	s.Require().True(errors.As(err, &saveErr))
	s.Equal(session.SaveReport{Written: identities}, saveErr.Report)
}

func (s *JoinLongRestTestSuite) storedWorldJSON() string {
	s.T().Helper()
	raw, err := json.Marshal(s.encounters.byID["world"])
	s.Require().NoError(err)
	return string(raw)
}

func (s *JoinLongRestTestSuite) assertCompleteRest(got *character.Data) {
	s.T().Helper()
	s.Require().NotNil(got)
	s.Equal(36, got.HitPoints)
	s.Equal(36, got.MaxHitPoints)
	s.Require().NotNil(got.DeathSaveState)
	s.Zero(got.DeathSaveState.Successes)
	s.Zero(got.DeathSaveState.Failures)
	s.False(got.DeathSaveState.Stabilized)
	s.False(got.DeathSaveState.Dead)

	s.Equal(character.RecoverableResourceData{Current: 3, Maximum: 4, ResetType: coreResources.ResetLongRest},
		got.Resources[resources.HitDice], "half the maximum hit dice recover without exceeding maximum")
	s.Equal(character.RecoverableResourceData{Current: 2, Maximum: 2, ResetType: coreResources.ResetShortRest},
		got.Resources[joinRestPool], "character-owned rest resources refill")
	s.Equal(character.SpellSlotData{Max: 3, Used: 0}, got.SpellSlots[1])
	s.Equal(character.SpellSlotData{Max: 2, Used: 0}, got.SpellSlots[2])

	var secondWind features.SecondWindData
	s.Require().NoError(json.Unmarshal(effectWithRef(s.T(), got.Features, refs.Features.SecondWind()), &secondWind))
	s.Equal(1, secondWind.Uses, "feature-owned resources hear the normal rest event")
	s.Equal(1, secondWind.MaxUses)

	var opportunity conditions.OpportunityAttackConditionData
	s.Require().NoError(json.Unmarshal(
		effectWithRef(s.T(), got.Conditions, refs.Conditions.OpportunityAttack()), &opportunity))
	s.False(opportunity.UsedThisTurn, "retained passive condition resets its mutable meter")
	s.Nil(effectWithRefOrNil(got.Conditions, refs.Conditions.Prone()),
		"temporary conditions are removed")
}

func spentJoinFighter(t *testing.T, id string) *character.Data {
	t.Helper()
	secondWind, err := json.Marshal(features.SecondWindData{
		Ref: refs.Features.SecondWind(), ID: id + "-second-wind", Name: "Second Wind",
		Level: 4, CharacterID: id, Uses: 0, MaxUses: 1,
	})
	if err != nil {
		t.Fatalf("build Second Wind: %v", err)
	}
	opportunity, err := (&conditions.OpportunityAttackCondition{
		MemberID: id, UsedThisTurn: true,
	}).ToJSON()
	if err != nil {
		t.Fatalf("build Opportunity Attack: %v", err)
	}
	prone, err := conditions.NewProneCondition(id).ToJSON()
	if err != nil {
		t.Fatalf("build Prone: %v", err)
	}

	return &character.Data{
		ID: id, PlayerID: "player-" + id, Name: "Spent Fighter",
		Level: 4, ProficiencyBonus: 2, RaceID: races.Human, ClassID: classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 7, MaxHitPoints: 36, ArmorClass: 16,
		DeathSaveState: &saves.DeathSaveState{
			Successes: 1, Failures: 2, Stabilized: true, Dead: true,
		},
		SpellSlots: map[int]character.SpellSlotData{
			1: {Max: 3, Used: 3},
			2: {Max: 2, Used: 1},
		},
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.HitDice: {Current: 1, Maximum: 4, ResetType: coreResources.ResetLongRest},
			joinRestPool:      {Current: 0, Maximum: 2, ResetType: coreResources.ResetShortRest},
		},
		Features:   []json.RawMessage{secondWind},
		Conditions: []json.RawMessage{opportunity, prone},
	}
}

func memberDownJoinWorld(t *testing.T) *encounter.EncounterData {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{},
		Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 4, 4)},
		},
		Endings: []encounter.EndingInput{
			{Key: "bob-down", Trigger: encounter.TriggerMemberDown{Member: "bob"}},
			{Key: "withdraw", Trigger: encounter.TriggerExternal{}},
		},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("build member-down Join world: %v", err)
	}
	data := enc.ToData()
	return &data
}

func malformedRage(id string) json.RawMessage {
	return json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"raging"},` +
		`"character_id":"` + id + `","turns_active":"not-a-number"}`)
}

func effectWithRef(t *testing.T, blobs []json.RawMessage, want *core.Ref) json.RawMessage {
	t.Helper()
	if got := effectWithRefOrNil(blobs, want); got != nil {
		return got
	}
	t.Fatalf("effect %s not found", want.String())
	return nil
}

func effectWithRefOrNil(blobs []json.RawMessage, want *core.Ref) json.RawMessage {
	for _, raw := range blobs {
		var envelope struct {
			Ref core.Ref `json:"ref"`
		}
		if json.Unmarshal(raw, &envelope) == nil && envelope.Ref.Equals(want) {
			return raw
		}
	}
	return nil
}

func TestJoinLongRestSuite(t *testing.T) {
	suite.Run(t, new(JoinLongRestTestSuite))
}
