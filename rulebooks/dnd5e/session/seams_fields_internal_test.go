// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestTheEquipmentSeamCarriesEveryStandingField guards equipmentBeside's
// field-by-field copy. The two seams used to convert, which made the compiler
// prove they carried the same fields; the standing seam now also carries the
// live encounter slot, which only the sheet seam reads, so the proof is here:
// every other standing field must exist on the equipment seam with the same
// type, and equipmentBeside must copy it. A field added to both seams but not
// to the copy fails the second half.
func TestTheEquipmentSeamCarriesEveryStandingField(t *testing.T) {
	standing := reflect.TypeOf(standingSeam{})
	equipment := reflect.TypeOf(equipmentSeam{})
	for i := 0; i < standing.NumField(); i++ {
		f := standing.Field(i)
		if f.Name == "live" {
			continue
		}
		got, ok := equipment.FieldByName(f.Name)
		if assert.True(t, ok, "equipmentSeam lacks standingSeam's %q", f.Name) {
			assert.Equal(t, f.Type, got.Type, "field %q", f.Name)
		}
	}
	assert.Equal(t, standing.NumField()-1, equipment.NumField(),
		"equipmentSeam carries a field standingSeam does not")

	// Every copied field is non-zero on the source, so a dropped one reads zero.
	source := standingSeam{
		ctx:    context.Background(),
		sheets: storeOf(participationCharacterStore{byID: map[string]*character.Data{}}),
		kinds:  map[string]encounter.MemberKind{"alice": encounter.KindPlayer},
		data:   &SessionData{},
		live:   &liveEncounter{},
	}
	copied := reflect.ValueOf(equipmentBeside(source))
	for i := 0; i < copied.NumField(); i++ {
		assert.False(t, copied.Field(i).IsZero(), "equipmentBeside dropped %q", copied.Type().Field(i).Name)
	}
}
