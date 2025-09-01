// Package spells provides D&D 5e spell definitions and mechanics
package spells

// Spell represents a specific spell or cantrip
type Spell string

// Common Cantrips
const (
	// Wizard Cantrips
	FireBolt         Spell = "fire-bolt"
	RayOfFrost       Spell = "ray-of-frost"
	ShockingGrasp    Spell = "shocking-grasp"
	MageHand         Spell = "mage-hand"
	MinorIllusion    Spell = "minor-illusion"
	Prestidigitation Spell = "prestidigitation"
	Light            Spell = "light"

	// Cleric Cantrips
	SacredFlame   Spell = "sacred-flame"
	Guidance      Spell = "guidance"
	Resistance    Spell = "resistance"
	Thaumaturgy   Spell = "thaumaturgy"
	SpareTheDying Spell = "spare-the-dying"

	// Warlock Cantrips
	EldritchBlast Spell = "eldritch-blast"
	ChillTouch    Spell = "chill-touch"

	// TODO: Add more cantrips as needed
)

// Level 1 Spells
const (
	// Wizard Level 1
	MagicMissile Spell = "magic-missile"
	Shield       Spell = "shield"
	Sleep        Spell = "sleep"
	CharmPerson  Spell = "charm-person"
	DetectMagic  Spell = "detect-magic"
	BurningHands Spell = "burning-hands"
	Identify     Spell = "identify"

	// Cleric Level 1
	CureWounds    Spell = "cure-wounds"
	HealingWord   Spell = "healing-word"
	Bless         Spell = "bless"
	Bane          Spell = "bane"
	ShieldOfFaith Spell = "shield-of-faith"

	// TODO: Add more level 1 spells as needed
)
