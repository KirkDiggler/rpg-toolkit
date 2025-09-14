// Package spells provides D&D 5e spell definitions and mechanics
package spells

import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"

// Spell represents a specific spell or cantrip
type Spell = shared.SelectionID

// Damage Cantrips
const (
	// Wizard Cantrips
	FireBolt      Spell = "fire-bolt"
	RayOfFrost    Spell = "ray-of-frost"
	ShockingGrasp Spell = "shocking-grasp"
	AcidSplash    Spell = "acid-splash"
	PoisonSpray   Spell = "poison-spray"
	ChillTouch    Spell = "chill-touch"

	// Cleric Cantrips
	SacredFlame    Spell = "sacred-flame"
	TollTheDead    Spell = "toll-the-dead"
	WordOfRadiance Spell = "word-of-radiance"

	// Warlock Cantrips
	EldritchBlast Spell = "eldritch-blast"

	// Druid Cantrips
	Frostbite      Spell = "frostbite"
	PrimalSavagery Spell = "primal-savagery"
	Thornwhip      Spell = "thornwhip"
	CreateBonfire  Spell = "create-bonfire"
	Druidcraft     Spell = "druidcraft"
	Infestation    Spell = "infestation"
	MagicStone     Spell = "magic-stone"
	MoldEarth      Spell = "mold-earth"
	ShapeWater     Spell = "shape-water"

	// Sorcerer Cantrips
	BoomingBlade    Spell = "booming-blade"
	ControlFlames   Spell = "control-flames"
	GreenFlameBlade Spell = "green-flame-blade"
	GustWind        Spell = "gust"
	SwordBurst      Spell = "sword-burst"
)

// Utility Cantrips
const (
	MageHand         Spell = "mage-hand"
	MinorIllusion    Spell = "minor-illusion"
	Prestidigitation Spell = "prestidigitation"
	Light            Spell = "light"
	Guidance         Spell = "guidance"
	Resistance       Spell = "resistance"
	Thaumaturgy      Spell = "thaumaturgy"
	SpareTheDying    Spell = "spare-the-dying"

	// Bard Cantrips
	BladeWard      Spell = "blade-ward"
	DancingLights  Spell = "dancing-lights"
	Friends        Spell = "friends"
	Mending        Spell = "mending"
	Message        Spell = "message"
	TrueStrike     Spell = "true-strike"
	ViciousMockery Spell = "vicious-mockery"
)

// Level 1 Damage Spells
const (
	// Wizard Level 1
	MagicMissile Spell = "magic-missile"
	BurningHands Spell = "burning-hands"
	ChromaticOrb Spell = "chromatic-orb"
	Thunderwave  Spell = "thunderwave"
	IceKnife     Spell = "ice-knife"
	WitchBolt    Spell = "witch-bolt"

	// Cleric Level 1
	GuidingBolt   Spell = "guiding-bolt"
	InflictWounds Spell = "inflict-wounds"

	// Ranger/Druid Level 1
	HailOfThorns    Spell = "hail-of-thorns"
	EnsnaringStrike Spell = "ensnaring-strike"

	// Warlock Level 1
	HellishRebuke Spell = "hellish-rebuke"
	ArmsOfHadar   Spell = "arms-of-hadar"
	Hex           Spell = "hex"

	// Paladin Level 1
	SearingSmite    Spell = "searing-smite"
	ThunderousSmite Spell = "thunderous-smite"
	WrathfulSmite   Spell = "wrathful-smite"
)

// Level 1 Utility Spells
const (
	Shield           Spell = "shield"
	Sleep            Spell = "sleep"
	CharmPerson      Spell = "charm-person"
	DetectMagic      Spell = "detect-magic"
	Identify         Spell = "identify"
	CureWounds       Spell = "cure-wounds"
	HealingWord      Spell = "healing-word"
	Bless            Spell = "bless"
	Bane             Spell = "bane"
	ShieldOfFaith    Spell = "shield-of-faith"
	AnimalFriendship Spell = "animal-friendship"
	Command          Spell = "command"
	DisguiseSelf     Spell = "disguise-self"
	DivineFavor      Spell = "divine-favor"
	FaerieFire       Spell = "faerie-fire"
	FalseLife        Spell = "false-life"
	FogCloud         Spell = "fog-cloud"
	RayOfSickness    Spell = "ray-of-sickness"
	SpeakWithAnimals Spell = "speak-with-animals"

	// Bard Level 1 specific
	ComprehendLanguages Spell = "comprehend-languages"
	FeatherFall         Spell = "feather-fall"
	Heroism             Spell = "heroism"
	HideousLaughter     Spell = "hideous-laughter"
	IllusoryScript      Spell = "illusory-script"
	Longstrider         Spell = "longstrider"
	SilentImage         Spell = "silent-image"
	UnseenServant       Spell = "unseen-servant"

	// Druid Level 1 specific
	AbsorbElements Spell = "absorb-elements"
	BeastBond      Spell = "beast-bond"
	Entangle       Spell = "entangle"
	GoodBerry      Spell = "goodberry"
	JumpSpell      Spell = "jump"
	PurifyFood     Spell = "purify-food-and-drink"

	// Sorcerer Level 1 specific
	CatapultSpell      Spell = "catapult"
	CauseFear          Spell = "cause-fear"
	ColorSpray         Spell = "color-spray"
	DistortValue       Spell = "distort-value"
	EarthTremor        Spell = "earth-tremor"
	ExpeditiousRetreat Spell = "expeditious-retreat"

	// Warlock Level 1 specific
	ProtectionEvil Spell = "protection-from-evil-and-good"
)

// Level 2 Damage Spells
const (
	ScorchingRay       Spell = "scorching-ray"
	Shatter            Spell = "shatter"
	AganazzarsScorcher Spell = "aganazzars-scorcher"
	CloudOfDaggers     Spell = "cloud-of-daggers"
	MelfsAcidArrow     Spell = "melfs-acid-arrow"
	Moonbeam           Spell = "moonbeam"
	SpiritualWeapon    Spell = "spiritual-weapon"
	FlamingSphere      Spell = "flaming-sphere"
	GustOfWind         Spell = "gust-of-wind"
	RayOfEnfeeblement  Spell = "ray-of-enfeeblement"
)

// Level 2 Utility Spells
const (
	Augury            Spell = "augury"
	Barkskin          Spell = "barkskin"
	BlindnessDeafness Spell = "blindness-deafness"
	LesserRestoration Spell = "lesser-restoration"
	MagicWeapon       Spell = "magic-weapon"
	MirrorImage       Spell = "mirror-image"
	PassWithoutTrace  Spell = "pass-without-trace"
	SpikeGrowth       Spell = "spike-growth"
	Suggestion        Spell = "suggestion"
)

// Level 3 Damage Spells
const (
	Fireball        Spell = "fireball"
	LightningBolt   Spell = "lightning-bolt"
	CallLightning   Spell = "call-lightning"
	VampiricTouch   Spell = "vampiric-touch"
	SleetStorm      Spell = "sleet-storm"
	SpiritGuardians Spell = "spirit-guardians"
)

// Level 3 Utility Spells
const (
	AnimateDead     Spell = "animate-dead"
	BeaconOfHope    Spell = "beacon-of-hope"
	Blink           Spell = "blink"
	CrusadersMantle Spell = "crusaders-mantle"
	Daylight        Spell = "daylight"
	DispelMagic     Spell = "dispel-magic"
	Nondetection    Spell = "nondetection"
	PlantGrowth     Spell = "plant-growth"
	Revivify        Spell = "revivify"
	SpeakWithDead   Spell = "speak-with-dead"
	WindWall        Spell = "wind-wall"
)

// Level 4 Spells
const (
	ArcaneEye         Spell = "arcane-eye"
	Blight            Spell = "blight"
	Confusion         Spell = "confusion"
	ControlWater      Spell = "control-water"
	DeathWard         Spell = "death-ward"
	DimensionDoor     Spell = "dimension-door"
	DominateBeast     Spell = "dominate-beast"
	FreedomOfMovement Spell = "freedom-of-movement"
	GraspingVine      Spell = "grasping-vine"
	GuardianOfFaith   Spell = "guardian-of-faith"
	IceStorm          Spell = "ice-storm"
	Polymorph         Spell = "polymorph"
	Stoneskin         Spell = "stoneskin"
	WallOfFire        Spell = "wall-of-fire"
)

// Level 5 Spells
const (
	AntiLifeShell   Spell = "antilife-shell"
	Cloudkill       Spell = "cloudkill"
	DestructiveWave Spell = "destructive-wave"
	DominatePerson  Spell = "dominate-person"
	FlameStrike     Spell = "flame-strike"
	HoldMonster     Spell = "hold-monster"
	InsectPlague    Spell = "insect-plague"
	LegendLore      Spell = "legend-lore"
	MassCureWounds  Spell = "mass-cure-wounds"
	ModifyMemory    Spell = "modify-memory"
	RaiseDead       Spell = "raise-dead"
	Scrying         Spell = "scrying"
	TreeStride      Spell = "tree-stride"
)

var spellName = map[Spell]string{
	// Cantrips
	FireBolt:       "Fire Bolt",
	RayOfFrost:     "Ray of Frost",
	ShockingGrasp:  "Shocking Grasp",
	AcidSplash:     "Acid Splash",
	PoisonSpray:    "Poison Spray",
	ChillTouch:     "Chill Touch",
	SacredFlame:    "Sacred Flame",
	TollTheDead:    "Toll the Dead",
	WordOfRadiance: "Word of Radiance",
	EldritchBlast:  "Eldritch Blast",
	Frostbite:      "Frostbite",
	PrimalSavagery: "Primal Savagery",
	Thornwhip:      "Thorn Whip",
	// Utility Cantrips
	MageHand:         "Mage Hand",
	MinorIllusion:    "Minor Illusion",
	Prestidigitation: "Prestidigitation",
	Light:            "Light",
	Guidance:         "Guidance",
	Resistance:       "Resistance",
	Thaumaturgy:      "Thaumaturgy",
	SpareTheDying:    "Spare the Dying",
	// Level 1 Damage
	MagicMissile:    "Magic Missile",
	BurningHands:    "Burning Hands",
	ChromaticOrb:    "Chromatic Orb",
	Thunderwave:     "Thunderwave",
	IceKnife:        "Ice Knife",
	WitchBolt:       "Witch Bolt",
	GuidingBolt:     "Guiding Bolt",
	InflictWounds:   "Inflict Wounds",
	HailOfThorns:    "Hail of Thorns",
	EnsnaringStrike: "Ensnaring Strike",
	HellishRebuke:   "Hellish Rebuke",
	ArmsOfHadar:     "Arms of Hadar",
	Hex:             "Hex",
	SearingSmite:    "Searing Smite",
	ThunderousSmite: "Thunderous Smite",
	WrathfulSmite:   "Wrathful Smite",
	// Level 1 Utility
	Shield:        "Shield",
	Sleep:         "Sleep",
	CharmPerson:   "Charm Person",
	DetectMagic:   "Detect Magic",
	Identify:      "Identify",
	CureWounds:    "Cure Wounds",
	HealingWord:   "Healing Word",
	Bless:         "Bless",
	Bane:          "Bane",
	ShieldOfFaith: "Shield of Faith",
	// Level 2 Damage
	ScorchingRay:       "Scorching Ray",
	Shatter:            "Shatter",
	AganazzarsScorcher: "Aganazzar's Scorcher",
	CloudOfDaggers:     "Cloud of Daggers",
	MelfsAcidArrow:     "Melf's Acid Arrow",
	Moonbeam:           "Moonbeam",
	SpiritualWeapon:    "Spiritual Weapon",
	FlamingSphere:      "Flaming Sphere",
	// Level 3 Damage
	Fireball:      "Fireball",
	LightningBolt: "Lightning Bolt",
	CallLightning: "Call Lightning",
	VampiricTouch: "Vampiric Touch",
	// Level 3 Utility
	AnimateDead:     "Animate Dead",
	BeaconOfHope:    "Beacon of Hope",
	Blink:           "Blink",
	CrusadersMantle: "Crusader's Mantle",
	Daylight:        "Daylight",
	DispelMagic:     "Dispel Magic",
	Nondetection:    "Nondetection",
	PlantGrowth:     "Plant Growth",
	Revivify:        "Revivify",
	SpeakWithDead:   "Speak with Dead",
	WindWall:        "Wind Wall",
	// Level 4 Damage
	ArcaneEye:         "Arcane Eye",
	Blight:            "Blight",
	Confusion:         "Confusion",
	ControlWater:      "Control Water",
	DeathWard:         "Death Ward",
	DimensionDoor:     "Dimension Door",
	DominateBeast:     "Dominate Beast",
	FreedomOfMovement: "Freedom of Movement",
	GraspingVine:      "Grasping Vine",
	GuardianOfFaith:   "Guardian of Faith",
	IceStorm:          "Ice Storm",
	Polymorph:         "Polymorph",
	Stoneskin:         "Stoneskin",
	WallOfFire:        "Wall of Fire",
	// Level 5 Damage
	AntiLifeShell:   "Anti-Life Shell",
	Cloudkill:       "Cloudkill",
	DestructiveWave: "Destructive Wave",
	DominatePerson:  "Dominate Person",
	FlameStrike:     "Flame Strike",
	HoldMonster:     "Hold Monster",
	InsectPlague:    "Insect Plague",
	LegendLore:      "Legend Lore",
	MassCureWounds:  "Mass Cure Wounds",
	ModifyMemory:    "Modify Memory",
	RaiseDead:       "Raise Dead",
	Scrying:         "Scrying",
	TreeStride:      "Tree Stride",
}

// Name returns the display name of the spell
func Name(s Spell) string {
	if name, ok := spellName[s]; ok {
		return name
	}

	return ""
}

var spellSlotDescription = map[Spell]string{
	FireBolt:        "Hurl a mote of fire at a creature or object (1d10 fire damage)",
	RayOfFrost:      "A frigid beam that deals 1d8 cold damage and reduces speed by 10 feet",
	ShockingGrasp:   "Lightning springs from your hand dealing 1d8 lightning damage, advantage vs metal armor",
	AcidSplash:      "Hurl a bubble of acid at creatures for 1d6 acid damage",
	PoisonSpray:     "Project a puff of noxious gas dealing 1d12 poison damage",
	ChillTouch:      "Assail with necrotic energy for 1d8 damage and prevent healing",
	SacredFlame:     "Flame-like radiance descends for 1d8 radiant damage",
	TollTheDead:     "Point at a creature and sound a dolorous bell for 1d8/1d12 necrotic damage",
	WordOfRadiance:  "Burning radiance erupts from you for 1d6 radiant damage to nearby enemies",
	EldritchBlast:   "A beam of crackling energy streaks toward a foe for 1d10 force damage",
	Frostbite:       "Cause numbing frost for 1d6 cold damage and disadvantage on next weapon attack",
	PrimalSavagery:  "Your teeth or nails sharpen for a 1d10 acid damage melee attack",
	Thornwhip:       "A vine-like whip deals 1d6 piercing damage and pulls the target closer",
	MagicMissile:    "Three darts of magical force, each dealing 1d4+1 damage, automatically hit",
	BurningHands:    "Cone of fire from your hands deals 3d6 fire damage",
	ChromaticOrb:    "Hurl a sphere of energy dealing 3d8 damage of a chosen type",
	Thunderwave:     "A wave of thunderous force deals 2d8 thunder damage and pushes creatures",
	IceKnife:        "Create a shard of ice that deals 1d10 piercing then explodes for 2d6 cold",
	WitchBolt:       "A beam of crackling energy deals 1d12 lightning damage with sustained arc",
	GuidingBolt:     "A flash of light deals 4d6 radiant damage and grants advantage on next attack",
	InflictWounds:   "Touch deals 3d10 necrotic damage to a creature",
	HailOfThorns:    "Next ranged attack deals extra 1d10 piercing damage in area",
	EnsnaringStrike: "Your next weapon hit entangles the target with thorny vines",
	HellishRebuke:   "Reactively engulf attacker in flames for 2d10 fire damage",
	ArmsOfHadar:     "Dark tendrils erupt for 2d6 necrotic damage and prevent reactions",
	Hex:             "Curse a target for extra 1d6 necrotic damage and disadvantage on ability checks",
	SearingSmite:    "Next melee hit deals extra 1d6 fire damage and ignites the target",
	ThunderousSmite: "Next melee hit deals extra 2d6 thunder damage and pushes the target",
	WrathfulSmite:   "Next melee hit deals extra 1d6 psychic damage and frightens the target",
	// Level 2 Damage
	ScorchingRay:       "Create three rays of fire, each dealing 2d6 fire damage on hit",
	Shatter:            "A sudden ringing noise deals 3d8 thunder damage in a 10-foot sphere",
	AganazzarsScorcher: "A line of roaring flame 30 feet long deals 3d8 fire damage",
	CloudOfDaggers:     "Fill the air with spinning daggers dealing 4d4 slashing damage",
	MelfsAcidArrow:     "A shimmering arrow deals 4d4 acid damage immediately and 2d4 at end of next turn",
	Moonbeam:           "A silvery beam of light deals 2d10 radiant damage each turn",
	SpiritualWeapon:    "Create a floating weapon that attacks for 1d8+modifier force damage",
	FlamingSphere:      "A 5-foot sphere of fire deals 2d6 damage and can be moved as bonus action",
	// Level 3 Damage
	Fireball:      "A bright streak explodes in a 20-foot sphere for 8d6 fire damage",
	LightningBolt: "A stroke of lightning 100 feet long deals 8d6 lightning damage",
	CallLightning: "Storm cloud strikes for 3d10 lightning damage, repeatable each turn",
	VampiricTouch: "Touch deals 3d6 necrotic damage and you regain half as hit points",
	// Utility spells
	Shield:        "Invisible barrier grants +5 AC until start of your next turn",
	Sleep:         "Send creatures into magical slumber (5d8 hit points affected)",
	CharmPerson:   "Charm a humanoid to regard you as a friendly acquaintance",
	DetectMagic:   "Sense the presence of magic within 30 feet",
	Identify:      "Learn the properties of a magic item or spell affecting a creature",
	CureWounds:    "Touch heals a creature for 1d8+modifier hit points",
	HealingWord:   "Speak a word of healing to restore 1d4+modifier hit points at range",
	Bless:         "Bless up to three creatures, adding 1d4 to attack rolls and saves",
	Bane:          "Curse enemies to subtract 1d4 from attack rolls and saves",
	ShieldOfFaith: "Shimmering field grants +2 AC for 10 minutes",
}

// Description returns a brief description of the spell's effect
func Description(s Spell) string {
	if desc, ok := spellSlotDescription[s]; ok {
		return desc
	}

	return ""
}
