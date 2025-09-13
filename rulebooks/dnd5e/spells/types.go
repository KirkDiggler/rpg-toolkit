// Package spells provides D&D 5e spell definitions and mechanics
package spells

// Spell represents a specific spell or cantrip
type Spell string

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

// Name returns the display name of the spell
func (s Spell) Name() string {
	switch s {
	// Cantrips
	case FireBolt:
		return "Fire Bolt"
	case RayOfFrost:
		return "Ray of Frost"
	case ShockingGrasp:
		return "Shocking Grasp"
	case AcidSplash:
		return "Acid Splash"
	case PoisonSpray:
		return "Poison Spray"
	case ChillTouch:
		return "Chill Touch"
	case SacredFlame:
		return "Sacred Flame"
	case TollTheDead:
		return "Toll the Dead"
	case WordOfRadiance:
		return "Word of Radiance"
	case EldritchBlast:
		return "Eldritch Blast"
	case Frostbite:
		return "Frostbite"
	case PrimalSavagery:
		return "Primal Savagery"
	case Thornwhip:
		return "Thorn Whip"
	// Utility Cantrips
	case MageHand:
		return "Mage Hand"
	case MinorIllusion:
		return "Minor Illusion"
	case Prestidigitation:
		return "Prestidigitation"
	case Light:
		return "Light"
	case Guidance:
		return "Guidance"
	case Resistance:
		return "Resistance"
	case Thaumaturgy:
		return "Thaumaturgy"
	case SpareTheDying:
		return "Spare the Dying"
	// Level 1 Damage
	case MagicMissile:
		return "Magic Missile"
	case BurningHands:
		return "Burning Hands"
	case ChromaticOrb:
		return "Chromatic Orb"
	case Thunderwave:
		return "Thunderwave"
	case IceKnife:
		return "Ice Knife"
	case WitchBolt:
		return "Witch Bolt"
	case GuidingBolt:
		return "Guiding Bolt"
	case InflictWounds:
		return "Inflict Wounds"
	case HailOfThorns:
		return "Hail of Thorns"
	case EnsnaringStrike:
		return "Ensnaring Strike"
	case HellishRebuke:
		return "Hellish Rebuke"
	case ArmsOfHadar:
		return "Arms of Hadar"
	case Hex:
		return "Hex"
	case SearingSmite:
		return "Searing Smite"
	case ThunderousSmite:
		return "Thunderous Smite"
	case WrathfulSmite:
		return "Wrathful Smite"
	// Level 1 Utility
	case Shield:
		return "Shield"
	case Sleep:
		return "Sleep"
	case CharmPerson:
		return "Charm Person"
	case DetectMagic:
		return "Detect Magic"
	case Identify:
		return "Identify"
	case CureWounds:
		return "Cure Wounds"
	case HealingWord:
		return "Healing Word"
	case Bless:
		return "Bless"
	case Bane:
		return "Bane"
	case ShieldOfFaith:
		return "Shield of Faith"
	// Level 2 Damage
	case ScorchingRay:
		return "Scorching Ray"
	case Shatter:
		return "Shatter"
	case AganazzarsScorcher:
		return "Aganazzar's Scorcher"
	case CloudOfDaggers:
		return "Cloud of Daggers"
	case MelfsAcidArrow:
		return "Melf's Acid Arrow"
	case Moonbeam:
		return "Moonbeam"
	case SpiritualWeapon:
		return "Spiritual Weapon"
	case FlamingSphere:
		return "Flaming Sphere"
	// Level 3 Damage
	case Fireball:
		return "Fireball"
	case LightningBolt:
		return "Lightning Bolt"
	case CallLightning:
		return "Call Lightning"
	case VampiricTouch:
		return "Vampiric Touch"
	default:
		return string(s)
	}
}

// Description returns a brief description of the spell's effect
func (s Spell) Description() string {
	switch s {
	// Damage Cantrips
	case FireBolt:
		return "Hurl a mote of fire at a creature or object (1d10 fire damage)"
	case RayOfFrost:
		return "A frigid beam that deals 1d8 cold damage and reduces speed by 10 feet"
	case ShockingGrasp:
		return "Lightning springs from your hand dealing 1d8 lightning damage, advantage vs metal armor"
	case AcidSplash:
		return "Hurl a bubble of acid at creatures for 1d6 acid damage"
	case PoisonSpray:
		return "Project a puff of noxious gas dealing 1d12 poison damage"
	case ChillTouch:
		return "Assail with necrotic energy for 1d8 damage and prevent healing"
	case SacredFlame:
		return "Flame-like radiance descends for 1d8 radiant damage"
	case TollTheDead:
		return "Point at a creature and sound a dolorous bell for 1d8/1d12 necrotic damage"
	case WordOfRadiance:
		return "Burning radiance erupts from you for 1d6 radiant damage to nearby enemies"
	case EldritchBlast:
		return "A beam of crackling energy streaks toward a foe for 1d10 force damage"
	case Frostbite:
		return "Cause numbing frost for 1d6 cold damage and disadvantage on next weapon attack"
	case PrimalSavagery:
		return "Your teeth or nails sharpen for a 1d10 acid damage melee attack"
	case Thornwhip:
		return "A vine-like whip deals 1d6 piercing damage and pulls the target closer"
	// Level 1 Damage
	case MagicMissile:
		return "Three darts of magical force, each dealing 1d4+1 damage, automatically hit"
	case BurningHands:
		return "Cone of fire from your hands deals 3d6 fire damage"
	case ChromaticOrb:
		return "Hurl a sphere of energy dealing 3d8 damage of a chosen type"
	case Thunderwave:
		return "A wave of thunderous force deals 2d8 thunder damage and pushes creatures"
	case IceKnife:
		return "Create a shard of ice that deals 1d10 piercing then explodes for 2d6 cold"
	case WitchBolt:
		return "A beam of crackling energy deals 1d12 lightning damage with sustained arc"
	case GuidingBolt:
		return "A flash of light deals 4d6 radiant damage and grants advantage on next attack"
	case InflictWounds:
		return "Touch deals 3d10 necrotic damage to a creature"
	case HailOfThorns:
		return "Next ranged attack deals extra 1d10 piercing damage in area"
	case EnsnaringStrike:
		return "Your next weapon hit entangles the target with thorny vines"
	case HellishRebuke:
		return "Reactively engulf attacker in flames for 2d10 fire damage"
	case ArmsOfHadar:
		return "Dark tendrils erupt for 2d6 necrotic damage and prevent reactions"
	case Hex:
		return "Curse a target for extra 1d6 necrotic damage and disadvantage on ability checks"
	case SearingSmite:
		return "Next melee hit deals extra 1d6 fire damage and ignites the target"
	case ThunderousSmite:
		return "Next melee hit deals extra 2d6 thunder damage and pushes the target"
	case WrathfulSmite:
		return "Next melee hit deals extra 1d6 psychic damage and frightens the target"
	// Level 2 Damage
	case ScorchingRay:
		return "Create three rays of fire, each dealing 2d6 fire damage on hit"
	case Shatter:
		return "A sudden ringing noise deals 3d8 thunder damage in a 10-foot sphere"
	case AganazzarsScorcher:
		return "A line of roaring flame 30 feet long deals 3d8 fire damage"
	case CloudOfDaggers:
		return "Fill the air with spinning daggers dealing 4d4 slashing damage"
	case MelfsAcidArrow:
		return "A shimmering arrow deals 4d4 acid damage immediately and 2d4 at end of next turn"
	case Moonbeam:
		return "A silvery beam of light deals 2d10 radiant damage each turn"
	case SpiritualWeapon:
		return "Create a floating weapon that attacks for 1d8+modifier force damage"
	case FlamingSphere:
		return "A 5-foot sphere of fire deals 2d6 damage and can be moved as bonus action"
	// Level 3 Damage
	case Fireball:
		return "A bright streak explodes in a 20-foot sphere for 8d6 fire damage"
	case LightningBolt:
		return "A stroke of lightning 100 feet long deals 8d6 lightning damage"
	case CallLightning:
		return "Storm cloud strikes for 3d10 lightning damage, repeatable each turn"
	case VampiricTouch:
		return "Touch deals 3d6 necrotic damage and you regain half as hit points"
	// Utility spells
	case Shield:
		return "Invisible barrier grants +5 AC until start of your next turn"
	case Sleep:
		return "Send creatures into magical slumber (5d8 hit points affected)"
	case CharmPerson:
		return "Charm a humanoid to regard you as a friendly acquaintance"
	case DetectMagic:
		return "Sense the presence of magic within 30 feet"
	case Identify:
		return "Learn the properties of a magic item or spell affecting a creature"
	case CureWounds:
		return "Touch heals a creature for 1d8+modifier hit points"
	case HealingWord:
		return "Speak a word of healing to restore 1d4+modifier hit points at range"
	case Bless:
		return "Bless up to three creatures, adding 1d4 to attack rolls and saves"
	case Bane:
		return "Curse enemies to subtract 1d4 from attack rolls and saves"
	case ShieldOfFaith:
		return "Shimmering field grants +2 AC for 10 minutes"
	default:
		return ""
	}
}
