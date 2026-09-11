// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The cast door, as a scene: a level-1 bard with two cantrips, a skeleton to
// point them at, and the panel a player actually reads.
//
// EVERY ASSERTION HERE IS ABOUT THE DOOR, not about what a cantrip does. What
// True Strike grants and what Vicious Mockery costs are the rulebook's and
// resolution's business, tested where they live; this suite pins that the row
// appears when it should, is priced once, is refused when the world moved, and
// that the beats a player reads reach the story.

type CastSuite struct {
	suite.Suite

	mgr        *session.Manager
	member     string
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	dice       *sequenceDice
}

func TestCastSuite(t *testing.T) {
	suite.Run(t, new(CastSuite))
}

// castingBard is levelOneBard with cantrips written onto the sheet — the sheet
// a finalized draft produces, written directly so this suite does not depend on
// the draft flow.
//
// CHA 16 and proficiency 2 make the spell save DC 13: 8 + 2 + 3.
func castingBard(id string, cantrips ...spells.Spell) *character.Data {
	bard := levelOneBard(id, 2)
	for _, cantrip := range cantrips {
		ref := refs.Spells.ByID(string(cantrip))
		if ref == nil {
			panic("the fixture named a spell the ref catalog does not have: " + cantrip)
		}
		bard.KnownCantrips = append(bard.KnownCantrips, ref.String())
	}
	return bard
}

// castingBardWithSpells extends the ordinary cantrip fixture with the
// finalized level-one spellbook and its provider-owned recoverable pool.
func castingBardWithSpells(id string, known ...spells.Spell) *character.Data {
	bard := castingBard(id)
	for _, spell := range known {
		ref := refs.Spells.ByID(string(spell))
		if ref == nil {
			panic("the fixture named a spell the ref catalog does not have: " + spell)
		}
		bard.KnownSpells = append(bard.KnownSpells, ref.String())
	}
	bard.Resources[resources.SpellSlotLevel1] = character.RecoverableResourceData{
		Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest,
	}
	return bard
}

// armForSwinging gives a sheet a longsword and the proficiency to use it, so
// it can open a post-roll window by swinging. Only the freeze scene needs it:
// a cast has nothing to do with what the caster is holding.
func armForSwinging(sheet *character.Data) {
	sheet.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponMartial}
	sheet.Inventory = []character.InventoryItemData{
		{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
	}
	sheet.EquipmentSlots = character.EquipmentSlots{
		character.SlotMainHand: string(weapons.Longsword),
	}
}

// bardSaveDC is what castingBard's own sheet answers: 8 + 2 proficiency + 3 for
// CHA 16. Spelled here because every save assertion below is against it, and a
// literal 13 in six places would hide the arithmetic that produced it.
const bardSaveDC = 13

// scene puts the bard in a fight with a skeleton `cells` away, with the dice
// scripted after initiative.
//
// The skeleton's distance is a parameter because RANGE IS PER SPELL: True
// Strike reaches 30 feet and Vicious Mockery 60, so one skeleton can be a
// candidate for one row and a shortfall on the other, which is the thing worth
// pinning.
func (s *CastSuite) scene(bard *character.Data, cells int, rolls ...int) {
	s.build(bard, nil, nil, cells, rolls...)
}

func (s *CastSuite) sceneWithAllies(
	bard *character.Data, allies []*character.Data, cells int, rolls ...int,
) {
	s.build(bard, allies, nil, cells, rolls...)
}

// sceneWithProps is the same fight with something drawn on the floor. Only the
// push needs it: everything a cast did before this slice happened to creatures,
// and the first thing that asks the MAP a question is a creature being shoved
// into it.
func (s *CastSuite) sceneWithProps(
	bard *character.Data, props []encounter.PropInput, cells int, rolls ...int,
) {
	s.build(bard, nil, props, cells, rolls...)
}

func (s *CastSuite) build(
	bard *character.Data, allies []*character.Data, props []encounter.PropInput,
	cells int, rolls ...int,
) {
	s.T().Helper()

	// One die per member of the bubble the spawn forms, ahead of whatever this
	// scene scripted.
	initiativeRolls := 2 + len(allies)
	scripted := append(make([]int, initiativeRolls), rolls...)

	sheets := append([]*character.Data{bard}, allies...)
	s.characters = newFakeCharacters(sheets...)
	s.member = bard.ID
	s.sessions = newFakeSessions()
	s.encounters = newFakeEncounters()
	s.stream = &fakeStream{}
	s.dice = &sequenceDice{rolls: scripted}
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: s.dice,
		TurnDriver: session.Pass{},
		Sessions:   s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	members := []encounter.MemberInput{{
		ID: encounter.MemberID(bard.ID), Kind: encounter.KindPlayer,
		Position: spatial.Position{X: 1, Y: 1},
	}}
	for i, ally := range allies {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(ally.ID), Kind: encounter.KindPlayer,
			Position: spatial.Position{X: float64(2 + i), Y: 1},
		})
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 40, 8)},
			Props:   props,
		},
		Members:   members,
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: &data,
	})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skeleton", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: float64(1 + cells), Y: 1},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "arriving in plain sight must start a fight")

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: bard.ID})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock, "a cast is priced on the turn clock")
	s.Require().Equal(bard.ID, turn.Active)
	s.stream.published = nil
}

// holdInspiration puts a Bardic Inspiration die on the member's stored sheet.
// The grant itself is exercised elsewhere; the freeze scene starts from a
// member who already holds one, so a change to the grant cannot quietly
// rewrite what this scene is testing.
func (s *CastSuite) holdInspiration(member string) {
	s.T().Helper()
	ctx := context.Background()
	stored, err := s.characters.GetCharacter(ctx, member)
	s.Require().NoError(err)
	raw, err := conditions.NewInspiredCondition(member, member, conditions.InspiredDie).ToJSON()
	s.Require().NoError(err)
	stored.Conditions = append(stored.Conditions, raw)
	s.Require().NoError(s.characters.SaveCharacter(ctx, stored))
}

// castRows is every VerbCast declaration the bard is currently offered.
func (s *CastSuite) castRows() []session.Declaration {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{
		Session: "sess", Member: s.member,
	})
	s.Require().NoError(err)
	rows := make([]session.Declaration, 0, 2)
	for _, declaration := range out.Declarations {
		if declaration.Verb == session.VerbCast {
			rows = append(rows, declaration)
		}
	}
	return rows
}

// castRow is the one row for a named spell.
func (s *CastSuite) castRow(spell spells.Spell) session.Declaration {
	s.T().Helper()
	want := refs.Spells.ByID(string(spell)).String()
	for _, row := range s.castRows() {
		if row.Spell != nil && row.Spell.Ref == want {
			return row
		}
	}
	s.Require().Failf("no row", "the bard was offered no Cast row for %s", spell)
	return session.Declaration{}
}

// candidateFor finds one member's row in a declaration's candidate universe.
func castCandidate(
	t *testing.T, row session.Declaration, member string,
) (session.TargetCandidate, bool) {
	t.Helper()
	for _, candidate := range row.Candidates {
		if candidate.Member == member {
			return candidate, true
		}
	}
	return session.TargetCandidate{}, false
}

func (s *CastSuite) TestUnsupportedKnownSpellMintsNoCastRow() {
	s.scene(castingBardWithSpells("bard", spells.Bless), 2)
	s.Empty(s.castRows(), "knowledge and executable rulebook content must intersect")
}

func (s *CastSuite) TestBaneKnownSpellCompilesProviderBoundsAndGenericPrice() {
	s.scene(castingBardWithSpells("bard", spells.Bane), 2)

	row := s.castRow(spells.Bane)
	s.Equal(1, row.MinTargets)
	s.Equal(3, row.MaxTargets)
	s.Equal(session.TargetMember, row.TargetKind)
	s.Equal([]session.CostComponent{
		{Currency: session.CurrencyAction, Needed: 1},
		{Currency: session.CurrencyCharges, Needed: 1, Label: "1st-level Spell Slots"},
	}, row.Cost)
	s.True(row.Available)
}

func (s *CastSuite) TestBaneWithoutASpellSlotReportsProviderLabelledChargeShortfall() {
	bard := castingBardWithSpells("bard", spells.Bane)
	spent := bard.Resources[resources.SpellSlotLevel1]
	spent.Current = 0
	bard.Resources[resources.SpellSlotLevel1] = spent
	s.scene(bard, 2)

	row := s.castRow(spells.Bane)
	s.False(row.Available)
	s.Require().NotNil(row.Why)
	s.Equal(session.CurrencyCharges, row.Why.Currency)
	s.Equal(1, row.Why.Needed)
	s.Equal(0, row.Why.Left)
	s.Equal("1st-level Spell Slots: 1 needed, 0 left", row.Why.Text)
}

func (s *CastSuite) TestCanonicalTargetsCastBaneAndLegacyConflictIsRejectedBeforeRolling() {
	s.scene(castingBardWithSpells("bard", spells.Bane), 2, 5)
	row := s.castRow(spells.Bane)
	targets := []string{"skeleton"}

	out, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: row.ID, Targets: targets,
	})
	s.Require().NoError(err)
	s.Equal(refs.Spells.Bane().String(), out.Spell.Ref)
	s.Equal([]string{"skeleton"}, targets, "entry normalization never mutates caller storage")

	castEvents := s.beats(session.EventCast)
	s.Require().Len(castEvents, 1)
	castBody, ok := castEvents[0].Body.(session.CastBody)
	s.Require().True(ok)
	s.Equal([]string{"skeleton"}, castBody.Targets)
	s.Equal("skeleton", castBody.Target, "one target retains the deprecated projection")

	savedEvents := s.beats(session.EventSaved)
	s.Require().Len(savedEvents, 1)
	savedBody, ok := savedEvents[0].Body.(session.SavedBody)
	s.Require().True(ok)
	s.Require().NotNil(savedBody.Calculation)
	s.Equal(savedBody.Total, savedBody.Calculation.Total)

	results := s.beats(session.EventActivationResult)
	s.Require().Len(results, 1)
	applied, ok := results[0].Body.(session.ActivationResultBody)
	s.Require().True(ok)
	s.Require().NotNil(applied.ConditionApplied)
	s.Equal("bard", applied.ConditionApplied.SourceID)

	before := s.dice.next
	_, err = s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: row.ID,
		Target: "skeleton", Targets: []string{"skeleton"},
	})
	s.ErrorIs(err, session.ErrBadCast)
	s.Equal(before, s.dice.next, "conflicting aliases are rejected before RNG")
}

func (s *CastSuite) TestBaneRefusesTargetCountsOutsideProviderBoundsWithoutSpending() {
	s.scene(castingBardWithSpells("bard", spells.Bane), 2)
	row := s.castRow(spells.Bane)
	for _, targets := range [][]string{nil, {"a", "b", "c", "d"}} {
		beforeRolls := s.dice.next
		beforePool := s.characters.byID["bard"].Resources[resources.SpellSlotLevel1].Current
		_, err := s.mgr.Cast(context.Background(), &session.CastInput{
			Session: "sess", Member: "bard", DeclarationID: row.ID, Targets: targets,
		})
		s.ErrorIs(err, session.ErrBadCast)
		s.Equal(beforeRolls, s.dice.next)
		s.Equal(beforePool, s.characters.byID["bard"].Resources[resources.SpellSlotLevel1].Current)
	}
}

func (s *CastSuite) TestBaneResolvesOneOrderedThreeTargetCast() {
	allyA := armedFighter("fighter-a")
	allyB := armedFighter("fighter-b")
	s.sceneWithAllies(castingBardWithSpells("bard", spells.Bane),
		[]*character.Data{allyA, allyB}, 4, 5, 5, 5)

	targets := []string{"fighter-b", "skeleton", "fighter-a"}
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: s.castRow(spells.Bane).ID,
		Targets: targets,
	})
	s.Require().NoError(err)

	casts := s.beats(session.EventCast)
	s.Require().Len(casts, 1)
	body, ok := casts[0].Body.(session.CastBody)
	s.Require().True(ok)
	s.Equal(targets, body.Targets)
	s.Empty(body.Target, "multi-target casts never invent a deprecated representative")

	saves := s.beats(session.EventSaved)
	s.Require().Len(saves, 3)
	for i, event := range saves {
		saved, ok := event.Body.(session.SavedBody)
		s.Require().True(ok)
		s.Equal(targets[i], saved.Saver, "per-target trains retain caller order")
	}
	s.Require().Len(s.beats(session.EventActivationResult), 3)

	stored := s.characters.byID["bard"]
	s.Equal(1, stored.Resources[resources.SpellSlotLevel1].Current,
		"the whole three-target cast spends one slot")

	refreshed := s.castRow(spells.Bane)
	s.False(refreshed.Available, "the spent action closes the same-turn cast door")
	s.Require().NotNil(refreshed.Why)
	s.Equal(session.CurrencyAction, refreshed.Why.Currency)
	beforeRolls := s.dice.next
	_, err = s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: refreshed.ID,
		Targets: []string{"fighter-a"},
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.Equal(beforeRolls, s.dice.next)
	s.Equal(1, s.characters.byID["bard"].Resources[resources.SpellSlotLevel1].Current,
		"a refused same-turn second cast spends neither RNG nor the remaining slot")
}

func (s *CastSuite) TestBaneAffectsTheTargetsNextAttackWithSourceQualifiedSubtraction() {
	fighter := armedFighter("fighter")
	s.sceneWithAllies(castingBardWithSpells("bard", spells.Bane), []*character.Data{fighter}, 2,
		5, 15, 3, 4)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: s.castRow(spells.Bane).ID,
		Targets: []string{"fighter"},
	})
	s.Require().NoError(err)

	_, err = s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "bard",
		DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", "bard"),
	})
	s.Require().NoError(err)

	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: "sess", Member: "fighter"})
	s.Require().NoError(err)
	s.Require().Equal("fighter", turn.Active)
	row := currentDeclaration(s.T(), s.mgr, "sess", "fighter", session.VerbAttack)
	s.Require().True(row.Available, "attack row refusal: %#v", row.Why)
	target, found := castCandidate(s.T(), row, "skeleton")
	s.Require().True(found, "attack candidates: %#v", row.Candidates)
	s.Require().True(target.Available, "skeleton candidate refusal: %#v", target.Why)
	attack, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "fighter", Target: "skeleton", DeclarationID: row.ID,
	})
	s.Require().NoError(err)
	s.Equal(15, attack.Roll)
	s.Equal(17, attack.Total, "15 + 5 attack bonus - 3 Bane")
	s.True(attack.Hit)
	s.Equal(7, attack.Damage, "4 on the longsword die + 3 Strength")
	s.Require().NotNil(attack.Calculation)
	s.Equal(attack.Total, attack.Calculation.Total)

	var bane *session.RollComponent
	for i := range attack.Calculation.Components {
		component := &attack.Calculation.Components[i]
		if component.Source.Ref == refs.Spells.Bane().String() {
			bane = component
			break
		}
	}
	s.Require().NotNil(bane)
	s.True(bane.SubtractDice)
	s.Equal("bard", bane.Source.SourceID)
	s.Require().NotNil(bane.Dice)
	s.Equal(3, bane.Dice.Subtotal)

	outcomes := s.beats(session.EventStruck, session.EventMissed)
	s.Require().Len(outcomes, 1)
	switch body := outcomes[0].Body.(type) {
	case session.StruckBody:
		s.Require().NotNil(body.Calculation)
		s.Equal("bard", body.Calculation.Components[len(body.Calculation.Components)-1].Source.SourceID)
	case session.MissedBody:
		s.Require().NotNil(body.Calculation)
		s.Equal("bard", body.Calculation.Components[len(body.Calculation.Components)-1].Source.SourceID)
	default:
		s.FailNow("attack outcome has no typed body")
	}
}

// TestTwoCastableCantripsAreTwoRows keeps the existing cantrip panel contract:
// two known supported cantrips remain two separately selectable rows.
func (s *CastSuite) TestTwoCastableCantripsAreTwoRows() {
	s.scene(castingBard("bard", spells.TrueStrike, spells.ViciousMockery), 2)

	rows := s.castRows()
	s.Require().Len(rows, 2, "one row per castable cantrip, never one row for the verb")

	// Sorted by the spell's ref, which is a fact about the character rather
	// than about the order the sheet happened to hold the choices in.
	s.Equal(refs.Spells.TrueStrike().String(), rows[0].Spell.Ref)
	s.Equal(refs.Spells.ViciousMockery().String(), rows[1].Spell.Ref)
	s.Equal("True Strike", rows[0].Spell.Name)
	s.Equal("Vicious Mockery", rows[1].Spell.Name)

	for _, row := range rows {
		s.Equal(session.SlotAction, row.Slot, "a cantrip costs the action and nothing else")
		s.Equal(session.TargetMember, row.TargetKind, "both name one creature")
		s.True(row.Available, "a bard with an action and a skeleton in range can cast")
		s.Nil(row.Why)
		s.NotEmpty(row.ID, "a compiled offer carries a selector")
		s.Nil(row.Ability, "a cast row is not an activation row")
		s.Nil(row.Attack)

		candidate, found := castCandidate(s.T(), row, "skeleton")
		s.Require().True(found, "the skeleton is in the candidate universe")
		s.True(candidate.Available)
	}
	s.NotEqual(rows[0].ID, rows[1].ID, "two spells are two selectors")
}

// TestCantripsWithNoBehaviourAreNoRowsAtAll is R9's consequence, made real: a
// bard who chose Mage Hand and Light is offered no Cast row, rather than two
// permanently disabled ones.
func (s *CastSuite) TestCantripsWithNoBehaviourAreNoRowsAtAll() {
	s.scene(castingBard("bard", spells.MageHand, spells.Light), 2)
	s.Empty(s.castRows(),
		"a known cantrip this build cannot cast mints no row — a row that resolved to nothing would be the lie")
}

// TestOnlyTheCastableHalfOfAMixedListMintsRows pins the intersection rather
// than either list alone.
func (s *CastSuite) TestOnlyTheCastableHalfOfAMixedListMintsRows() {
	s.scene(castingBard("bard", spells.MageHand, spells.ViciousMockery), 2)

	rows := s.castRows()
	s.Require().Len(rows, 1)
	s.Equal(refs.Spells.ViciousMockery().String(), rows[0].Spell.Ref)
}

// TestAFighterIsOfferedNoCastRow — the sheet's known list is empty for
// everybody who never chose a cantrip, and an empty list is not an error.
func (s *CastSuite) TestAFighterIsOfferedNoCastRow() {
	s.scene(armedFighter("bard"), 2)
	s.Empty(s.castRows(), "a fighter knows no cantrips and is offered no cast")
}

// TestRangeIsPerSpell is the point of compiling a definition per cantrip: one
// skeleton, eight cells away, is inside Vicious Mockery's 60 feet and outside
// True Strike's 30.
//
// The out-of-range target KEEPS ITS ROW with its own reason — the same law
// Attack's candidates keep — and the declaration says no target in range
// without erasing the candidate the player can see standing there.
func (s *CastSuite) TestRangeIsPerSpell() {
	s.scene(castingBard("bard", spells.TrueStrike, spells.ViciousMockery), 8)

	mockery := s.castRow(spells.ViciousMockery)
	s.True(mockery.Available, "40 feet is inside Vicious Mockery's 60")
	mockeryTarget, found := castCandidate(s.T(), mockery, "skeleton")
	s.Require().True(found)
	s.True(mockeryTarget.Available)

	trueStrike := s.castRow(spells.TrueStrike)
	s.False(trueStrike.Available, "40 feet is outside True Strike's 30")
	s.Require().NotNil(trueStrike.Why)
	s.Equal(session.ShortfallNoTargetInReach, trueStrike.Why.Reason)

	strikeTarget, found := castCandidate(s.T(), trueStrike, "skeleton")
	s.Require().True(found, "a shortfall does not erase the candidate rows")
	s.False(strikeTarget.Available)
	s.Require().NotNil(strikeTarget.Why)
	s.Equal(session.ShortfallTargetOutOfReach, strikeTarget.Why.Reason)

	// AND IT IS A SHORTFALL RATHER THAN AN ERROR: the read succeeded and said
	// why, which is the whole job of the row.
	s.NotEmpty(trueStrike.ID, "an unavailable offer still carries its selector")
}

// TestTheCastRowIsBlockedWhileAWindowIsOpen — while an interrupt window is
// open anywhere in the fight, every declaring verb is announced as unavailable
// before the click, because every one of them will be refused at its own door.
//
// The window is a REAL one: the bard, holding a Bardic Inspiration die, swings
// and the swing stops after the d20 to ask whether to spend it. Borrowed
// machinery rather than a fabricated ledger, so what this pins is what the seam
// actually does while the table is waiting.
func (s *CastSuite) TestTheCastRowIsBlockedWhileAWindowIsOpen() {
	bard := castingBard("bard", spells.TrueStrike, spells.ViciousMockery)
	armForSwinging(bard)
	// Initiative twice, then the attack's own d20 — which pauses before
	// anything else is rolled.
	s.scene(bard, 1, 12)
	s.holdInspiration("bard")

	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "bard", Target: "skeleton",
		DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "bard"),
	})
	s.Require().NoError(err)

	// ONE blocked Cast row, not one per cantrip: the reason is identical for
	// every spell, and two copies of "the table is waiting" would be a panel
	// that looks like it has choices.
	rows := s.castRows()
	s.Require().Len(rows, 1)
	s.False(rows[0].Available)
	s.Require().NotNil(rows[0].Why)
	s.Equal(session.ShortfallWindowOpen, rows[0].Why.Reason)
	s.Empty(rows[0].ID, "a blocker has compiled no offer")
	s.Nil(rows[0].Spell, "a blocker names no spell")

	// And the door refuses what the panel announced.
	_, err = s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton", DeclarationID: "anything",
	})
	s.ErrorIs(err, session.ErrWindowOpen,
		"Cast is refused while the table is waiting, exactly as Afford announced")
}

// TestNotYourTurnIsOneCastRow — the same collapse, for the same reason.
func (s *CastSuite) TestNotYourTurnIsOneCastRow() {
	s.scene(castingBard("bard", spells.TrueStrike, spells.ViciousMockery), 2)

	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{
		Session: "sess", Member: "skeleton",
	})
	s.Require().NoError(err)
	rows := 0
	for _, declaration := range out.Declarations {
		if declaration.Verb != session.VerbCast {
			continue
		}
		rows++
		s.False(declaration.Available)
		s.Require().NotNil(declaration.Why)
		s.Equal(session.ShortfallNotYourTurn, declaration.Why.Reason)
		s.Empty(declaration.ID)
	}
	s.Equal(1, rows, "one blocked Cast row, not one per cantrip")
}

// TestCastIsRefusedForAMonster — a monster's innate cast is driven by
// behaviour rather than declared, and its economy belongs to whoever runs its
// turn. The same line Attack and Activate draw.
func (s *CastSuite) TestCastIsRefusedForAMonster() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "skeleton", DeclarationID: "anything", Target: "bard",
	})
	s.ErrorIs(err, session.ErrNotACharacter)
}

// TestCastRefusesAnEmptySelector — the caller's own omission, and it is that
// rather than a stale offer.
func (s *CastSuite) TestCastRefusesAnEmptySelector() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton",
	})
	s.ErrorIs(err, session.ErrNoDeclarationID)
}

// TestCastRefusesATargetOutsideTheOffer — the offer's per-candidate gate is
// re-enforced at execution, not only shown at the door. A client that echoes a
// selector against somebody the offer never listed is refused as stale.
func (s *CastSuite) TestCastRefusesATargetOutsideTheOffer() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "nobody",
		DeclarationID: s.castRow(spells.ViciousMockery).ID,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
}

// TestCastRefusesNoTargetForASpellThatNeedsOne — a cast that names one
// creature and was given none is a caller defect, refused rather than resolved
// against nobody.
func (s *CastSuite) TestCastRefusesNoTargetForASpellThatNeedsOne() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard",
		DeclarationID: s.castRow(spells.ViciousMockery).ID,
	})
	s.ErrorIs(err, session.ErrBadCast)
}

// cast points the named spell at the skeleton through the current offer.
func (s *CastSuite) cast(spell spells.Spell) (*session.CastOutput, error) {
	s.T().Helper()
	return s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: s.member, Target: "skeleton",
		DeclarationID: s.castRow(spell).ID,
	})
}

// storedSkeleton reads the NPC's hit points back out of the session record.
func (s *CastSuite) storedSkeleton() int {
	s.T().Helper()
	for _, npc := range s.sessions.byID["sess"].NPCs {
		if npc.ID == "skeleton" {
			return npc.HitPoints
		}
	}
	s.FailNow("no skeleton in the record")
	return 0
}

// woundSkeleton sets the skeleton's stored hit points, so a 1d4 can finish it.
func (s *CastSuite) woundSkeleton(hp int) {
	s.T().Helper()
	for i := range s.sessions.byID["sess"].NPCs {
		if s.sessions.byID["sess"].NPCs[i].ID == "skeleton" {
			s.sessions.byID["sess"].NPCs[i].HitPoints = hp
			return
		}
	}
	s.FailNow("no skeleton in the record")
}

// beats is every event of the named kinds the bard received, in order.
func (s *CastSuite) beats(kinds ...session.EventKind) []session.Event {
	s.T().Helper()
	wanted := make(map[session.EventKind]bool, len(kinds))
	for _, kind := range kinds {
		wanted[kind] = true
	}
	out := make([]session.Event, 0, 4)
	for _, event := range eventsFor(s.stream.published, s.member) {
		if wanted[event.Kind] {
			out = append(out, event)
		}
	}
	return out
}

// TestTrueStrikeIsACastBeatAndAConditionOnTheCaster is the gateless half of the
// door: no roll, no save beat, and the condition lands on the BARD rather than
// on the creature it was pointed at.
func (s *CastSuite) TestTrueStrikeIsACastBeatAndAConditionOnTheCaster() {
	s.scene(castingBard("bard", spells.TrueStrike), 2)

	out, err := s.cast(spells.TrueStrike)
	s.Require().NoError(err)
	s.Equal(refs.Spells.TrueStrike().String(), out.Spell.Ref)
	s.Nil(out.Saved, "nobody resists True Strike, and a save report would say one did")
	s.Require().Len(out.Seqs, 2, "the cast beat and one result beat")

	events := s.beats(session.EventCast, session.EventSaved, session.EventActivationResult)
	s.Require().Len(events, 2, "a cast beat and its result, and no save beat at all")

	cast, ok := events[0].Body.(session.CastBody)
	s.Require().True(ok)
	s.Equal("bard", cast.Actor)
	s.Equal(refs.Spells.TrueStrike().String(), cast.Spell.Ref)
	s.Equal("True Strike", cast.Spell.Name, "the server authors the label")
	s.Equal("skeleton", cast.Target, "the creature it was pointed at is on the beat")

	result, ok := events[1].Body.(session.ActivationResultBody)
	s.Require().True(ok)
	s.Require().NotNil(result.ConditionApplied)
	s.Equal("bard", result.ConditionApplied.Target,
		"True Strike's advantage is the CASTER's, keyed to the creature named")
	s.Equal(refs.Conditions.TrueStrike().String(), result.ConditionApplied.Ref)
	s.Nil(result.DamageApplied)
}

// TestAFailedSaveAgainstViciousMockeryDeliversBothHalves is the gated half: the
// save beat carries the whole roll, and the failure delivers the psychic damage
// and the rider.
func (s *CastSuite) TestAFailedSaveAgainstViciousMockeryDeliversBothHalves() {
	// The skeleton's Wisdom save reaches 10, under the bard's DC 13, and the
	// 1d4 comes up 4.
	s.scene(castingBard("bard", spells.ViciousMockery), 2, 10, 4)
	before := s.storedSkeleton()

	out, err := s.cast(spells.ViciousMockery)
	s.Require().NoError(err)
	s.Require().NotNil(out.Saved, "a gated cast rolled a save and must say so")
	s.Equal("skeleton", out.Saved.Saver)
	s.Equal(bardSaveDC, out.Saved.DC, "the caster's own DC: 8 + 2 proficiency + 3 for CHA 16")
	s.False(out.Saved.Succeeded)

	saved := s.beats(session.EventSaved)
	s.Require().Len(saved, 1)
	body, ok := saved[0].Body.(session.SavedBody)
	s.Require().True(ok)
	s.Equal("skeleton", body.Saver)
	s.Equal(bardSaveDC, body.DC)
	s.False(body.Succeeded, "false is the answer here, not an absence")
	s.Equal(refs.Spells.ViciousMockery().String(), body.Source.Ref,
		"the save names what demanded it")

	results := s.beats(session.EventActivationResult)
	s.Require().Len(results, 2, "damage and the rider")
	damage, ok := results[0].Body.(session.ActivationResultBody)
	s.Require().True(ok)
	s.Require().NotNil(damage.DamageApplied)
	s.Equal("skeleton", damage.DamageApplied.Target)
	s.Equal(refs.Spells.ViciousMockery().String(), damage.DamageApplied.SourceRef,
		"the spell is what dealt it")
	s.Equal(session.DamagePsychic, damage.DamageApplied.DamageType,
		"psychic reaches the client as itself — a client that had to infer it from "+
			"the spell ref would be deriving a rule the record already holds")
	s.Require().NotNil(damage.DamageApplied.Calculation,
		"the 1d4 face reaches the client, not only its total")

	rider, ok := results[1].Body.(session.ActivationResultBody)
	s.Require().True(ok)
	s.Require().NotNil(rider.ConditionApplied)
	s.Equal("skeleton", rider.ConditionApplied.Target,
		"Vicious Mockery's disadvantage is the TARGET's")
	s.Equal(refs.Conditions.ViciousMockery().String(), rider.ConditionApplied.Ref)

	s.Less(s.storedSkeleton(), before, "the psychic damage reached the stored sheet")
}

// TestASuccessfulSaveDeliversNothing — RAW 2014: a made Wisdom save negates the
// damage AND the rider. The save beat is the whole story, and that is a
// complete cast rather than a cast that failed.
func (s *CastSuite) TestASuccessfulSaveDeliversNothing() {
	// 15 beats the bard's DC 13.
	s.scene(castingBard("bard", spells.ViciousMockery), 2, 15)
	before := s.storedSkeleton()

	out, err := s.cast(spells.ViciousMockery)
	s.Require().NoError(err)
	s.Require().NotNil(out.Saved)
	s.True(out.Saved.Succeeded)
	s.Require().Len(out.Seqs, 2, "the cast beat and the save beat, and nothing after them")

	s.Empty(s.beats(session.EventActivationResult),
		"a made save negates both halves, so there is nothing to narrate")
	s.Equal(before, s.storedSkeleton(), "and nothing reached the sheet")
}

// TestTheActionIsChargedOnceAndASecondCastIsRefused is R10 at the door: the
// price is paid there, and nothing in the verb counts casts — the ledger does.
func (s *CastSuite) TestTheActionIsChargedOnceAndASecondCastIsRefused() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2, 15)

	first := s.castRow(spells.ViciousMockery)
	s.Require().True(first.Available)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton", DeclarationID: first.ID,
	})
	s.Require().NoError(err)

	// The regenerated offer reads the same ledger the door charged, so the
	// panel goes dark before the second click rather than after it.
	second := s.castRow(spells.ViciousMockery)
	s.False(second.Available, "the action is spent")
	s.Require().NotNil(second.Why)
	s.Equal(session.ShortfallNoBudget, second.Why.Reason)
	s.Equal(session.CurrencyAction, second.Why.Currency)
	s.Equal(0, second.Why.Left)

	// And the door refuses the selector the panel just disabled.
	_, err = s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton", DeclarationID: second.ID,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration,
		"charged once, and the second cast is refused rather than charged again")
}

// TestACantripCanDropATarget — damage delivered by a cast flows exactly like a
// strike's: the sheet is written, and the composition notices the drop for
// itself rather than being told.
func (s *CastSuite) TestACantripCanDropATarget() {
	s.scene(castingBard("bard", spells.ViciousMockery), 2, 10, 4)
	s.woundSkeleton(2)

	_, err := s.cast(spells.ViciousMockery)
	s.Require().NoError(err)

	s.Equal(0, s.storedSkeleton(), "4 psychic through 2 hit points leaves none")
	s.NotEmpty(s.beats(session.EventDowned),
		"the world noticed the drop; nobody announced it")
}

// THE SEAM, end to end, and the regression that motivated the fix underneath
// it: a self-targeted cast used to resolve, apply its condition to the sheet,
// and then DIE at RecordCast with ErrNoMember -- after the mechanical writes
// were already durable, leaving the ward on the character and no beat naming it.
//
// resolution/newCast fabricated a one-element target list holding the empty
// string so its per-target loop would run once. Blade Ward is the first content
// that declares CastTargetSelf, so it is the first thing to reach that path.
// Nothing here asserts the sentinel is gone; it asserts the cast SUCCEEDS and
// the record says who received it, which is what the sentinel made impossible.
func (s *CastSuite) TestBladeWardCastsWithNoTargetAndRecordsTheCasterAsItsRecipient() {
	s.scene(castingBard("bard", spells.BladeWard), 2)

	row := s.castRow(spells.BladeWard)
	s.Equal(session.TargetNone, row.TargetKind, "the player aims a self cast at nobody")
	s.Empty(row.Candidates, "and is offered nobody to aim it at")
	s.True(row.Available)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: row.ID,
	})
	s.Require().NoError(err, "the whole point: this used to fail at RecordCast after the sheet was written")

	casts := s.beats(session.EventCast)
	s.Require().Len(casts, 1)
	body, ok := casts[0].Body.(session.CastBody)
	s.Require().True(ok)
	s.Equal(refs.Spells.BladeWard().String(), body.Spell.Ref)
	s.Equal([]string{"bard"}, body.Targets,
		"one recipient, and it is the caster -- an empty list would have nowhere to hang the ward")
}

// TestThunderclapIsOfferedAsAnAreaWithNobodyToAimAt.
//
// The first offer whose selector shape is neither "pick a creature" nor "this
// lands on you". An area cast prompts for nobody the way a self cast does, and
// carries no candidates — but it is deliberately NOT TargetNone, because that
// value already means the spell lands on the caster and this one lands on
// everyone but.
func (s *CastSuite) TestThunderclapIsOfferedAsAnAreaWithNobodyToAimAt() {
	s.scene(castingBard("bard", spells.Thunderclap), 1)

	row := s.castRow(spells.Thunderclap)
	s.Equal(session.TargetArea, row.TargetKind, "the engine decides who it catches, not the player")
	s.Empty(row.Candidates, "there is nothing to choose between")
	s.True(row.Available, "and casting it into an empty room would still be legal")
	s.Zero(row.MinTargets)
	s.Zero(row.MaxTargets)
}

// TestThunderwaveIsOfferedAsACellToAimAt is the other half of the area offer,
// and the reason TargetCell exists at all.
//
// Thunderclap's burst is centred on the caster and needs nothing from the
// player. Thunderwave's cube hangs off the caster's own edge and has to be
// POINTED, so the offer says a cell is wanted — and still carries no
// candidates, because a cell is not a creature and there is nobody to choose
// between.
func (s *CastSuite) TestThunderwaveIsOfferedAsACellToAimAt() {
	s.scene(castingBardWithSpells("bard", spells.Thunderwave), 1)

	row := s.castRow(spells.Thunderwave)
	s.Equal(session.TargetCell, row.TargetKind, "a caster-edge box is aimed, and a cell is what aims it")
	s.Empty(row.Candidates, "there is nothing to choose between")
	s.True(row.Available)
}

// cellOf is where the composition actually put somebody. Members are placed by
// AUTHORED OFFSET and reported in absolute axial, so a test that aimed at the
// offset it wrote would be aiming somewhere else.
func (s *CastSuite) cellOf(member string) spatial.Position {
	s.T().Helper()
	where, err := s.mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return where.Position
}

// TestACellCastRefusesEveryAimButACellOfItsOwn.
//
// The cell is the only thing this cast takes from the player, so each of the
// three ways of getting it wrong is REFUSED rather than repaired: a client that
// believed it had pointed a spell somewhere must be told it had not, which is
// the same argument the self and area arms make about a named target.
func (s *CastSuite) TestACellCastRefusesEveryAimButACellOfItsOwn() {
	s.scene(castingBardWithSpells("bard", spells.Thunderwave), 1)
	ctx := context.Background()

	s.Run("a cast with no cell is refused, and the refusal says so", func() {
		_, err := s.mgr.Cast(ctx, &session.CastInput{
			Session: "sess", Member: "bard", DeclarationID: s.castRow(spells.Thunderwave).ID,
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrBadCast)
		s.Contains(err.Error(), "cell", "a refusal that does not name what is missing teaches nothing")
	})

	s.Run("the caster's own cell is not a direction", func() {
		own := s.cellOf("bard")
		_, err := s.mgr.Cast(ctx, &session.CastInput{
			Session: "sess", Member: "bard", Cell: &own,
			DeclarationID: s.castRow(spells.Thunderwave).ID,
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrBadCast)
	})

	s.Run("naming somebody is refused as it is for any derived cast", func() {
		ahead := s.cellOf("skeleton")
		_, err := s.mgr.Cast(ctx, &session.CastInput{
			Session: "sess", Member: "bard", Cell: &ahead, Targets: []string{"skeleton"},
			DeclarationID: s.castRow(spells.Thunderwave).ID,
		})
		s.Require().Error(err)
		s.ErrorIs(err, session.ErrBadCast)
	})
}

// TestAnAreaCastRefusesACellItWouldNeverRead is the guard read from the other
// side. Thunderclap's burst is centred on the caster and has no direction to
// take, so a cell arriving with it is a client aiming a spell that offers no
// aim — and a cell silently discarded is a client that never finds out.
func (s *CastSuite) TestAnAreaCastRefusesACellItWouldNeverRead() {
	s.scene(castingBard("bard", spells.Thunderclap), 1)

	ahead := s.cellOf("skeleton")
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Cell: &ahead,
		DeclarationID: s.castRow(spells.Thunderclap).ID,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadCast)
	s.Contains(err.Error(), "no cell")
}

// TestThunderclapCatchesTheCreatureStandingInIt is the whole capability, end to
// end through the verb: content declared a shape, the composition said who was
// standing in it, and the cast resolved against them — with the player naming
// nobody at any point.
func (s *CastSuite) TestThunderclapCatchesTheCreatureStandingInIt() {
	s.scene(castingBard("bard", spells.Thunderclap), 1, 3, 4)
	before := s.storedSkeleton()
	s.Require().Positive(before, "the scene starts with a skeleton worth hitting")

	row := s.castRow(spells.Thunderclap)
	out, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", DeclarationID: row.ID,
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)

	s.Less(s.storedSkeleton(), before,
		"the skeleton was standing in the burst and took it, without ever being named")
}

// TestAnAreaCastRefusesACallerThatNamesSomebody — the recipients are derived,
// so a populated target is a client believing it aimed a spell that offers no
// aim. Refused rather than ignored, exactly as a self cast refuses one.
func (s *CastSuite) TestAnAreaCastRefusesACallerThatNamesSomebody() {
	s.scene(castingBard("bard", spells.Thunderclap), 1)

	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Target: "skeleton",
		DeclarationID: s.castRow(spells.Thunderclap).ID,
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrBadCast)
}

// storyOf is every beat the bard's story holds, reduced to the four fields
// these tests ask about, in delivered order.
//
// READ OFF THE STORY rather than off the verb's return value, because the ORDER
// is the thing under test and only the story has one.
type storyBeat struct {
	Beat   string `json:"beat"`
	Member string `json:"member"`
	Cause  string `json:"cause"`
	Result *struct {
		Kind      string `json:"kind"`
		Target    string `json:"target"`
		Ref       string `json:"ref"`
		Moved     *int   `json:"moved"`
		StoppedBy string `json:"stopped_by"`
	} `json:"result"`
}

func (s *CastSuite) storyOf(member string) []storyBeat {
	s.T().Helper()
	entries, err := s.mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	out := make([]storyBeat, 0, len(entries))
	for _, entry := range entries {
		var beat storyBeat
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}
	return out
}

// TestThunderwaveShovesTheSkeletonAndTheStoryReadsInOrder is the whole slice
// through the one verb: the bard points at a cell, the cube catches what is
// standing in it, the failed save takes the thunder and is shoved — and the
// pillar two cells out gives it one cell instead of two.
//
// # The order is the assertion
//
// Route is a pure computation and Direct is the walk, so they sit either side
// of the record: the route is known before RecordCast, which lets the CAST's
// own beat say how far the push went and what stopped it, and the walk happens
// after it, which puts the movement beats where a client animates them. Thunder
// first, then the slide.
func (s *CastSuite) TestThunderwaveShovesTheSkeletonAndTheStoryReadsInOrder() {
	// The pillar stands two cells past the skeleton, which is one cell past
	// where a two-cell push would end. Authored offsets, the frame the scene
	// places everybody in.
	s.sceneWithProps(
		castingBardWithSpells("bard", spells.Thunderwave),
		blockingProps(spatial.Position{X: 4, Y: 1}),
		1,
		// The skeleton's save, then the one 2d8 every failure shares. A 1
		// fails against any DC, and 10 thunder leaves a 13-hit-point skeleton
		// standing — the dropped are not pushed, and this one must be pushed.
		1, 5, 5,
	)

	ahead := s.cellOf("skeleton")
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Cell: &ahead,
		DeclarationID: s.castRow(spells.Thunderwave).ID,
	})
	s.Require().NoError(err)

	s.Positive(s.storedSkeleton(), "a dropped skeleton is not pushed, and this test is about the push")
	after := s.cellOf("skeleton")
	s.NotEqual(ahead, after, "the skeleton was shoved")

	story := s.storyOf("bard")
	var recordedAt, walkedAt int
	var moved *storyBeat
	for i := range story {
		switch {
		case story[i].Result != nil && story[i].Result.Kind == "moved":
			recordedAt = i
			moved = &story[i]
		case story[i].Beat == "moved" && story[i].Member == "skeleton":
			walkedAt = i
		}
	}

	s.Require().NotNil(moved, "the cast's own account of what its push achieved")
	s.Require().NotNil(moved.Result.Moved)
	s.Equal(1, *moved.Result.Moved, "one open cell, then the pillar")
	s.Contains(moved.Result.StoppedBy, "pillar", "the beat names what got in the way")
	s.Equal("skeleton", moved.Result.Target)
	s.Equal(refs.Spells.Thunderwave().String(), moved.Result.Ref)

	s.Require().Positive(walkedAt, "the walk wrote a movement beat")
	s.Less(recordedAt, walkedAt, "thunder, then the slide")
	s.Equal(refs.Spells.Thunderwave().String(), story[walkedAt].Cause,
		"a step a creature chose carries no cause; this one names the spell")
}

// TestAShovedCreatureProvokesNobody is the directive's other half, and the one
// a walk would get wrong. The skeleton leaves the bard's reach and the bard is
// a player, so an ordinary step here would stop the whole fight to ask her
// whether she swings. Nobody chose to leave anybody's reach, so there is
// nothing to ask.
func (s *CastSuite) TestAShovedCreatureProvokesNobody() {
	s.sceneWithProps(
		castingBardWithSpells("bard", spells.Thunderwave), nil, 1, 1, 5, 5)

	ahead := s.cellOf("skeleton")
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Cell: &ahead,
		DeclarationID: s.castRow(spells.Thunderwave).ID,
	})
	s.Require().NoError(err, "a step that stopped to ask would be news here, and there is none")

	for _, beat := range s.storyOf("bard") {
		s.NotEqual("window-opened", beat.Beat, "nobody was asked about a shove")
	}

	afford, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "bard"})
	s.Require().NoError(err)
	for _, declaration := range afford.Declarations {
		s.NotEqual(session.VerbReact, declaration.Verb, "no reaction was offered against a push")
	}
}

// TestTheShoveReachesTheClientAsATypedResult. The beat on the story is the
// record; this is what a client actually decodes, and a result kind with no arm
// in the projection arrives as a body nobody can read — a push that happened,
// was recorded, and cannot be drawn.
func (s *CastSuite) TestTheShoveReachesTheClientAsATypedResult() {
	s.sceneWithProps(
		castingBardWithSpells("bard", spells.Thunderwave),
		blockingProps(spatial.Position{X: 4, Y: 1}), 1, 1, 5, 5)

	ahead := s.cellOf("skeleton")
	_, err := s.mgr.Cast(context.Background(), &session.CastInput{
		Session: "sess", Member: "bard", Cell: &ahead,
		DeclarationID: s.castRow(spells.Thunderwave).ID,
	})
	s.Require().NoError(err)

	var pushed *session.MoveImposedBody
	for _, event := range s.beats(session.EventActivationResult) {
		body, ok := event.Body.(session.ActivationResultBody)
		s.Require().True(ok, "an activation result reached the bard as %T", event.Body)
		if body.MoveImposed != nil {
			pushed = body.MoveImposed
		}
	}

	s.Require().NotNil(pushed, "the push reached the client as something it can read")
	s.Equal("skeleton", pushed.Target)
	s.Equal(1, pushed.MovedCells, "one open cell, then the pillar")
	s.Contains(pushed.StoppedBy, "pillar")
	s.Equal(refs.Spells.Thunderwave().String(), pushed.SourceRef)
	s.Equal("Thunderwave", pushed.SourceName)
}
