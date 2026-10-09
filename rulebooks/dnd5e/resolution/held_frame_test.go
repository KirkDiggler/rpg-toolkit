// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// held_frame_test.go holds target-held effects through resolution's frames
// (rpg-project#520, R16–R18): what each member holds and who sees whom, in the
// information frame from the actor's sightings alone and in the strike's
// execution frames from authoritative state — and the row a candidate shows
// agreeing with what the swing does.

const heldCaster = "cleric"

// sheetConditions answers the conditions capability from the scene's sheets,
// as a host does, through the rulebook's own reader
// ([conditions.HeldAddresses]): each member with a sheet reports what it
// holds, at each condition's own address; a member with no sheet has nothing
// to observe.
// Hands are never observed. The sheets are read when asked, so a change to
// one is seen at the next sight refresh and not before.
type sheetConditions struct {
	noHandsAreObserved
	sheets map[encounter.MemberID]*[]json.RawMessage
}

var _ encounter.EquipmentWithConditions = sheetConditions{}

func (c sheetConditions) Conditions(
	members []encounter.MemberID,
) (map[encounter.MemberID]*encounter.ConditionSet, error) {
	out := make(map[encounter.MemberID]*encounter.ConditionSet, len(members))
	for _, id := range members {
		blobs, ok := c.sheets[id]
		if !ok {
			out[id] = nil
			continue
		}
		addresses, err := conditions.HeldAddresses(string(id), *blobs)
		if err != nil {
			return nil, err
		}
		set := &encounter.ConditionSet{Conditions: []encounter.ConditionKey{}}
		for _, address := range addresses {
			set.Conditions = append(set.Conditions, encounter.ConditionKey{
				ConditionRef: address.ConditionRef, SourceID: address.SourceID,
			})
		}
		out[id] = set
	}
	return out, nil
}

// failingSight is a Sight capability that cannot answer: the run then knows
// no line of sight, so every Sees fact the strike builds is unknown.
type failingSight struct{}

func (failingSight) Sight([]encounter.MemberID) (map[encounter.MemberID]int, error) {
	return nil, errors.New("sight is unavailable")
}

// heldScene is the rogue with goblin one at a chosen place and goblin two
// well away, both hostile. Goblin one's sheet holds whatever the test puts on
// it; goblin two holds only what a goblin is built with, none of it bearing on
// an attack against it. unobserved names members whose sightings observe no
// conditions at all.
type heldScene struct {
	goblinAt   spatial.Position
	sight      encounter.Sight
	goblin     *monster.Data
	other      *monster.Data
	unobserved map[encounter.MemberID]bool
}

func (s *FrameTestSuite) newHeldScene(goblinX float64, held ...json.RawMessage) *heldScene {
	goblin := monsters.NewGoblin(informGoblin1).ToData()
	goblin.Conditions = held
	return &heldScene{
		goblinAt: spatial.Position{X: goblinX, Y: 1},
		sight:    everyoneSeesTheWholeMap{},
		goblin:   goblin,
		other:    monsters.NewGoblin(informGoblin2).ToData(),
	}
}

func (h *heldScene) equipment() sheetConditions {
	sheets := map[encounter.MemberID]*[]json.RawMessage{
		informGoblin1: &h.goblin.Conditions,
		informGoblin2: &h.other.Conditions,
	}
	for id := range h.unobserved {
		delete(sheets, id)
	}
	return sheetConditions{sheets: sheets}
}

// encounter seats the scene; the sight pass at setup snapshots what the rogue
// sees, conditions included.
func (s *FrameTestSuite) heldEncounter(h *heldScene) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:   hexCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("cave", 0, 0, 10, 5)},
			Factions: []encounter.FactionInput{{ID: informGoblins}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{informGoblins, encounter.FactionParty}, Stance: encounter.StanceHostile,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: informRogue, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: informGoblin1, Kind: encounter.KindMonster, Position: h.goblinAt, Faction: informGoblins},
			{ID: informGoblin2, Kind: encounter.KindMonster, Position: spatial.Position{X: 7, Y: 3}, Faction: informGoblins},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Driver:     passDriver{},
			Standing:   everyoneStanding{},
			Sight:      h.sight,
			Equipment:  h.equipment(),
			Sheets:     standStillSheets{},
			Actors: encounter.Actors{
				Striker:   noAttacksExpected{},
				Mover:     encounter.RefusingMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	s.Require().NoError(err)
	return enc
}

// heldRogue is the acting rogue's sheet, holding nothing that bears.
func (s *FrameTestSuite) heldRogue() *character.Data {
	informs := &InformAttackTestSuite{}
	informs.SetT(s.T())
	informs.ctx = s.ctx
	data, err := informs.rogue().ToData()
	s.Require().NoError(err)
	return data
}

// informHeld asks the rogue's rows for an attack over the scene's observed
// context.
func (s *FrameTestSuite) informHeld(
	enc *encounter.Encounter, attack combatActions.Definition, targets ...string,
) *InformAttackOutput {
	observed, err := enc.ObservedContext(&encounter.ViewInput{Member: informRogue})
	s.Require().NoError(err)
	actor, err := character.Load(s.ctx, s.heldRogue())
	s.Require().NoError(err)
	out, err := InformAttack(&InformAttackInput{Observed: observed, Actor: actor, Attack: attack.Attack, Targets: targets})
	s.Require().NoError(err)
	return out
}

// strikeHeld swings the rogue's attack at goblin one on the scene's world,
// from authoritative state. The world is seated under full sight and with no
// conditions observed — the strike reads what members hold from their sheets,
// never from testimony; the strike asks the scene's own sight live.
func (s *FrameTestSuite) strikeHeld(
	bus events.EventBus, h *heldScene, attack combatActions.Definition, d20 int,
) (*Output, error) {
	seated := *h
	seated.sight = everyoneSeesTheWholeMap{}
	seated.unobserved = map[encounter.MemberID]bool{informGoblin1: true, informGoblin2: true}
	world := s.heldEncounter(&seated).ToData()
	return resolveOn(s.ctx, &Input{
		World: world,
		Participants: []Participant{
			{Character: s.heldRogue()}, {Monster: h.goblin}, {Monster: h.other},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: informRogue, TargetID: informGoblin1, Definition: attack, Roller: facedRoller{d20: d20, other: 1},
		}),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Standing:   everyoneStanding{},
			Sight:      h.sight,
			Equipment:  h.equipment(),
			Sheets:     noSheetsAsked{},
			Driver:     passDriver{},
			Roller:     facedRoller{d20: d20, other: 1},
			Actors:     Actors,
		},
	}, newSurface(bus))
}

func (s *FrameTestSuite) faerieFireJSON(member string) json.RawMessage {
	fire, err := conditions.NewFaerieFireCondition(conditions.NewFaerieFireConditionInput{
		MemberID: member, SourceID: heldCaster, SourceRef: refs.Spells.FaerieFire(),
	})
	s.Require().NoError(err)
	raw, err := fire.ToJSON()
	s.Require().NoError(err)
	return raw
}

func (s *FrameTestSuite) proneJSON(member string) json.RawMessage {
	raw, err := conditions.NewProneCondition(member).ToJSON()
	s.Require().NoError(err)
	return raw
}

func (s *FrameTestSuite) dodgingJSON(member string) json.RawMessage {
	raw, err := conditions.NewDodgingCondition(member).ToJSON()
	s.Require().NoError(err)
	return raw
}

var (
	faerieFireRowID = "target:" + refs.Conditions.FaerieFire().String() + "@" + heldCaster
	proneRowID      = "target:" + refs.Conditions.Prone().String()
	dodgingRowID    = "target:" + refs.Conditions.Dodging().String()
)

// rowNamed finds the row with the given ID, reporting whether it is there.
func rowNamed(effects []contributions.Effect, id string) (contributions.Effect, bool) {
	for _, effect := range effects {
		if effect.ID == id {
			return effect, true
		}
	}
	return contributions.Effect{}, false
}

// sourcedBy reports whether a folded advantage or disadvantage list carries
// the given condition.
func sourcedBy(sources []refSource, ref string) bool {
	for _, source := range sources {
		if source.ref == ref {
			return true
		}
	}
	return false
}

type refSource struct{ ref, id string }

func foldedSources(out *Output) (advantage, disadvantage []refSource) {
	outcome := out.Outcome.(StrikeOutcome)
	for _, source := range outcome.Folded.AdvantageSources {
		if source.SourceRef != nil {
			advantage = append(advantage, refSource{source.SourceRef.String(), source.SourceID})
		}
	}
	for _, source := range outcome.Folded.DisadvantageSources {
		if source.SourceRef != nil {
			disadvantage = append(disadvantage, refSource{source.SourceRef.String(), source.SourceID})
		}
	}
	return advantage, disadvantage
}

func (s *FrameTestSuite) TestInformationFrameHeldFromSightingsOnly() {
	out, err := informationFrame(&informationFrameInput{
		Observed: &encounter.ObservedContextOutput{
			Observer: holdOutRogue,
			Members: []encounter.ObservedContextMember{
				{ID: holdOutScout, Conditions: &encounter.ConditionSet{Conditions: []encounter.ConditionKey{
					{ConditionRef: refs.Conditions.FaerieFire().String(), SourceID: heldCaster},
				}}},
				{ID: holdOutChief},
				{ID: holdOutAlly, Conditions: &encounter.ConditionSet{}},
			},
		},
		Attack:           dagger().Attack,
		Target:           holdOutScout,
		ActorHeld:        []contributions.HeldCondition{{Ref: refs.Conditions.Prone().String()}},
		ActorClassLevels: contributions.KnownClassLevels(),
	})
	s.Require().NoError(err)
	frame := out.Frame

	scout, known := frame.HeldBy(holdOutScout)
	s.True(known, "a sighting that observed conditions lists its member")
	s.Equal([]contributions.HeldCondition{{Ref: refs.Conditions.FaerieFire().String(), SourceID: heldCaster}}, scout)

	_, known = frame.HeldBy(holdOutChief)
	s.False(known, "a sighting that observed no conditions is unknown, not empty")

	ally, known := frame.HeldBy(holdOutAlly)
	s.True(known, "seen holding nothing is known")
	s.Empty(ally)

	_, known = frame.HeldBy(holdOutLetter)
	s.False(known, "an unsighted member is unknown")

	actor, known := frame.HeldBy(holdOutRogue)
	s.True(known, "the actor's own holdings are known from its own sheet")
	s.Equal([]contributions.HeldCondition{{Ref: refs.Conditions.Prone().String()}}, actor)
}

func (s *FrameTestSuite) TestInformationFrameSeesOnlyObserverToSighted() {
	out, err := informationFrame(&informationFrameInput{
		Observed: &encounter.ObservedContextOutput{
			Observer: holdOutRogue,
			Members:  []encounter.ObservedContextMember{{ID: holdOutScout}, {ID: holdOutChief}},
			Pairs: []encounter.ObservedContextPair{
				{From: holdOutChief, To: holdOutRogue, DistanceCells: 3, Stance: encounter.StanceHostile},
				{From: holdOutChief, To: holdOutScout, DistanceCells: 2, Stance: encounter.StanceAllied},
				{From: holdOutRogue, To: holdOutChief, DistanceCells: 3, Stance: encounter.StanceHostile},
				{From: holdOutRogue, To: holdOutScout, DistanceCells: 1, Stance: encounter.StanceHostile},
				{From: holdOutScout, To: holdOutChief, DistanceCells: 2, Stance: encounter.StanceAllied},
				{From: holdOutScout, To: holdOutRogue, DistanceCells: 1, Stance: encounter.StanceHostile},
			},
		},
		Attack:           dagger().Attack,
		Target:           holdOutScout,
		ActorClassLevels: contributions.KnownClassLevels(),
	})
	s.Require().NoError(err)
	frame := out.Frame

	s.Equal(contributions.Known(true), frame.Pair(holdOutRogue, holdOutScout).Sees, "the observer sees what it sighted")
	s.Equal(contributions.Known(true), frame.Pair(holdOutRogue, holdOutChief).Sees)
	s.Equal(contributions.Unknown[bool](), frame.Pair(holdOutScout, holdOutRogue).Sees,
		"whether the scout sees the observer is not the observer's to know")
	s.Equal(contributions.Unknown[bool](), frame.Pair(holdOutScout, holdOutChief).Sees)
	s.Equal(contributions.Unknown[bool](), frame.Pair(holdOutChief, holdOutRogue).Sees)
}

// TestInformAttackHeldRowsPerTarget: goblin one is seen holding Faerie Fire,
// so its candidate carries the row; goblin two is seen holding nothing, so its
// candidate carries none; the actor's own rows never carry a target's effect.
func (s *FrameTestSuite) TestInformAttackHeldRowsPerTarget() {
	scene := s.newHeldScene(2, s.faerieFireJSON(informGoblin1))
	out := s.informHeld(s.heldEncounter(scene), dagger(), informGoblin1, informGoblin2)

	row, found := rowNamed(out.HeldByTarget[informGoblin1], faerieFireRowID)
	s.Require().True(found, "%+v", out.HeldByTarget[informGoblin1])
	s.Equal(contributions.StateApplies, row.State)
	s.Equal("Advantage on the attack roll", row.Benefit)
	s.Equal(heldCaster, row.Source.SourceID)

	s.Contains(out.HeldByTarget, informGoblin2)
	s.Empty(out.HeldByTarget[informGoblin2], "seen holding nothing that bears gives no rows")

	_, onDeclaration := rowNamed(out.Effects, faerieFireRowID)
	s.False(onDeclaration, "the declaration's rows are the actor's own")
	_, onTarget := rowNamed(out.ByTarget[informGoblin1], faerieFireRowID)
	s.False(onTarget, "ByTarget is the actor's own effects per candidate")
}

// TestUnknownHoldingsGiveNoRowsAndNoError: a candidate whose sighting
// observed no conditions, and a candidate not sighted at all, carry no held
// rows and are no error — no effect is known to exist there.
func (s *FrameTestSuite) TestUnknownHoldingsGiveNoRowsAndNoError() {
	scene := s.newHeldScene(2, s.faerieFireJSON(informGoblin1))
	scene.unobserved = map[encounter.MemberID]bool{informGoblin1: true}
	scene.sight = sightOf{informRogue: 3}
	out := s.informHeld(s.heldEncounter(scene), dagger(), informGoblin1, informGoblin2)

	s.Contains(out.HeldByTarget, informGoblin1)
	s.Empty(out.HeldByTarget[informGoblin1], "a sighting that observed no conditions gives no rows")
	s.Contains(out.HeldByTarget, informGoblin2)
	s.Empty(out.HeldByTarget[informGoblin2], "an unsighted candidate gives no rows")
}

// TestHeldRowsAreTestimony is K3 and R16: a condition the observer has not
// seen does not change the rows — goblin one starts dodging after the sight
// pass, and the rows still show only what was seen. The next sight refresh
// sees it.
func (s *FrameTestSuite) TestHeldRowsAreTestimony() {
	scene := s.newHeldScene(2, s.faerieFireJSON(informGoblin1))
	enc := s.heldEncounter(scene)
	before := s.informHeld(enc, dagger(), informGoblin1)

	scene.goblin.Conditions = append(scene.goblin.Conditions, s.dodgingJSON(informGoblin1))
	unseen := s.informHeld(enc, dagger(), informGoblin1)
	s.Equal(before.HeldByTarget, unseen.HeldByTarget, "hidden state cannot change an information answer")
	_, dodging := rowNamed(unseen.HeldByTarget[informGoblin1], dodgingRowID)
	s.False(dodging)

	_, err := enc.Recheck(&encounter.RecheckInput{Members: []encounter.MemberID{informGoblin1}})
	s.Require().NoError(err)
	seen := s.informHeld(enc, dagger(), informGoblin1)
	row, dodging := rowNamed(seen.HeldByTarget[informGoblin1], dodgingRowID)
	s.Require().True(dodging, "once seen, the row is there: %+v", seen.HeldByTarget[informGoblin1])
	s.Equal(contributions.StateApplies, row.State)
}

// TestHeldRowAgreesWithTheStrike: for each scene the candidate's row and the
// swing give the same answer, because they ask the same rule over frames
// resolution built from the same facts — the row from what the rogue saw, the
// swing from the truth.
func (s *FrameTestSuite) TestHeldRowAgreesWithTheStrike() {
	fire := refs.Conditions.FaerieFire().String()
	prone := refs.Conditions.Prone().String()
	for _, tc := range []struct {
		name    string
		goblinX float64
		sight   encounter.Sight
		held    func() json.RawMessage
		attack  combatActions.Definition
		rowID   string
		// row is the candidate's row state; empty means no row at all.
		row          contributions.EffectState
		benefit      string
		advantage    string
		disadvantage string
	}{
		{
			name: "faerie fire, the rogue sees the goblin", goblinX: 2, sight: everyoneSeesTheWholeMap{},
			held: func() json.RawMessage { return s.faerieFireJSON(informGoblin1) }, attack: dagger(),
			rowID: faerieFireRowID, row: contributions.StateApplies, benefit: "Advantage on the attack roll", advantage: fire,
		},
		{
			name: "faerie fire, the rogue cannot see the goblin", goblinX: 4, sight: sightOf{informRogue: 1},
			held: func() json.RawMessage { return s.faerieFireJSON(informGoblin1) }, attack: handaxe(),
			rowID: faerieFireRowID,
		},
		{
			name: "prone, within five feet", goblinX: 2, sight: everyoneSeesTheWholeMap{},
			held: func() json.RawMessage { return s.proneJSON(informGoblin1) }, attack: dagger(),
			rowID: proneRowID, row: contributions.StateApplies, benefit: "Advantage on the attack roll", advantage: prone,
		},
		{
			name: "prone, beyond five feet", goblinX: 4, sight: everyoneSeesTheWholeMap{},
			held: func() json.RawMessage { return s.proneJSON(informGoblin1) }, attack: handaxe(),
			rowID: proneRowID, row: contributions.StateApplies, benefit: "Disadvantage on the attack roll", disadvantage: prone,
		},
	} {
		s.Run(tc.name, func() {
			scene := s.newHeldScene(tc.goblinX, tc.held())
			scene.sight = tc.sight
			informed := s.informHeld(s.heldEncounter(scene), tc.attack, informGoblin1)
			row, found := rowNamed(informed.HeldByTarget[informGoblin1], tc.rowID)
			if tc.row == "" {
				s.False(found, "no row: %+v", informed.HeldByTarget[informGoblin1])
			} else {
				s.Require().True(found, "%+v", informed.HeldByTarget[informGoblin1])
				s.Equal(tc.row, row.State)
				s.Equal(tc.benefit, row.Benefit)
			}

			bus := events.NewEventBus()
			rolled := s.watchAttackFrames(bus)
			out, err := s.strikeHeld(bus, s.newHeldSceneLike(scene, tc.held()), tc.attack, 1)
			s.Require().NoError(err)
			advantage, disadvantage := foldedSources(out)
			for _, ref := range []string{fire, prone} {
				s.Equal(ref == tc.advantage, sourcedBy(advantage, ref), "advantage from %s: %+v", ref, advantage)
				s.Equal(ref == tc.disadvantage, sourcedBy(disadvantage, ref), "disadvantage from %s: %+v", ref, disadvantage)
			}

			// The same rule over the execution frame answers as the swing did.
			s.Require().Len(*rolled, 1)
			executed, err := conditions.AssessTargetHeldEffects(&conditions.AssessTargetHeldEffectsInput{Frame: (*rolled)[0]})
			s.Require().NoError(err)
			truth, found := rowNamed(executed.Effects, tc.rowID)
			s.Require().True(found)
			if tc.row == "" {
				s.Equal(contributions.StateDoesNotApply, truth.State, "the swing knew the rogue could not see")
			} else {
				s.Equal(row, truth, "the row the candidate showed is the swing's answer")
			}
		})
	}
}

// newHeldSceneLike is a fresh copy of a scene with the same held effect, so
// the strike's sheets are not the ones the information pass read.
func (s *FrameTestSuite) newHeldSceneLike(h *heldScene, held json.RawMessage) *heldScene {
	fresh := s.newHeldScene(h.goblinAt.X, held)
	fresh.sight = h.sight
	return fresh
}

// TestStrikeFrameCarriesHeldAndSight: the attack chain's frame lists what
// each participant holds and who sees whom, from authoritative state, and the
// damage frame is that frame with only Advantage settled.
func (s *FrameTestSuite) TestStrikeFrameCarriesHeldAndSight() {
	scene := s.newHeldScene(2, s.faerieFireJSON(informGoblin1))
	bus := events.NewEventBus()
	rolled := s.watchAttackFrames(bus)
	spy := s.watchFrames(bus)

	_, err := s.strikeHeld(bus, scene, dagger(), 18)
	s.Require().NoError(err)

	s.Require().Len(*rolled, 1)
	s.Require().Len(spy.damaged, 1)
	before, after := (*rolled)[0], spy.damaged[0]
	held, known := before.HeldBy(informGoblin1)
	s.True(known)
	s.Contains(held, contributions.HeldCondition{Ref: refs.Conditions.FaerieFire().String(), SourceID: heldCaster})
	other, known := before.HeldBy(informGoblin2)
	s.True(known, "the sheet is the authority")
	s.NotContains(other, contributions.HeldCondition{Ref: refs.Conditions.FaerieFire().String(), SourceID: heldCaster})
	_, known = before.HeldBy(informRogue)
	s.True(known)
	s.Equal(contributions.Known(true), before.Pair(informRogue, informGoblin1).Sees)
	s.Equal(contributions.Known(true), before.Pair(informGoblin1, informRogue).Sees)

	s.Equal(contributions.Known(true), after.Action.Advantage, "faerie fire granted it")
	before.Action.Advantage = after.Action.Advantage
	s.Equal(before, after, "the damage frame only adds advantage")
}

// TestExecutionHeldIsEveryLoadedAddress: the strike's frame lists, for every
// participant with a sheet — the monsters included — exactly what the
// rulebook's stored-sheet reader says it holds ([conditions.HeldAddresses]:
// its stored conditions plus the free reactions every combatant carries), in
// the same order. A loaded handler asks its held rule about
// its own address, and a frame that lists a holder without that address is a
// frame the rule cannot answer from (R13), so a dropped condition or a lost
// source qualifier must not get past here.
func (s *FrameTestSuite) TestExecutionHeldIsEveryLoadedAddress() {
	scene := s.newHeldScene(2, s.faerieFireJSON(informGoblin1), s.proneJSON(informGoblin1))
	rogue := s.heldRogue()
	bus := events.NewEventBus()
	rolled := s.watchAttackFrames(bus)

	_, err := s.strikeHeld(bus, scene, dagger(), 18)
	s.Require().NoError(err)
	s.Require().NotEmpty(*rolled)
	frame := (*rolled)[0]

	for member, stored := range map[string][]json.RawMessage{
		informRogue: rogue.Conditions, informGoblin1: scene.goblin.Conditions, informGoblin2: scene.other.Conditions,
	} {
		addresses, err := conditions.HeldAddresses(member, stored)
		s.Require().NoError(err)
		want := []contributions.HeldCondition{}
		for _, address := range addresses {
			want = append(want, contributions.HeldCondition{Ref: address.ConditionRef, SourceID: address.SourceID})
		}
		held, known := frame.HeldBy(member)
		s.True(known, "%s has a sheet, so what it holds is known", member)
		s.Equal(want, held, "%s's execution held list is what its record says it holds", member)
	}
	goblin, _ := frame.HeldBy(informGoblin1)
	s.Contains(goblin, contributions.HeldCondition{Ref: refs.Conditions.FaerieFire().String(), SourceID: heldCaster},
		"the goblin's Faerie Fire keeps its caster as source")
}

// TestUnreadableStoredConditionFailsTheStrike: the execution frame lists what
// the loaded sheets hold, so it can be no more honest than the load. A target
// whose stored Faerie Fire does not load is refused before any step runs —
// the strike fails loudly, and the goblin is never framed as holding less
// than its record says.
func (s *FrameTestSuite) TestUnreadableStoredConditionFailsTheStrike() {
	unreadable := json.RawMessage(`{"ref":"` + refs.Conditions.FaerieFire().String() + `"}`)
	_, err := conditions.LoadJSON(unreadable)
	s.Require().Error(err, "the blob names a condition that does not load")
	scene := s.newHeldScene(2, unreadable)
	before := scene.goblin.HitPoints
	bus := events.NewEventBus()
	rolled := s.watchAttackFrames(bus)

	out, err := s.strikeHeld(bus, scene, dagger(), 18)

	s.Require().Error(err)
	s.Contains(err.Error(), informGoblin1)
	s.Nil(out)
	s.Empty(*rolled, "no frame was built from a sheet that could not be read")
	s.Equal(before, scene.goblin.HitPoints)
}

// TestStrikeFailsWhenSightCannotBeAnswered is R13: the target holds Faerie
// Fire and the run cannot say whether the rogue sees it. The strike's sight
// rule reads that unknown off the execution frame and fails the strike before
// the attack chain folds, rather than counting the target as seen; and the
// Faerie Fire rule, asked over the same unknown, cannot answer either, so no
// path grants or withholds its advantage silently.
func (s *FrameTestSuite) TestStrikeFailsWhenSightCannotBeAnswered() {
	scene := s.newHeldScene(2, s.faerieFireJSON(informGoblin1))
	scene.sight = failingSight{}
	before := scene.goblin.HitPoints
	bus := events.NewEventBus()
	rolled := s.watchAttackFrames(bus)

	out, err := s.strikeHeld(bus, scene, dagger(), 18)

	s.Require().Error(err)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer), "%v", err)
	s.Contains(err.Error(), "sight", "the sight rule is the first to find it cannot answer")
	s.Nil(out, "nothing comes back to be saved")
	s.Equal(before, scene.goblin.HitPoints)
	s.Empty(*rolled, "the attack chain never folded")

	fire := contributions.HeldCondition{Ref: refs.Conditions.FaerieFire().String(), SourceID: heldCaster}
	_, err = conditions.ExecuteHeldEffect(&conditions.ExecuteHeldEffectInput{
		Holder: informGoblin1,
		Held:   fire,
		Frame: contributions.Frame{
			Actor:  informRogue,
			Target: contributions.Known(informGoblin1),
			Action: attackActionFacts(dagger().Attack, false),
			Pairs:  []contributions.PairFacts{{From: informRogue, To: informGoblin1, Sees: contributions.Unknown[bool]()}},
			Held:   []contributions.MemberHeld{{Member: informGoblin1, Conditions: []contributions.HeldCondition{fire}}},
		},
	})
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer), "Faerie Fire cannot answer an unknown sight either: %v", err)
}

// TestStrikeWardsComeFromTheHeldRule: the strike's ward selection is the held
// rule's answer over the frame, not a second predicate over the sheet — a
// ward the frame does not show is not a ward the strike invents, and a frame
// that cannot say what the target holds fails the strike.
func (s *FrameTestSuite) TestStrikeWardsComeFromTheHeldRule() {
	target := actionHero()
	target.Conditions = []json.RawMessage{sanctuaryJSON(s.T(), heroID)}
	sheet, err := character.Load(s.ctx, target)
	s.Require().NoError(err)
	cast := &Participants{characters: map[string]*character.Character{heroID: sheet}, order: []string{heroID, wolfID}}
	ward := contributions.HeldCondition{Ref: refs.Conditions.Sanctuary().String(), SourceID: "cleric-1"}
	frame := func(actor string, held ...contributions.MemberHeld) contributions.Frame {
		return contributions.Frame{
			Actor:  actor,
			Target: contributions.Known(heroID),
			Action: attackActionFacts(validMeleeDefinition().Attack, false),
			Held:   held,
		}
	}

	wards, err := strikeWards(frame(wolfID, contributions.MemberHeld{Member: heroID, Conditions: []contributions.HeldCondition{ward}}), cast, heroID)
	s.Require().NoError(err)
	s.Equal([]string{"cleric-1"}, wardSources(wards), "the wolf must save against the cleric's ward")

	wards, err = strikeWards(frame(heroID, contributions.MemberHeld{Member: heroID, Conditions: []contributions.HeldCondition{ward}}), cast, heroID)
	s.Require().NoError(err)
	s.Empty(wards, "the holder's own ward does not stop its own attack")

	_, err = strikeWards(frame(wolfID, contributions.MemberHeld{Member: heroID, Conditions: []contributions.HeldCondition{}}), cast, heroID)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer),
		"a frame that lists the holder without the ward it applies cannot answer for it: %v", err)

	_, err = strikeWards(frame(wolfID), cast, heroID)
	s.True(errors.Is(err, contributions.ErrRuleCannotAnswer), "unknown holdings fail the strike: %v", err)
}

func wardSources(wards []*conditions.SanctuaryCondition) []string {
	sources := make([]string, 0, len(wards))
	for _, ward := range wards {
		sources = append(sources, ward.SourceID)
	}
	return sources
}

// TestStrikeSanctuaryWardsComeFromTheHeldRule: through the whole strike, a
// warded target makes the attacker save first, and an attacker's own ward
// asks nothing of it.
func (s *FrameTestSuite) TestStrikeSanctuaryWardsComeFromTheHeldRule() {
	warded := actionHero()
	warded.Conditions = []json.RawMessage{sanctuaryJSON(s.T(), heroID)}
	out, err := resolveHeroStrikeAs(s, wolfID, heroID, warded, facedRoller{d20: 1, other: 1})
	s.Require().NoError(err)
	outcome := out.Outcome.(StrikeOutcome)
	s.Require().NotNil(outcome.Warded, "the failed ward save lost the attack")
	s.Equal("cleric-1", outcome.Warded.SourceID)

	attacker := actionHero()
	attacker.Conditions = []json.RawMessage{sanctuaryJSON(s.T(), heroID)}
	out, err = resolveHeroStrikeAs(s, heroID, wolfID, attacker, facedRoller{d20: 1, other: 1})
	s.Require().NoError(err)
	s.Nil(out.Outcome.(StrikeOutcome).Warded, "an attacker's own ward asks no save")
}

// TestStrikeWardCheckReadsTheExecutionFrame: the ward check asks the held rule
// over the strike's execution frame, so the frame exists before any ward save
// is rolled. A warded target whose holdings cannot be framed — its record
// lists the same Prone twice, which no frame may carry — fails the strike
// with no save asked. A ward check that read the sheet on its own would roll
// the attacker's save, lose the attack and report a ward that no frame ever
// agreed to.
func (s *FrameTestSuite) TestStrikeWardCheckReadsTheExecutionFrame() {
	prone, err := conditions.NewProneCondition(heroID).ToJSON()
	s.Require().NoError(err)
	warded := actionHero()
	warded.Conditions = []json.RawMessage{sanctuaryJSON(s.T(), heroID), prone, prone}

	out, err := resolveHeroStrikeAs(s, wolfID, heroID, warded, facedRoller{d20: 1, other: 1})

	s.Require().Error(err, "the frame the ward rule reads cannot be built: %+v", out)
	s.Contains(err.Error(), "attack frame")
	s.Nil(out, "no ward outcome, no world to save")
}

// resolveHeroStrikeAs runs one strike between the hero and the wolf, with the
// cleric whose ward is on the hero's sheet present to answer its save DC.
func resolveHeroStrikeAs(
	s *FrameTestSuite, attackerID, targetID string, hero *character.Data, roller facedRoller,
) (*Output, error) {
	definition := validMeleeDefinition()
	return resolveOn(s.ctx, &Input{
		World: actionWorld(s.T(), 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: hero}, {Character: clericWarder()},
		},
		Machine: NewStrike(&StrikeInput{AttackerID: attackerID, TargetID: targetID, Definition: definition, Roller: roller}),
		Capabilities: encounter.Capabilities{
			Initiative: orderAsGiven{},
			Standing:   everyoneStanding{},
			Sight:      everyoneSeesTheWholeMap{},
			Equipment:  noHandsAreObserved{},
			Sheets:     noSheetsAsked{},
			Driver:     passDriver{},
			Roller:     roller,
			Actors:     Actors,
		},
	}, newSurface(events.NewEventBus()))
}
