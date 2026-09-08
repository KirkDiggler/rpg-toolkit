package classes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// apiStartingEquipment reads one class's FIXED starting kit out of the
// dnd5eapi snapshot the choices package keeps.
//
// The snapshot is the source of record for what a class starts with, and this
// reads it rather than restating it: a test that hand-copied the same numbers
// would agree with itself and with nothing else.
func apiStartingEquipment(t *testing.T, class string) map[string]int {
	t.Helper()

	path := filepath.Join("..", "character", "choices", "testdata", "api", "classes", class+".json")
	raw, err := os.ReadFile(path) //nolint:gosec // a test fixture path built from a literal
	require.NoError(t, err)

	var payload struct {
		StartingEquipment []struct {
			Equipment struct {
				Index string `json:"index"`
			} `json:"equipment"`
			Quantity int `json:"quantity"`
		} `json:"starting_equipment"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))

	kit := map[string]int{}
	for _, row := range payload.StartingEquipment {
		kit[row.Equipment.Index] += row.Quantity
	}
	return kit
}

// grantedEquipment is what a class's level-1 grants hand over, by id.
func grantedEquipment(class Class) map[shared.EquipmentID]int {
	out := map[shared.EquipmentID]int{}
	for _, grant := range GetGrantsForLevel(class, 1) {
		for _, item := range grant.Equipment {
			out[item.ID] += item.Quantity
		}
	}
	return out
}

// TestTheBardStartsWithLeatherArmourAndADagger is the walk finding — "bard
// gets no armor?" — at its source.
func TestTheBardStartsWithLeatherArmourAndADagger(t *testing.T) {
	granted := grantedEquipment(Bard)

	require.Equal(t, 1, granted["leather"], "PHB gives a bard leather armour outright")
	require.Equal(t, 1, granted["dagger"])
	require.Len(t, granted, 2, "and nothing else is fixed; the rest is chosen")
}

// TestFixedKitsMatchTheApiSnapshotExceptWhereItIsAChoice is the rule this data
// actually follows, written down.
//
// A grant carries what NO requirement offers. The API's starting_equipment
// lists items this codebase models as a choice — the barbarian's explorer's
// pack is fixed there and offered by choices.BarbarianPack here — so granting
// the API list wholesale would put two packs in one bag. Each divergence is
// named rather than tolerated, so a new one has to be argued for.
func TestFixedKitsMatchTheApiSnapshotExceptWhereItIsAChoice(t *testing.T) {
	for _, tc := range []struct {
		class Class
		file  string
		// offeredAsChoice are ids the API calls fixed that this codebase
		// offers through an equipment requirement instead.
		offeredAsChoice []string
		// idAliases maps an API index onto this codebase's own id where the
		// two spell it differently.
		idAliases map[string]string
	}{
		{class: Bard, file: "bard", idAliases: map[string]string{"leather-armor": "leather"}},
		{class: Rogue, file: "rogue", idAliases: map[string]string{"leather-armor": "leather"}},
		{class: Monk, file: "monk"},
		{class: Cleric, file: "cleric"},
		{class: Fighter, file: "fighter"},
		{class: Barbarian, file: "barbarian", offeredAsChoice: []string{"explorers-pack"}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			want := map[shared.EquipmentID]int{}
			for index, quantity := range apiStartingEquipment(t, tc.file) {
				if slicesContains(tc.offeredAsChoice, index) {
					continue
				}
				id := index
				if alias, ok := tc.idAliases[index]; ok {
					id = alias
				}
				want[shared.EquipmentID(id)] = quantity
			}

			require.Equal(t, want, grantedEquipment(tc.class))
		})
	}
}

// TestAClassWithNoFixedKitGrantsNoEquipment — the fighter's whole kit is
// chosen, and an empty grant says so rather than inventing a default.
func TestAClassWithNoFixedKitGrantsNoEquipment(t *testing.T) {
	require.Empty(t, apiStartingEquipment(t, "fighter"))
	require.Empty(t, grantedEquipment(Fighter))
}

// TestUnmigratedClassesGrantNothingAtAll pins the state of the other six, so
// their absence stays a deliberate gap rather than becoming a surprise.
//
// They return NO grants — not an empty kit, no grants at all — which means no
// proficiencies and no features either. Cleric now supplies creation grants
// (proficiencies and fixed kit), but still has no executable spellcasting or
// domain features; its presence in GetGrants is not a playable-class claim.
func TestUnmigratedClassesGrantNothingAtAll(t *testing.T) {
	for _, class := range []Class{
		Druid, Paladin, Ranger, Sorcerer, Warlock, Wizard,
	} {
		require.Nil(t, GetGrants(class), "%s is not migrated to the grant system", class)
	}
}

func slicesContains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
