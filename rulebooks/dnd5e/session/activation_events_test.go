// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type activationEventScene struct {
	mgr        *session.Manager
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *actCapture
	// marks is each member's last record seq when the scene was set.
	marks map[string]uint64
}

// newActivationEventScene starts alice and bob in different sealed regions,
// then spawns a skeleton beside alice. Bob cannot see the activation but is
// still in today's full-roster activation audience; that policy belongs to the
// encounter composition, and Session must neither narrow nor widen it.
func newActivationEventScene(
	t *testing.T, alice, bob *character.Data,
) *activationEventScene {
	t.Helper()

	es := &activationEventScene{
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(alice, bob), stream: &actCapture{},
	}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: es.stream, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: es.sessions, Encounters: es.encounters,
		Characters: es.characters,
	})
	require.NoError(t, err)
	es.mgr = mgr

	sc := scene{
		Field: encounter.FieldInput{
			Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("hall", 0, 0, 8, 8),
				rectRegion("vault", 20, 0, 8, 8),
			},
		},
		Party: []sceneSeat{seatAt(alice.ID, 1, 1), seatAt(bob.ID, 21, 1)},
		Monsters: []dungeonspec.MonsterPlacement{
			monsterAt("skeleton", refs.Monsters.Skeleton().String(), 2, 1),
		},
	}

	ctx := context.Background()
	launchScene(t, mgr, sc)

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, "alice", turn.Active, "the deterministic initiative fixture must leave alice active")
	es.stream.acts = nil
	es.marks = map[string]uint64{}
	for _, member := range []string{alice.ID, bob.ID, "skeleton"} {
		es.marks[member] = lastSeq(storyBeats(t, mgr, "sess", member))
	}
	return es
}

// since is what one member's story told them after the scene was set.
func (es *activationEventScene) since(t *testing.T, member string) []witnessed {
	t.Helper()
	return storyBeatsAfter(t, es.mgr, "sess", member, es.marks[member])
}

func secondWindFighter(t *testing.T, id string, hp, maxHP int) *character.Data {
	t.Helper()
	fighter := armedFighter(id)
	setLevel(fighter, 3)
	fighter.HitPoints = hp
	fighter.MaxHitPoints = maxHP
	feature, err := json.Marshal(features.SecondWindData{
		Ref: refs.Features.SecondWind(), ID: id + "-second-wind", Name: "Second Wind",
		CharacterID: id, Uses: 1, MaxUses: 1,
	})
	require.NoError(t, err)
	fighter.Features = []json.RawMessage{feature}
	return fighter
}

// activationEvents is the activated and activation-result beats of a story,
// asserted to be exactly one activation and its result, adjacent in it.
func activationEvents(t *testing.T, story []witnessed) []witnessed {
	t.Helper()
	var out []witnessed
	for i, beat := range story {
		if beat.Beat != encounter.BeatActivated {
			continue
		}
		require.Less(t, i+1, len(story), "the activation has a result after it")
		require.Equal(t, encounter.BeatActivationResult, story[i+1].Beat,
			"activation and result are adjacent in each recipient's story")
		out = append(out, beat, story[i+1])
	}
	require.Len(t, out, 2, "one activation and its result")
	require.Len(t, beatsNamed(story, encounter.BeatActivated, encounter.BeatActivationResult), 2)
	return out
}

// activatedBeat is an activated beat's payload.
type activatedBeat struct {
	Actor   string           `json:"actor"`
	Ability session.SpellRef `json:"ability"`
	Target  string           `json:"target"`
}

// TestSecondWindActivationEventsReachTheFullRoster pins the
// complete transaction: one activated beat before the ordered result, exact
// post-clamp healing facts, and current full-roster fan-out.
func TestSecondWindActivationEventsReachTheFullRoster(t *testing.T) {
	scene := newActivationEventScene(t,
		secondWindFighter(t, "alice", 30, 30), armedFighter("bob"))

	id := activationSelector(t, scene.mgr, "alice", refs.Features.SecondWind().String())
	// Second Wind rolls through the host's dice seam now (rpg-toolkit#1427):
	// the scene's testDice hands the d10 its maximum face, deterministically.
	out, err := scene.mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: id,
	})
	require.NoError(t, err)
	require.Equal(t, refs.Features.SecondWind().String(), out.Ability)
	require.Empty(t, out.Delivery.Failed)

	for _, recipient := range []string{"alice", "bob", "skeleton"} {
		told := activationEvents(t, scene.since(t, recipient))
		requireActID(t, told[0].Correlation, "activate")
		require.Equal(t, told[0].Correlation, told[1].Correlation, "its result is the same act")

		var activated activatedBeat
		told[0].decode(t, &activated)
		require.Equal(t, activatedBeat{
			Actor:   "alice",
			Ability: session.SpellRef{Ref: refs.Features.SecondWind().String(), Name: "Second Wind"},
		}, activated)

		result := resultOf(t, told[1])
		require.Equal(t, "alice", result.Actor)
		require.Equal(t, encounter.ResultHealingApplied, result.Kind)
		require.Equal(t, "alice", result.Target)
		require.Equal(t, refs.Features.SecondWind().String(), result.Ref)
		require.Equal(t, "Second Wind", result.Name)
		for name, got := range map[string]*int{"amount": result.Amount, "requested": result.Requested, "before": result.Before, "after": result.After} {
			require.NotNil(t, got, name)
		}
		require.Equal(t, 0, *result.Amount)
		require.Equal(t, 13, *result.Requested)
		require.Equal(t, 30, *result.Before)
		require.Equal(t, 30, *result.After)
		modifier := 3
		require.Equal(t,
			&session.RollCalculation{
				Components: []session.RollComponent{
					{
						Source: session.RollSource{
							Ref: refs.Features.SecondWind().String(), Name: "Second Wind",
							// The healing dice are the fighter's own: every
							// dice pool names the entity whose rule threw it
							// (rpg-project#462 R7).
							SourceID: "alice",
						},
						Dice: &session.DiceTrace{
							Notation: "1d10", DieSize: 10,
							OriginalRolls: []int{10}, FinalRolls: []int{10}, Subtotal: 10,
						},
					},
					{
						Source: session.RollSource{
							Ref: refs.Classes.Fighter().String(), Name: "Fighter", Label: "Fighter level",
						},
						Modifier: &modifier,
					},
				},
				Total: 13,
			}, result.Calculation,
			"the sourced roll trace is the healing's only roll record, retained even when the HP clamp applies zero")

		var delivered bool
		for _, act := range scene.stream.of(recipient) {
			delivered = delivered || act.Act == told[0].Correlation
		}
		require.True(t, delivered, "%s was handed the activation live", recipient)
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
	rageEvents := activationEvents(t, scene.since(t, "bob"))
	rageResult := resultOf(t, rageEvents[1])
	require.Equal(t, "alice", rageResult.Actor)
	require.Equal(t, encounter.ResultConditionApplied, rageResult.Kind)
	require.Equal(t, "alice", rageResult.Target)
	require.Equal(t, refs.Conditions.Raging().String(), rageResult.Ref)
	require.Equal(t, "Raging", rageResult.Name)
	require.Empty(t, rageResult.SourceID)
	require.Nil(t, rageResult.Detail)

	scene.marks["bob"] = lastSeq(storyBeats(t, scene.mgr, "sess", "bob"))
	dashID := activationSelector(t, scene.mgr, "alice", refs.CombatAbilities.Dash().String())
	dashOut, err := scene.mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "alice", DeclarationID: dashID,
	})
	require.NoError(t, err)
	require.Equal(t, "30ft movement", dashOut.GrantedCapacity)
	dashEvents := activationEvents(t, scene.since(t, "bob"))
	dashResult := resultOf(t, dashEvents[1])
	require.Equal(t, encounter.ResultCapacityGranted, dashResult.Kind)
	require.Equal(t, "alice", dashResult.Target)
	require.Equal(t, "30ft movement", dashResult.Description)
}

func storyLengths(t *testing.T, mgr *session.Manager, members ...string) map[string]int {
	t.Helper()
	out := make(map[string]int, len(members))
	for _, member := range members {
		out[member] = len(storyBeats(t, mgr, "sess", member))
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
	armed       bool
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
	f.saved = f.armed
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
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: session.DiscardActs{}, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions,
		Encounters: encounters, Characters: characters,
	})
	require.NoError(t, err)

	sc := scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 8, 8),
		}},
		Party: []sceneSeat{seatAt("alice", 1, 1)},
		Monsters: []dungeonspec.MonsterPlacement{
			monsterAt("skeleton", refs.Monsters.Skeleton().String(), 2, 1),
		},
	}
	ctx := context.Background()
	launchScene(t, mgr, sc)

	id := activationSelector(t, mgr, "alice", refs.Features.Rage().String())
	characters.armed = true
	persistedBefore, err := copyOf(encounters.byID[testSession])
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
		Written: []string{"character:alice"}, Failed: []string{"encounter:sess"},
	}, saveErr.Report)
	require.True(t, saveErr.Report.Partial())
	require.Positive(t, characters.failedReads,
		"the configured Standing seam must fail after the durable character write")

	require.Equal(t, encounterSavesBefore, encounters.saves,
		"the encounter scope whose post-append notice failed must never be committed")
	require.Equal(t, persistedBefore, encounters.byID[testSession],
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
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: stream, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions,
		Encounters: encounters, Characters: characters,
	})
	require.NoError(t, err)

	sc := cryptWorld()
	sc.Monsters = append(sc.Monsters, monsterAt("skeleton", refs.Monsters.Skeleton().String(), 2, 1))
	launchScene(t, mgr, sc)
	for i := range sessions.byID["sess"].NPCs {
		if sessions.byID["sess"].NPCs[i].ID == "skeleton" {
			sessions.byID["sess"].NPCs[i].HitPoints = 0
		}
	}

	declaration := activationSelector(t, mgr, "alice", refs.Features.Rage().String())
	persistedWorldBefore, err := encounters.GetEncounter(ctx, testSession)
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
		Failed:  []string{"character:alice", "encounter:sess"},
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

	persistedWorldAfter, getErr := encounters.GetEncounter(ctx, testSession)
	require.NoError(t, getErr)
	require.Equal(t, persistedWorldBefore, persistedWorldAfter,
		"activation, result, down, and dissolve beats remain only on the discarded scope")
	persistedSessionAfter, getErr := sessions.GetSession(ctx, "sess")
	require.NoError(t, getErr)
	require.Equal(t, persistedSessionBefore, persistedSessionAfter,
		"the dissolved in-memory fight must not reach the persisted session")
	require.Empty(t, stream.published, "nothing unpersisted is delivered")
}
