// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	proneID    = "goblin-on-the-floor"
	attackerID = "attacker-1"
)

// proneTestEntity implements core.Entity so a combatant can be placed in a room.
type proneTestEntity struct {
	id string
}

func (e *proneTestEntity) GetID() string            { return e.id }
func (e *proneTestEntity) GetType() core.EntityType { return "character" }

// ProneConditionSuite covers prone's two attack rules, the range split between
// them, and what happens when the range cannot be known.
type ProneConditionSuite struct {
	suite.Suite

	ctx  context.Context
	bus  events.EventBus
	room spatial.Room
}

func TestProneConditionSuite(t *testing.T) {
	suite.Run(t, new(ProneConditionSuite))
}

func (s *ProneConditionSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.room = spatial.NewBasicRoom(spatial.BasicRoomConfig{
		ID:   "test-room",
		Type: "dungeon",
		Grid: spatial.NewSquareGrid(spatial.SquareGridConfig{Width: 20, Height: 20}),
	})
}

func (s *ProneConditionSuite) place(id string, x, y float64) {
	s.Require().NoError(s.room.PlaceEntity(&proneTestEntity{id: id}, spatial.Position{X: x, Y: y}))
}

// applied returns a prone condition already on the bus.
func (s *ProneConditionSuite) applied() *conditions.ProneCondition {
	prone := conditions.NewProneCondition(proneID)
	s.Require().NoError(prone.Apply(s.ctx, s.bus))

	return prone
}

// resolveAttack runs one attack through the chain and returns the event as the
// modifiers left it. ctx is explicit because whether a room is installed is the
// variable half of these tests.
func (s *ProneConditionSuite) resolveAttack(ctx context.Context, attacker, target string) dnd5eEvents.AttackChainEvent {
	event := dnd5eEvents.AttackChainEvent{
		AttackerID: attacker,
		TargetID:   target,
		IsMelee:    true,
	}

	staged := events.NewStagedChain[dnd5eEvents.AttackChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.AttackChain.On(s.bus).PublishWithChain(ctx, event, staged)
	s.Require().NoError(err)

	final, err := modified.Execute(ctx, event)
	s.Require().NoError(err)

	return final
}

// withRoom is the context an attack resolves in when somebody knows where
// everyone is standing.
func (s *ProneConditionSuite) withRoom() context.Context {
	return gamectx.WithRoom(s.ctx, s.room)
}

func (s *ProneConditionSuite) TestApplyAndRemove() {
	prone := conditions.NewProneCondition(proneID)
	s.Assert().Equal(proneID, prone.CharacterID)
	s.Assert().False(prone.IsApplied())

	s.Require().NoError(prone.Apply(s.ctx, s.bus))
	s.Assert().True(prone.IsApplied())

	s.Assert().Error(prone.Apply(s.ctx, s.bus), "a second Apply would double every modifier")

	s.Require().NoError(prone.Remove(s.ctx, s.bus))
	s.Assert().False(prone.IsApplied())
}

// A failed Apply must leave nothing behind, so the condition can be applied to
// another bus afterwards.
func (s *ProneConditionSuite) TestApplyRejectsANilBus() {
	prone := conditions.NewProneCondition(proneID)

	s.Require().Error(prone.Apply(s.ctx, nil))
	s.Assert().False(prone.IsApplied(), "a refused Apply does not leave the condition holding a bus")
	s.Require().NoError(prone.Apply(s.ctx, s.bus), "and it can still be applied afterwards")
}

// Rule one: the creature on the floor attacks at disadvantage. No room needed —
// this half of prone is not about geometry at all.
func (s *ProneConditionSuite) TestProneAttackerRollsAtDisadvantage() {
	s.applied()

	final := s.resolveAttack(s.ctx, proneID, "someone-else")

	s.Require().Len(final.DisadvantageSources, 1)
	s.Assert().Equal(refs.Conditions.Prone(), final.DisadvantageSources[0].SourceRef)
	s.Assert().Equal(proneID, final.DisadvantageSources[0].SourceID)
	s.Assert().Empty(final.AdvantageSources)
}

// Rule two, near half: standing over a prone creature is advantage.
func (s *ProneConditionSuite) TestAttackerWithinFiveFeetGetsAdvantage() {
	s.place(proneID, 5, 5)
	s.place(attackerID, 5, 6) // one square away
	s.applied()

	final := s.resolveAttack(s.withRoom(), attackerID, proneID)

	s.Require().Len(final.AdvantageSources, 1)
	s.Assert().Equal(refs.Conditions.Prone(), final.AdvantageSources[0].SourceRef)
	s.Assert().Empty(final.DisadvantageSources)
}

// Rule two, far half — the direction that is easy to leave unimplemented,
// because forgetting it looks like "no modifier" rather than like a failure.
func (s *ProneConditionSuite) TestAttackerBeyondFiveFeetGetsDisadvantage() {
	s.place(proneID, 5, 5)
	s.place(attackerID, 5, 8) // three squares away
	s.applied()

	final := s.resolveAttack(s.withRoom(), attackerID, proneID)

	s.Require().Len(final.DisadvantageSources, 1)
	s.Assert().Equal(refs.Conditions.Prone(), final.DisadvantageSources[0].SourceRef)
	s.Assert().Empty(final.AdvantageSources)
}

// The boundary, from both sides, in one test: two squares away is out of reach,
// one square is in it. A range check written with the wrong comparison passes
// one of these and fails the other.
func (s *ProneConditionSuite) TestTheBoundaryIsOneSquare() {
	s.place(proneID, 5, 5)
	s.place(attackerID, 5, 7) // exactly two squares — 10 feet
	s.applied()

	final := s.resolveAttack(s.withRoom(), attackerID, proneID)

	s.Require().Len(final.DisadvantageSources, 1, "10 feet is beyond reach")
	s.Assert().Empty(final.AdvantageSources)
}

// Diagonals are one square on this grid (Chebyshev distance), so a diagonal
// attacker is within 5 feet. Measuring with Euclidean distance would make this
// ~1.41 and quietly move the attacker out of reach.
func (s *ProneConditionSuite) TestDiagonalAdjacencyIsWithinFiveFeet() {
	s.place(proneID, 5, 5)
	s.place(attackerID, 6, 6)
	s.applied()

	final := s.resolveAttack(s.withRoom(), attackerID, proneID)

	s.Require().Len(final.AdvantageSources, 1)
	s.Assert().Empty(final.DisadvantageSources)
}

// The documented gap: with no room installed, the range cannot be decided, so
// the target-side rule contributes nothing and the attack rolls straight.
// Erroring instead would abort the whole attack chain for every caller that has
// not installed a room — resolution, today, is one of them.
func (s *ProneConditionSuite) TestNoRoomLeavesTheTargetSideRuleUnapplied() {
	s.place(proneID, 5, 5)
	s.place(attackerID, 5, 6)
	s.applied()

	final := s.resolveAttack(s.ctx, attackerID, proneID) // no gamectx.WithRoom

	s.Assert().Empty(final.AdvantageSources)
	s.Assert().Empty(final.DisadvantageSources)
}

// Same gap, other cause: a room exists but somebody is not on the map. "Not
// within reach" and "nobody knows where they are" must not collapse into each
// other, or an unplaced attacker would roll at disadvantage on no evidence.
func (s *ProneConditionSuite) TestAnUnplacedAttackerLeavesTheRuleUnapplied() {
	s.place(proneID, 5, 5)
	s.applied()

	final := s.resolveAttack(s.withRoom(), attackerID, proneID)

	s.Assert().Empty(final.AdvantageSources)
	s.Assert().Empty(final.DisadvantageSources)
}

func (s *ProneConditionSuite) TestAnUnplacedProneCreatureLeavesTheRuleUnapplied() {
	s.place(attackerID, 5, 6)
	s.applied()

	final := s.resolveAttack(s.withRoom(), attackerID, proneID)

	s.Assert().Empty(final.AdvantageSources)
	s.Assert().Empty(final.DisadvantageSources)
}

// An attack between two other people is none of this condition's business.
func (s *ProneConditionSuite) TestAnUnrelatedAttackIsUntouched() {
	s.place(proneID, 5, 5)
	s.place(attackerID, 5, 6)
	s.applied()

	final := s.resolveAttack(s.withRoom(), attackerID, "third-party")

	s.Assert().Empty(final.AdvantageSources)
	s.Assert().Empty(final.DisadvantageSources)
}

// A removed condition stops modifying attacks. Without this, "Remove" could
// unsubscribe nothing and no other test would notice.
func (s *ProneConditionSuite) TestRemovedProneStopsModifyingAttacks() {
	s.place(proneID, 5, 5)
	s.place(attackerID, 5, 6)
	prone := s.applied()

	s.Require().NoError(prone.Remove(s.ctx, s.bus))
	final := s.resolveAttack(s.withRoom(), attackerID, proneID)

	s.Assert().Empty(final.AdvantageSources)
	s.Assert().Empty(final.DisadvantageSources)
}

// unsubscribeFailingBus is a real bus whose Unsubscribe always refuses, so a
// test can reach Remove's failure path — which no real bus in this repo takes,
// and which is exactly why it is worth pinning.
type unsubscribeFailingBus struct {
	events.EventBus
}

func (b *unsubscribeFailingBus) Unsubscribe(_ context.Context, _ string) error {
	return errUnsubscribeRefused
}

var errUnsubscribeRefused = errors.New("bus refused the unsubscribe")

// Remove with no bus falls back to the one Apply was given. Without the
// fallback this is a nil-interface panic, not a graceful no-op.
func (s *ProneConditionSuite) TestRemoveWithANilBusUsesTheAppliedOne() {
	s.place(proneID, 5, 5)
	s.place(attackerID, 5, 6)
	prone := s.applied()

	s.Require().NoError(prone.Remove(s.ctx, nil))
	s.Assert().False(prone.IsApplied())

	final := s.resolveAttack(s.withRoom(), attackerID, proneID)
	s.Assert().Empty(final.AdvantageSources, "the handler really is off the bus")
	s.Assert().Empty(final.DisadvantageSources)
}

// A Remove that could not unsubscribe leaves the condition applied rather than
// reporting a removal that did not happen. The handler is still live, and
// IsApplied still says so.
func (s *ProneConditionSuite) TestAFailedRemoveLeavesTheConditionApplied() {
	s.place(proneID, 5, 5)
	s.place(attackerID, 5, 6)
	prone := s.applied()

	err := prone.Remove(s.ctx, &unsubscribeFailingBus{EventBus: s.bus})

	s.Require().Error(err)
	s.Require().ErrorIs(err, errUnsubscribeRefused)
	s.Assert().True(prone.IsApplied(), "it is still on the bus, so it still says so")

	final := s.resolveAttack(s.withRoom(), attackerID, proneID)
	s.Require().Len(final.AdvantageSources, 1, "and the handler is indeed still live")

	// The retry, on a bus that cooperates, works.
	s.Require().NoError(prone.Remove(s.ctx, s.bus))
	s.Assert().False(prone.IsApplied())
}

// The persisted form round-trips through the loader that routes on its ref, and
// the condition that comes back is a working one rather than a shell.
func (s *ProneConditionSuite) TestRoundTripsThroughTheLoader() {
	raw, err := conditions.NewProneCondition(proneID).ToJSON()
	s.Require().NoError(err)

	var data conditions.ProneConditionData
	s.Require().NoError(json.Unmarshal(raw, &data))
	s.Require().Equal(refs.Conditions.Prone(), data.Ref, "the blob names the ref its loader routes on")
	s.Require().Equal(proneID, data.CharacterID)

	loaded, err := conditions.LoadJSON(raw)
	s.Require().NoError(err)

	reloaded, ok := loaded.(*conditions.ProneCondition)
	s.Require().True(ok, "the loader routes prone's ref to a ProneCondition")
	s.Require().Equal(proneID, reloaded.CharacterID)

	again, err := reloaded.ToJSON()
	s.Require().NoError(err)
	s.Require().JSONEq(string(raw), string(again))

	// And it still works: a loaded condition that subscribes nothing would
	// round-trip perfectly and change no roll.
	s.Require().NoError(reloaded.Apply(s.ctx, s.bus))
	final := s.resolveAttack(s.ctx, proneID, "someone-else")
	s.Require().Len(final.DisadvantageSources, 1)
}

// The other routing site: a host holding only the ref string can build one.
func (s *ProneConditionSuite) TestCreatesFromItsRef() {
	out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{
		Ref:         refs.Conditions.Prone().String(),
		CharacterID: proneID,
	})
	s.Require().NoError(err)

	prone, ok := out.Condition.(*conditions.ProneCondition)
	s.Require().True(ok)
	s.Assert().Equal(proneID, prone.CharacterID)
}

// recordingBus delegates to a real bus and notes which effect was mid-Apply
// when each subscription was made — the shape of the instrumented surface
// resolution owns, reduced to what this test needs.
type recordingBus struct {
	inner  events.EventBus
	ref    core.Ref
	byRef  map[core.Ref][]events.Topic
	asked  *[]core.Ref
	shared *recordingBus
}

func newRecordingBus() *recordingBus {
	b := &recordingBus{
		inner: events.NewEventBus(),
		byRef: make(map[core.Ref][]events.Topic),
		asked: &[]core.Ref{},
	}
	b.shared = b

	return b
}

func (b *recordingBus) Subscribe(ctx context.Context, topic events.Topic, handler any) (string, error) {
	id, err := b.inner.Subscribe(ctx, topic, handler)
	if err != nil {
		return "", err
	}
	b.shared.byRef[b.ref] = append(b.shared.byRef[b.ref], topic)

	return id, nil
}

func (b *recordingBus) Unsubscribe(ctx context.Context, id string) error {
	return b.inner.Unsubscribe(ctx, id)
}

func (b *recordingBus) Publish(ctx context.Context, topic events.Topic, event any) error {
	return b.inner.Publish(ctx, topic, event)
}

// ScopeToEffect implements dnd5eEvents.EffectScoper.
func (b *recordingBus) ScopeToEffect(ref core.Ref) events.EventBus {
	*b.shared.asked = append(*b.shared.asked, ref)

	return &recordingBus{inner: b.inner, ref: ref, shared: b.shared}
}

// Prone's destination is resolution, which applies every effect through a view
// of the bus stamped with that effect's ref. The condition has to work through
// that view, and its subscription has to land under its own name — otherwise a
// registration ledger would credit prone's hooks to nobody.
func (s *ProneConditionSuite) TestWorksThroughAScopedView() {
	root := newRecordingBus()
	proneRef := *refs.Conditions.Prone()

	scoped := dnd5eEvents.BusForEffect(root, proneRef)
	prone := conditions.NewProneCondition(proneID)
	s.Require().NoError(prone.Apply(s.ctx, scoped))

	s.Require().Equal([]core.Ref{proneRef}, *root.asked)
	s.Require().Contains(root.byRef[proneRef], events.Topic("dnd5e.combat.attack.chain"))

	// And it still modifies attacks published on the underlying bus.
	s.bus = root
	s.place(proneID, 5, 5)
	s.place(attackerID, 5, 6)

	final := s.resolveAttack(s.withRoom(), attackerID, proneID)
	s.Require().Len(final.AdvantageSources, 1)
	s.Assert().Equal(refs.Conditions.Prone(), final.AdvantageSources[0].SourceRef)
}
