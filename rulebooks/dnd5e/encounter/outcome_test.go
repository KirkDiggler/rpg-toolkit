// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"math"
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// OutcomeTestSuite covers Record: the one way a rule resolved somewhere else
// reaches this encounter's story.
type OutcomeTestSuite struct {
	suite.Suite
}

func TestOutcomeSuite(t *testing.T) {
	suite.Run(t, new(OutcomeTestSuite))
}

const outcomeRoom = "yard"

// scene opens a room with alice and a goblin standing apart, out of each
// other's sight, so nothing forms a fight before a test asks for one.
func (s *OutcomeTestSuite) scene() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{Canvas: encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()}, Regions: []encounter.RegionInput{rectRegion(outcomeRoom, 0, 0, 12, 12)}, Props: wallRow(6, 4, 8)},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// TestARuleResolvedElsewhereReachesTheStory is why Record exists.
//
// A strike resolves in another module entirely: it returns a value and writes
// no beat, and this package's appendBeat is unexported. Before Record there
// was no way for that to become something a player could read — the SDK builds
// every client event by reading each member's story, so an attack was
// invisible in both places at once (rpg-toolkit#966).
func (s *OutcomeTestSuite) TestARuleResolvedElsewhereReachesTheStory() {
	enc := s.scene()

	out, err := enc.Record(&encounter.RecordInput{
		Kind:    encounter.OutcomeStruck,
		Actor:   alice,
		Targets: []encounter.MemberID{goblin},
		Values: map[encounter.OutcomeValue]int{
			encounter.ValueRoll:    17,
			encounter.ValueTotal:   22,
			encounter.ValueAgainst: 15,
			encounter.ValueAmount:  9,
		},
	})
	s.Require().NoError(err)
	s.NotZero(out.Seq)

	story, err := enc.Story(&encounter.StoryInput{Audience: goblin})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)
	last := story[len(story)-1]
	s.Equal(out.Seq, last.Seq, "RecordOutput.Seq references the beat it wrote")
	s.Equal("outcome", last.Tags["tag"])

	var beat map[string]any
	s.Require().NoError(json.Unmarshal(last.Payload, &beat))
	s.Equal("struck", beat["beat"])
	s.Equal("alice", beat["actor"])
	s.Equal([]any{"goblin"}, beat["targets"])
	s.Equal(float64(17), beat["roll"])
	s.Equal(float64(22), beat["total"])
	s.Equal(float64(15), beat["against"])
	s.Equal(float64(9), beat["amount"])
}

// TestARecordedStrikeCarriesWhatWasSwung pins rpg-toolkit#866/#941's half of
// Record: Critical and Attack are carried into the beat's payload exactly as
// the numeric Values already are, so a witness who was not the one swinging
// still learns what hit — a longsword, dealing slashing — from the SAME beat
// everyone else reads, not from a second channel only the attacker sees.
func (s *OutcomeTestSuite) TestARecordedStrikeCarriesWhatWasSwung() {
	enc := s.scene()

	_, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{goblin},
		Values: map[encounter.OutcomeValue]int{
			encounter.ValueRoll: 20, encounter.ValueTotal: 25, encounter.ValueAgainst: 15, encounter.ValueAmount: 12,
		},
		Critical: true,
		Attack:   &encounter.AttackIdentity{Ref: "longsword", Name: "Longsword", DamageType: "slashing"},
	})
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: goblin})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)

	var beat map[string]any
	s.Require().NoError(json.Unmarshal(story[len(story)-1].Payload, &beat))
	s.Equal(true, beat["critical"])
	attack, ok := beat["attack"].(map[string]any)
	s.Require().True(ok, "attack identity present in the payload")
	s.Equal("longsword", attack["ref"])
	s.Equal("Longsword", attack["name"])
	s.Equal("slashing", attack["damage_type"])
}

// TestARecordedStrikeCarriesOrderedDetail pins the closed primitive carrier
// that lets session replay the rulebook's strike evidence without giving this
// composition rule imports or a prose channel. A multiplier-only component
// still names its source on Roll — the same shape root's own immunity trait
// produces — because every persisted roll fact names who provided it.
func (s *OutcomeTestSuite) TestARecordedStrikeCarriesOrderedDetail() {
	immunity := 0.0
	in := &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice,
		Targets: []encounter.MemberID{goblin},
		DamageComponents: []encounter.DamageComponent{
			{
				Source: "weapon",
				Roll: encounter.RollComponent{
					Source: encounter.RollSource{Ref: "dnd5e:weapons:longsword", Name: "Longsword"},
					Dice: &encounter.DiceTrace{
						Notation: "1d8", DieSize: 8,
						OriginalRolls: []int{4}, FinalRolls: []int{4}, Subtotal: 4,
					},
				},
				DamageType: "slashing",
			},
			{
				Source: "monster_trait",
				Roll: encounter.RollComponent{
					Source: encounter.RollSource{Ref: "dnd5e:monster-traits:immunity", Name: "Immunity"},
				},
				DamageType: "slashing", Multiplier: &immunity,
			},
		},
		AdvantageSources: []encounter.AttackModifierSource{
			{SourceRef: "dnd5e:conditions:hidden", SourceID: "alice"},
		},
		DisadvantageSources: []encounter.AttackModifierSource{
			{SourceRef: "dnd5e:conditions:dodging", SourceID: "goblin"},
		},
	}

	record := func() []byte {
		enc := s.scene()
		_, err := enc.Record(in)
		s.Require().NoError(err)
		story, serr := enc.Story(&encounter.StoryInput{Audience: goblin})
		s.Require().NoError(serr)
		s.Require().NotEmpty(story)
		return story[len(story)-1].Payload
	}

	first := record()
	second := record()
	s.Equal(first, second, "identical ordered input produces identical story bytes")
	s.NotContains(string(first), `"reason"`, "the carrier has no prose field")

	var beat struct {
		DamageComponents    []encounter.DamageComponent      `json:"damage_components"`
		AdvantageSources    []encounter.AttackModifierSource `json:"advantage_sources"`
		DisadvantageSources []encounter.AttackModifierSource `json:"disadvantage_sources"`
	}
	s.Require().NoError(json.Unmarshal(first, &beat))
	s.Equal(in.DamageComponents, beat.DamageComponents)
	s.Equal(in.AdvantageSources, beat.AdvantageSources)
	s.Equal(in.DisadvantageSources, beat.DisadvantageSources)
	s.Require().NotNil(beat.DamageComponents[1].Multiplier,
		"zero is a present immunity multiplier, not an absent multiplier")
	s.Zero(*beat.DamageComponents[1].Multiplier)

	// New writes carry only the roll representation: the component's closed
	// key set is exactly the damage facts plus the nested roll. The legacy
	// flat fields are session's read concern, never this transcript's write.
	var rawBeat struct {
		DamageComponents []map[string]json.RawMessage `json:"damage_components"`
	}
	s.Require().NoError(json.Unmarshal(first, &rawBeat))
	s.Require().Len(rawBeat.DamageComponents, 2)
	componentKeys := make([]string, 0, 4)
	for key := range rawBeat.DamageComponents[0] {
		componentKeys = append(componentKeys, key)
	}
	sort.Strings(componentKeys)
	s.Equal([]string{"damage_type", "roll", "source"}, componentKeys,
		"a persisted damage component carries exactly its category, its roll, and its damage type")
}

// TestARecordedStruckDamageComponentCarriesOrderedRollFacts pins the GWF
// shape end to end: a struck outcome persists the ORIGINAL faces, the ordered
// sourced reroll, the FINAL faces, the authoritative subtotal, and the
// sourced modifier component — the facts a client needs to replay "2d6,
// rerolled the 1 into a 4, plus 3 Strength" without ever recomputing it.
func (s *OutcomeTestSuite) TestARecordedStruckDamageComponentCarriesOrderedRollFacts() {
	enc := s.scene()

	_, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: alice,
		Targets: []encounter.MemberID{goblin},
		Values: map[encounter.OutcomeValue]int{
			encounter.ValueRoll: 17, encounter.ValueTotal: 22,
			encounter.ValueAgainst: 15, encounter.ValueAmount: 12,
		},
		DamageComponents: gwfDamageComponents(),
	})
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: goblin})
	s.Require().NoError(err)
	s.Require().NotEmpty(story)

	var beat struct {
		DamageComponents []encounter.DamageComponent `json:"damage_components"`
	}
	s.Require().NoError(json.Unmarshal(story[len(story)-1].Payload, &beat))
	s.Require().Len(beat.DamageComponents, 2, "component order is preserved")

	weapon := beat.DamageComponents[0]
	s.Equal("weapon", weapon.Source)
	s.Equal("slashing", weapon.DamageType)
	s.Equal("dnd5e:weapons:greatsword", weapon.Roll.Source.Ref)
	s.Equal("Greatsword", weapon.Roll.Source.Name)
	s.Require().NotNil(weapon.Roll.Dice, "the weapon component rolled dice")
	s.Equal("2d6", weapon.Roll.Dice.Notation)
	s.Equal(6, weapon.Roll.Dice.DieSize)
	s.Equal([]int{1, 5}, weapon.Roll.Dice.OriginalRolls, "original faces survive")
	s.Require().Len(weapon.Roll.Dice.Rerolls, 1)
	s.Equal(0, weapon.Roll.Dice.Rerolls[0].DieIndex)
	s.Equal(1, weapon.Roll.Dice.Rerolls[0].Before)
	s.Equal(4, weapon.Roll.Dice.Rerolls[0].After)
	s.Equal("dnd5e:conditions:fighting_style_great_weapon_fighting",
		weapon.Roll.Dice.Rerolls[0].Source.Ref, "the reroll names its rule")
	s.Equal("Great Weapon Fighting", weapon.Roll.Dice.Rerolls[0].Source.Name)
	s.Equal([]int{4, 5}, weapon.Roll.Dice.FinalRolls, "final faces survive")
	s.Equal(9, weapon.Roll.Dice.Subtotal, "the provider's subtotal is authoritative")
	s.Nil(weapon.Roll.Modifier, "the dice component contributed no modifier")

	strength := beat.DamageComponents[1]
	s.Equal("ability", strength.Source)
	s.Nil(strength.Roll.Dice, "the modifier component rolled no dice")
	s.Require().NotNil(strength.Roll.Modifier)
	s.Equal(3, *strength.Roll.Modifier)
}

// TestRecordDamageComponentRollRefusals is the durable validation boundary
// for struck outcomes: every nested roll fact is validated BEFORE anything is
// appended, so a malformed component costs the transcript nothing. Each case
// mutates one field of an otherwise accepted payload, which is what makes the
// table non-vacuous — the unchanged payload is accepted by
// TestARecordedStruckDamageComponentCarriesOrderedRollFacts.
func (s *OutcomeTestSuite) TestRecordDamageComponentRollRefusals() {
	type testCase struct {
		name   string
		change func([]encounter.DamageComponent)
	}
	var cases []testCase
	add := func(name string, change func(*encounter.DamageComponent)) {
		cases = append(cases, testCase{name: name, change: func(cs []encounter.DamageComponent) {
			change(&cs[0])
		}})
	}

	add("roll source ref is missing", func(c *encounter.DamageComponent) {
		c.Roll.Source.Ref = ""
	})
	add("roll source ref is not a canonical ref", func(c *encounter.DamageComponent) {
		c.Roll.Source.Ref = "greatsword"
	})
	add("roll source name is missing", func(c *encounter.DamageComponent) {
		c.Roll.Source.Name = ""
	})
	add("dice notation is invalid", func(c *encounter.DamageComponent) {
		c.Roll.Dice.Notation = "not dice"
	})
	add("die size does not match notation", func(c *encounter.DamageComponent) {
		c.Roll.Dice.DieSize = 8
	})
	add("die size is not positive", func(c *encounter.DamageComponent) {
		c.Roll.Dice.DieSize = 0
	})
	add("original face is outside die range", func(c *encounter.DamageComponent) {
		c.Roll.Dice.OriginalRolls[0] = 0
	})
	add("final face is outside die range", func(c *encounter.DamageComponent) {
		c.Roll.Dice.FinalRolls[0] = 9
	})
	add("original and final cardinality differ", func(c *encounter.DamageComponent) {
		c.Roll.Dice.FinalRolls = []int{4}
	})
	add("reroll index is outside rolls", func(c *encounter.DamageComponent) {
		c.Roll.Dice.Rerolls[0].DieIndex = 2
	})
	add("reroll before does not match current face", func(c *encounter.DamageComponent) {
		c.Roll.Dice.Rerolls[0].Before = 2
	})
	add("reroll after is outside die range", func(c *encounter.DamageComponent) {
		c.Roll.Dice.Rerolls[0].After = 7
	})
	add("reroll after is not propagated to final rolls", func(c *encounter.DamageComponent) {
		c.Roll.Dice.FinalRolls[0] = 3
	})
	add("kept index is duplicated", func(c *encounter.DamageComponent) {
		c.Roll.Dice.KeptIndices = []int{0, 0}
	})
	add("kept index is outside final rolls", func(c *encounter.DamageComponent) {
		c.Roll.Dice.KeptIndices = []int{2}
	})
	add("dice subtotal does not equal kept faces", func(c *encounter.DamageComponent) {
		c.Roll.Dice.Subtotal = 8
	})
	add("roll has neither dice, modifier, nor multiplier", func(c *encounter.DamageComponent) {
		c.Roll.Dice = nil
		c.Multiplier = nil
	})

	for _, tc := range cases {
		tc := tc
		s.Run(tc.name, func() {
			enc := s.scene()
			beforeLog := enc.WorldView().Log

			components := gwfDamageComponents()
			tc.change(components)

			_, err := enc.Record(&encounter.RecordInput{
				Kind: encounter.OutcomeStruck, Actor: alice,
				Targets:          []encounter.MemberID{goblin},
				DamageComponents: components,
			})

			s.Require().ErrorIs(err, encounter.ErrInvalidData)
			s.Equal(beforeLog, enc.WorldView().Log,
				"a malformed component must leave the story untouched")
		})
	}

	s.Run("a malformed component after a valid one appends nothing", func() {
		enc := s.scene()
		beforeLog := enc.WorldView().Log

		components := gwfDamageComponents()
		components[1].Roll.Modifier = nil // second component loses its facts

		_, err := enc.Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice,
			Targets:          []encounter.MemberID{goblin},
			DamageComponents: components,
		})
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Equal(beforeLog, enc.WorldView().Log,
			"validation runs over every component before the first append")
	})
}

// gwfDamageComponents is the representative greatsword strike: the 2d6 pool
// with the ordered Great Weapon Fighting reroll, plus the sourced +3 Strength
// modifier component. Used as the accepted base of the refusal table.
func gwfDamageComponents() []encounter.DamageComponent {
	strength := 3
	return []encounter.DamageComponent{
		{
			Source: "weapon",
			Roll: encounter.RollComponent{
				Source: encounter.RollSource{Ref: "dnd5e:weapons:greatsword", Name: "Greatsword"},
				Dice: &encounter.DiceTrace{
					Notation:      "2d6",
					DieSize:       6,
					OriginalRolls: []int{1, 5},
					Rerolls: []encounter.DiceReroll{{
						DieIndex: 0,
						Before:   1,
						After:    4,
						Source: encounter.RollSource{
							Ref:  "dnd5e:conditions:fighting_style_great_weapon_fighting",
							Name: "Great Weapon Fighting",
						},
					}},
					FinalRolls: []int{4, 5},
					Subtotal:   9,
				},
			},
			DamageType: "slashing",
		},
		{
			Source: "ability",
			Roll: encounter.RollComponent{
				Source:   encounter.RollSource{Ref: "dnd5e:abilities:strength", Name: "Strength"},
				Modifier: &strength,
			},
			DamageType: "slashing",
		},
	}
}

// TestARecordedMissCarriesNoCriticalKey pins that a miss's payload never
// says "critical" at all — a whiff cannot crit, so there is nothing to
// answer false about, unlike a hit where false is itself a meaningful
// answer.
func (s *OutcomeTestSuite) TestARecordedMissCarriesNoCriticalKey() {
	enc := s.scene()

	_, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
		Attack: &encounter.AttackIdentity{Ref: "longsword", Name: "Longsword", DamageType: "slashing"},
	})
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: goblin})
	s.Require().NoError(err)
	var beat map[string]any
	s.Require().NoError(json.Unmarshal(story[len(story)-1].Payload, &beat))
	_, present := beat["critical"]
	s.False(present)
	attack, ok := beat["attack"].(map[string]any)
	s.Require().True(ok, "a miss still names what was swung")
	s.Equal("longsword", attack["ref"])
}

// TestTheTargetHearsItToo pins the audience rule.
//
// An outcome is not secret. A fight is localized, but it is not private: a
// client that learned about a strike only from the striker's own response
// could not render the scene the rest of the party is standing in.
func (s *OutcomeTestSuite) TestTheTargetHearsItToo() {
	enc := s.scene()
	_, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
	})
	s.Require().NoError(err)

	for _, who := range []encounter.MemberID{alice, goblin} {
		story, serr := enc.Story(&encounter.StoryInput{Audience: who})
		s.Require().NoError(serr)
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(story[len(story)-1].Payload, &beat))
		s.Equal("missed", beat["beat"], "%s hears it", who)
	}
}

// TestAnOutcomeCarriesNoProse is the shape's whole claim, asserted rather than
// promised.
//
// The reason the composition owns its record is that a reader can trust what
// is in it. An append taking free bytes would have kept the transcript in one
// place and given that up — any caller could narrate at other players through
// it. So RecordInput has NO PROSE field: the kind is a closed enum, members
// are IDs checked against the roster, values are integers under closed keys.
//
// This test reads the type rather than exercising it, because the guarantee is
// structural: prose is not filtered here, it is INEXPRESSIBLE, and the day
// someone adds a Description field this fails and says why.
//
// CRITICAL AND ATTACK ARE THE ARGUMENT (rpg-toolkit#866, rpg-toolkit#941).
// Critical is a bool — nothing to narrate. Attack is a pointer to
// [encounter.AttackIdentity], meant to carry a catalog-owned identifier or
// the rulebook's own closed word for what happened ("longsword",
// "slashing") — not free text a caller composes for effect. Record refuses
// an Attack whose Ref or Name is empty (ErrInvalidData) — the same presence
// floor Actor and Targets are already held to; see
// TestRefusalsAreCheckedAgainstTheRoster. What it still cannot do is
// validate that Ref or Name names anything real, or that DamageType is one
// of the rulebook's own words: this module's go.mod cannot import the
// catalog that would answer that (C1), so that half of the promise is
// about session's one caller — which only ever supplies what an
// already-compiled attack profile named — not something this composition
// enforces. Widening RecordInput to accept Attack was still the right
// call — it widens what a caller can IDENTIFY, the same category
// Actor/Targets already occupy, not what a caller can SAY — and "no prose"
// here now holds for presence exactly as it does for the fields beside it;
// only meaning is still the rulebook's to guarantee.
//
// DAMAGE COMPONENTS AND MODIFIER SOURCES MAKE THE SAME TRADE (rpg-project#265).
// Their nested fields are identifiers, dice notation, numbers, and closed
// rulebook words carried as primitives. AttackModifierSource deliberately has
// no Reason string: the rules engine's human-readable explanation does not
// become caller-authored prose in this transcript.
func (s *OutcomeTestSuite) TestAnOutcomeCarriesNoProse() {
	s.Equal([]string{
		"Kind", "Actor", "Targets", "Values", "Critical", "Attack",
		"DamageComponents", "AdvantageSources", "DisadvantageSources", "DeathSave",
	}, structFieldNames(encounter.RecordInput{}),
		"a new field on RecordInput needs an argument: free text here is prose "+
			"in a transcript other players read")
}

// TestRefusalsAreCheckedAgainstTheRoster pins that the composition validates
// what it stamps, which is the other half of owning the record.
func (s *OutcomeTestSuite) TestRefusalsAreCheckedAgainstTheRoster() {
	s.Run("nil input", func() {
		_, err := s.scene().Record(nil)
		s.ErrorIs(err, encounter.ErrNilInput)
	})

	s.Run("a kind this composition does not know", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeKind("disintegrated"), Actor: alice,
		})
		s.ErrorIs(err, encounter.ErrInvalidData)
	})

	s.Run("a value name this composition does not know", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice,
			Values: map[encounter.OutcomeValue]int{encounter.OutcomeValue("temp_hp"): 3},
		})
		s.ErrorIs(err, encounter.ErrInvalidData)
	})

	for _, tc := range []struct {
		name       string
		multiplier float64
	}{
		{"NaN multiplier", math.NaN()},
		{"positive infinite multiplier", math.Inf(1)},
		{"negative infinite multiplier", math.Inf(-1)},
	} {
		s.Run(tc.name, func() {
			_, err := s.scene().Record(&encounter.RecordInput{
				Kind: encounter.OutcomeStruck, Actor: alice,
				DamageComponents: []encounter.DamageComponent{{Multiplier: &tc.multiplier}},
			})
			s.ErrorIs(err, encounter.ErrInvalidData,
				"an unrepresentable JSON number is structural invalid data")
		})
	}

	s.Run("an actor who is not a member", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: "nobody",
		})
		s.ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("an empty actor", func() {
		_, err := s.scene().Record(&encounter.RecordInput{Kind: encounter.OutcomeStruck})
		s.ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("a target who is not a member", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice,
			Targets: []encounter.MemberID{"ghost"},
		})
		s.ErrorIs(err, encounter.ErrNoMember)
	})

	s.Run("a closed encounter", func() {
		enc := s.scene()
		_, err := enc.End(&encounter.EndInput{Ending: "withdrawn"})
		s.Require().NoError(err)
		_, err = enc.Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice,
		})
		s.ErrorIs(err, encounter.ErrClosed)
	})
}

// TestAnAttackWithNoRefOrNameIsRefused pins the minimum Record CAN check
// about AttackIdentity without a catalog: Ref and Name are non-empty, held
// to the same floor as Actor. What names is real, and what DamageType
// means, stays the rulebook's to guarantee (AttackIdentity's own doc) —
// this is presence, not meaning.
func (s *OutcomeTestSuite) TestAnAttackWithNoRefOrNameIsRefused() {
	s.Run("empty ref", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{goblin},
			Attack: &encounter.AttackIdentity{Name: "Longsword", DamageType: "slashing"},
		})
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
	})

	s.Run("empty name", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{goblin},
			Attack: &encounter.AttackIdentity{Ref: "longsword", DamageType: "slashing"},
		})
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
	})

	s.Run("both empty", func() {
		_, err := s.scene().Record(&encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: alice, Targets: []encounter.MemberID{goblin},
			Attack: &encounter.AttackIdentity{},
		})
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
	})
}

// TestTheOutcomeLandsAfterTheVerbThatCausedIt is Record's half of the
// cause-before-effect law (see refreshSight).
//
// An outcome is a CONSEQUENCE — of a walk, an arrival, whatever produced it —
// so it lands after that verb's own beat. Record holds that by appending its own
// beat FIRST and refreshing no sight, detecting no trigger and moving no clock,
// so there is nothing it could append ahead of the outcome and nothing it could
// reorder.
//
// Nobody is down in this scene, so the standing consult Record now runs
// (rpg-toolkit#1083) finds nothing to say and the beat list is the same list it
// always was. That is the point of leaving this pin exactly as it was written:
// the news a consult has none of must cost the story nothing. killingblow_test.go
// owns the scene where there IS news, and pins that it lands after the outcome
// rather than before it.
func (s *OutcomeTestSuite) TestTheOutcomeLandsAfterTheVerbThatCausedIt() {
	enc := s.scene()

	moved, err := enc.Step(&encounter.StepInput{Member: alice, To: spatial.Position{X: 5, Y: 2}})
	s.Require().NoError(err)
	s.Require().Nil(moved.Formed, "the wall keeps this walk quiet")

	recorded, err := enc.Record(&encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: alice, Targets: []encounter.MemberID{goblin},
	})
	s.Require().NoError(err)
	s.Greater(recorded.Seq, moved.Seq, "the walk happened, THEN what came of it")

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)
	kinds := make([]string, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		kinds = append(kinds, beat["beat"].(string))
	}
	s.Equal([]string{"scene-opened", "moved", "missed"}, kinds,
		"recording appends exactly one beat and nothing before it")
}
