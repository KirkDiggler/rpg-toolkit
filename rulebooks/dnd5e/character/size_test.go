package character

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

func TestCharacterSizeDefaultsAndPersists(t *testing.T) {
	char := &Character{size: dnd5e.SizeSmall}
	if got := char.Size(); got != dnd5e.SizeSmall {
		t.Fatalf("character size = %q, want %q", got, dnd5e.SizeSmall)
	}
	if got := char.ToData().Size; got != dnd5e.SizeSmall {
		t.Fatalf("persisted character size = %q, want %q", got, dnd5e.SizeSmall)
	}

	legacyChar := &Character{}
	if got := legacyChar.Size(); got != dnd5e.SizeMedium {
		t.Fatalf("legacy character size = %q, want %q", got, dnd5e.SizeMedium)
	}
}
