// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ProjectionTestSuite proves the entry that lets a caller obey "folds live in
// resolution" without being handed a cast.
//
// The case it exists for is the one that was broken: a barbarian with Unarmored
// Defense whose AC is a DERIVED number. Read off the sheet it is 10; folded
// with its condition attached it is 15. Every assertion below states the number
// rather than comparing two runs of the same code, because a projection and a
// fold that are both wrong the same way agree perfectly.
type ProjectionTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestProjectionSuite(t *testing.T) {
	suite.Run(t, new(ProjectionTestSuite))
}

func (s *ProjectionTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// projectedHeroID is this suite's own, so a fixture change here cannot move a
// number in somebody else's suite.
const projectedHeroID = "projected-hero"

// barbarian is unarmoured, with DEX +2 and CON +3 — DELIBERATELY DIFFERENT, so
// a formula that reached for the wrong ability produces a different total
// instead of the same one by luck. Unarmored Defense makes the answer
// 10 + 2 + 3 = 15.
//
// ArmorClass on the sheet says 10 and is meant to: it is the stale scalar the
// old read path returned, so any test below that accidentally reads the sheet
// instead of folding reports 10 and says so out loud.
func (s *ProjectionTestSuite) barbarian(conds ...json.RawMessage) *character.Data {
	return &character.Data{
		ID:       projectedHeroID,
		PlayerID: "player-1",
		Name:     "Standre",
		Level:    1,
		ClassID:  classes.Barbarian,
		RaceID:   races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 14,
			abilities.CON: 16,
			abilities.INT: 10,
			abilities.WIS: 12,
			abilities.CHA: 8,
		},
		HitPoints:        14,
		MaxHitPoints:     14,
		ArmorClass:       10,
		ProficiencyBonus: 2,
		Conditions:       conds,
	}
}

func (s *ProjectionTestSuite) unarmoredDefense() json.RawMessage {
	raw, err := (&conditions.UnarmoredDefenseCondition{
		MemberID: projectedHeroID,
		Type:     conditions.UnarmoredDefenseBarbarian,
	}).ToJSON()
	s.Require().NoError(err)

	return raw
}

// TestTheProjectionFoldsTheConditionIn is the base-AC-barbarian case, stated as
// a value.
//
// 15, not "whatever the sheet said" and not "whatever another call returned":
// 10 base + 2 DEX + 3 CON, each of which appears as its own component. A
// projection that lost the condition would answer 12 and still look like a
// working fold, which is exactly how the original bug read.
//
// THE NUMBER NOW DEPENDS ON THE DOOR, and this comment used to say the
// opposite. It recorded an honest limit — Unarmored Defense read its own sheet
// through the owner handle character.Attach wired, so deleting the installTruth
// call left this whole suite green, and the door was held on this path only
// structurally by TestOnlyTheDoorInstallsGameContext.
//
// The reader migration that limit was waiting for has landed. Unarmored Defense
// reads itself out of the CAST, the cast is installed by the door, and so the
// 15 below is now downstream of installTruth: delete that call and this fails
// with 12. Measured, not assumed — TestTheDoorIsWhatMakesTheNumberRight pins
// exactly that, and it is the pin this comment promised would become possible.
func (s *ProjectionTestSuite) TestTheProjectionFoldsTheConditionIn() {
	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{
		Character: s.barbarian(s.unarmoredDefense()),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.ArmorClass)

	s.Require().Equal(15, out.ArmorClass.Total,
		"10 base + 2 DEX + 3 CON: the condition reached the fold")

	var fromFeature int
	for _, component := range out.ArmorClass.Components {
		if component.Type == combat.ACSourceFeature {
			fromFeature += component.Value
		}
	}
	s.Require().Equal(3, fromFeature,
		"and the +3 arrived as a feature component, not as a bigger base")
}

// TestTheProjectionBeatsAnAttachedFold was the parity half, and its baseline
// moved on purpose.
//
// It used to assert that both paths answered 15 for the same record — the
// projection, and the old load-attach-fold written out in full because it was
// the thing being replaced. Both were pinned independently so it could not pass
// by two wrongs agreeing.
//
// The old path now answers 12. Not a regression: an attached sheet folding on a
// bare context has no CAST, so Unarmored Defense cannot find itself and leaves
// the chain alone, and 10 + 2 DEX is what is left. That is R6 — a chain folded
// outside resolution is the bug rather than a mode — and it is the entire point
// of the phase, so the test that used to say "these agree" now says WHY THEY NO
// LONGER DO. A parity assertion kept here would have to be satisfied by
// breaking the thing that was just fixed.
//
// Both numbers stay pinned independently, for the original reason: 15 names the
// fold that had the truth, 12 names the fold that did not, and three points
// apart is far enough that the assertion says which one moved.
func (s *ProjectionTestSuite) TestTheProjectionBeatsAnAttachedFold() {
	direct, err := character.Load(s.ctx, s.barbarian(s.unarmoredDefense()))
	s.Require().NoError(err)
	s.Require().NoError(character.Attach(s.ctx, direct, events.NewEventBus()))

	outside, err := direct.EffectiveAC(s.ctx)
	s.Require().NoError(err,
		"a fold with no cast is degraded, not refused: the condition leaves the chain alone")
	s.Require().Equal(12, outside.Total,
		"10 base + 2 DEX. No cast, so Unarmored Defense contributes nothing")

	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{
		Character: s.barbarian(s.unarmoredDefense()),
	})
	s.Require().NoError(err)

	s.Require().Equal(15, out.ArmorClass.Total,
		"the entry installs the truth, so the condition can read itself: +3 CON")
	s.Require().Greater(out.ArmorClass.Total, outside.Total,
		"the door is worth exactly the contributors a cast-less fold silently drops")
}

// TestTheDoorIsWhatMakesTheNumberRight is the pin the honest-limit note on
// TestTheProjectionFoldsTheConditionIn promised would become possible.
//
// Before the reader migration, deleting installTruth from this entry changed no
// number anywhere — the fold depended on the ATTACH, and the door was held
// structurally or not at all. Now the cast IS the read channel, so the door is
// load-bearing for the value, and that is asserted here rather than described.
//
// It works by contrast rather than by reaching inside: the same record folded
// through the entry (door called) and through an attached sheet on a bare
// context (no door) differ by exactly the condition's contribution.
func (s *ProjectionTestSuite) TestTheDoorIsWhatMakesTheNumberRight() {
	record := s.barbarian(s.unarmoredDefense())

	withDoor, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: record})
	s.Require().NoError(err)

	direct, err := character.Load(s.ctx, s.barbarian(s.unarmoredDefense()))
	s.Require().NoError(err)
	s.Require().NoError(character.Attach(s.ctx, direct, events.NewEventBus()))
	withoutDoor, err := direct.EffectiveAC(s.ctx)
	s.Require().NoError(err)

	s.Require().Equal(3, withDoor.ArmorClass.Total-withoutDoor.Total,
		"the difference between folding with the truth installed and without it "+
			"is precisely Unarmored Defense's +3 CON")
}

// TestTheProjectionInstallsNoWorld is the M4 pin, and the answer to "how did
// you enter the door without a room".
//
// A join is not an encounter, so there is no world to install and none is
// invented. The door is still called unconditionally; what it installs is an
// ABSENT room, and absence is a value that states what the author meant:
// Room answers not-ok, RequireRoom answers ErrNoRoom, and a reader that needs
// the world fails closed with the reason readable where it failed.
//
// The alternative — an empty room, to keep the shape tidy — would answer "you
// are somewhere, and there is nothing there", which is false and unfalsifiable
// at the point a predicate reads it.
func (s *ProjectionTestSuite) TestTheProjectionInstallsNoWorld() {
	ctx := installTruth(s.ctx, nil, &Participants{
		characters: map[string]*character.Character{},
		monsters:   map[string]*monster.Monster{},
		order:      []string{},
	})

	room, ok := gamectx.Room(ctx)
	s.Require().False(ok, "no world was installed, and the read says so")
	s.Require().Nil(room)

	_, err := gamectx.RequireRoom(ctx)
	s.Require().ErrorIs(err, gamectx.ErrNoRoom,
		"and a reader that requires one fails closed, naming what is missing")

	// The other two tenants are present on the same context — absence is the
	// room's honest answer, not a projection that installs nothing.
	_, castOK := gamectx.CastOf(ctx)
	s.Require().True(castOK, "the cast is installed even when the world is not")
}

// TestTheProjectionLeavesNothingOnTheBus holds the teardown half.
//
// Asserted on the bus rather than on the teardown call, the same shape
// TestARefusedPaymentLeavesNothingOnTheBus uses: the pin survives a change to
// which mechanism does the work. Unarmored Defense joins the AC chain on the
// way in, and after the projection returns that chain answers nobody — a
// projection that leaked would keep folding a sheet its caller has forgotten.
func (s *ProjectionTestSuite) TestTheProjectionLeavesNothingOnTheBus() {
	inner := events.NewEventBus()

	out, err := projectCharacterOn(s.ctx, &ProjectCharacterInput{
		Character: s.barbarian(s.unarmoredDefense()),
	}, newSurface(inner))
	s.Require().NoError(err)
	s.Require().Equal(15, out.ArmorClass.Total, "it folded before it tore down")

	event := &combat.ACChainEvent{
		CharacterID: projectedHeroID,
		Breakdown:   &combat.ACBreakdown{Total: 0, Components: []combat.ACComponent{}},
	}
	chain := events.NewStagedChain[*combat.ACChainEvent](combat.ModifierStages)

	modified, err := combat.ACChain.On(inner).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)

	folded, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	s.Require().Empty(folded.Breakdown.Components,
		"Unarmored Defense attached during the projection and does not answer this chain")
}

// TestTheProjectionRefusesARecordItCannotName covers the validation the entry
// borrows rather than reinvents: a sheet with no ID cannot be read back out of
// the cast it was just put into, so it is refused on the way in.
func (s *ProjectionTestSuite) TestTheProjectionRefusesARecordItCannotName() {
	s.Run("nil input", func() {
		_, err := ProjectCharacter(s.ctx, nil)
		s.Require().ErrorIs(err, ErrNilInput)
	})

	s.Run("no character", func() {
		_, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{})
		s.Require().ErrorIs(err, ErrBadParticipant)
	})

	s.Run("no id", func() {
		nameless := s.barbarian()
		nameless.ID = ""

		_, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{Character: nameless})
		s.Require().ErrorIs(err, ErrBadParticipant)
	})
}

// world is a one-cell-apart hex field with this suite's hero on it, built only
// so the contrast below can run a real interaction through Resolve.
func (s *ProjectionTestSuite) world() encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{},
		Announcer: quietAnnouncer{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Field: encounter.FieldInput{
			Canvas:  hexCanvas(),
			Regions: []encounter.RegionInput{rectRegion("room-1", 0, 0, 10, 10)},
		},
		Members: []encounter.MemberInput{
			{ID: projectedHeroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc.ToData()
}

// TestTheProjectionReadsWhatResolveRefuses is the D10 contrast, and the reason
// the policy is an argument rather than a mode.
//
// ONE RECORD, carrying one condition that parses and one blob that does not.
// The two entries answer differently and both answers are right, because
// strictness is a property of what the entry DOES:
//
//   - Resolve hands back sheets to be persisted. A sheet loaded past a silently
//     dropped condition is a sheet that, written back, has had that condition
//     deleted by a verb that merely moved somebody (rpg-toolkit#948). So it
//     refuses, and names the blob.
//   - The projection only reads. Refusing there would put one unreadable blob
//     between a player and the game, and the character is no less playable for
//     it — so it folds what parsed. The drop is not silent: the loader warns,
//     which is D10's "fail loudly means observable, not refused".
//
// The same 15 as every other case in this suite, so the lenient path is doing
// the whole fold rather than limping to a plausible number.
func (s *ProjectionTestSuite) TestTheProjectionReadsWhatResolveRefuses() {
	unreadable := json.RawMessage(`{"ref":"nonsense","x":`)

	out, err := ProjectCharacter(s.ctx, &ProjectCharacterInput{
		Character: s.barbarian(s.unarmoredDefense(), unreadable),
	})
	s.Require().NoError(err,
		"a read entry does not put an unreadable blob between a player and the game")
	s.Require().Equal(15, out.ArmorClass.Total,
		"and folds every condition that did parse: 10 + 2 DEX + 3 CON")

	_, err = Resolve(s.ctx, &Input{
		World:        s.world(),
		Participants: []Participant{{Character: s.barbarian(s.unarmoredDefense(), unreadable)}},
		Machine:      &captureMachine{},
		Initiative:   orderAsGiven{}, TurnDriver: passDriver{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{},
		Roller: dice.NewRoller(),
	})
	s.Require().Error(err,
		"a write entry refuses rather than handing back a sheet with a condition deleted")
	s.Require().Contains(err.Error(), "nonsense",
		"and names the blob it could not read")
}

// TestTheDefaultAttachRefuses pins the shape Kirk asked for on #1289: the
// policy is a field whose ZERO VALUE is the answer that cannot destroy
// anything.
//
// It reaches past both entries on purpose. Resolve refusing and the projection
// dropping are pinned by the contrast above, and both of those pass whichever
// way round the default sits — each entry says what it wants. What no
// behavioural test covers is the entry NOT WRITTEN YET: somebody adds a third
// caller, does not read the field's comment, and gets whatever the zero value
// happens to mean. This asserts that what they get is the refusal.
func (s *ProjectionTestSuite) TestTheDefaultAttachRefuses() {
	surf := newSurface(events.NewEventBus())

	cast, err := attachAll(s.ctx, surf, &attachAllInput{
		Participants: []Participant{{
			Character: s.barbarian(s.unarmoredDefense(), json.RawMessage(`{"ref":"nonsense","x":`)),
		}},
		Roller: refusingRoller{},
		// DropUnreadable deliberately unset — this is the whole test.
	})

	s.Require().Error(err,
		"an attach that was asked for nothing gets the policy that cannot delete a condition")
	s.Require().Nil(cast)
	s.Require().Contains(err.Error(), "nonsense", "and names the blob it refused")

	s.Require().NoError(surf.teardown(s.ctx))
}
