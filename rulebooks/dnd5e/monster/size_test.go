package monster

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

func TestMonsterSizeDefaultsAndPersists(t *testing.T) {
	defaultMonster := New(Config{ID: "default", Name: "Default", HP: 1, AC: 10})
	if got := defaultMonster.Size(); got != dnd5e.SizeMedium {
		t.Fatalf("default monster size = %q, want %q", got, dnd5e.SizeMedium)
	}

	largeMonster := New(Config{ID: "large", Name: "Large", Size: dnd5e.SizeLarge, HP: 1, AC: 10})
	if got := largeMonster.Size(); got != dnd5e.SizeLarge {
		t.Fatalf("large monster size = %q, want %q", got, dnd5e.SizeLarge)
	}
	if got := largeMonster.ToData().Size; got != dnd5e.SizeLarge {
		t.Fatalf("persisted monster size = %q, want %q", got, dnd5e.SizeLarge)
	}
}
