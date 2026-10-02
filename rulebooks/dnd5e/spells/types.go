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
	Shillelagh     Spell = "shillelagh"
	Frostbite      Spell = "frostbite"
	PrimalSavagery Spell = "primal-savagery"
	Thornwhip      Spell = "thorn-whip"
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
	Thunderclap    Spell = "thunderclap"
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

	// Bard Level 1
	DissonantWhispers Spell = "dissonant-whispers"

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
	Sanctuary        Spell = "sanctuary"
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

// uncataloguedSpellNames preserves name-only reads for identifiers that do not
// yet have catalogue metadata. An entry moves to SpellData when its metadata
// is authored; catalogue names are never duplicated here.
var uncataloguedSpellNames = map[Spell]string{
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

// Name returns the canonical catalogue name, or a pre-existing name-only label
// for an identifier whose catalogue metadata has not been authored.
func Name(s Spell) string {
	if data := GetData(s); data != nil {
		return data.Name
	}
	return uncataloguedSpellNames[s]
}

// Description returns the catalogue's brief description, or an empty string
// when the spell has no catalogue metadata.
func Description(s Spell) string {
	if data := GetData(s); data != nil {
		return data.Description
	}
	return ""
}
