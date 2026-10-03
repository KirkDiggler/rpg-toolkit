package spells

// Data contains character-independent catalogue metadata for a spell.
// Executable mechanics are authored separately in the cast profiles; metadata
// lookup neither compiles an action nor grants permission to cast.
type Data struct {
	ID          Spell  // The spell this data represents
	Level       int    // 0 for cantrips, 1-9 for leveled spells
	Name        string // Display name
	Description string // Player-facing effect, choices, duration and important limits
	// NotYetImplemented marks catalog-only content, selectable/grantable but not castable.
	NotYetImplemented bool
}

// SpellData is the lookup map for all spell data
// Shared by every class and domain; canonical refs also identify catalog-only spells.
var SpellData = map[Spell]*Data{
	BladeWard:        {ID: BladeWard, Level: 0, Name: "Blade Ward", Description: "Use an action to protect yourself until the end of your next turn. You take half damage, rounded down, from bludgeoning, piercing, and slashing damage dealt by weapon attacks. This does not protect against other damage types. No concentration required."},
	FaerieFire:       {ID: FaerieFire, Level: 1, Name: "Faerie Fire", Description: "Use an action to outline creatures in a 20-foot cube within 60 feet. Each creature makes a Dexterity save; on a failure, attacks against it have advantage if the attacker can see it. The area can catch allies too. Requires concentration, up to 1 minute."},
	FogCloud:         {ID: FogCloud, Level: 1, Name: "Fog Cloud", Description: "Use an action to create a 20-foot-radius fog cloud around a point within 120 feet. The cloud blocks sight through its area, obscuring allies and enemies alike. Requires concentration, up to 1 hour. It deals no damage."},
	Thunderclap:      {ID: Thunderclap, Level: 0, Name: "Thunderclap", Description: "Use an action to burst with thunder. Every other creature within 5 feet, including allies, makes a Constitution save. A failed save deals 1d6 thunder damage; a successful save deals none. You are not hit by your own burst. No concentration required."},
	TrueStrike:       {ID: TrueStrike, Level: 0, Name: "True Strike", Description: "Use an action to focus on one creature within 30 feet. While concentrating, your next attack against that creature has advantage: roll two d20s and use the higher result, unless disadvantage cancels it. The benefit is consumed by that attack and otherwise lasts until the end of your next turn. It adds no damage."},
	ViciousMockery:   {ID: ViciousMockery, Level: 0, Name: "Vicious Mockery", Description: "Use an action to mock one creature within 60 feet. It makes a Wisdom save. On a failure, it takes 1d4 psychic damage and has disadvantage on its next attack roll before the end of its next turn: roll two d20s and use the lower result, unless advantage cancels it. A successful save prevents both effects. No concentration required."},
	Shillelagh:       {ID: Shillelagh, Level: 0, Name: "Shillelagh", Description: "Use a bonus action to enchant a club or quarterstaff you are holding. Its weapon die becomes 1d8 and it counts as magical. This build uses your spellcasting ability for its attacks and damage when that modifier is better than Strength. The enchantment lasts 1 minute, ending early if you cast it again or release the weapon. No concentration required."},
	Druidcraft:       {ID: Druidcraft, Level: 0, Name: "Druidcraft", Description: "Create minor natural wonders; its environmental interactions are not yet implemented", NotYetImplemented: true},
	Mending:          {ID: Mending, Level: 0, Name: "Mending", Description: "Repair a small break or tear in an object; object repair is not yet implemented", NotYetImplemented: true},
	AnimalFriendship: {ID: AnimalFriendship, Level: 1, Name: "Animal Friendship", Description: "Charm a beast; animal interactions are not yet implemented", NotYetImplemented: true},
	SpeakWithAnimals: {ID: SpeakWithAnimals, Level: 1, Name: "Speak with Animals", Description: "Understand and communicate with beasts; animal conversations are not yet implemented", NotYetImplemented: true},

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
		Description: "Use an action to spray one creature you can see within 10 feet. It makes a Constitution save, taking 1d12 poison damage on a failure and none on a success. This is a saving throw, not an attack roll. No concentration required.",
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
		Description: "Use an action to strike one creature you can see within 60 feet with radiant fire. It makes a Dexterity save, taking 1d8 radiant damage on a failure and none on a success. This is a saving throw, not an attack roll. No concentration required.",
	},
	TollTheDead: {
		ID:          TollTheDead,
		Level:       0,
		Name:        "Toll the Dead",
		Description: "Use an action to target one creature you can see within 60 feet. It makes a Wisdom save. On a failure, it takes 1d8 necrotic damage, or 1d12 instead if it is already missing any hit points. A successful save deals no damage. No concentration required.",
	},
	WordOfRadiance: {
		ID:          WordOfRadiance,
		Level:       0,
		Name:        "Word of Radiance",
		Description: "Use an action to choose creatures you can see within 5 feet. Each chosen creature makes a Constitution save, taking 1d6 radiant damage on a failure and none on a success. Unlike Thunderclap, you choose which nearby creatures to affect. No concentration required.",
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
		Description: "Use an action to make a melee spell attack against a creature within 30 feet. A hit deals 1d6 piercing damage. Choose No pull, Pull 5 feet, or Pull 10 feet; a chosen pull draws the creature toward you as far as the terrain permits. A miss deals no damage and pulls nothing. No concentration required.",
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
		NotYetImplemented: true,
		ID:                Light,
		Level:             0,
		Name:              "Light",
		Description:       "Touch an object to make it shed bright light",
	},
	Guidance: {
		ID:          Guidance,
		Level:       0,
		Name:        "Guidance",
		Description: "Use an action to touch a creature, including yourself, and give it a one-use d4 for an ability check. In this build, the creature chooses whether to add the die after rolling but before the result is revealed. Spending the die ends the benefit. Requires concentration, up to 1 minute; it does not help attack rolls or saving throws.",
	},
	Resistance: {
		ID:          Resistance,
		Level:       0,
		Name:        "Resistance",
		Description: "Use an action to touch a creature, including yourself, and give it a one-use d4 for a saving throw. In this build, the creature chooses whether to add the die after rolling but before the result is revealed. Spending the die ends the benefit. Requires concentration, up to 1 minute. This improves a save; it does not directly reduce damage.",
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
		Description: "Use an action to touch a living character at 0 hit points and make them stable. They stop making death saves, but regain no hit points and remain unconscious. This cannot revive someone who is already dead. This build supports character targets, not monsters. No concentration required.",
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
		Description: "Use an action to project a 15-foot cone of fire from yourself. Creatures in the cone, including allies, make a Dexterity save. They take 3d6 fire damage on a failure or half as much, rounded down, on a success. No concentration required.",
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
		Description: "Use an action to send a wave through a 15-foot cube extending from you. Other creatures caught in it, including allies, make a Constitution save. On a failure they take 2d8 thunder damage and are pushed up to 10 feet away, subject to terrain. On a success they take half damage, rounded down, and are not pushed. No concentration required.",
	},
	DissonantWhispers: {
		ID:          DissonantWhispers,
		Level:       1,
		Name:        "Dissonant Whispers",
		Description: "Use an action to target one creature within 60 feet. It makes a Wisdom save. On a failure it takes 3d6 psychic damage and, if it has a reaction available, spends it moving away from you up to its speed. This movement can provoke opportunity attacks. A successful save halves the damage and prevents the movement. No concentration required.",
	},
	Command: {
		ID:          Command,
		Level:       1,
		Name:        "Command",
		Description: "Use an action to command one creature within 60 feet. If it fails a Wisdom save, it obeys on its next turn. Choose Approach: move toward you, then end its turn; Flee: spend its movement going away from you, then end its turn; or Grovel: fall prone, then end its turn. A prone creature has disadvantage on its own attacks; attacks against it have advantage from within 5 feet and disadvantage from farther away. A successful save ignores the command. This build offers those three words only, not Drop, Halt, or custom commands, and does not enforce the book's language or undead restrictions. No concentration required.",
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
		Description: "Use an action to make a ranged spell attack against one creature within 120 feet. A hit deals 4d6 radiant damage and gives the next attack roll against that creature advantage, whether made by you or an ally. That advantage is consumed by the attack or expires at the end of your next turn. A miss causes neither effect. No concentration required.",
	},
	InflictWounds: {
		ID:          InflictWounds,
		Level:       1,
		Name:        "Inflict Wounds",
		Description: "Use an action to make a melee spell attack against one creature within touch reach, 5 feet. A hit deals 3d10 necrotic damage; a miss deals none. This requires an attack roll, not a saving throw. No concentration required.",
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
		NotYetImplemented: true,
		ID:                CharmPerson,
		Level:             1,
		Name:              "Charm Person",
		Description:       "Charm a humanoid to regard you as a friendly acquaintance",
	},
	DetectMagic: {
		ID:          DetectMagic,
		Level:       1,
		Name:        "Detect Magic",
		Description: "Sense the presence of magic within 30 feet",
	},
	DisguiseSelf: {
		ID: DisguiseSelf, Level: 1, Name: "Disguise Self",
		Description:       "Create an illusion that changes your appearance",
		NotYetImplemented: true,
	},
	Identify: {
		NotYetImplemented: true,
		ID:                Identify,
		Level:             1,
		Name:              "Identify",
		Description:       "Learn the properties of a magic item or spell affecting a creature",
	},
	CureWounds: {
		ID:          CureWounds,
		Level:       1,
		Name:        "Cure Wounds",
		Description: "Use an action to touch a creature, including yourself, and restore 1d8 plus your spellcasting ability modifier in hit points, up to its maximum. Healing features can add further bonuses. It does not heal undead or constructs. No concentration required.",
	},
	HealingWord: {
		ID:          HealingWord,
		Level:       1,
		Name:        "Healing Word",
		Description: "Use a bonus action to heal one creature you can see within 60 feet, including yourself. Restore 1d4 plus your spellcasting ability modifier in hit points, up to its maximum; healing features can add further bonuses. It does not heal undead or constructs. No concentration required.",
	},
	Bless: {
		ID:          Bless,
		Level:       1,
		Name:        "Bless",
		Description: "Use an action to choose up to three creatures within 30 feet, including yourself. While you concentrate, each adds 1d4 to its attack rolls and saving throws. The d4 is rolled for each affected roll; it does not add to damage or ability checks. Requires concentration, up to 1 minute.",
	},
	Bane: {
		ID:          Bane,
		Level:       1,
		Name:        "Bane",
		Description: "Use an action to choose up to three creatures within 30 feet. Each makes a Charisma save. On a failure, it subtracts 1d4 from its attack rolls and saving throws while you concentrate; a successful save avoids the curse. The penalty is rerolled for each affected roll, not applied to damage. Requires concentration, up to 1 minute.",
	},
	DivineFavor: {
		ID: DivineFavor, Level: 1, Name: "Divine Favor",
		Description: "Use a bonus action to empower your weapon attacks. Each weapon hit deals an extra 1d4 radiant damage in addition to the weapon's normal damage. This adds damage, not a bonus to hit, and does not enhance spell attacks. Requires concentration, up to 1 minute.",
	},
	ShieldOfFaith: {
		ID:          ShieldOfFaith,
		Level:       1,
		Name:        "Shield of Faith",
		Description: "Use a bonus action to protect one creature within 60 feet, including yourself. It gains +2 Armor Class, making attacks against it harder to hit. It does not improve saving throws or directly reduce damage. Requires concentration, up to 10 minutes.",
	},
	Sanctuary: {
		ID:          Sanctuary,
		Level:       1,
		Name:        "Sanctuary",
		Description: "In this build, use a bonus action to touch a creature and protect it while you concentrate, up to 1 minute. A creature attempting a direct attack or harmful targeted spell against it must first pass a Wisdom save; a failure blocks that attempt against the protected creature. Area effects are not blocked. The protection ends if the protected creature attacks or casts a harmful spell. Receiving the ward also prevents another Sanctuary for 20 of the recipient's turn ends, or until combat ends or they rest; this limit survives losing the ward.",
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
