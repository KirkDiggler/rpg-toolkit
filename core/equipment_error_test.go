package core_test

import (
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

func TestEquipmentError(t *testing.T) {
	tests := []struct {
		name           string
		equipmentError *core.EquipmentError
		expectedMsg    string
		shouldUnwrap   bool
		unwrappedErr   error
	}{
		{
			name: "equipment error with all fields",
			equipmentError: core.NewEquipmentError(
				"equip",
				"char-123",
				"sword-456",
				"main-hand",
				core.ErrSlotOccupied,
			),
			expectedMsg:  "equip item sword-456 to slot main-hand: slot occupied",
			shouldUnwrap: true,
			unwrappedErr: core.ErrSlotOccupied,
		},
		{
			name: "equipment error without slot",
			equipmentError: core.NewEquipmentError(
				"unequip",
				"char-123",
				"sword-456",
				"",
				errors.New("item not equipped"),
			),
			expectedMsg:  "unequip item sword-456: item not equipped",
			shouldUnwrap: true,
			unwrappedErr: errors.New("item not equipped"),
		},
		{
			name: "equipment error without item ID",
			equipmentError: &core.EquipmentError{
				Op:  "clear",
				Err: errors.New("inventory full"),
			},
			expectedMsg:  "clear: inventory full",
			shouldUnwrap: true,
			unwrappedErr: errors.New("inventory full"),
		},
		{
			name: "equipment error with item but no slot",
			equipmentError: &core.EquipmentError{
				Op:     "drop",
				ItemID: "potion-789",
				Err:    errors.New("cannot drop quest item"),
			},
			expectedMsg:  "drop item potion-789: cannot drop quest item",
			shouldUnwrap: true,
			unwrappedErr: errors.New("cannot drop quest item"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Error() method
			msg := tt.equipmentError.Error()
			if msg != tt.expectedMsg {
				t.Errorf("Error() = %q, want %q", msg, tt.expectedMsg)
			}

			// Test Unwrap() method
			if tt.shouldUnwrap {
				unwrapped := tt.equipmentError.Unwrap()
				if unwrapped == nil {
					t.Error("Unwrap() returned nil, expected error")
				} else if unwrapped.Error() != tt.unwrappedErr.Error() {
					t.Errorf("Unwrap() = %v, want %v", unwrapped.Error(), tt.unwrappedErr.Error())
				}
			}
		})
	}
}
