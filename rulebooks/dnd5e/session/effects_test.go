// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/proficiencies"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	erSession = "sess"
	erAlly    = "fighter"
	erGoblin1 = "goblin-1"
	erGoblin2 = "goblin-2"
	erGoblins = "goblins"
)

// EffectRowsSuite pins rpg-project#520's session half: Afford carries each
// Attack declaration's, and each spell-attack Cast declaration's, effect rows,
// with the answers that change by target on the candidates.
//
// Session carries and projects; every name, reason, benefit and description
// asserted here is the rulebook's own answer, so a test that fails here on a
// wording change is reading the content owner's change, not this seam's.
type EffectRowsSuite struct {
	suite.Suite

	ctx        context.Context
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
	actor      string
}

func TestEffectRowsSuite(t *testing.T) { suite.Run(t, new(EffectRowsSuite)) }

func (s *EffectRowsSuite) SetupTest() { s.ctx = context.Background() }

// cave seats the actor beside goblin one, the ally on goblin one's far side,
// and goblin two alone across the room — resolution's own Sneak Attack scene,
// played through the seam. The goblins are a declared faction with nothing
// said about the party, which the composition reads as hostile.
//
// testDice answers 10 on every d20; each actor fixture carries DEX 16 and an
// ID that wins the tie-break against the ally, so the actor acts first — and
// the scene asserts it rather than trusting the arithmetic.
func (s *EffectRowsSuite) cave(actor *character.Data) {
	s.T().Helper()
	s.actor = actor.ID
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(actor, armedFighter(erAlly))
	s.stream = &fakeStream{}

	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters, Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("cave", 0, 0, 10, 5)},
			Factions: []encounter.FactionInput{{ID: erGoblins}},
		},
		Members: []encounter.MemberInput{
			{ID: encounter.MemberID(actor.ID), Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: erAlly, Kind: encounter.KindPlayer, Position: spatial.Position{X: 3, Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()
	_, err = mgr.StartSession(s.ctx, &session.StartSessionInput{Session: erSession, Encounter: "world", World: &data})
	s.Require().NoError(err)

	for _, goblin := range []struct {
		id string
		at spatial.Position
	}{
		{erGoblin1, spatial.Position{X: 2, Y: 1}},
		{erGoblin2, spatial.Position{X: 7, Y: 3}},
	} {
		_, err := mgr.Spawn(s.ctx, &session.SpawnInput{
			Session: erSession, ID: goblin.id, Ref: refs.Monsters.Goblin().String(),
			Position: goblin.at, Faction: erGoblins,
		})
		s.Require().NoError(err)
	}

	turn, err := mgr.Turn(s.ctx, &session.TurnInput{Session: erSession, Member: actor.ID})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock, "precondition: the goblins started a fight")
	s.Require().Equal(actor.ID, turn.Active, "precondition: the actor acts first")
	s.stream.published = nil
}

func (s *EffectRowsSuite) afford(member string) *session.AffordOutput {
	s.T().Helper()
	out, err := s.mgr.Afford(s.ctx, &session.AffordInput{Session: erSession, Member: member})
	s.Require().NoError(err)
	return out
}

// mainAttack is the actor's main-hand Attack declaration.
func (s *EffectRowsSuite) mainAttack(out *session.AffordOutput) session.Declaration {
	s.T().Helper()
	for _, declaration := range out.Declarations {
		if declaration.Verb == session.VerbAttack && declaration.Attack != nil {
			return declaration
		}
	}
	s.Require().Fail("no compiled Attack declaration", "%+v", out.Declarations)
	return session.Declaration{}
}

// castOf is the Cast declaration for one spell.
func (s *EffectRowsSuite) castOf(out *session.AffordOutput, spell spells.Spell) session.Declaration {
	s.T().Helper()
	want := refs.Spells.ByID(string(spell)).String()
	for _, declaration := range out.Declarations {
		if declaration.Verb == session.VerbCast && declaration.Spell != nil && declaration.Spell.Ref == want {
			return declaration
		}
	}
	s.Require().Fail("no Cast declaration", "for %s in %+v", spell, out.Declarations)
	return session.Declaration{}
}

// row finds the declaration row with the given id.
func (s *EffectRowsSuite) row(declaration session.Declaration, id string) session.EffectRow {
	s.T().Helper()
	for _, row := range declaration.Effects {
		if row.ID == id {
			return row
		}
	}
	s.Require().Fail("no effect row", "%s in %+v", id, declaration.Effects)
	return session.EffectRow{}
}

// candidate finds one candidate on a declaration.
func (s *EffectRowsSuite) candidate(declaration session.Declaration, member string) session.TargetCandidate {
	s.T().Helper()
	for _, candidate := range declaration.Candidates {
		if candidate.Member == member {
			return candidate
		}
	}
	s.Require().Fail("no candidate", "%s in %+v", member, declaration.Candidates)
	return session.TargetCandidate{}
}

func (s *EffectRowsSuite) raw(c interface {
	ToJSON() (json.RawMessage, error)
}) json.RawMessage {
	s.T().Helper()
	b, err := c.ToJSON()
	s.Require().NoError(err)
	return b
}

// blessedBy is a Bless on member cast by source.
func (s *EffectRowsSuite) blessedBy(member, source string) json.RawMessage {
	s.T().Helper()
	bless, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
		MemberID: member, SourceID: source, SourceRef: refs.Spells.Bless(),
	})
	s.Require().NoError(err)
	return s.raw(bless)
}

// landCondition adds a condition to a member's stored sheet, the way a cast
// that landed between two reads leaves it.
func (s *EffectRowsSuite) landCondition(member string, raw json.RawMessage) {
	s.T().Helper()
	stored, err := s.characters.GetCharacter(s.ctx, member)
	s.Require().NoError(err)
	stored.Conditions = append(stored.Conditions, raw)
	s.Require().NoError(s.characters.SaveCharacter(s.ctx, stored))
}

// rogue is a level-1 rogue with a shortsword and Sneak Attack, as resolution's
// own information test seats one.
func (s *EffectRowsSuite) rogue() *character.Data {
	sneak := s.raw(conditions.NewSneakAttackCondition(conditions.SneakAttackInput{MemberID: "alice", Level: 1}))
	return &character.Data{
		ID: "alice", PlayerID: "player-alice", Name: "alice", Level: 1,
		Levels: syntheticLevels(classes.Rogue, 1), ClassID: classes.Rogue, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, abilities.DEX: 16, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 10,
		},
		HitPoints: 10, MaxHitPoints: 10, ArmorClass: 14, ProficiencyBonus: 2,
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponShortsword},
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Shortsword), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{character.SlotMainHand: string(weapons.Shortsword)},
		Conditions:     []json.RawMessage{sneak},
	}
}

// barbarian is a raging level-1 barbarian swinging a greataxe.
func (s *EffectRowsSuite) barbarian() *character.Data {
	rage := s.raw(&conditions.RagingCondition{
		CharacterID: "alice", DamageBonus: 2, Level: 1, Source: refs.Features.Rage().String(),
	})
	return &character.Data{
		ID: "alice", PlayerID: "player-alice", Name: "alice", Level: 1,
		Levels: syntheticLevels(classes.Barbarian, 1), ClassID: classes.Barbarian, RaceID: races.Human,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 16, abilities.CON: 14,
			abilities.INT: 8, abilities.WIS: 10, abilities.CHA: 8,
		},
		HitPoints: 14, MaxHitPoints: 14, ArmorClass: 13, ProficiencyBonus: 2,
		WeaponProficiencies: []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial},
		Inventory: []character.InventoryItemData{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Greataxe), Quantity: 1},
		},
		EquipmentSlots: character.EquipmentSlots{character.SlotMainHand: string(weapons.Greataxe)},
		Conditions:     []json.RawMessage{rage},
	}
}

// fighter is armedFighter quick enough to act first.
func (s *EffectRowsSuite) fighter(effects ...json.RawMessage) *character.Data {
	alice := armedFighter("alice")
	alice.AbilityScores[abilities.DEX] = 16
	alice.Conditions = effects
	return alice
}

// blessedCleric knows Guiding Bolt (a spell attack) and Sacred Flame (a save)
// and has blessed themself.
func (s *EffectRowsSuite) blessedCleric() *character.Data {
	cleric := healingCleric()
	cleric.AbilityScores[abilities.DEX] = 16
	cleric.KnownSpells = append(cleric.KnownSpells, refs.Spells.GuidingBolt().String())
	cleric.Conditions = []json.RawMessage{s.blessedBy(cleric.ID, cleric.ID)}
	return cleric
}

func (s *EffectRowsSuite) TestAttackDeclarationCarriesRageRow() {
	s.cave(s.barbarian())

	attack := s.mainAttack(s.afford("alice"))
	rage := s.row(attack, refs.Conditions.Raging().String())

	display, found := conditions.DisplayFor(*refs.Conditions.Raging())
	s.Require().True(found)
	s.Equal(refs.Conditions.Raging().String(), rage.Ref)
	s.Equal("Raging", rage.Name)
	s.Equal(session.EffectApplies, rage.State)
	s.Equal("The melee weapon attack uses Strength", rage.Reason)
	s.Equal("+2 damage", rage.Benefit)
	s.Equal(session.ContributesNow, rage.Participation)
	s.Equal(display.Detail, rage.Description, "the description is the catalog's, carried verbatim")

	s.Require().NotEmpty(attack.Candidates)
	for _, candidate := range attack.Candidates {
		s.Empty(candidate.Effects, "%s: Rage answers the same for every target, so no candidate repeats it", candidate.Member)
	}
}

func (s *EffectRowsSuite) TestSneakAttackAnswersPerTarget() {
	s.cave(s.rogue())
	sneakID := refs.Features.SneakAttack().String()

	attack := s.mainAttack(s.afford("alice"))
	sneak := s.row(attack, sneakID)
	s.Equal(session.EffectDepends, sneak.State)
	s.Equal("Depends on the target", sneak.Reason)
	s.Empty(sneak.Benefit)

	beside := s.candidate(attack, erGoblin1)
	s.Require().Len(beside.Effects, 1, "only the answer that changes rides the candidate")
	s.Equal(session.TargetEffect{
		ID: sneakID, State: session.EffectApplies,
		Reason: "Another enemy of the target is within 5 feet", Benefit: "+1d6 damage",
	}, beside.Effects[0])

	alone := s.candidate(attack, erGoblin2)
	s.Require().Len(alone.Effects, 1)
	s.Equal(session.TargetEffect{
		ID: sneakID, State: session.EffectDepends,
		Reason: "Needs advantage or another enemy of the target within 5 feet",
	}, alone.Effects[0])
}

func (s *EffectRowsSuite) TestBlessAndInspirationRows() {
	s.cave(s.fighter(
		s.blessedBy("alice", erAlly),
		s.raw(conditions.NewInspiredCondition("alice", erAlly, conditions.InspiredDie)),
	))

	attack := s.mainAttack(s.afford("alice"))

	bless := s.row(attack, refs.Conditions.Blessed().String()+"@"+erAlly)
	s.Equal(session.EffectApplies, bless.State)
	s.Equal("+1d4 to the attack roll", bless.Benefit)
	s.Equal(session.ContributesNow, bless.Participation)

	inspired := s.row(attack, refs.Conditions.Inspired().String())
	s.Equal(session.EffectApplies, inspired.State)
	s.Equal(session.LaterChoice, inspired.Participation, "available after the roll, never already added")
	s.Equal("May add "+conditions.InspiredDie+" after seeing the roll", inspired.Benefit)
}

func (s *EffectRowsSuite) TestUnavailableAttackStillCarriesRows() {
	s.cave(s.barbarian())
	_, err := s.mgr.Attack(s.ctx, &session.AttackInput{
		Session: erSession, Attacker: "alice", Target: erGoblin1,
		DeclarationID: s.mainAttack(s.afford("alice")).ID,
	})
	s.Require().NoError(err)

	attack := s.mainAttack(s.afford("alice"))
	s.Require().False(attack.Available, "precondition: the turn's one swing is spent")
	s.Equal(session.EffectApplies, s.row(attack, refs.Conditions.Raging().String()).State,
		"availability and applicability are independent")
}

func (s *EffectRowsSuite) TestNoRowsOffTurnOrWhileFrozen() {
	s.cave(s.fighter(s.raw(conditions.NewInspiredCondition("alice", erAlly, conditions.InspiredDie))))
	s.Require().NotEmpty(s.mainAttack(s.afford("alice")).Effects, "precondition: on turn the swing has rows")

	for _, declaration := range s.afford(erAlly).Declarations {
		s.Empty(declaration.Effects, "off turn: %s", declaration.Verb)
		for _, candidate := range declaration.Candidates {
			s.Empty(candidate.HeldEffects, "off turn: %s -> %s", declaration.Verb, candidate.Member)
		}
	}

	// Inspiration's post-roll offer opens a window: the table waits on alice.
	out, err := s.mgr.Attack(s.ctx, &session.AttackInput{
		Session: erSession, Attacker: "alice", Target: erGoblin1,
		DeclarationID: s.mainAttack(s.afford("alice")).ID,
	})
	s.Require().NoError(err)
	s.Require().True(out.Paused, "precondition: the roll paused on the offer")

	for _, member := range []string{"alice", erAlly} {
		frozen := s.afford(member)
		for _, declaration := range frozen.Declarations {
			s.Empty(declaration.Effects, "%s while frozen: %s", member, declaration.Verb)
			for _, candidate := range declaration.Candidates {
				s.Empty(candidate.Effects, "%s while frozen: %s -> %s", member, declaration.Verb, candidate.Member)
				s.Empty(candidate.HeldEffects, "%s while frozen: %s -> %s", member, declaration.Verb, candidate.Member)
			}
		}
	}
}

func (s *EffectRowsSuite) TestNonAttackDeclarationsCarryNoRows() {
	s.cave(s.blessedCleric())
	out := s.afford("cleric")

	seen := map[session.Verb]bool{}
	for _, declaration := range out.Declarations {
		switch declaration.Verb {
		case session.VerbMove, session.VerbEndTurn, session.VerbActivate:
			seen[declaration.Verb] = true
			s.Empty(declaration.Effects, "%s carries no action content to describe", declaration.Verb)
		}
	}
	s.True(seen[session.VerbMove] && seen[session.VerbEndTurn] && seen[session.VerbActivate],
		"precondition: Move, EndTurn and Activate were offered: %v", seen)
	s.Require().NotEmpty(s.castOf(out, spells.GuidingBolt).Effects,
		"precondition: the same blessed cleric's spell attack does carry rows")

	sacredFlame := s.castOf(out, spells.SacredFlame)
	s.Empty(sacredFlame.Effects, "a save cast makes no attack roll")
	for _, candidate := range sacredFlame.Candidates {
		s.Empty(candidate.Effects, candidate.Member)
	}
}

func (s *EffectRowsSuite) TestSpellAttackCastCarriesBlessRow() {
	s.cave(s.blessedCleric())

	guidingBolt := s.castOf(s.afford("cleric"), spells.GuidingBolt)
	bless := s.row(guidingBolt, refs.Conditions.Blessed().String()+"@cleric")
	s.Equal(session.EffectApplies, bless.State)
	s.Equal("+1d4 to the attack roll", bless.Benefit)
}

// TestUnansweringEffectShownUnavailable: Sanctuary bears on its holder's own
// attack (attacking ends the ward) but no rule here answers for it yet, so its
// row is unavailable — shown, never dropped and never not-applying.
func (s *EffectRowsSuite) TestUnansweringEffectShownUnavailable() {
	ward, err := conditions.NewSanctuaryCondition(conditions.NewSanctuaryConditionInput{
		MemberID: "alice", SourceID: erAlly, SourceRef: refs.Spells.Sanctuary(),
	})
	s.Require().NoError(err)
	s.cave(s.fighter(s.raw(ward)))

	sanctuary := s.row(s.mainAttack(s.afford("alice")), refs.Conditions.Sanctuary().String()+"@"+erAlly)
	display, found := conditions.DisplayFor(*refs.Conditions.Sanctuary())
	s.Require().True(found)
	s.Equal(session.EffectUnavailable, sanctuary.State, "never dropped and never shown as not applying")
	s.Equal(display.Detail, sanctuary.Description)
	s.NotEmpty(sanctuary.Reason)
}

func (s *EffectRowsSuite) TestReadingRowsChangesNothing() {
	s.cave(s.rogue())
	saves := s.characters.saves
	before, err := json.Marshal(s.characters.byID["alice"])
	s.Require().NoError(err)

	first := s.mainAttack(s.afford("alice"))
	s.Require().NotEmpty(first.Effects, "precondition: there were rows to read")
	s.mainAttack(s.afford("alice"))

	s.Equal(saves, s.characters.saves, "reading rows saves nothing")
	after, err := json.Marshal(s.characters.byID["alice"])
	s.Require().NoError(err)
	s.JSONEq(string(before), string(after), "and spends nothing: Sneak Attack is still unused")

	_, err = s.mgr.Attack(s.ctx, &session.AttackInput{
		Session: erSession, Attacker: "alice", Target: erGoblin1, DeclarationID: first.ID,
	})
	s.Require().NoError(err)
	struck := eventsOfKind(s.stream.published, "alice", session.EventStruck)
	s.Require().Len(struck, 1)
	body, ok := struck[0].Body.(session.StruckBody)
	s.Require().True(ok)
	sneak := false
	for _, component := range body.DamageComponents {
		if component.Roll.Source.Ref == refs.Features.SneakAttack().String() {
			sneak = true
		}
	}
	s.True(sneak, "the swing still rolled Sneak Attack: %+v", body.DamageComponents)
}

func (s *EffectRowsSuite) TestEffectsAreNotSelectorMaterial() {
	s.cave(s.fighter())
	before := s.mainAttack(s.afford("alice"))
	s.Empty(before.Effects)

	s.landCondition("alice", s.blessedBy("alice", erAlly))

	after := s.mainAttack(s.afford("alice"))
	s.Require().NotEmpty(after.Effects, "precondition: Bless landed")
	s.Equal(before.ID, after.ID, "rows describe the offer; they are not part of what selects it")
}

func (s *EffectRowsSuite) TestEffectRowWireKeys() {
	s.cave(s.rogue())
	attack := s.mainAttack(s.afford("alice"))

	raw, err := json.Marshal(attack)
	s.Require().NoError(err)
	var wire struct {
		Effects    []map[string]json.RawMessage `json:"effects"`
		Candidates []struct {
			Member  string                       `json:"member"`
			Effects []map[string]json.RawMessage `json:"effects"`
		} `json:"candidates"`
	}
	s.Require().NoError(json.Unmarshal(raw, &wire))
	s.Require().NotEmpty(wire.Effects, "%s", raw)
	for _, key := range []string{"id", "ref", "name", "description", "state", "reason", "participation"} {
		s.Contains(wire.Effects[0], key, "%s", raw)
	}
	s.JSONEq(`"depends"`, string(wire.Effects[0]["state"]))
	s.JSONEq(`"contributes_now"`, string(wire.Effects[0]["participation"]))

	answered := false
	for _, candidate := range wire.Candidates {
		if candidate.Member != erGoblin1 {
			continue
		}
		s.Require().Len(candidate.Effects, 1, "%s", raw)
		for _, key := range []string{"id", "state", "reason", "benefit"} {
			s.Contains(candidate.Effects[0], key, "%s", raw)
		}
		s.NotContains(candidate.Effects[0], "description", "a target answer never repeats the description")
		s.NotContains(candidate.Effects[0], "participation", "nor the participation")
		answered = true
	}
	s.True(answered, "goblin one answered: %s", raw)
}

// TestAttachRunsOnlyInAfford is the structural half of "rows can never refuse
// a command": across every non-test source file in this package, the one call
// to attachEffects sits inside Manager.Afford. The execution callers of
// compileOffersFor (attack, cast, activate, move, death save) therefore never
// compute a row, so nothing they decide can read one.
func (s *EffectRowsSuite) TestAttachRunsOnlyInAfford() {
	files, err := filepath.Glob("*.go")
	s.Require().NoError(err)
	callers := map[string]int{}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		s.Require().NoError(err)
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "attachEffects" {
						callers[fn.Name.Name]++
					}
				}
				return true
			})
		}
	}
	s.Equal(map[string]int{"Afford": 1}, callers)
}

// TestNoRowsOnTheWorldClock closes the third early return: a blessed fighter
// in free roam is offered the social verbs and nothing on them carries a row,
// because the world clock compiles no attack.
func (s *EffectRowsSuite) TestNoRowsOnTheWorldClock() {
	s.characters = newFakeCharacters(s.fighter(s.blessedBy("alice", "bob")), armedFighter("bob"))
	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: s.characters,
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(s.ctx, &session.StartSessionInput{
		Session: erSession, Encounter: "world", World: freeRoamDuelWorld(s.T()),
	})
	s.Require().NoError(err)

	out, err := mgr.Afford(s.ctx, &session.AffordInput{Session: erSession, Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, out.Clock, "precondition: free roam")
	s.Require().NotEmpty(out.Declarations, "precondition: the social verbs are offered")
	for _, declaration := range out.Declarations {
		s.Empty(declaration.Effects, "world clock: %s", declaration.Verb)
		for _, candidate := range declaration.Candidates {
			s.Empty(candidate.Effects, "world clock: %s -> %s", declaration.Verb, candidate.Member)
		}
	}
}

// TestAffordFailsClosedWhenRowsCannotBeRead pins the fail-closed contract on a
// genuinely inconsistent sheet. A doubled Rage cannot arise from play: the
// rulebook refuses Rage while one is already active (features.Rage
// CanActivate, "already raging"), and the character sheet replaces a
// condition applied at an identity it already holds —
// TestRagingAgainIsRefusedAndThePanelSurvives walks that path. Loading a
// stored sheet does not pass through that door, so the duplicate is staged as
// stored data. Everything Afford compiles before the rows still compiles —
// the sheet loads, the offers price — and the rulebook then refuses to list
// two rows under one id. Afford returns that error rather than a panel with
// some rows missing, because an empty list reads as "nothing bears on this
// attack", which would be false.
func (s *EffectRowsSuite) TestAffordFailsClosedWhenRowsCannotBeRead() {
	barbarian := s.barbarian()
	s.cave(barbarian)
	// Planted after the fight starts: a sight refresh would refuse the
	// repeated address first (the encounter's own conditions door), and this
	// test is about the read.
	stored := s.characters.byID["alice"]
	stored.Conditions = append(stored.Conditions, append(json.RawMessage(nil), stored.Conditions[0]...))

	out, err := s.mgr.Afford(s.ctx, &session.AffordInput{Session: erSession, Member: "alice"})
	s.Require().Error(err, "a row the rulebook cannot list fails the read")
	s.Nil(out, "and no partial panel is returned beside it")
	// The frame refuses the repeated holding before any row is listed.
	s.Contains(err.Error(), `"dnd5e:conditions:raging"@"" twice`)
}

// TestRagingAgainIsRefusedAndThePanelSurvives drives the play path that once
// produced a doubled Rage, with nothing planted: a barbarian with two charges
// rages and swings, the turn comes back round, and the rulebook (features.Rage
// CanActivate, "already raging") now refuses the second Rage. The Rage row
// reads unavailable with that reason, activating it anyway is refused, the
// sheet still holds one Raging and one spent charge, and Afford keeps
// returning the whole panel with the Raging row on Attack.
func (s *EffectRowsSuite) TestRagingAgainIsRefusedAndThePanelSurvives() {
	barbarian := ragingBarbarian("alice", 2)
	barbarian.AbilityScores[abilities.DEX] = 16
	barbarian.WeaponProficiencies = []proficiencies.Weapon{proficiencies.WeaponSimple, proficiencies.WeaponMartial}
	s.cave(barbarian)
	rageRef := refs.Features.Rage().String()

	rage := activationFor(s.T(), s.afford("alice").Declarations, rageRef)
	s.Require().True(rage.Available, "precondition: the first Rage is offered")
	_, err := s.mgr.Activate(s.ctx, &session.ActivateInput{Session: erSession, Member: "alice", DeclarationID: rage.ID})
	s.Require().NoError(err)
	// A swing keeps the rage up past the turn's end.
	_, err = s.mgr.Attack(s.ctx, &session.AttackInput{
		Session: erSession, Attacker: "alice", Target: erGoblin1,
		DeclarationID: s.mainAttack(s.afford("alice")).ID,
	})
	s.Require().NoError(err)

	for _, member := range []string{"alice", erAlly} {
		_, err := s.mgr.EndTurn(s.ctx, &session.EndTurnInput{
			Session: erSession, Member: member, DeclarationID: currentEndTurnID(s.T(), s.mgr, erSession, member),
		})
		s.Require().NoError(err, "ending %s's turn", member)
	}
	turn, err := s.mgr.Turn(s.ctx, &session.TurnInput{Session: erSession, Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal("alice", turn.Active, "precondition: the turn came back round")
	s.Require().Equal(2, turn.Round)

	out := s.afford("alice")
	again := activationFor(s.T(), out.Declarations, rageRef)
	s.False(again.Available, "a barbarian already raging is not offered a second Rage")
	s.Require().NotNil(again.Why)
	s.Contains(again.Why.Text, "already raging", "the rulebook's own reason, carried verbatim")

	_, err = s.mgr.Activate(s.ctx, &session.ActivateInput{Session: erSession, Member: "alice", DeclarationID: again.ID})
	s.Require().Error(err, "and activating it anyway is refused")

	ragings := 0
	for _, ref := range storedConditionRefs(s.T(), s.characters, "alice") {
		if ref == refs.Conditions.Raging().String() {
			ragings++
		}
	}
	s.Equal(1, ragings, "the sheet holds one Raging")
	s.Equal(1, storedSheet(s.T(), s.characters, "alice").Resources[resources.RageCharges].Current,
		"one charge spent, by the first Rage only")

	for _, panel := range []*session.AffordOutput{out, s.afford("alice")} {
		verbs := map[session.Verb]bool{}
		for _, declaration := range panel.Declarations {
			verbs[declaration.Verb] = true
		}
		s.True(verbs[session.VerbAttack] && verbs[session.VerbEndTurn] && verbs[session.VerbMove],
			"the whole panel survives: %v", verbs)
		s.Equal(session.EffectApplies, s.row(s.mainAttack(panel), refs.Conditions.Raging().String()).State)
	}
}
