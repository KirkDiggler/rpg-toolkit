// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// cryptoReaderMu serializes the narrow replacement below. Second Wind still
// rolls through dice's package default (rpg-toolkit#1427); these tests pin the
// story facts without trying to solve that separate provider seam.
var cryptoReaderMu sync.Mutex

func activateWithCryptoByte(
	mgr *session.Manager, in *session.ActivateInput, value byte,
) (*session.ActivateOutput, error) {
	cryptoReaderMu.Lock()
	defer cryptoReaderMu.Unlock()

	original := cryptorand.Reader
	cryptorand.Reader = bytes.NewReader([]byte{value})
	defer func() { cryptorand.Reader = original }()

	return mgr.Activate(context.Background(), in)
}

type activationEventScene struct {
	mgr        *session.Manager
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
}

// newActivationEventScene starts alice and bob in different sealed regions,
// then spawns a skeleton beside alice. Bob cannot see the activation but is
// still in today's full-roster activation audience; that policy belongs to the
// encounter composition, and Session must neither narrow nor widen it.
func newActivationEventScene(
	t *testing.T, alice, bob *character.Data,
) *activationEventScene {
	t.Helper()

	scene := &activationEventScene{
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(alice, bob), stream: &fakeStream{},
	}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: scene.sessions, Encounters: scene.encounters,
		Characters: scene.characters, Events: scene.stream,
	})
	require.NoError(t, err)
	scene.mgr = mgr

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{},
		Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{},
		TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("hall", 0, 0, 8, 8),
				rectRegion("vault", 20, 0, 8, 8),
			},
		},
		Members: []encounter.MemberInput{
			{ID: encounter.MemberID(alice.ID), Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: encounter.MemberID(bob.ID), Kind: encounter.KindPlayer, Position: spatial.Position{X: 21, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	require.NoError(t, err)
	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	require.NoError(t, err)

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active, "the deterministic initiative fixture must leave alice active")
	scene.stream.published = nil
	return scene
}

func secondWindFighter(t *testing.T, id string, hp, maxHP int) *character.Data {
	t.Helper()
	fighter := armedFighter(id)
	fighter.Level = 3
	fighter.HitPoints = hp
	fighter.MaxHitPoints = maxHP
	feature, err := json.Marshal(features.SecondWindData{
		Ref: refs.Features.SecondWind(), ID: id + "-second-wind", Name: "Second Wind",
		Level: fighter.Level, CharacterID: id, Uses: 1, MaxUses: 1,
	})
	require.NoError(t, err)
	fighter.Features = []json.RawMessage{feature}
	return fighter
}

func activationEvents(events []session.Event) []session.Event {
	out := make([]session.Event, 0, len(events))
	for _, event := range events {
		if event.Kind == session.EventActivated || event.Kind == session.EventActivationResult {
			out = append(out, event)
		}
	}
	return out
}

func requireActivationOrder(t *testing.T, events []session.Event) {
	t.Helper()
	require.Len(t, events, 2)
	require.Equal(t, session.EventActivated, events[0].Kind)
	require.Equal(t, session.EventActivationResult, events[1].Kind)
	require.Equal(t, events[0].Seq+1, events[1].Seq, "activation and result are adjacent in each recipient's stream")
}

// TestSecondWindActivationEventsReachTheFullRosterAndCatchUpExactly pins the
// complete transaction: one activated beat before the ordered result, exact
// post-clamp healing facts, current full-roster fan-out, and Story/live parity.
func TestSecondWindActivationEventsReachTheFullRosterAndCatchUpExactly(t *testing.T) {
	scene := newActivationEventScene(t,
		secondWindFighter(t, "alice", 30, 30), armedFighter("bob"))

	id := activationSelector(t, scene.mgr, "alice", refs.Features.SecondWind().String())
	// crypto/rand.Int reads [0,10); byte 6 therefore makes the d10 roll 7.
	out, err := activateWithCryptoByte(scene.mgr, &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	}, 6)
	require.NoError(t, err)
	require.Equal(t, refs.Features.SecondWind().String(), out.Ability)
	require.Equal(t, 6, out.Delivery.Events, "two beats times the complete three-member roster")

	for _, recipient := range []string{"alice", "bob", "skeleton"} {
		live := activationEvents(eventsFor(scene.stream.published, recipient))
		requireActivationOrder(t, live)
		require.Equal(t, map[string]string{"tag": "outcome"}, live[0].Tags)
		require.Equal(t, live[0].Tags, live[1].Tags)

		activated, ok := live[0].Body.(session.ActivatedBody)
		require.True(t, ok, "%s received ActivatedBody, got %T", recipient, live[0].Body)
		require.Equal(t, session.ActivatedBody{
			Actor:   "alice",
			Ability: session.AbilityRef{Ref: refs.Features.SecondWind().String(), Name: "Second Wind"},
		}, activated)

		result, ok := live[1].Body.(session.ActivationResultBody)
		require.True(t, ok, "%s received ActivationResultBody, got %T", recipient, live[1].Body)
		require.Nil(t, result.ConditionApplied)
		require.Nil(t, result.ConditionRemoved)
		require.Nil(t, result.CapacityGranted)
		require.Equal(t, &session.HealingAppliedBody{
			Target: "alice", Amount: 0, Requested: 10, Roll: 7, Modifier: 3,
			SourceRef: refs.Features.SecondWind().String(), SourceName: "Second Wind",
			HPBefore: 30, HPAfter: 30,
		}, result.HealingApplied, "the requested heal is retained even when the HP clamp applies zero")

		catchUp, storyErr := scene.mgr.Story(context.Background(), &session.StoryInput{
			Session: "sess", Member: recipient, FromSeq: live[0].Seq,
		})
		require.NoError(t, storyErr)
		require.Equal(t, live, catchUp,
			"%s must receive byte-for-byte equivalent projection and order on catch-up", recipient)
	}
}

// TestRageAndDashActivationResultVariants pins the two non-healing variants a
// current level-1 activation can produce. Names and descriptions are provider
// facts copied onto the story, not reconstructed by Session.
func TestRageAndDashActivationResultVariants(t *testing.T) {
	scene := newActivationEventScene(t, ragingBarbarian("alice", 2), armedFighter("bob"))

	rageID := activationSelector(t, scene.mgr, "alice", refs.Features.Rage().String())
	_, err := scene.mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: rageID,
	})
	require.NoError(t, err)
	rageEvents := activationEvents(eventsFor(scene.stream.published, "bob"))
	requireActivationOrder(t, rageEvents)
	rageResult := rageEvents[1].Body.(session.ActivationResultBody)
	require.Equal(t, "alice", rageResult.Actor)
	require.Equal(t, &session.ConditionAppliedBody{
		Target: "alice", Ref: refs.Conditions.Raging().String(), Name: "Raging",
	}, rageResult.ConditionApplied)
	require.Nil(t, rageResult.HealingApplied)
	require.Nil(t, rageResult.ConditionRemoved)
	require.Nil(t, rageResult.CapacityGranted)

	beforeDash := len(scene.stream.published)
	dashID := activationSelector(t, scene.mgr, "alice", refs.CombatAbilities.Dash().String())
	dashOut, err := scene.mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: dashID,
	})
	require.NoError(t, err)
	require.Equal(t, "30ft movement", dashOut.GrantedCapacity)
	dashEvents := activationEvents(eventsFor(scene.stream.published[beforeDash:], "bob"))
	requireActivationOrder(t, dashEvents)
	dashResult := dashEvents[1].Body.(session.ActivationResultBody)
	require.Equal(t, &session.CapacityGrantedBody{
		Member: "alice", Description: "30ft movement",
	}, dashResult.CapacityGranted)
	require.Nil(t, dashResult.HealingApplied)
	require.Nil(t, dashResult.ConditionApplied)
	require.Nil(t, dashResult.ConditionRemoved)
}

func storyLengths(t *testing.T, mgr *session.Manager, members ...string) map[string]int {
	t.Helper()
	out := make(map[string]int, len(members))
	for _, member := range members {
		story, err := mgr.Story(context.Background(), &session.StoryInput{
			Session: "sess", Member: member,
		})
		require.NoError(t, err)
		out[member] = len(story)
	}
	return out
}

// TestUnsuccessfulActivationAttemptsAppendNoStory covers the three refusal
// postures callers observe: stale selection, an execution-time malformed
// request, and acting out of turn. None earns an activated acknowledgement.
func TestUnsuccessfulActivationAttemptsAppendNoStory(t *testing.T) {
	alice, bob := ragingBarbarian("alice", 2), ragingBarbarian("bob", 2)
	mgr, _ := aTwoPlayerFightAt(t, alice, spatial.Position{X: 1, Y: 1}, bob, spatial.Position{X: 2, Y: 1})
	members := []string{"alice", "bob", "skel-1"}

	rageID := activationSelector(t, mgr, "alice", refs.Features.Rage().String())
	cases := []struct {
		name string
		in   *session.ActivateInput
		want error
	}{
		{
			name: "stale selector",
			in:   &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: "v1.stale"},
			want: session.ErrStaleDeclaration,
		},
		{
			name: "malformed selected activation",
			in:   &session.ActivateInput{Session: "sess", Member: "alice", DeclarationID: rageID, Target: "bob"},
			want: session.ErrBadActivation,
		},
		{
			name: "refused out of turn",
			in:   &session.ActivateInput{Session: "sess", Member: "bob", DeclarationID: "v1.any"},
			want: session.ErrNotYourTurn,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := storyLengths(t, mgr, members...)
			out, err := mgr.Activate(context.Background(), tc.in)
			require.Nil(t, out)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, before, storyLengths(t, mgr, members...),
				"an unsuccessful activation must append no beat for any recipient")
		})
	}
}

type failAfterCharacterSave struct {
	inner       *fakeCharacters
	err         error
	saved       bool
	failedReads int
}

func (f *failAfterCharacterSave) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	if f.saved {
		f.failedReads++
		return nil, f.err
	}
	return f.inner.GetCharacter(ctx, id)
}

func (f *failAfterCharacterSave) SaveCharacter(ctx context.Context, data *character.Data) error {
	if err := f.inner.SaveCharacter(ctx, data); err != nil {
		return err
	}
	f.saved = true
	return nil
}

// TestActivationRecordFailureReportsTheDurableSheetAndDropsTheEncounterScope
// fails the Standing seam only after saveDirty lands the changed sheet. The
// next consult is RecordActivation's post-append noticeDown: its in-memory
// beats must be dropped while the durable sheet is reported as written.
func TestActivationRecordFailureReportsTheDurableSheetAndDropsTheEncounterScope(t *testing.T) {
	alice := ragingBarbarian("alice", 2)
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	innerCharacters := newFakeCharacters(alice)
	standingErr := errors.New("standing unavailable after character save")
	characters := &failAfterCharacterSave{inner: innerCharacters, err: standingErr}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions,
		Encounters: encounters, Characters: characters, Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{},
		Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{},
		TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 8, 8),
		}},
		Members: []encounter.MemberInput{{
			ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1},
		}},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	world := enc.ToData()
	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &world})
	require.NoError(t, err)
	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	require.NoError(t, err)

	id := activationSelector(t, mgr, "alice", refs.Features.Rage().String())
	persistedBefore, err := copyOf(encounters.byID["world"])
	require.NoError(t, err)
	encounterSavesBefore := encounters.saves

	out, err := mgr.Activate(ctx, &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
	require.Nil(t, out, "a missing encounter record cannot be acknowledged as success")
	require.ErrorIs(t, err, session.ErrSaveFailed)
	require.ErrorIs(t, err, standingErr)
	var saveErr *session.SaveError
	require.ErrorAs(t, err, &saveErr)
	require.Equal(t, session.SaveReport{
		Written: []string{"character:alice"}, Failed: []string{"encounter:world"},
	}, saveErr.Report)
	require.True(t, saveErr.Report.Partial())
	require.Positive(t, characters.failedReads,
		"the configured Standing seam must fail after the durable character write")

	require.Equal(t, encounterSavesBefore, encounters.saves,
		"the encounter scope whose post-append notice failed must never be committed")
	require.Equal(t, persistedBefore, encounters.byID["world"],
		"the activated/result beats existed only in the discarded write scope")
	require.Contains(t, storedConditionRefs(t, innerCharacters, "alice"), refs.Conditions.Raging().String(),
		"the mechanical condition write remains durable and must be reported")
}

// TestActivationDissolveReportsNestedBoundarySaveFailure is the reachable
// activation counterpart to #1403's Attack regression. Rage first lands its
// paid sheet. RecordActivation then notices the already-down last monster,
// dissolves the fight, and announces combat end; that boundary removes Rage,
// but the newer save of Alice's sheet is refused.
//
// The recording boundary must compose the nested report rather than replace
// it. Alice therefore truthfully occurs in both lists: the activation write
// landed, while its newer boundary update did not. None of the activation,
// down, or dissolve story — nor the session working copy — may become durable.
func TestActivationDissolveReportsNestedBoundarySaveFailure(t *testing.T) {
	ctx := context.Background()
	alice := ragingBarbarian("alice", 2)
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	innerCharacters := newFakeCharacters(alice, armedFighter("bob"))
	boundaryStoreUnavailable := errors.New("boundary character store unavailable")
	boundaryWriteRefused := errors.New("boundary character write refused")
	characters := &failNthArmedSaveCharacters{
		fakeCharacters: innerCharacters,
		failAt:         2,
		err:            errors.Join(boundaryStoreUnavailable, boundaryWriteRefused),
	}
	stream := &fakeStream{}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions,
		Encounters: encounters, Characters: characters, Events: stream,
	})
	require.NoError(t, err)

	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: cryptWorld(t),
	})
	require.NoError(t, err)
	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	require.NoError(t, err)
	for i := range sessions.byID["sess"].NPCs {
		if sessions.byID["sess"].NPCs[i].ID == "skeleton" {
			sessions.byID["sess"].NPCs[i].HitPoints = 0
		}
	}

	declaration := activationSelector(t, mgr, "alice", refs.Features.Rage().String())
	persistedWorldBefore, err := encounters.GetEncounter(ctx, "world")
	require.NoError(t, err)
	persistedSessionBefore, err := sessions.GetSession(ctx, "sess")
	require.NoError(t, err)
	stream.published = nil
	characters.armed = true

	out, err := mgr.Activate(ctx, &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: declaration,
	})

	require.Nil(t, out, "an unrecorded activation cannot be acknowledged")
	require.ErrorIs(t, err, session.ErrSaveFailed)
	require.ErrorIs(t, err, boundaryStoreUnavailable)
	require.ErrorIs(t, err, boundaryWriteRefused)
	require.Equal(t, 2, characters.attempts,
		"Rage lands once before combat end attempts the newer sheet")

	var reported *session.SaveError
	require.ErrorAs(t, err, &reported, "ordinary errors.As must reach the complete outer report")
	require.Equal(t, session.SaveReport{
		Written: []string{"character:alice"},
		Failed:  []string{"character:alice", "encounter:world"},
	}, reported.Report)
	require.Contains(t, reported.Report.Written, "character:alice")
	require.Contains(t, reported.Report.Failed, "character:alice",
		"one aggregate can truthfully have an older write and newer failure")

	storedAlice := storedSheet(t, innerCharacters, "alice")
	require.Contains(t, storedConditionRefs(t, innerCharacters, "alice"), refs.Conditions.Raging().String(),
		"the initial activation condition is durable; combat-end removal was not")
	require.Equal(t, 1, storedAlice.Resources[resources.RageCharges].Current,
		"the initial activation resource spend is durable")
	require.NotNil(t, storedAlice.ActionEconomy)
	require.Zero(t, storedAlice.ActionEconomy.BonusActionsRemaining,
		"the initial activation economy spend is durable")

	persistedWorldAfter, getErr := encounters.GetEncounter(ctx, "world")
	require.NoError(t, getErr)
	require.Equal(t, persistedWorldBefore, persistedWorldAfter,
		"activation, result, down, and dissolve beats remain only on the discarded scope")
	persistedSessionAfter, getErr := sessions.GetSession(ctx, "sess")
	require.NoError(t, getErr)
	require.Equal(t, persistedSessionBefore, persistedSessionAfter,
		"the dissolved in-memory fight must not reach the persisted session")
	require.Empty(t, stream.published, "nothing unpersisted is delivered")
}
