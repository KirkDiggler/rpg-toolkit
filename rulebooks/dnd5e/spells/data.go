package spells

// Data contains all the game mechanics data for a spell
type Data struct {
	ID          Spell  // The spell this data represents
	Level       int    // 0 for cantrips, 1-9 for leveled spells
	Name        string // Display name
	Description string // Brief description of the spell's effect
}

// SpellData is the lookup map for all spell data
// Only includes spells that are defined in the proto enum
var SpellData = map[Spell]*Data{
	// Cantrips (Level 0)
	FireBolt: {
		ID:          FireBolt,
		Level:       0,
		Name:        "Fire Bolt",
		Description: "Hurl a mote of fire at a creature or object (1d10 fire damage)",
	},
	RayOfFrost: {
		ID:          RayOfFrost,
		Level:       0,
		Name:        "Ray of Frost",
		Description: "A frigid beam that deals 1d8 cold damage and reduces speed by 10 feet",
	},
	ShockingGrasp: {
		ID:          ShockingGrasp,
		Level:       0,
		Name:        "Shocking Grasp",
		Description: "Lightning springs from your hand dealing 1d8 lightning damage, advantage vs metal armor",
	},
	AcidSplash: {
		ID:          AcidSplash,
		Level:       0,
		Name:        "Acid Splash",
		Description: "Hurl a bubble of acid at creatures for 1d6 acid damage",
	},
	PoisonSpray: {
		ID:          PoisonSpray,
		Level:       0,
		Name:        "Poison Spray",
		Description: "Project a puff of noxious gas dealing 1d12 poison damage",
	},
	ChillTouch: {
		ID:          ChillTouch,
		Level:       0,
		Name:        "Chill Touch",
		Description: "Assail with necrotic energy for 1d8 damage and prevent healing",
	},
	SacredFlame: {
		ID:          SacredFlame,
		Level:       0,
		Name:        "Sacred Flame",
		Description: "Flame-like radiance descends for 1d8 radiant damage",
	},
	TollTheDead: {
		ID:          TollTheDead,
		Level:       0,
		Name:        "Toll the Dead",
		Description: "Point at a creature and sound a dolorous bell for 1d8/1d12 necrotic damage",
	},
	WordOfRadiance: {
		ID:          WordOfRadiance,
		Level:       0,
		Name:        "Word of Radiance",
		Description: "Burning radiance erupts from you for 1d6 radiant damage to nearby enemies",
	},
	EldritchBlast: {
		ID:          EldritchBlast,
		Level:       0,
		Name:        "Eldritch Blast",
		Description: "A beam of crackling energy streaks toward a foe for 1d10 force damage",
	},
	Frostbite: {
		ID:          Frostbite,
		Level:       0,
		Name:        "Frostbite",
		Description: "Cause numbing frost for 1d6 cold damage and disadvantage on next weapon attack",
	},
	PrimalSavagery: {
		ID:          PrimalSavagery,
		Level:       0,
		Name:        "Primal Savagery",
		Description: "Your teeth or nails sharpen for a 1d10 acid damage melee attack",
	},
	Thornwhip: {
		ID:          Thornwhip,
		Level:       0,
		Name:        "Thorn Whip",
		Description: "A vine-like whip deals 1d6 piercing damage and pulls the target closer",
	},
	MageHand: {
		ID:          MageHand,
		Level:       0,
		Name:        "Mage Hand",
		Description: "Create a spectral hand that can manipulate objects at range",
	},
	MinorIllusion: {
		ID:          MinorIllusion,
		Level:       0,
		Name:        "Minor Illusion",
		Description: "Create a sound or image that lasts for 1 minute",
	},
	Prestidigitation: {
		ID:          Prestidigitation,
		Level:       0,
		Name:        "Prestidigitation",
		Description: "Perform a minor magical trick",
	},
	Light: {
		ID:          Light,
		Level:       0,
		Name:        "Light",
		Description: "Touch an object to make it shed bright light",
	},
	Guidance: {
		ID:          Guidance,
		Level:       0,
		Name:        "Guidance",
		Description: "Touch a willing creature to add 1d4 to one ability check",
	},
	Resistance: {
		ID:          Resistance,
		Level:       0,
		Name:        "Resistance",
		Description: "Touch a willing creature to add 1d4 to one saving throw",
	},
	Thaumaturgy: {
		ID:          Thaumaturgy,
		Level:       0,
		Name:        "Thaumaturgy",
		Description: "Manifest minor wonders that show supernatural power",
	},
	SpareTheDying: {
		ID:          SpareTheDying,
		Level:       0,
		Name:        "Spare the Dying",
		Description: "Stabilize a dying creature",
	},

	// Level 1 Spells
	MagicMissile: {
		ID:          MagicMissile,
		Level:       1,
		Name:        "Magic Missile",
		Description: "Three darts of magical force, each dealing 1d4+1 damage, automatically hit",
	},
	BurningHands: {
		ID:          BurningHands,
		Level:       1,
		Name:        "Burning Hands",
		Description: "Cone of fire from your hands deals 3d6 fire damage",
	},
	ChromaticOrb: {
		ID:          ChromaticOrb,
		Level:       1,
		Name:        "Chromatic Orb",
		Description: "Hurl a sphere of energy dealing 3d8 damage of a chosen type",
	},
	Thunderwave: {
		ID:          Thunderwave,
		Level:       1,
		Name:        "Thunderwave",
		Description: "A wave of thunderous force deals 2d8 thunder damage and pushes creatures",
	},
	IceKnife: {
		ID:          IceKnife,
		Level:       1,
		Name:        "Ice Knife",
		Description: "Create a shard of ice that deals 1d10 piercing then explodes for 2d6 cold",
	},
	WitchBolt: {
		ID:          WitchBolt,
		Level:       1,
		Name:        "Witch Bolt",
		Description: "A beam of crackling energy deals 1d12 lightning damage with sustained arc",
	},
	GuidingBolt: {
		ID:          GuidingBolt,
		Level:       1,
		Name:        "Guiding Bolt",
		Description: "A flash of light deals 4d6 radiant damage and grants advantage on next attack",
	},
	InflictWounds: {
		ID:          InflictWounds,
		Level:       1,
		Name:        "Inflict Wounds",
		Description: "Touch deals 3d10 necrotic damage to a creature",
	},
	HailOfThorns: {
		ID:          HailOfThorns,
		Level:       1,
		Name:        "Hail of Thorns",
		Description: "Next ranged attack deals extra 1d10 piercing damage in area",
	},
	EnsnaringStrike: {
		ID:          EnsnaringStrike,
		Level:       1,
		Name:        "Ensnaring Strike",
		Description: "Your next weapon hit entangles the target with thorny vines",
	},
	HellishRebuke: {
		ID:          HellishRebuke,
		Level:       1,
		Name:        "Hellish Rebuke",
		Description: "Reactively engulf attacker in flames for 2d10 fire damage",
	},
	ArmsOfHadar: {
		ID:          ArmsOfHadar,
		Level:       1,
		Name:        "Arms of Hadar",
		Description: "Dark tendrils erupt for 2d6 necrotic damage and prevent reactions",
	},
	Hex: {
		ID:          Hex,
		Level:       1,
		Name:        "Hex",
		Description: "Curse a target for extra 1d6 necrotic damage and disadvantage on ability checks",
	},
	SearingSmite: {
		ID:          SearingSmite,
		Level:       1,
		Name:        "Searing Smite",
		Description: "Next melee hit deals extra 1d6 fire damage and ignites the target",
	},
	ThunderousSmite: {
		ID:          ThunderousSmite,
		Level:       1,
		Name:        "Thunderous Smite",
		Description: "Next melee hit deals extra 2d6 thunder damage and pushes the target",
	},
	WrathfulSmite: {
		ID:          WrathfulSmite,
		Level:       1,
		Name:        "Wrathful Smite",
		Description: "Next melee hit deals extra 1d6 psychic damage and frightens the target",
	},
	Shield: {
		ID:          Shield,
		Level:       1,
		Name:        "Shield",
		Description: "Invisible barrier grants +5 AC until start of your next turn",
	},
	Sleep: {
		ID:          Sleep,
		Level:       1,
		Name:        "Sleep",
		Description: "Send creatures into magical slumber (5d8 hit points affected)",
	},
	CharmPerson: {
		ID:          CharmPerson,
		Level:       1,
		Name:        "Charm Person",
		Description: "Charm a humanoid to regard you as a friendly acquaintance",
	},
	DetectMagic: {
		ID:          DetectMagic,
		Level:       1,
		Name:        "Detect Magic",
		Description: "Sense the presence of magic within 30 feet",
	},
	Identify: {
		ID:          Identify,
		Level:       1,
		Name:        "Identify",
		Description: "Learn the properties of a magic item or spell affecting a creature",
	},
	CureWounds: {
		ID:          CureWounds,
		Level:       1,
		Name:        "Cure Wounds",
		Description: "Touch heals a creature for 1d8+modifier hit points",
	},
	HealingWord: {
		ID:          HealingWord,
		Level:       1,
		Name:        "Healing Word",
		Description: "Speak a word of healing to restore 1d4+modifier hit points at range",
	},
	Bless: {
		ID:          Bless,
		Level:       1,
		Name:        "Bless",
		Description: "Bless up to three creatures, adding 1d4 to attack rolls and saves",
	},
	Bane: {
		ID:          Bane,
		Level:       1,
		Name:        "Bane",
		Description: "Curse enemies to subtract 1d4 from attack rolls and saves",
	},
	ShieldOfFaith: {
		ID:          ShieldOfFaith,
		Level:       1,
		Name:        "Shield of Faith",
		Description: "Shimmering field grants +2 AC for 10 minutes",
	},

	// Level 2 Spells
	ScorchingRay: {
		ID:          ScorchingRay,
		Level:       2,
		Name:        "Scorching Ray",
		Description: "Create three rays of fire, each dealing 2d6 fire damage on hit",
	},
	Shatter: {
		ID:          Shatter,
		Level:       2,
		Name:        "Shatter",
		Description: "A sudden ringing noise deals 3d8 thunder damage in a 10-foot sphere",
	},
	AganazzarsScorcher: {
		ID:          AganazzarsScorcher,
		Level:       2,
		Name:        "Aganazzar's Scorcher",
		Description: "A line of roaring flame 30 feet long deals 3d8 fire damage",
	},
	CloudOfDaggers: {
		ID:          CloudOfDaggers,
		Level:       2,
		Name:        "Cloud of Daggers",
		Description: "Fill the air with spinning daggers dealing 4d4 slashing damage",
	},
	MelfsAcidArrow: {
		ID:          MelfsAcidArrow,
		Level:       2,
		Name:        "Melf's Acid Arrow",
		Description: "A shimmering arrow deals 4d4 acid damage immediately and 2d4 at end of next turn",
	},
	Moonbeam: {
		ID:          Moonbeam,
		Level:       2,
		Name:        "Moonbeam",
		Description: "A silvery beam of light deals 2d10 radiant damage each turn",
	},
	SpiritualWeapon: {
		ID:          SpiritualWeapon,
		Level:       2,
		Name:        "Spiritual Weapon",
		Description: "Create a floating weapon that attacks for 1d8+modifier force damage",
	},
	FlamingSphere: {
		ID:          FlamingSphere,
		Level:       2,
		Name:        "Flaming Sphere",
		Description: "A 5-foot sphere of fire deals 2d6 damage and can be moved as bonus action",
	},

	// Level 3 Spells
	Fireball: {
		ID:          Fireball,
		Level:       3,
		Name:        "Fireball",
		Description: "A bright streak explodes in a 20-foot sphere for 8d6 fire damage",
	},
	LightningBolt: {
		ID:          LightningBolt,
		Level:       3,
		Name:        "Lightning Bolt",
		Description: "A stroke of lightning 100 feet long deals 8d6 lightning damage",
	},
	CallLightning: {
		ID:          CallLightning,
		Level:       3,
		Name:        "Call Lightning",
		Description: "Storm cloud strikes for 3d10 lightning damage, repeatable each turn",
	},
	VampiricTouch: {
		ID:          VampiricTouch,
		Level:       3,
		Name:        "Vampiric Touch",
		Description: "Touch deals 3d6 necrotic damage and you regain half as hit points",
	},
}

// GetData returns the spell data for a given spell ID
func GetData(spellID Spell) *Data {
	return SpellData[spellID]
}

// GetSpellsByLevel returns all spells of a given level
func GetSpellsByLevel(level int) []*Data {
	var spells []*Data

	for _, data := range SpellData {
		if data.Level == level {
			spells = append(spells, data)
		}
	}

	return spells
}
