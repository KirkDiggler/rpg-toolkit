package choices

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// SubclassModifications represents how a subclass modifies the base class
type SubclassModifications struct {
	// Additional choice requirements
	AdditionalSkills    *SkillRequirement      // Knowledge Cleric: 2 from Arcana/History/Nature/Religion
	AdditionalLanguages []*LanguageRequirement // Knowledge Cleric: 2 languages
	AdditionalTools     *ToolRequirement       // Some artificer subclasses

	// Equipment modifications based on new proficiencies
	// These are OPTIONS added because of proficiency grants
	AdditionalEquipmentOptions map[ChoiceID][]EquipmentOption // Keyed by requirement ID (e.g., ClericWeapons)

	// Automatic grants (not choices)
	GrantedProficiencies GrantedProficiencies // Weapons, armor, tools
	GrantedSpells        []SpellGrant         // Domain spells by level
	GrantedCantrips      []string             // Light Domain gets Light cantrip

	// Special modifications
	ModifyFunction func(*Requirements) // For complex modifications that don't fit the pattern
}

// GrantedProficiencies represents automatic proficiency grants from a subclass
type GrantedProficiencies struct {
	Weapons []shared.EquipmentCategory // Weapon categories
	Armor   []shared.EquipmentCategory // Armor categories
	Tools   []tools.ToolID             // Tool proficiencies
	Skills  []skills.Skill             // Skill proficiencies (e.g., Scout Rogue)
}

// SpellGrant represents spells automatically granted at a specific level
type SpellGrant struct {
	Level  int            // Character level when granted
	Spells []spells.Spell // Spell IDs
}

// GetSubclassModifications returns the modifications for a specific subclass
func GetSubclassModifications(subclass classes.Subclass) *SubclassModifications {
	mods, ok := subclassModifications[subclass]
	if !ok {
		return nil
	}
	return mods
}

// ApplySubclassModifications applies subclass modifications to base requirements
func ApplySubclassModifications(reqs *Requirements, mods *SubclassModifications) {
	if mods == nil {
		return
	}

	// Add additional skill requirements
	if mods.AdditionalSkills != nil {
		if reqs.AdditionalSkills == nil {
			reqs.AdditionalSkills = []*SkillRequirement{}
		}
		reqs.AdditionalSkills = append(reqs.AdditionalSkills, mods.AdditionalSkills)
	}

	// Add additional language requirements
	if len(mods.AdditionalLanguages) > 0 {
		if reqs.Languages == nil {
			reqs.Languages = []*LanguageRequirement{}
		}
		reqs.Languages = append(reqs.Languages, mods.AdditionalLanguages...)
	}

	// Add additional tool requirements
	if mods.AdditionalTools != nil {
		// TODO: Handle multiple tool requirements if needed
		// For now, we'll just replace if there's a conflict
		if reqs.Tools == nil {
			reqs.Tools = mods.AdditionalTools
		}
	}

	// Add additional equipment options based on proficiencies
	for reqID, options := range mods.AdditionalEquipmentOptions {
		for i, eq := range reqs.Equipment {
			if eq.ID == reqID {
				reqs.Equipment[i].Options = append(reqs.Equipment[i].Options, options...)
				break
			}
		}
	}

	// Apply any custom modifications
	if mods.ModifyFunction != nil {
		mods.ModifyFunction(reqs)
	}
}

// subclassModifications defines all subclass modifications
var subclassModifications = map[classes.Subclass]*SubclassModifications{
	// Cleric Domains
	classes.LifeDomain: {
		GrantedProficiencies: GrantedProficiencies{
			Armor: []shared.EquipmentCategory{armor.CategoryHeavy},
		},
		AdditionalEquipmentOptions: map[ChoiceID][]EquipmentOption{
			ClericArmor: {
				{
					ID:    "cleric-armor-life",
					Label: "chain mail (Life Domain)",
					Items: []EquipmentItem{
						{ID: armor.ChainMail, Quantity: 1},
					},
				},
			},
		},
		GrantedSpells: []SpellGrant{
			{Level: 1, Spells: []spells.Spell{spells.Bless, spells.CureWounds}},
			{Level: 3, Spells: []spells.Spell{spells.LesserRestoration, spells.SpiritualWeapon}},
			{Level: 5, Spells: []spells.Spell{spells.BeaconOfHope, spells.Revivify}},
			{Level: 7, Spells: []spells.Spell{spells.DeathWard, spells.GuardianOfFaith}},
			{Level: 9, Spells: []spells.Spell{spells.MassCureWounds, spells.RaiseDead}},
		},
	},

	classes.LightDomain: {
		GrantedCantrips: []string{string(spells.Light)},
		GrantedSpells: []SpellGrant{
			{Level: 1, Spells: []spells.Spell{spells.BurningHands, spells.FaerieFire}},
			{Level: 3, Spells: []spells.Spell{spells.FlamingSphere, spells.ScorchingRay}},
			{Level: 5, Spells: []spells.Spell{spells.Daylight, spells.Fireball}},
			{Level: 7, Spells: []spells.Spell{spells.GuardianOfFaith, spells.WallOfFire}},
			{Level: 9, Spells: []spells.Spell{spells.FlameStrike, spells.Scrying}},
		},
	},

	classes.NatureDomain: {
		// Nature Domain gets a druid cantrip
		ModifyFunction: func(reqs *Requirements) {
			// Add a druid cantrip choice
			// This is complex enough to warrant a custom function
			natureCantrip := &CantripRequirement{
				ID:    ChoiceID("cleric-nature-cantrip"),
				Count: 1,
				Options: []spells.Spell{
					spells.Guidance,
					spells.Resistance,
					spells.PoisonSpray,
					spells.Thornwhip,
				},
				Label: "Choose 1 druid cantrip (Nature Domain)",
			}

			// Add as additional cantrip requirement
			// TODO: May need to handle multiple cantrip requirements better
			if reqs.Cantrips != nil {
				// For now, increase the count
				reqs.Cantrips.Count++
				// And add druid cantrips to options if not present
				for _, cantrip := range natureCantrip.Options {
					found := false
					for _, existing := range reqs.Cantrips.Options {
						if existing == cantrip {
							found = true
							break
						}
					}
					if !found {
						reqs.Cantrips.Options = append(reqs.Cantrips.Options, cantrip)
					}
				}
			}
		},
		GrantedProficiencies: GrantedProficiencies{
			Armor: []shared.EquipmentCategory{armor.CategoryHeavy},
		},
		GrantedSpells: []SpellGrant{
			{Level: 1, Spells: []spells.Spell{spells.AnimalFriendship, spells.SpeakWithAnimals}},
			{Level: 3, Spells: []spells.Spell{spells.Barkskin, spells.SpikeGrowth}},
			{Level: 5, Spells: []spells.Spell{spells.PlantGrowth, spells.WindWall}},
			{Level: 7, Spells: []spells.Spell{spells.DominateBeast, spells.GraspingVine}},
			{Level: 9, Spells: []spells.Spell{spells.InsectPlague, spells.TreeStride}},
		},
	},

	classes.TempestDomain: {
		GrantedProficiencies: GrantedProficiencies{
			Weapons: []shared.EquipmentCategory{
				weapons.CategoryMartialMelee,
				weapons.CategoryMartialRanged,
			},
			Armor: []shared.EquipmentCategory{armor.CategoryHeavy},
		},
		AdditionalEquipmentOptions: map[ChoiceID][]EquipmentOption{
			ClericWeapons: {
				{
					ID:    "cleric-weapon-tempest",
					Label: "martial weapon (Tempest Domain)",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Categories: []shared.EquipmentCategory{
								weapons.CategoryMartialMelee,
								weapons.CategoryMartialRanged,
							},
							Type:   shared.EquipmentTypeWeapon,
							Choose: 1,
							Label:  "Choose a martial weapon",
						},
					},
				},
			},
			ClericArmor: {
				{
					ID:    "cleric-armor-tempest",
					Label: "chain mail (Tempest Domain)",
					Items: []EquipmentItem{
						{ID: armor.ChainMail, Quantity: 1},
					},
				},
			},
		},
		GrantedSpells: []SpellGrant{
			{Level: 1, Spells: []spells.Spell{spells.FogCloud, spells.Thunderwave}},
			{Level: 3, Spells: []spells.Spell{spells.GustOfWind, spells.Shatter}},
			{Level: 5, Spells: []spells.Spell{spells.CallLightning, spells.SleetStorm}},
			{Level: 7, Spells: []spells.Spell{spells.ControlWater, spells.IceStorm}},
			{Level: 9, Spells: []spells.Spell{spells.DestructiveWave, spells.InsectPlague}},
		},
	},

	classes.TrickeryDomain: {
		GrantedSpells: []SpellGrant{
			{Level: 1, Spells: []spells.Spell{spells.CharmPerson, spells.DisguiseSelf}},
			{Level: 3, Spells: []spells.Spell{spells.MirrorImage, spells.PassWithoutTrace}},
			{Level: 5, Spells: []spells.Spell{spells.Blink, spells.DispelMagic}},
			{Level: 7, Spells: []spells.Spell{spells.DimensionDoor, spells.Polymorph}},
			{Level: 9, Spells: []spells.Spell{spells.DominatePerson, spells.ModifyMemory}},
		},
	},

	classes.WarDomain: {
		GrantedProficiencies: GrantedProficiencies{
			Weapons: []shared.EquipmentCategory{
				weapons.CategoryMartialMelee,
				weapons.CategoryMartialRanged,
			},
			Armor: []shared.EquipmentCategory{armor.CategoryHeavy},
		},
		AdditionalEquipmentOptions: map[ChoiceID][]EquipmentOption{
			ClericWeapons: {
				{
					ID:    "cleric-weapon-war",
					Label: "martial weapon (War Domain)",
					CategoryChoices: []EquipmentCategoryChoice{
						{
							Categories: []shared.EquipmentCategory{
								weapons.CategoryMartialMelee,
								weapons.CategoryMartialRanged,
							},
							Type:   shared.EquipmentTypeWeapon,
							Choose: 1,
							Label:  "Choose a martial weapon",
						},
					},
				},
			},
			ClericArmor: {
				{
					ID:    "cleric-armor-war",
					Label: "chain mail (War Domain)",
					Items: []EquipmentItem{
						{ID: armor.ChainMail, Quantity: 1},
					},
				},
			},
		},
		GrantedSpells: []SpellGrant{
			{Level: 1, Spells: []spells.Spell{spells.DivineFavor, spells.ShieldOfFaith}},
			{Level: 3, Spells: []spells.Spell{spells.MagicWeapon, spells.SpiritualWeapon}},
			{Level: 5, Spells: []spells.Spell{spells.CrusadersMantle, spells.SpiritGuardians}},
			{Level: 7, Spells: []spells.Spell{spells.FreedomOfMovement, spells.Stoneskin}},
			{Level: 9, Spells: []spells.Spell{spells.FlameStrike, spells.HoldMonster}},
		},
	},

	classes.KnowledgeDomain: {
		AdditionalSkills: &SkillRequirement{
			ID:    ChoiceID("cleric-knowledge-skills"),
			Count: 2,
			Options: []skills.Skill{
				skills.Arcana,
				skills.History,
				skills.Nature,
				skills.Religion,
			},
			Label: "Choose 2 Knowledge Domain skills",
		},
		AdditionalLanguages: []*LanguageRequirement{
			{
				ID:      ChoiceID("cleric-knowledge-languages"),
				Count:   2,
				Options: nil, // nil means any language
				Label:   "Choose 2 languages (Knowledge Domain)",
			},
		},
		GrantedSpells: []SpellGrant{
			{Level: 1, Spells: []spells.Spell{spells.Command, spells.Identify}},
			{Level: 3, Spells: []spells.Spell{spells.Augury, spells.Suggestion}},
			{Level: 5, Spells: []spells.Spell{spells.Nondetection, spells.SpeakWithDead}},
			{Level: 7, Spells: []spells.Spell{spells.ArcaneEye, spells.Confusion}},
			{Level: 9, Spells: []spells.Spell{spells.LegendLore, spells.Scrying}},
		},
	},

	// Death Domain (DMG)
	classes.DeathDomain: {
		GrantedProficiencies: GrantedProficiencies{
			Weapons: []shared.EquipmentCategory{
				weapons.CategoryMartialMelee,
			},
		},
		GrantedSpells: []SpellGrant{
			{Level: 1, Spells: []spells.Spell{spells.FalseLife, spells.RayOfSickness}},
			{Level: 3, Spells: []spells.Spell{spells.BlindnessDeafness, spells.RayOfEnfeeblement}},
			{Level: 5, Spells: []spells.Spell{spells.AnimateDead, spells.VampiricTouch}},
			{Level: 7, Spells: []spells.Spell{spells.Blight, spells.DeathWard}},
			{Level: 9, Spells: []spells.Spell{spells.AntiLifeShell, spells.Cloudkill}},
		},
	},

	// TODO: Add Fighter subclasses at level 3
	// TODO: Add other class subclasses
}
