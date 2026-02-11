# Dungeon Generator Example

The dungeon package provides D&D 5e dungeon generation with themed encounters and spatial layouts.

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/dungeon"
)

func main() {
    // Create the generator
    gen := dungeon.NewGenerator(nil)

    // Generate a 5-room crypt dungeon with CR 5 total budget
    output, err := gen.Generate(context.Background(), &dungeon.GenerateInput{
        Theme:     dungeon.ThemeCrypt,
        TargetCR:  5.0,
        RoomCount: 5,
    })
    if err != nil {
        panic(err)
    }

    // Access the generated dungeon
    d := output.Dungeon
    fmt.Printf("Generated dungeon: %s\n", d.ID())
    fmt.Printf("Start room: %s, Boss room: %s\n", d.StartRoom(), d.BossRoom())
    fmt.Printf("Seed for reproducibility: %d\n", output.Seed)
}
```

## Available Themes

- `dungeon.ThemeCrypt` - Ancient crypts with undead (skeletons, zombies, ghouls)
- `dungeon.ThemeCave` - Natural caves with beasts (rats, spiders, wolves)
- `dungeon.ThemeBanditLair` - Humanoid hideouts with bandits and thugs

## For More Examples

See the narrated example tests in `example_test.go`. Run them with:

```bash
go test -v ./rulebooks/dnd5e/dungeon/example/...
```

The tests demonstrate:
- Basic generation with themes
- Seed reproducibility (same seed = same structure)
- How different themes produce different encounters
- Accessing dungeon data after generation
