// Package classes provides D&D 5e class constants
package classes

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Class represents a D&D 5e character class
type Class string

// Subclass represents a D&D 5e character subclass/archetype
type Subclass string

// Subclass constants
const (
	SubclassNone Subclass = "none" // No subclass (for characters below subclass level)
)

// Fighter subclasses
const (
	Champion       Subclass = "champion"
	BattleMaster   Subclass = "battle-master"
	EldritchKnight Subclass = "eldritch-knight"
)

// Barbarian subclasses
const (
	Berserker         Subclass = "berserker"
	Totem             Subclass = "totem-warrior"
	AncestralGuardian Subclass = "ancestral-guardian"
)

// Bard subclasses
const (
	Lore    Subclass = "lore"
	Valor   Subclass = "valor"
	Glamour Subclass = "glamour"
)

// Cleric subclasses
const (
	LifeDomain      Subclass = "life-domain"
	LightDomain     Subclass = "light-domain"
	NatureDomain    Subclass = "nature-domain"
	TempestDomain   Subclass = "tempest-domain"
	TrickeryDomain  Subclass = "trickery-domain"
	WarDomain       Subclass = "war-domain"
	KnowledgeDomain Subclass = "knowledge-domain"
	DeathDomain     Subclass = "death-domain"
)

// Druid subclasses
const (
	CircleLand   Subclass = "circle-land"
	CircleMoon   Subclass = "circle-moon"
	CircleDreams Subclass = "circle-dreams"
)

// Monk subclasses
const (
	OpenHand     Subclass = "open-hand"
	Shadow       Subclass = "shadow"
	FourElements Subclass = "four-elements"
)

// Paladin subclasses
const (
	Devotion    Subclass = "devotion"
	Ancients    Subclass = "ancients"
	Vengeance   Subclass = "vengeance"
	Oathbreaker Subclass = "oathbreaker"
)

// Ranger subclasses
const (
	Hunter       Subclass = "hunter"
	BeastMaster  Subclass = "beast-master"
	Gloomstalker Subclass = "gloom-stalker"
)

// Rogue subclasses
const (
	Thief           Subclass = "thief"
	Assassin        Subclass = "assassin"
	ArcaneTrickster Subclass = "arcane-trickster"
)

// Sorcerer subclasses
const (
	DraconicBloodline Subclass = "draconic-bloodline"
	WildMagic         Subclass = "wild-magic"
	DivineSoul        Subclass = "divine-soul"
)

// Warlock subclasses
const (
	Archfey     Subclass = "archfey"
	Fiend       Subclass = "fiend"
	GreatOldOne Subclass = "great-old-one"
	Hexblade    Subclass = "hexblade"
)

// Wizard subclasses
const (
	Abjuration    Subclass = "abjuration"
	Conjuration   Subclass = "conjuration"
	Divination    Subclass = "divination"
	Enchantment   Subclass = "enchantment"
	Evocation     Subclass = "evocation"
	Illusion      Subclass = "illusion"
	Necromancy    Subclass = "necromancy"
	Transmutation Subclass = "transmutation"
)

// Core classes from Player's Handbook
const (
	Invalid   Class = "invalid" // Invalid or unknown class
	Barbarian Class = "barbarian"
	Bard      Class = "bard"
	Cleric    Class = "cleric"
	Druid     Class = "druid"
	Fighter   Class = "fighter"
	Monk      Class = "monk"
	Paladin   Class = "paladin"
	Ranger    Class = "ranger"
	Rogue     Class = "rogue"
	Sorcerer  Class = "sorcerer"
	Warlock   Class = "warlock"
	Wizard    Class = "wizard"
)

// All provides map lookup for classes
// Deprecated: Use ClassData directly - it now contains ID field and Name()/Description() methods
var All = map[string]Class{
	"barbarian": Barbarian,
	"bard":      Bard,
	"cleric":    Cleric,
	"druid":     Druid,
	"fighter":   Fighter,
	"monk":      Monk,
	"paladin":   Paladin,
	"ranger":    Ranger,
	"rogue":     Rogue,
	"sorcerer":  Sorcerer,
	"warlock":   Warlock,
	"wizard":    Wizard,
}

// GetByID returns a class by its ID
func GetByID(id string) (Class, error) {
	class, ok := All[id]
	if !ok {
		validClasses := make([]string, 0, len(All))
		for k := range All {
			validClasses = append(validClasses, k)
		}
		return "", rpgerr.New(rpgerr.CodeInvalidArgument, "invalid class",
			rpgerr.WithMeta("provided", id),
			rpgerr.WithMeta("valid_options", validClasses))
	}
	return class, nil
}

// subclassNames maps subclass constants to their display names
//
//nolint:dupl // Intentionally separate from subclassDescriptions and subclassParents
var subclassNames = map[Subclass]string{
	// Fighter
	Champion:       "Champion",
	BattleMaster:   "Battle Master",
	EldritchKnight: "Eldritch Knight",
	// Barbarian
	Berserker:         "Path of the Berserker",
	Totem:             "Path of the Totem Warrior",
	AncestralGuardian: "Path of the Ancestral Guardian",
	// Bard
	Lore:    "College of Lore",
	Valor:   "College of Valor",
	Glamour: "College of Glamour",
	// Cleric
	LifeDomain:      "Life Domain",
	LightDomain:     "Light Domain",
	NatureDomain:    "Nature Domain",
	TempestDomain:   "Tempest Domain",
	TrickeryDomain:  "Trickery Domain",
	WarDomain:       "War Domain",
	KnowledgeDomain: "Knowledge Domain",
	DeathDomain:     "Death Domain",
	// Druid
	CircleLand:   "Circle of the Land",
	CircleMoon:   "Circle of the Moon",
	CircleDreams: "Circle of Dreams",
	// Monk
	OpenHand:     "Way of the Open Hand",
	Shadow:       "Way of Shadow",
	FourElements: "Way of the Four Elements",
	// Paladin
	Devotion:    "Oath of Devotion",
	Ancients:    "Oath of the Ancients",
	Vengeance:   "Oath of Vengeance",
	Oathbreaker: "Oathbreaker",
	// Ranger
	Hunter:       "Hunter",
	BeastMaster:  "Beast Master",
	Gloomstalker: "Gloom Stalker",
	// Rogue
	Thief:           "Thief",
	Assassin:        "Assassin",
	ArcaneTrickster: "Arcane Trickster",
	// Sorcerer
	DraconicBloodline: "Draconic Bloodline",
	WildMagic:         "Wild Magic",
	DivineSoul:        "Divine Soul",
	// Warlock
	Archfey:     "The Archfey",
	Fiend:       "The Fiend",
	GreatOldOne: "The Great Old One",
	Hexblade:    "Hexblade",
	// Wizard
	Abjuration:    "School of Abjuration",
	Conjuration:   "School of Conjuration",
	Divination:    "School of Divination",
	Enchantment:   "School of Enchantment",
	Evocation:     "School of Evocation",
	Illusion:      "School of Illusion",
	Necromancy:    "School of Necromancy",
	Transmutation: "School of Transmutation",
}

// subclassDescriptions maps subclass constants to their descriptions
//
//nolint:dupl // Intentionally separate from subclassNames and subclassParents
var subclassDescriptions = map[Subclass]string{
	// Fighter
	Champion:       "A master of raw physical prowess and athletic ability",
	BattleMaster:   "A student of combat tactics and battlefield control",
	EldritchKnight: "A warrior who combines martial might with arcane magic",
	// Barbarian
	Berserker:         "Fueled by fury to become an unstoppable force of destruction",
	Totem:             "Draws power from spirit animals to gain supernatural abilities",
	AncestralGuardian: "Calls upon ancestral spirits for protection and vengeance",
	// Bard
	Lore:    "Masters of song, speech, and the magic they contain",
	Valor:   "Daring skalds whose tales keep alive the memory of great heroes",
	Glamour: "Taught by satyrs and fey, weaving magic through captivating performance",
	// Cleric
	LifeDomain:      "Focuses on the vibrant positive energy that sustains all life",
	LightDomain:     "Infused with radiance, wielding fire and light against darkness",
	NatureDomain:    "Masters of nature granted power by nature deities",
	TempestDomain:   "Commands the storm, delivering divine wrath through thunder and lightning",
	TrickeryDomain:  "Channels divine mischief and subterfuge",
	WarDomain:       "Champions of battle, inspiring others to fight",
	KnowledgeDomain: "Values learning and understanding above all",
	DeathDomain:     "Concerned with the forces that cause death and undeath",
	// Druid
	CircleLand:   "Mystics and sages who safeguard ancient knowledge and rites",
	CircleMoon:   "Fierce guardians of the wilds who channel primal beast forms",
	CircleDreams: "Has ties to the Feywild and its dreamlike realms",
	// Monk
	OpenHand:     "Masters of unarmed combat and martial arts techniques",
	Shadow:       "Follows a tradition of stealth and subterfuge",
	FourElements: "Harnesses the power of the four elements",
	// Paladin
	Devotion:    "Bound to the highest ideals of justice, virtue, and order",
	Ancients:    "Fights for light and life in an eternal struggle against darkness",
	Vengeance:   "A solemn commitment to punish those who have committed grievous sins",
	Oathbreaker: "Has forsaken their sacred oath in pursuit of dark ambitions",
	// Ranger
	Hunter:       "Accepts the place as a bulwark between civilization and wilderness",
	BeastMaster:  "Forms a powerful bond with a beast companion",
	Gloomstalker: "At home in the darkest places, ambushing threats from the shadows",
	// Rogue
	Thief:           "Hones skills in larceny and agility",
	Assassin:        "Focuses on the grim art of death",
	ArcaneTrickster: "Enhances cunning with magic",
	// Sorcerer
	DraconicBloodline: "Magic inherited from draconic ancestry",
	WildMagic:         "Power from the raw chaos of wild magic",
	DivineSoul:        "Divine magic from a connection to the divine",
	// Warlock
	Archfey:     "Formed a pact with a lord or lady of the fey",
	Fiend:       "Made a pact with a fiend from the lower planes",
	GreatOldOne: "Bound to an alien entity from the Far Realm",
	Hexblade:    "Forged a pact with a mysterious entity from the Shadowfell",
	// Wizard
	Abjuration:    "Masters of protective magic and banishment",
	Conjuration:   "Savants of summoning and teleportation",
	Divination:    "Masters of discernment, remote viewing, and foresight",
	Enchantment:   "Masters of enchanting and beguiling others",
	Evocation:     "Sculptors of spells, shaping energy into desired effects",
	Illusion:      "Masters of deception and tricks of the mind",
	Necromancy:    "Explorers of the cosmic forces of life, death, and undeath",
	Transmutation: "Students of spells that modify energy and matter",
}

// subclassParents maps subclasses to their parent class
//
//nolint:dupl // Intentionally separate from subclassNames and subclassDescriptions
var subclassParents = map[Subclass]Class{
	// Fighter
	Champion:       Fighter,
	BattleMaster:   Fighter,
	EldritchKnight: Fighter,
	// Barbarian
	Berserker:         Barbarian,
	Totem:             Barbarian,
	AncestralGuardian: Barbarian,
	// Bard
	Lore:    Bard,
	Valor:   Bard,
	Glamour: Bard,
	// Cleric
	LifeDomain:      Cleric,
	LightDomain:     Cleric,
	NatureDomain:    Cleric,
	TempestDomain:   Cleric,
	TrickeryDomain:  Cleric,
	WarDomain:       Cleric,
	KnowledgeDomain: Cleric,
	DeathDomain:     Cleric,
	// Druid
	CircleLand:   Druid,
	CircleMoon:   Druid,
	CircleDreams: Druid,
	// Monk
	OpenHand:     Monk,
	Shadow:       Monk,
	FourElements: Monk,
	// Paladin
	Devotion:    Paladin,
	Ancients:    Paladin,
	Vengeance:   Paladin,
	Oathbreaker: Paladin,
	// Ranger
	Hunter:       Ranger,
	BeastMaster:  Ranger,
	Gloomstalker: Ranger,
	// Rogue
	Thief:           Rogue,
	Assassin:        Rogue,
	ArcaneTrickster: Rogue,
	// Sorcerer
	DraconicBloodline: Sorcerer,
	WildMagic:         Sorcerer,
	DivineSoul:        Sorcerer,
	// Warlock
	Archfey:     Warlock,
	Fiend:       Warlock,
	GreatOldOne: Warlock,
	Hexblade:    Warlock,
	// Wizard
	Abjuration:    Wizard,
	Conjuration:   Wizard,
	Divination:    Wizard,
	Enchantment:   Wizard,
	Evocation:     Wizard,
	Illusion:      Wizard,
	Necromancy:    Wizard,
	Transmutation: Wizard,
}

// String returns the display name of the class
func (c Class) String() string {
	switch c {
	case Barbarian:
		return "Barbarian"
	case Bard:
		return "Bard"
	case Cleric:
		return "Cleric"
	case Druid:
		return "Druid"
	case Fighter:
		return "Fighter"
	case Monk:
		return "Monk"
	case Paladin:
		return "Paladin"
	case Ranger:
		return "Ranger"
	case Rogue:
		return "Rogue"
	case Sorcerer:
		return "Sorcerer"
	case Warlock:
		return "Warlock"
	case Wizard:
		return "Wizard"
	default:
		return string(c)
	}
}

// Name returns the display name of the class
func (c Class) Name() string {
	return c.String()
}

// Description returns a brief description of the class
func (c Class) Description() string {
	switch c {
	case Barbarian:
		return "A fierce warrior of primitive background who can enter a battle rage"
	case Bard:
		return "An inspiring magician whose power echoes the music of creation"
	case Cleric:
		return "A priestly champion who wields divine magic in service of a higher power"
	case Druid:
		return "A priest of nature, wielding elemental forces and transforming into beasts"
	case Fighter:
		return "A master of martial combat, skilled with a variety of weapons and armor"
	case Monk:
		return "A master of martial arts, harnessing inner power through discipline"
	case Paladin:
		return "A holy warrior bound to a sacred oath, smiting foes with divine might"
	case Ranger:
		return "A warrior of the wilderness, skilled in tracking, survival, and combat"
	case Rogue:
		return "A scoundrel who uses stealth and trickery to overcome obstacles"
	case Sorcerer:
		return "A spellcaster who draws on inherent magic from a gift or bloodline"
	case Warlock:
		return "A wielder of magic derived from a bargain with an extraplanar entity"
	case Wizard:
		return "A scholarly magic-user capable of manipulating the structures of reality"
	default:
		return ""
	}
}

// Parent returns the base class for a subclass
func (s Subclass) Parent() Class {
	if parent, ok := subclassParents[s]; ok {
		return parent
	}
	return Invalid
}

// String returns the display name of the subclass
func (s Subclass) String() string {
	if name, ok := subclassNames[s]; ok {
		return name
	}

	return string(s)
}

// Name returns the display name of the subclass
func (s Subclass) Name() string {
	return s.String()
}

// Description returns a brief description of the subclass
func (s Subclass) Description() string {
	if desc, ok := subclassDescriptions[s]; ok {
		return desc
	}
	return ""
}
