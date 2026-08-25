// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// offers_test.go pins Task 6's compiled-offer projection: one Declaration
// per compiled verb carrying a selector ID, a fixed target kind, and — for
// Attack — the full candidate universe with per-candidate availability. The
// four named helpers keep assertions off declaration ordering beyond the
// documented deterministic verb sort.

// candidateFight builds a turn-clock fight with alice active and two live
// skeletons in sight: skeleton-near at (2,1), one cell from alice's longsword
// reach, and skeleton-far at (7,1), six cells away and so out of reach but
// still within encEveryoneSees' unbounded sight. The caller mutates the
// persisted encounter data through the returned repositories to inject the
// stale and self holdings runAffordCandidateFixture adds on top.
func candidateFight(t *testing.T) (*session.Manager, *fakeSessions, *fakeEncounters, *fakeCharacters, *sequenceDice) {
	t.Helper()
	alice := armedFighter("alice")
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := newFakeCharacters(alice)

	// Enough scripted rolls for the fight's initiative and nothing else:
	// Afford is a read and rolls nothing.
	roller := &sequenceDice{rolls: []int{10, 1, 1, 1, 1, 1, 1, 1, 1, 1}}
	mgr, err := session.NewManager(&session.Config{
		Dice: roller, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: characters, Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field:    encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	require.NoError(t, err)

	// skeleton-near lands one cell from alice and forms the fight, putting
	// alice on the turn clock as the active member.
	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton-near", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	require.NoError(t, err)
	require.NotNil(t, spawned.Formed, "skeleton-near in plain sight of alice must start a fight")

	// skeleton-far is well outside a longsword's one-cell reach but inside
	// encEveryoneSees' unbounded sight, so it is a LIVE candidate that fails
	// the reach gate rather than a stale memory.
	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton-far", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 7, Y: 1},
	})
	require.NoError(t, err)

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, turn.Clock, "alice is on the turn clock")
	require.Equal(t, 1, turn.Round)
	return mgr, sessions, encounters, characters, roller
}

// injectHolding adds one holding for alice as observer, with the given subject
// and current-via channels. It round-trips the persisted encounter data
// through the fake repository so the next Afford reloads the mutated intel.
func injectHolding(
	t *testing.T, encounters *fakeEncounters, subject string, currentVia []intel.Channel,
) {
	t.Helper()
	stored, err := encounters.GetEncounter(context.Background(), "world")
	require.NoError(t, err)
	if stored.Intel.Holdings == nil {
		stored.Intel.Holdings = map[core.EntityID]map[intel.Subject]intel.HoldingData{}
	}
	obs := core.EntityID("alice")
	if stored.Intel.Holdings[obs] == nil {
		stored.Intel.Holdings[obs] = map[intel.Subject]intel.HoldingData{}
	}
	stored.Intel.Holdings[obs][intel.Subject(subject)] = intel.HoldingData{
		Channel:    intel.Sight,
		CurrentVia: currentVia,
		Payload:    nil,
	}
	require.NoError(t, encounters.SaveEncounter(context.Background(), "world", stored))
}

// runAffordCandidateFixture builds the candidate universe fixture: in-range
// (skeleton-near), out-of-range (skeleton-far), a stale memory
// (skeleton-stale, CurrentVia empty) and a self holding (alice observing
// herself). Only the two live non-self candidates should survive the
// projection, sorted by member ID.
func runAffordCandidateFixture(t *testing.T) *session.AffordOutput {
	t.Helper()
	mgr, _, encounters, _, _ := candidateFight(t)
	// A stale memory: a holding the intel log retains with no live channel.
	// skeleton-stale is not a roster member, but the CurrentVia-empty filter
	// excludes it before the position lookup, so the missing position is
	// never an inconsistency this fixture raises.
	injectHolding(t, encounters, "skeleton-stale", nil)
	// A self holding: alice observing herself. The subject==member filter
	// excludes the actor from the candidate universe even when a holding
	// exists.
	injectHolding(t, encounters, "alice", []intel.Channel{intel.Sight})

	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	return out
}

// requireSingleAttackDeclaration returns the one Attack declaration in decls,
// failing loudly if there is not exactly one. Declarations are sorted by the
// seam's documented verb rank (Attack, Move, EndTurn), but this helper finds
// the Attack by verb rather than by index so it never couples to ordering
// beyond that sort.
func requireSingleAttackDeclaration(t *testing.T, decls []session.Declaration) session.Declaration {
	t.Helper()
	var attack *session.Declaration
	for i := range decls {
		if decls[i].Verb == session.VerbAttack {
			if attack != nil {
				t.Fatalf("found more than one Attack declaration")
			}
			attack = &decls[i]
		}
	}
	if attack == nil {
		t.Fatalf("expected exactly one Attack declaration, got none")
	}
	return *attack
}

// requireSingleDeclaration returns the one declaration with the given verb.
func requireSingleDeclaration(t *testing.T, decls []session.Declaration, verb session.Verb) session.Declaration {
	t.Helper()
	for i := range decls {
		if decls[i].Verb == verb {
			return decls[i]
		}
	}
	t.Fatalf("expected a %s declaration, got none", verb)
	return session.Declaration{}
}

// candidateIDs extracts the member IDs from a candidate slice in order, for
// assertions that do not repeat each candidate's availability.
func candidateIDs(candidates []session.TargetCandidate) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, c.Member)
	}
	return out
}

// TestAffordReturnsOneAttackOfferWithEveryLiveCandidate is the brief's
// load-bearing pin: one Attack declaration carries the full candidate
// universe — in-range and out-of-range live candidates, sorted by member ID
// — and excludes stale memories and the actor. The out-of-range candidate
// keeps its row with a target-specific ShortfallTargetOutOfReach; the
// declaration-level Why stays nil because at least one candidate is in reach.
func TestAffordReturnsOneAttackOfferWithEveryLiveCandidate(t *testing.T) {
	out := runAffordCandidateFixture(t)
	attack := requireSingleAttackDeclaration(t, out.Declarations)

	require.Equal(t, "dnd5e:weapons:longsword", attack.Attack.Ref,
		"the compiled Attack carries the full definition ref")
	require.Equal(t, "Longsword", attack.Attack.Name)
	require.Equal(t, session.TargetMember, attack.TargetKind)
	require.Equal(t, []string{"skeleton-far", "skeleton-near"}, candidateIDs(attack.Candidates),
		"candidates are the live non-self universe, sorted by member ID")

	// skeleton-far sorts before skeleton-near lexicographically.
	require.False(t, attack.Candidates[0].Available, "skeleton-far is out of reach")
	require.NotNil(t, attack.Candidates[0].Why)
	require.Equal(t, session.ShortfallTargetOutOfReach, attack.Candidates[0].Why.Reason)

	require.True(t, attack.Candidates[1].Available, "skeleton-near is in reach")
	require.Nil(t, attack.Candidates[1].Why)

	require.True(t, attack.Available, "budget passes and one candidate is in reach")
	require.Nil(t, attack.Why)
	require.NotEmpty(t, attack.ID, "a compiled Attack carries a selector ID")
	require.Equal(t, session.SlotAction, attack.Slot)
}

// TestAffordProjectsThreeCompiledDeclarationsOnTheTurnClock pins the shape
// of a full compiled turn: Attack, Move and EndTurn each carry a selector
// ID and the fixed target kind, Move carries Remaining, EndTurn carries no
// candidates, and the declarations arrive in the documented verb order.
func TestAffordProjectsThreeCompiledDeclarationsOnTheTurnClock(t *testing.T) {
	mgr, _, _, _, _ := candidateFight(t)
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)

	require.Equal(t, session.ClockTurn, out.Clock)
	require.Len(t, out.Declarations, 3, "Attack, Move and EndTurn")

	// Documented verb order: Attack, Move, EndTurn.
	require.Equal(t, session.VerbAttack, out.Declarations[0].Verb)
	require.Equal(t, session.VerbMove, out.Declarations[1].Verb)
	require.Equal(t, session.VerbEndTurn, out.Declarations[2].Verb)

	attack := out.Declarations[0]
	require.NotEmpty(t, attack.ID, "compiled Attack has a selector ID")
	require.NotNil(t, attack.Attack)
	require.Equal(t, session.TargetMember, attack.TargetKind)
	require.NotEmpty(t, attack.Candidates)

	move := out.Declarations[1]
	require.NotEmpty(t, move.ID, "compiled Move has a selector ID")
	require.Nil(t, move.Attack)
	require.Equal(t, session.TargetPath, move.TargetKind)
	require.NotNil(t, move.Remaining, "Move carries remaining feet")
	require.Empty(t, move.Candidates, "Move carries no candidates")

	endTurn := out.Declarations[2]
	require.NotEmpty(t, endTurn.ID, "compiled EndTurn has a selector ID")
	require.Nil(t, endTurn.Attack)
	require.Equal(t, session.TargetNone, endTurn.TargetKind)
	require.True(t, endTurn.Available, "EndTurn follows the clock alone")
	require.Empty(t, endTurn.Candidates)
}

// TestNotYourTurnBlocksAllThreeVerbs pins the cheap, sheet-free blocker:
// every verb is blocked with the same NotYourTurn reason, an empty selector
// ID, no AttackRef, an empty candidate slice, and the fixed target kind.
func TestNotYourTurnBlocksAllThreeVerbs(t *testing.T) {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	require.NoError(t, err)

	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	require.NoError(t, err)

	out, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "bob"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, out.Clock)
	require.Len(t, out.Declarations, 3)

	for _, d := range out.Declarations {
		require.False(t, d.Available)
		require.Empty(t, d.ID, "a blocker carries no selector ID")
		require.Nil(t, d.Attack, "a blocker carries no AttackRef")
		require.Empty(t, d.Candidates, "a blocker carries no candidates")
		require.NotNil(t, d.Why)
		require.Equal(t, session.ShortfallNotYourTurn, d.Why.Reason)
	}
	require.Equal(t, session.TargetMember, out.Declarations[0].TargetKind)
	require.Equal(t, session.TargetPath, out.Declarations[1].TargetKind)
	require.Equal(t, session.TargetNone, out.Declarations[2].TargetKind)
}

// TestDownedBlocksAttackAndMoveButNotEndTurn pins the per-verb blocker
// matrix for Downed: Attack and Move are blocked with the Downed reason,
// while EndTurn follows the clock alone and stays compiled with a selector
// ID. A downed active member is an edge case — the standing seam splices a
// downed member out of the turn order, so NotYourTurn already covers a
// downed bystander. This fixture reproduces the edge case by forming the
// fight first (alice active on the turn clock) and only then dropping her
// stored sheet to zero hit points, so the encounter's clock still names her
// active while the session's sheet-based standing seam reads her downed.
func TestDownedBlocksAttackAndMoveButNotEndTurn(t *testing.T) {
	alice := armedFighter("alice")
	mgr, _, _, characters := aFight(t, alice, []int{1, 1})

	// Drop alice's stored sheet to zero AFTER the fight formed, so the
	// encounter's clock still names her active while the standing seam reads
	// her downed.
	characters.byID["alice"].HitPoints = 0

	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, out.Clock)
	require.Len(t, out.Declarations, 3)

	attack := requireSingleDeclaration(t, out.Declarations, session.VerbAttack)
	require.False(t, attack.Available)
	require.Empty(t, attack.ID)
	require.Nil(t, attack.Attack)
	require.Empty(t, attack.Candidates)
	require.Equal(t, session.TargetMember, attack.TargetKind)
	require.NotNil(t, attack.Why)
	require.Equal(t, session.ShortfallDowned, attack.Why.Reason)

	move := requireSingleDeclaration(t, out.Declarations, session.VerbMove)
	require.False(t, move.Available)
	require.Empty(t, move.ID)
	require.Empty(t, move.Candidates)
	require.Equal(t, session.TargetPath, move.TargetKind)
	require.NotNil(t, move.Why)
	require.Equal(t, session.ShortfallDowned, move.Why.Reason)

	endTurn := requireSingleDeclaration(t, out.Declarations, session.VerbEndTurn)
	require.True(t, endTurn.Available, "EndTurn follows the clock alone, even downed")
	require.NotEmpty(t, endTurn.ID, "EndTurn is still compiled with a selector ID")
	require.Equal(t, session.TargetNone, endTurn.TargetKind)
	require.Empty(t, endTurn.Candidates)
}

// TestBadAttackCompilationBlocksAttackOnly pins the per-verb blocker matrix
// for an unreadable Attack: a sheet that loads fine but names a weapon the
// catalog cannot resolve blocks Attack alone, while Move and EndTurn
// continue off the readied sheet and the clock.
func TestBadAttackCompilationBlocksAttackOnly(t *testing.T) {
	alice := armedFighter("alice")
	// A shield is valid equipment (so the sheet loads) but not a weapon, so
	// AssembleAttack fails with "holds no weapon" where Move and EndTurn do
	// not — the per-verb blocker matrix for a bad Attack compilation.
	alice.EquipmentSlots[character.SlotMainHand] = "shield"
	alice.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeArmor, ID: "shield", Quantity: 1},
	}

	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := newFakeCharacters(alice)
	mgr, err := session.NewManager(&session.Config{
		Dice: &sequenceDice{rolls: []int{10, 1, 1, 1, 1, 1, 1, 1, 1, 1}}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: characters, Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	require.NoError(t, err)
	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton-near", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	require.NoError(t, err)

	out, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, out.Clock)
	require.Len(t, out.Declarations, 3)

	attack := requireSingleDeclaration(t, out.Declarations, session.VerbAttack)
	require.False(t, attack.Available)
	require.Empty(t, attack.ID, "a blocked Attack carries no selector ID")
	require.Nil(t, attack.Attack)
	require.Empty(t, attack.Candidates)
	require.Equal(t, session.TargetMember, attack.TargetKind)
	require.NotNil(t, attack.Why)
	require.Equal(t, session.ShortfallUnreadable, attack.Why.Reason)

	move := requireSingleDeclaration(t, out.Declarations, session.VerbMove)
	require.True(t, move.Available, "Move continues off the readied sheet")
	require.NotEmpty(t, move.ID, "Move is still compiled with a selector ID")
	require.Equal(t, session.TargetPath, move.TargetKind)
	require.NotNil(t, move.Remaining)

	endTurn := requireSingleDeclaration(t, out.Declarations, session.VerbEndTurn)
	require.True(t, endTurn.Available)
	require.NotEmpty(t, endTurn.ID)
}

// TestUnreadableCharacterBlocksAttackAndMoveButNotEndTurn pins the per-verb
// blocker matrix for an unreadable character: a member whose sheet the
// repository does not hold blocks Attack and Move — there is no sheet to
// compile a swing or read movement from — while EndTurn, governed by the
// clock alone, stays compiled with a selector ID.
func TestUnreadableCharacterBlocksAttackAndMoveButNotEndTurn(t *testing.T) {
	// alice is armed and in the repository; bob is a player member the
	// repository does not hold, so loading bob's sheet fails with
	// ErrNoCharacter once bob is the active member.
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := newFakeCharacters(armedFighter("alice"))

	mgr, err := session.NewManager(&session.Config{
		Dice: &sequenceDice{rolls: []int{10, 1, 1, 1, 1, 1, 1, 1, 1, 1}}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters, Characters: characters, Events: session.DiscardEvents{},
	})
	require.NoError(t, err)

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	require.NoError(t, err)
	_, err = mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton-near", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	require.NoError(t, err)

	// End alice's turn: the skeleton's turn is driven through (Pass driver)
	// and bob — a player — becomes active.
	_, err = mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(t, mgr, "sess", "alice"),
	})
	require.NoError(t, err)

	out, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "bob"})
	require.NoError(t, err)
	require.Equal(t, session.ClockTurn, out.Clock)
	require.Len(t, out.Declarations, 3)

	attack := requireSingleDeclaration(t, out.Declarations, session.VerbAttack)
	require.False(t, attack.Available)
	require.Empty(t, attack.ID)
	require.Nil(t, attack.Attack)
	require.Empty(t, attack.Candidates)
	require.Equal(t, session.TargetMember, attack.TargetKind)
	require.NotNil(t, attack.Why)
	require.Equal(t, session.ShortfallUnreadable, attack.Why.Reason)

	move := requireSingleDeclaration(t, out.Declarations, session.VerbMove)
	require.False(t, move.Available)
	require.Empty(t, move.ID)
	require.Empty(t, move.Candidates)
	require.Equal(t, session.TargetPath, move.TargetKind)
	require.NotNil(t, move.Why)
	require.Equal(t, session.ShortfallUnreadable, move.Why.Reason)

	endTurn := requireSingleDeclaration(t, out.Declarations, session.VerbEndTurn)
	require.True(t, endTurn.Available, "EndTurn follows the clock alone")
	require.NotEmpty(t, endTurn.ID, "EndTurn is still compiled with a selector ID")
	require.Equal(t, session.TargetNone, endTurn.TargetKind)

	ended, err := mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: "bob", DeclarationID: endTurn.ID,
	})
	require.NoError(t, err, "EndTurn execution has no sheet, standing, or economy gate")
	require.NotNil(t, ended)
}

// TestLiveCandidateMissingPositionFailsClosed pins the fail-closed law: a
// live holding (CurrentVia non-empty) whose subject the roster no longer
// places is an internal inconsistency Afford surfaces as an error rather
// than silently omitting the candidate.
func TestLiveCandidateMissingPositionFailsClosed(t *testing.T) {
	mgr, _, encounters, _, _ := candidateFight(t)
	// A live holding for a subject that is NOT in the roster: the CurrentVia
	// filter keeps it in the candidate universe, but the roster has no
	// position for it, so Afford must fail closed.
	injectHolding(t, encounters, "ghost", []intel.Channel{intel.Sight})

	_, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.Error(t, err, "a live candidate with no position must fail rather than be omitted")
}

// TestNoTargetInReachKeepsCandidateRowsWithNoTargetInReach pins the
// declaration-level precedence: when the budget passes but no candidate is
// in reach, the Attack declaration's Why is NoTargetInReach while the
// candidate rows remain, each carrying its own target-specific verdict.
// TestAttackSelectorGatesFailBeforeDiceOrDurableMutation pins the execution
// trust boundary: the client may echo only the current available Attack offer.
// Unknown IDs, IDs belonging to another verb, and unavailable candidates all
// fail as stale without rolling or changing a repository-backed fact.
func TestAttackSelectorGatesFailBeforeDiceOrDurableMutation(t *testing.T) {
	mgr, sessions, encounters, characters, roller := candidateFight(t)
	ctx := context.Background()

	afford, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	attack := requireSingleDeclaration(t, afford.Declarations, session.VerbAttack)
	move := requireSingleDeclaration(t, afford.Declarations, session.VerbMove)

	beforeRolls := roller.next
	beforeSession, err := json.Marshal(sessions.byID["sess"])
	require.NoError(t, err)
	beforeEncounter, err := json.Marshal(encounters.byID["world"])
	require.NoError(t, err)
	beforeCharacter := cloneCharacter(characters.byID["alice"])
	beforeStory, err := mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)

	cases := []struct {
		name     string
		selector string
		target   string
	}{
		{name: "unknown selector", selector: "v1.stale", target: "skeleton-near"},
		{name: "selector belongs to Move", selector: move.ID, target: "skeleton-near"},
		{name: "candidate is currently unavailable", selector: attack.ID, target: "skeleton-far"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, attackErr := mgr.Attack(ctx, &session.AttackInput{
				Session: "sess", Attacker: "alice", Target: tc.target, DeclarationID: tc.selector,
			})
			require.ErrorIs(t, attackErr, session.ErrStaleDeclaration)
			require.Nil(t, out)
		})
	}

	require.Equal(t, beforeRolls, roller.next, "selector refusals roll no dice")
	afterSession, err := json.Marshal(sessions.byID["sess"])
	require.NoError(t, err)
	afterEncounter, err := json.Marshal(encounters.byID["world"])
	require.NoError(t, err)
	require.JSONEq(t, string(beforeSession), string(afterSession), "selector refusals write no session state")
	require.JSONEq(t, string(beforeEncounter), string(afterEncounter), "selector refusals record no story")
	require.Equal(t, beforeCharacter, characters.byID["alice"], "selector refusals spend nothing")
	afterStory, err := mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Len(t, afterStory, len(beforeStory))
}

// TestAttackRequiresADeclarationID pins missing selector as invalid input,
// distinct from a current-world stale selector.
func TestAttackRequiresADeclarationID(t *testing.T) {
	mgr, _, _, _, _ := candidateFight(t)
	out, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton-near",
	})
	require.ErrorIs(t, err, session.ErrNoDeclarationID)
	require.Nil(t, out)
}

// TestCompiledAttackSelectorIncludesItsActualPrice proves the load-bearing
// selector material is the priced definition, not the costless weapon profile.
// Changing only the actor's level changes the first-swing SpendProfile while
// keeping session/member/verb/slot/weapon fixed, and therefore changes the ID.
// Executing the new selector then banks the priced level-5 second swing.
func TestCompiledAttackSelectorIncludesItsActualPrice(t *testing.T) {
	alice := armedFighter("alice")
	alice.Level = 3
	mgr, _, _, characters := aFight(t, alice, []int{1, 1, 1, 1})
	ctx := context.Background()

	before, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	level3 := requireSingleDeclaration(t, before.Declarations, session.VerbAttack)
	require.Equal(t, session.SlotAction, level3.Slot)

	characters.byID["alice"].Level = 5
	after, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	level5 := requireSingleDeclaration(t, after.Declarations, session.VerbAttack)
	require.Equal(t, session.SlotAction, level5.Slot, "slot stayed fixed; the price is what changed")
	require.NotEqual(t, level3.ID, level5.ID, "the actual SpendProfile participates in selector identity")

	_, err = mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: level5.ID,
	})
	require.NoError(t, err)

	banked, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	second := requireSingleDeclaration(t, banked.Declarations, session.VerbAttack)
	require.Equal(t, session.SlotNone, second.Slot, "the selected priced definition banked Extra Attack")
	require.True(t, second.Available)
	_, err = mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton", DeclarationID: second.ID,
	})
	require.NoError(t, err, "execution consumes the same priced variant Afford selected")
}

func TestNoTargetInReachKeepsCandidateRowsWithNoTargetInReach(t *testing.T) {
	alice := armedFighter("alice")
	mgr, _, _, _ := aFight(t, alice, []int{1, 1})

	// Walk alice five cells from her spawn (1,1), well past a longsword's
	// one-cell reach from the skeleton at (2,1).
	_, err := mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", DeclarationID: currentMoveID(t, mgr, "sess", "alice"),
		Path: []spatial.Position{{X: 1, Y: 2}, {X: 1, Y: 3}, {X: 1, Y: 4}, {X: 1, Y: 5}, {X: 1, Y: 6}},
	})
	require.NoError(t, err, "test fixture must be able to walk alice out of reach")

	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)

	attack := requireSingleAttackDeclaration(t, out.Declarations)
	require.False(t, attack.Available)
	require.NotNil(t, attack.Why)
	require.Equal(t, session.ShortfallNoTargetInReach, attack.Why.Reason)
	require.NotEmpty(t, attack.ID, "a compiled Attack keeps its selector ID even with no target in reach")
	require.NotNil(t, attack.Attack, "a compiled Attack keeps its AttackRef even with no target in reach")
	require.Len(t, attack.Candidates, 1, "the candidate row remains")
	require.False(t, attack.Candidates[0].Available)
	require.NotNil(t, attack.Candidates[0].Why)
	require.Equal(t, session.ShortfallTargetOutOfReach, attack.Candidates[0].Why.Reason)
}
