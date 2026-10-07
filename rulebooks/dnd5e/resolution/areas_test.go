// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// inFog is the membership label Fog Cloud's area carries. No sheet may ever
// hold it: membership is the encounter's answer, asked at use.
var inFog = refs.Conditions.InFog().String()

// fogCloudArea is the area a bard's Fog Cloud stands as, centred on the hero.
func fogCloudArea(casterID string) encounter.SightAreaData {
	id := casterID + "/concentration"
	return encounter.SightAreaData{
		ID: id, SourceID: casterID, Name: "Fog Cloud", Ref: refs.Spells.FogCloud().String(),
		Center: encounter.PositionData{X: 1, Y: 1}, RadiusFeet: 20,
		MembershipRef: inFog, MembershipName: "In Fog", MembershipSourceID: areaMembershipSourceID(id),
	}
}

// noSheetHoldsAMembership fails if any sheet the interaction handed back
// carries the area's membership label as a condition.
func (s *ConcentrationTestSuite) noSheetHoldsAMembership(out *Output) {
	for _, data := range out.DirtyCharacters {
		s.NotContains(storedRefs(s.T(), data.Conditions), inFog, "%s holds no membership", data.ID)
	}
	for _, data := range out.DirtyMonsters {
		s.NotContains(storedRefs(s.T(), data.Conditions), inFog, "%s holds no membership", data.ID)
	}
}

// A Fog Cloud cast reports the one area it opens, for the host to open on the
// live encounter, and writes no condition on any sheet — though every member
// on the board stands inside it. Resolution does not open it itself: the world
// it hands back carries the area set it was handed.
func (s *ConcentrationTestSuite) TestAFogCloudReportsOneOpenedAreaAndWritesNoSheet() {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.FogCloud, SpellSaveDC: spellSaveDC})
	s.Require().NotNil(definition)
	center := spatial.Position{X: 1, Y: 1}
	machine, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: bardID, AreaCenter: &center,
		Roller: facedRoller{d20: straightRoll, other: psychicFace},
	})
	s.Require().NoError(err)

	fixtures := s.fixtures()
	out, err := Resolve(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
		Roller: dice.NewRoller(), World: fixtures.world(), Machine: machine, Cost: castCost(),
		Participants: []Participant{
			{Character: fixtures.saver(14)}, {Monster: fixtures.wolfData()}, {Character: baneCaster(1, 2)},
		},
	})
	s.Require().NoError(err)

	s.Require().Len(out.OpenedAreas, 1, "one cast, one area")
	opened := out.OpenedAreas[0]
	s.Equal(bardID, opened.SourceID)
	s.Equal(center, opened.Center)
	s.Equal(20, opened.RadiusFeet)
	s.Equal(inFog, opened.MembershipRef, "the label is content's, carried by the area")
	s.Empty(out.ClosedAreas, "nothing was concentrating before")
	s.Empty(out.World.SightAreas, "the host opens it through the encounter's verb; resolution does not")
	s.noSheetHoldsAMembership(out)
}

// Breaking the caster's concentration on Fog Cloud reports its area closed —
// and a break by a caster who opened no area reports nothing. Nothing reads
// the spell's ref to decide it: the area set says who opened what.
func (s *ConcentrationTestSuite) TestBreakingConcentrationReportsTheAreaClosed() {
	hold := conditions.NewConcentratingCondition(heroID, refs.Spells.FogCloud().String(), "Fog Cloud", 600)
	holdJSON, err := hold.ToJSON()
	s.Require().NoError(err)

	strike := func(world encounter.EncounterData) *Output {
		fixtures := s.fixtures()
		// The claw's d20, then the concentration save's: CON +2 against DC
		// 10 fails on a 3.
		out, err := resolveOn(s.ctx, &Input{
			Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
			Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{},
			Roller: dice.NewRoller(), World: world,
			Participants: []Participant{{Character: fixtures.saver(40, holdJSON)}, {Monster: fixtures.wolfData()}},
			Machine: NewStrike(&StrikeInput{
				AttackerID: wolfID, TargetID: heroID, Definition: claw("1d6"),
				Roller: &sequenceRoller{singles: []int{straightRoll, straightRoll}, pair: []int{6}},
			}),
		}, newSurface(events.NewEventBus()))
		s.Require().NoError(err)
		s.Require().Len(out.ConcentrationBreaks, 1, "the hold broke")
		return out
	}

	world := s.fixtures().world()
	world.SightAreas = []encounter.SightAreaData{fogCloudArea(heroID)}
	out := strike(world)
	s.Equal([]string{heroID}, out.ClosedAreas)
	s.Empty(out.OpenedAreas)
	s.Equal(world.SightAreas, out.World.SightAreas, "the host ends it through the encounter's verb")
	s.noSheetHoldsAMembership(out)

	out = strike(s.fixtures().world())
	s.Empty(out.ClosedAreas, "a caster who opened no area closes none")
}

// Known-creature targeting decides from the encounter's believed-aim answer
// alone: the targeting code measures nothing and decodes no payload, and no
// production file in this module decodes a sight testimony at all.
func (s *ConcentrationTestSuite) TestKnownTargetingAsksTheEncounterAndDecodesNothing() {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	s.Require().NoError(err)

	measuring := map[string]bool{
		"Distance": true, "IsLineOfSightBlocked": true, "GetEntityPosition": true,
		"GetGrid": true, "CellsFromFeet": true, "View": true,
	}
	var offenders []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		s.Require().NoError(err)
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name == "DecodeSightTestimony" || (name == "known_targets.go" && measuring[sel.Sel.Name]) {
				offenders = append(offenders, fset.Position(sel.Pos()).String()+": "+sel.Sel.Name)
			}
			return true
		})
	}
	s.Empty(offenders, "the believed point, its range and its path are encounter.BelievedAim's answer")
}
