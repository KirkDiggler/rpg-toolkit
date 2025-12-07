# Items and Choices Implementation Design

## Overview

This document describes the implementation of D&D 5e items (weapons, armor, gear) and a simplified choice system that game servers can use for character creation. The toolkit owns all item definitions and provides explicit choice structures that eliminate ambiguity.

## Phase 1: Core Items and Choice System

### Design Principles

1. **All items are defined in code** - No external lookups needed
2. **Choices are explicit** - Clear what each choice represents
3. **Categories enable lookups** - Game server can query by category
4. **Bundles are explicit** - No ambiguous "choose a martial weapon"
5. **Game server has clear path** - Simple functions to build choice lists

## Item System

### Weapon Definitions

```go
// rulebooks/dnd5e/weapons/types.go
package weapons

type Weapon struct {
    ID         string
    Name       string
    Category   Category    // Simple or Martial
    Type       Type        // Melee or Ranged
    Damage     Damage
    Properties []Property
    Weight     float64
    Cost       Cost
}

type Category string

const (
    Simple  Category = "simple"
    Martial Category = "martial"
)

// rulebooks/dnd5e/weapons/definitions.go

// All weapons defined as package variables
var (
    // Simple Melee
    Club       = Weapon{ID: "club", Name: "Club", Category: Simple, Type: Melee, Damage: Damage{Dice: "1d4", Type: damage.Bludgeoning}}
    Dagger     = Weapon{ID: "dagger", Name: "Dagger", Category: Simple, Type: Melee, Damage: Damage{Dice: "1d4", Type: damage.Piercing}}
    Greatclub  = Weapon{ID: "greatclub", Name: "Greatclub", Category: Simple, Type: Melee, Damage: Damage{Dice: "1d8", Type: damage.Bludgeoning}}
    Handaxe    = Weapon{ID: "handaxe", Name: "Handaxe", Category: Simple, Type: Melee, Damage: Damage{Dice: "1d6", Type: damage.Slashing}}
    
    // Martial Melee
    Battleaxe  = Weapon{ID: "battleaxe", Name: "Battleaxe", Category: Martial, Type: Melee, Damage: Damage{Dice: "1d8", Type: damage.Slashing}}
    Longsword  = Weapon{ID: "longsword", Name: "Longsword", Category: Martial, Type: Melee, Damage: Damage{Dice: "1d8", Type: damage.Slashing}}
    Rapier     = Weapon{ID: "rapier", Name: "Rapier", Category: Martial, Type: Melee, Damage: Damage{Dice: "1d8", Type: damage.Piercing}}
    Warhammer  = Weapon{ID: "warhammer", Name: "Warhammer", Category: Martial, Type: Melee, Damage: Damage{Dice: "1d8", Type: damage.Bludgeoning}}
    
    // ... all other weapons
)

// All weapons in a map for O(1) lookup
var All = map[string]Weapon{
    "club":       Club,
    "dagger":     Dagger,
    "greatclub":  Greatclub,
    "handaxe":    Handaxe,
    // ... all simple weapons
    "battleaxe":  Battleaxe,
    "longsword":  Longsword,
    "rapier":     Rapier,
    "warhammer":  Warhammer,
    // ... all martial weapons
}
```

### Weapon Lookups

```go
// rulebooks/dnd5e/weapons/lookup.go
package weapons

// GetByID returns a weapon by ID - O(1) lookup
func GetByID(id string) (Weapon, bool) {
    w, ok := All[id]
    return w, ok
}

// ListByCategory returns all weapons in a category
func ListByCategory(cat Category) []Weapon {
    var result []Weapon
    for _, w := range All {
        if w.Category == cat {
            result = append(result, w)
        }
    }
    return result
}

// ListSimple returns all simple weapons
func ListSimple() []Weapon {
    return ListByCategory(Simple)
}

// ListMartial returns all martial weapons
func ListMartial() []Weapon {
    return ListByCategory(Martial)
}
```

### Armor Definitions

```go
// rulebooks/dnd5e/armor/types.go
package armor

type Armor struct {
    ID         string
    Name       string
    Category   Category
    AC         ArmorClass
    Strength   int      // Minimum strength
    Stealth    bool     // Disadvantage on stealth
    Weight     float64
    Cost       Cost
}

type Category string

const (
    Light  Category = "light"
    Medium Category = "medium"
    Heavy  Category = "heavy"
    Shield Category = "shield"
)

// rulebooks/dnd5e/armor/definitions.go

var (
    // Light Armor
    Leather        = Armor{ID: "leather", Name: "Leather", Category: Light, AC: ArmorClass{Base: 11, AddDex: true}}
    StuddedLeather = Armor{ID: "studded-leather", Name: "Studded Leather", Category: Light, AC: ArmorClass{Base: 12, AddDex: true}}
    
    // Medium Armor
    Hide        = Armor{ID: "hide", Name: "Hide", Category: Medium, AC: ArmorClass{Base: 12, AddDex: true, MaxDex: 2}}
    ChainShirt  = Armor{ID: "chain-shirt", Name: "Chain Shirt", Category: Medium, AC: ArmorClass{Base: 13, AddDex: true, MaxDex: 2}}
    ScaleMail   = Armor{ID: "scale-mail", Name: "Scale Mail", Category: Medium, AC: ArmorClass{Base: 14, AddDex: true, MaxDex: 2}, Stealth: true}
    
    // Heavy Armor
    RingMail    = Armor{ID: "ring-mail", Name: "Ring Mail", Category: Heavy, AC: ArmorClass{Base: 14}, Stealth: true}
    ChainMail   = Armor{ID: "chain-mail", Name: "Chain Mail", Category: Heavy, AC: ArmorClass{Base: 16}, Strength: 13, Stealth: true}
    Plate       = Armor{ID: "plate", Name: "Plate", Category: Heavy, AC: ArmorClass{Base: 18}, Strength: 15, Stealth: true}
    
    // Shield
    Shield      = Armor{ID: "shield", Name: "Shield", Category: Shield, AC: ArmorClass{Bonus: 2}}
)

var All = []Armor{
    Leather, StuddedLeather,
    Hide, ChainShirt, ScaleMail,
    RingMail, ChainMail, Plate,
    Shield,
}
```

## Choice System

### Core Types

```go
// rulebooks/dnd5e/choices/types.go
package choices

// Choice represents a character creation choice
type Choice struct {
    ID          string
    Category    Category
    Description string
    Choose      int        // How many to choose
    Options     []Option
    Source      Source
}

// Option types - explicit and clear
type Option interface {
    GetID() string
    GetType() OptionType
}

type OptionType string

const (
    OptionTypeSingle         OptionType = "single"          // Single item
    OptionTypeBundle         OptionType = "bundle"          // Multiple items together
    OptionTypeWeaponCategory OptionType = "weapon_category" // Choose from weapon category
    OptionTypeArmorCategory  OptionType = "armor_category"  // Choose from armor category
)

// SingleOption - a single item
type SingleOption struct {
    ItemType ItemType
    ItemID   string
}

// BundleOption - multiple items as one choice
type BundleOption struct {
    ID    string
    Items []CountedItem
}

// CountedItem - item with quantity
type CountedItem struct {
    ItemType ItemType
    ItemID   string
    Quantity int
}

// WeaponCategoryOption - choose from weapon category
type WeaponCategoryOption struct {
    Category weapons.Category  // "simple" or "martial"
    Count    int              // How many to choose
}

// ArmorCategoryOption - choose from armor category  
type ArmorCategoryOption struct {
    Category armor.Category   // "light", "medium", "heavy"
    Count    int
}
```

### Fighter Choices - Explicit

```go
// rulebooks/dnd5e/classes/fighter/choices.go
package fighter

import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
)

// EquipmentChoices returns fighter's starting equipment choices
func EquipmentChoices() []choices.Choice {
    return []choices.Choice{
        // Choice 1: Armor
        {
            ID:          "fighter-armor",
            Category:    choices.CategoryEquipment,
            Description: "(a) chain mail or (b) leather armor, longbow, and 20 arrows",
            Choose:      1,
            Source:      choices.SourceClass,
            Options: []choices.Option{
                // Option A: Just chain mail
                choices.SingleOption{
                    ItemType: choices.ItemTypeArmor,
                    ItemID:   armor.ChainMail.ID,
                },
                // Option B: Bundle
                choices.BundleOption{
                    ID: "leather-bow-arrows",
                    Items: []choices.CountedItem{
                        {ItemType: choices.ItemTypeArmor, ItemID: armor.Leather.ID, Quantity: 1},
                        {ItemType: choices.ItemTypeWeapon, ItemID: weapons.Longbow.ID, Quantity: 1},
                        {ItemType: choices.ItemTypeGear, ItemID: gear.Arrow.ID, Quantity: 20},
                    },
                },
            },
        },
        
        // Choice 2: Primary Weapon - EXPLICIT category reference
        {
            ID:          "fighter-primary-weapon",
            Category:    choices.CategoryEquipment,
            Description: "Choose a martial weapon",
            Choose:      1,
            Source:      choices.SourceClass,
            Options: []choices.Option{
                choices.WeaponCategoryOption{
                    Category: weapons.Martial,
                    Count:    1,
                },
            },
        },
        
        // Choice 3: Shield or Second Weapon
        // NOTE: Instead of complex "(a) weapon and shield or (b) two weapons",
        // we split into two choices: First choose weapon, then choose shield OR weapon
        // This eliminates complex nested choices!
        {
            ID:          "fighter-shield-or-weapon",
            Category:    choices.CategoryEquipment,
            Description: "Choose a shield or a second martial weapon",
            Choose:      1,
            Source:      choices.SourceClass,
            Options: []choices.Option{
                choices.SingleOption{
                    ItemType: choices.ItemTypeArmor,
                    ItemID:   armor.Shield.ID,
                },
                choices.WeaponCategoryOption{
                    Category: weapons.Martial,
                    Count:    1,
                },
            },
        },
        
        // Choice 4: Ranged Option
        {
            ID:          "fighter-ranged",
            Category:    choices.CategoryEquipment,
            Description: "(a) light crossbow and 20 bolts or (b) two handaxes",
            Choose:      1,
            Source:      choices.SourceClass,
            Options: []choices.Option{
                choices.BundleOption{
                    ID: "crossbow-bolts",
                    Items: []choices.CountedItem{
                        {ItemType: choices.ItemTypeWeapon, ItemID: weapons.CrossbowLight.ID, Quantity: 1},
                        {ItemType: choices.ItemTypeGear, ItemID: gear.CrossbowBolt.ID, Quantity: 20},
                    },
                },
                choices.BundleOption{
                    ID: "two-handaxes",
                    Items: []choices.CountedItem{
                        {ItemType: choices.ItemTypeWeapon, ItemID: weapons.Handaxe.ID, Quantity: 2},
                    },
                },
            },
        },
    }
}
```

## Game Server Integration

### Building Choice Lists for Players

```go
// Game server code
func (gs *GameServer) BuildChoiceList(choice choices.Choice) (*pb.CharacterChoice, error) {
    pbChoice := &pb.CharacterChoice{
        Id:          choice.ID,
        Description: choice.Description,
        Choose:      int32(choice.Choose),
    }
    
    for _, opt := range choice.Options {
        switch o := opt.(type) {
        case choices.SingleOption:
            // Single item - straightforward
            pbChoice.Options = append(pbChoice.Options, &pb.ChoiceOption{
                Id:   o.ItemID,
                Type: "single",
                Items: []*pb.Item{{
                    Id:   o.ItemID,
                    Name: gs.getItemName(o.ItemType, o.ItemID),
                }},
            })
            
        case choices.BundleOption:
            // Bundle - list all items
            var items []*pb.Item
            for _, item := range o.Items {
                items = append(items, &pb.Item{
                    Id:       item.ItemID,
                    Name:     gs.getItemName(item.ItemType, item.ItemID),
                    Quantity: int32(item.Quantity),
                })
            }
            pbChoice.Options = append(pbChoice.Options, &pb.ChoiceOption{
                Id:    o.ID,
                Type:  "bundle",
                Items: items,
            })
            
        case choices.WeaponCategoryOption:
            // EXPLICIT: Game server knows to list weapons by category
            weapons := weapons.ListByCategory(o.Category)
            for _, w := range weapons {
                pbChoice.Options = append(pbChoice.Options, &pb.ChoiceOption{
                    Id:   w.ID,
                    Type: "single",
                    Items: []*pb.Item{{
                        Id:   w.ID,
                        Name: w.Name,
                    }},
                })
            }
            
        case choices.ArmorCategoryOption:
            // EXPLICIT: Game server knows to list armors by category
            armors := armor.ListByCategory(o.Category)
            for _, a := range armors {
                pbChoice.Options = append(pbChoice.Options, &pb.ChoiceOption{
                    Id:   a.ID,
                    Type: "single",
                    Items: []*pb.Item{{
                        Id:   a.ID,
                        Name: a.Name,
                    }},
                })
            }
        }
    }
    
    return pbChoice, nil
}

// Applying player selection
func (gs *GameServer) ApplyChoice(character *Character, choice choices.Choice, selection string) error {
    // Find which option was selected
    for _, opt := range choice.Options {
        switch o := opt.(type) {
        case choices.SingleOption:
            if o.ItemID == selection {
                return gs.addItem(character, o.ItemType, o.ItemID, 1)
            }
            
        case choices.BundleOption:
            if o.ID == selection {
                // Add all items in bundle
                for _, item := range o.Items {
                    if err := gs.addItem(character, item.ItemType, item.ItemID, item.Quantity); err != nil {
                        return err
                    }
                }
                return nil
            }
            
        case choices.WeaponCategoryOption:
            // Validate selection is from correct category
            if weapon, ok := weapons.GetByID(selection); ok {
                if weapon.Category == o.Category {
                    return gs.addItem(character, choices.ItemTypeWeapon, selection, 1)
                }
            }
            
        case choices.ArmorCategoryOption:
            // Validate selection is from correct category
            if armor, ok := armor.GetByID(selection); ok {
                if armor.Category == o.Category {
                    return gs.addItem(character, choices.ItemTypeArmor, selection, 1)
                }
            }
        }
    }
    
    return fmt.Errorf("invalid selection: %s", selection)
}
```

## Key Benefits

1. **No Ambiguity** - WeaponCategoryOption explicitly means "list all weapons of this category"
2. **Simple Lookups** - Game server calls `weapons.ListByCategory(weapons.Martial)`
3. **Type Safety** - All items are defined, no magic strings
4. **Clear Path** - Game server knows exactly how to handle each option type
5. **Extensible** - Can add new option types as needed

## Phase 1 Implementation Plan

1. **Define all PHB weapons** (~40 weapons)
2. **Define all PHB armor** (~15 armors)
3. **Define common gear** (arrows, bolts, packs, etc.)
4. **Implement lookup functions**
5. **Create choice structures for each class**
6. **Document game server integration pattern**

## Phase 2: Mechanics System

The mechanics system (damage, healing, light, etc.) will build on top of this foundation, allowing spells and magic items to reference these base items and modify them.

## Examples

### Rogue Equipment Choices

```go
func RogueEquipmentChoices() []choices.Choice {
    return []choices.Choice{
        {
            ID:          "rogue-weapon",
            Description: "(a) a rapier or (b) a shortsword",
            Choose:      1,
            Options: []choices.Option{
                choices.SingleOption{ItemType: choices.ItemTypeWeapon, ItemID: "rapier"},
                choices.SingleOption{ItemType: choices.ItemTypeWeapon, ItemID: "shortsword"},
            },
        },
        {
            ID:          "rogue-secondary",
            Description: "(a) shortbow and 20 arrows or (b) shortsword",
            Choose:      1,
            Options: []choices.Option{
                choices.BundleOption{
                    ID: "bow-and-arrows",
                    Items: []choices.CountedItem{
                        {ItemType: choices.ItemTypeWeapon, ItemID: "shortbow", Quantity: 1},
                        {ItemType: choices.ItemTypeGear, ItemID: "arrow", Quantity: 20},
                    },
                },
                choices.SingleOption{ItemType: choices.ItemTypeWeapon, ItemID: "shortsword"},
            },
        },
    }
}
```

### Wizard Spell Choices

```go
// For spell choices (Phase 2 preview)
type SpellListOption struct {
    Level    int
    Count    int
    ListName string  // "wizard", "cleric", etc.
}

func WizardSpellChoices() []choices.Choice {
    return []choices.Choice{
        {
            ID:          "wizard-cantrips",
            Description: "Choose 3 cantrips from the wizard spell list",
            Choose:      3,
            Options: []choices.Option{
                choices.SpellListOption{
                    Level:    0,  // Cantrips
                    Count:    3,
                    ListName: "wizard",
                },
            },
        },
    }
}
```