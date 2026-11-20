package encounter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// newInitiativeEnc builds the same small scene TestFormRejections does: four
// players and a goblin next door, so first light starts no fight by itself.
func newInitiativeEnc(t *testing.T) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{Canvas: CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()}, Regions: []RegionInput{rectRegion("r1", 0, 0, 8, 8), rectRegion("r2", 8, 0, 8, 8)}, Walls: sealedSeam(7, 8)},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 14, Y: 6}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
		Capabilities: Capabilities{
			Sight:         everyoneSeesTheWholeMap{},
			Equipment:     UnobservedEquipment{},
			Conditions:    UnobservedEquipment{},
			Sheets:        zeroSheets{},
			Participation: everyoneStanding{},
			Initiative:    orderAsGiven{},
			Driver:        passDriver{},
			Actors: Actors{
				Striker:   passStriker{},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)
	return enc
}

// initiativeRoll builds one member's recorded roll: a d20 face, then each
// modifier with its source name, totalled.
func initiativeRoll(member MemberID, face int, mods map[string]int, order []string) InitiativeRoll {
	total := face
	components := []RollComponent{{
		Source: RollSource{Ref: "dnd5e:dice:d20", Name: "d20", SourceID: string(member)},
		Dice:   &DiceTrace{Notation: "1d20", DieSize: 20, OriginalRolls: []int{face}, FinalRolls: []int{face}, Subtotal: face},
	}}
	for _, name := range order {
		v := mods[name]
		components = append(components, RollComponent{Source: RollSource{Ref: "dnd5e:modifiers:" + strings.ReplaceAll(strings.ToLower(name), " ", "_"), Name: name}, Modifier: &v})
		total += v
	}
	return InitiativeRoll{
		Member: member, Roll: face, Total: total,
		Calculation: &RollCalculation{Components: components, Total: total},
	}
}

func lastFormedPayload(t *testing.T, enc *Encounter) map[string]json.RawMessage {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	var found map[string]json.RawMessage
	for _, entry := range story {
		if beatKind(t, entry.Payload) == BeatFightStarted {
			found = nil
			require.NoError(t, json.Unmarshal(entry.Payload, &found))
		}
	}
	return found
}

func countFormed(t *testing.T, enc *Encounter) int {
	t.Helper()
	story, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	n := 0
	for _, entry := range story {
		if beatKind(t, entry.Payload) == BeatFightStarted {
			n++
		}
	}
	return n
}

func TestFormRecordsInitiativeRolls(t *testing.T) {
	enc := newInitiativeEnc(t)
	_, err := enc.form(&FormInput{
		Order: []MemberID{"alice", "goblin"},
		Rolls: []InitiativeRoll{
			// Given out of order on purpose: the record follows the order.
			initiativeRoll("goblin", 11, map[string]int{"Dexterity": 1}, []string{"Dexterity"}),
			initiativeRoll("alice", 14, map[string]int{"Dexterity": 2, "Warmed up": 1}, []string{"Dexterity", "Warmed up"}),
		},
	})
	require.NoError(t, err)

	payload := lastFormedPayload(t, enc)
	require.NotNil(t, payload)
	var order []string
	require.NoError(t, json.Unmarshal(payload["order"], &order))
	require.Equal(t, []string{"alice", "goblin"}, order)

	var rolls []struct {
		Member      string `json:"member"`
		Roll        int    `json:"roll"`
		Total       int    `json:"total"`
		Calculation struct {
			Components []struct {
				Source struct {
					Name string `json:"name"`
				} `json:"source"`
			} `json:"components"`
		} `json:"calculation"`
	}
	require.NoError(t, json.Unmarshal(payload["rolls"], &rolls))
	require.Len(t, rolls, 2)
	require.Equal(t, "alice", rolls[0].Member)
	require.Equal(t, "goblin", rolls[1].Member)

	alice := rolls[0]
	require.Equal(t, 14, alice.Roll)
	require.Equal(t, 17, alice.Total)
	var names []string
	for _, c := range alice.Calculation.Components {
		names = append(names, c.Source.Name)
	}
	require.Equal(t, []string{"d20", "Dexterity", "Warmed up"}, names)

	goblin := rolls[1]
	require.Equal(t, 11, goblin.Roll)
	require.Equal(t, 12, goblin.Total)
}

func TestFormWithoutRollsRecordsNone(t *testing.T) {
	order := []MemberID{"alice", "goblin"}
	for name, rolls := range map[string][]InitiativeRoll{
		"no rolls given":  nil,
		"only placements": PlacedInOrder(order),
	} {
		t.Run(name, func(t *testing.T) {
			enc := newInitiativeEnc(t)
			_, err := enc.form(&FormInput{Order: order, Rolls: rolls})
			require.NoError(t, err)
			payload := lastFormedPayload(t, enc)
			require.NotNil(t, payload)
			require.Contains(t, payload, "order")
			require.NotContains(t, payload, "rolls")
		})
	}
}

func TestFormRefusesABadInitiativeRoll(t *testing.T) {
	good := func(m MemberID, face int) InitiativeRoll {
		return initiativeRoll(m, face, map[string]int{"Dexterity": 1}, []string{"Dexterity"})
	}
	wrongTotal := good("alice", 10)
	wrongTotal.Total = 99
	wrongFace := good("alice", 10)
	wrongFace.Roll = 3

	cases := []struct {
		name  string
		rolls []InitiativeRoll
		want  error
	}{
		{"a roll for a member not in the order", []InitiativeRoll{good("bob", 10)}, ErrNotMember},
		{"two rolls for one member", []InitiativeRoll{good("alice", 10), good("alice", 12)}, ErrInvalidData},
		{"a total that disagrees with the calculation", []InitiativeRoll{wrongTotal}, ErrInvalidData},
		{"a roll that disagrees with the d20", []InitiativeRoll{wrongFace}, ErrInvalidData},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc := newInitiativeEnc(t)
			_, err := enc.form(&FormInput{Order: []MemberID{"alice", "goblin"}, Rolls: tc.rolls})
			require.ErrorIs(t, err, tc.want)
			require.Zero(t, countFormed(t, enc), "a refused form writes no bubble-formed entry")
		})
	}
}

func TestPlacedInOrder(t *testing.T) {
	got := PlacedInOrder([]MemberID{"goblin", "alice", "bob"})
	require.Len(t, got, 3)
	for i, id := range []MemberID{"goblin", "alice", "bob"} {
		require.Equal(t, id, got[i].Member)
		require.Nil(t, got[i].Calculation)
	}
}

// rollsWithCalculations answers every member with a roll that carries its
// calculation, so a fight formed on sight has something to record.
type rollsWithCalculations struct{}

func (rollsWithCalculations) RollInitiative(members []MemberID) ([]InitiativeRoll, error) {
	out := make([]InitiativeRoll, len(members))
	for i, m := range members {
		out[i] = initiativeRoll(m, 15-i, map[string]int{"Dexterity": 1}, []string{"Dexterity"})
	}

	return out, nil
}

func TestAFightThatFormsOnSightRecordsItsRolls(t *testing.T) {
	g := bumpGuard(t)
	g.Behaviors.Instances[0].Instance.Table["barred"] = []Answer{{Weight: 1, Say: "It's them!", Fact: string(bumpKnown)}}
	in := postedSetup(everyoneSeesTheWholeMap{}, g)
	in.Initiative = rollsWithCalculations{}
	in.Field.Factions = []FactionInput{{ID: postedGarrison, Mind: postedGuard}}
	in.Field.Dispositions = []DispositionInput{{
		Between: [2]FactionID{postedGarrison, FactionParty}, Stance: StanceNeutral,
		Until: []UntilStep{{When: TriggerFact{Fact: bumpKnown}, Then: StanceHostile}},
	}}
	enc, err := NewEncounter(in)
	require.NoError(t, err)

	out := bumped(t, enc, postedPost, postedGuard)
	require.NotNil(t, out.Formed, "the fight formed through trigger detection")

	formed := beatsTagged(t, enc, BeatFightStarted)
	require.Len(t, formed, 1)
	rolls, ok := formed[0]["rolls"].([]any)
	require.True(t, ok, "the fight-forming entry carries the rolls")
	var members []string
	for _, r := range rolls {
		members = append(members, r.(map[string]any)["member"].(string))
	}
	require.ElementsMatch(t, []string{"alice", string(postedGuard)}, members)
}
