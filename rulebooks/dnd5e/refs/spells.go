package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Spells provides type-safe, discoverable references to D&D 5e spells.
// Use IDE autocomplete: refs.Spells.<tab> to discover available spells.
var Spells = spellsNS{}

type spellsNS struct{}

// Cantrips - Damage

// FireBolt returns a reference to the Fire Bolt cantrip.
func (spellsNS) FireBolt() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "fire-bolt"}
}

// RayOfFrost returns a reference to the Ray of Frost cantrip.
func (spellsNS) RayOfFrost() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "ray-of-frost"}
}

// ShockingGrasp returns a reference to the Shocking Grasp cantrip.
func (spellsNS) ShockingGrasp() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "shocking-grasp"}
}

// AcidSplash returns a reference to the Acid Splash cantrip.
func (spellsNS) AcidSplash() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "acid-splash"}
}

// PoisonSpray returns a reference to the Poison Spray cantrip.
func (spellsNS) PoisonSpray() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "poison-spray"}
}

// ChillTouch returns a reference to the Chill Touch cantrip.
func (spellsNS) ChillTouch() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "chill-touch"}
}

// SacredFlame returns a reference to the Sacred Flame cantrip.
func (spellsNS) SacredFlame() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "sacred-flame"}
}

// TollTheDead returns a reference to the Toll the Dead cantrip.
func (spellsNS) TollTheDead() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "toll-the-dead"}
}

// WordOfRadiance returns a reference to the Word of Radiance cantrip.
func (spellsNS) WordOfRadiance() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "word-of-radiance"}
}

// EldritchBlast returns a reference to the Eldritch Blast cantrip.
func (spellsNS) EldritchBlast() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "eldritch-blast"}
}

// Frostbite returns a reference to the Frostbite cantrip.
func (spellsNS) Frostbite() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "frostbite"}
}

// PrimalSavagery returns a reference to the Primal Savagery cantrip.
func (spellsNS) PrimalSavagery() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "primal-savagery"}
}

// Thornwhip returns a reference to the Thorn Whip cantrip.
func (spellsNS) Thornwhip() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "thornwhip"}
}

// CreateBonfire returns a reference to the Create Bonfire cantrip.
func (spellsNS) CreateBonfire() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "create-bonfire"}
}

// Druidcraft returns a reference to the Druidcraft cantrip.
func (spellsNS) Druidcraft() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "druidcraft"}
}

// Infestation returns a reference to the Infestation cantrip.
func (spellsNS) Infestation() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "infestation"}
}

// MagicStone returns a reference to the Magic Stone cantrip.
func (spellsNS) MagicStone() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "magic-stone"}
}

// MoldEarth returns a reference to the Mold Earth cantrip.
func (spellsNS) MoldEarth() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "mold-earth"}
}

// ShapeWater returns a reference to the Shape Water cantrip.
func (spellsNS) ShapeWater() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "shape-water"}
}

// BoomingBlade returns a reference to the Booming Blade cantrip.
func (spellsNS) BoomingBlade() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "booming-blade"}
}

// ControlFlames returns a reference to the Control Flames cantrip.
func (spellsNS) ControlFlames() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "control-flames"}
}

// GreenFlameBlade returns a reference to the Green-Flame Blade cantrip.
func (spellsNS) GreenFlameBlade() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "green-flame-blade"}
}

// GustWind returns a reference to the Gust cantrip.
func (spellsNS) GustWind() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "gust"}
}

// SwordBurst returns a reference to the Sword Burst cantrip.
func (spellsNS) SwordBurst() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "sword-burst"}
}

// Cantrips - Utility

// MageHand returns a reference to the Mage Hand cantrip.
func (spellsNS) MageHand() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "mage-hand"}
}

// MinorIllusion returns a reference to the Minor Illusion cantrip.
func (spellsNS) MinorIllusion() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "minor-illusion"}
}

// Prestidigitation returns a reference to the Prestidigitation cantrip.
func (spellsNS) Prestidigitation() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "prestidigitation"}
}

// Light returns a reference to the Light cantrip.
func (spellsNS) Light() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "light"}
}

// Guidance returns a reference to the Guidance cantrip.
func (spellsNS) Guidance() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "guidance"}
}

// Resistance returns a reference to the Resistance cantrip.
func (spellsNS) Resistance() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "resistance"}
}

// Thaumaturgy returns a reference to the Thaumaturgy cantrip.
func (spellsNS) Thaumaturgy() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "thaumaturgy"}
}

// SpareTheDying returns a reference to the Spare the Dying cantrip.
func (spellsNS) SpareTheDying() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "spare-the-dying"}
}

// BladeWard returns a reference to the Blade Ward cantrip.
func (spellsNS) BladeWard() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "blade-ward"}
}

// DancingLights returns a reference to the Dancing Lights cantrip.
func (spellsNS) DancingLights() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "dancing-lights"}
}

// Friends returns a reference to the Friends cantrip.
func (spellsNS) Friends() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "friends"}
}

// Mending returns a reference to the Mending cantrip.
func (spellsNS) Mending() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "mending"}
}

// Message returns a reference to the Message cantrip.
func (spellsNS) Message() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "message"}
}

// TrueStrike returns a reference to the True Strike cantrip.
func (spellsNS) TrueStrike() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "true-strike"}
}

// ViciousMockery returns a reference to the Vicious Mockery cantrip.
func (spellsNS) ViciousMockery() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "vicious-mockery"}
}

// Level 1 - Damage Spells

// MagicMissile returns a reference to the Magic Missile spell.
func (spellsNS) MagicMissile() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "magic-missile"}
}

// BurningHands returns a reference to the Burning Hands spell.
func (spellsNS) BurningHands() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "burning-hands"}
}

// ChromaticOrb returns a reference to the Chromatic Orb spell.
func (spellsNS) ChromaticOrb() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "chromatic-orb"}
}

// Thunderwave returns a reference to the Thunderwave spell.
func (spellsNS) Thunderwave() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "thunderwave"}
}

// IceKnife returns a reference to the Ice Knife spell.
func (spellsNS) IceKnife() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "ice-knife"}
}

// WitchBolt returns a reference to the Witch Bolt spell.
func (spellsNS) WitchBolt() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "witch-bolt"}
}

// GuidingBolt returns a reference to the Guiding Bolt spell.
func (spellsNS) GuidingBolt() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "guiding-bolt"}
}

// InflictWounds returns a reference to the Inflict Wounds spell.
func (spellsNS) InflictWounds() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "inflict-wounds"}
}

// HailOfThorns returns a reference to the Hail of Thorns spell.
func (spellsNS) HailOfThorns() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "hail-of-thorns"}
}

// EnsnaringStrike returns a reference to the Ensnaring Strike spell.
func (spellsNS) EnsnaringStrike() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "ensnaring-strike"}
}

// HellishRebuke returns a reference to the Hellish Rebuke spell.
func (spellsNS) HellishRebuke() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "hellish-rebuke"}
}

// ArmsOfHadar returns a reference to the Arms of Hadar spell.
func (spellsNS) ArmsOfHadar() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "arms-of-hadar"}
}

// Hex returns a reference to the Hex spell.
func (spellsNS) Hex() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "hex"}
}

// SearingSmite returns a reference to the Searing Smite spell.
func (spellsNS) SearingSmite() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "searing-smite"}
}

// ThunderousSmite returns a reference to the Thunderous Smite spell.
func (spellsNS) ThunderousSmite() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "thunderous-smite"}
}

// WrathfulSmite returns a reference to the Wrathful Smite spell.
func (spellsNS) WrathfulSmite() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "wrathful-smite"}
}

// Level 1 - Utility Spells

// Shield returns a reference to the Shield spell.
func (spellsNS) Shield() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "shield"}
}

// Sleep returns a reference to the Sleep spell.
func (spellsNS) Sleep() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "sleep"}
}

// CharmPerson returns a reference to the Charm Person spell.
func (spellsNS) CharmPerson() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "charm-person"}
}

// DetectMagic returns a reference to the Detect Magic spell.
func (spellsNS) DetectMagic() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "detect-magic"}
}

// Identify returns a reference to the Identify spell.
func (spellsNS) Identify() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "identify"}
}

// CureWounds returns a reference to the Cure Wounds spell.
func (spellsNS) CureWounds() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "cure-wounds"}
}

// HealingWord returns a reference to the Healing Word spell.
func (spellsNS) HealingWord() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "healing-word"}
}

// Bless returns a reference to the Bless spell.
func (spellsNS) Bless() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "bless"}
}

// Bane returns a reference to the Bane spell.
func (spellsNS) Bane() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "bane"}
}

// ShieldOfFaith returns a reference to the Shield of Faith spell.
func (spellsNS) ShieldOfFaith() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "shield-of-faith"}
}

// AnimalFriendship returns a reference to the Animal Friendship spell.
func (spellsNS) AnimalFriendship() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "animal-friendship"}
}

// Command returns a reference to the Command spell.
func (spellsNS) Command() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "command"}
}

// DisguiseSelf returns a reference to the Disguise Self spell.
func (spellsNS) DisguiseSelf() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "disguise-self"}
}

// DivineFavor returns a reference to the Divine Favor spell.
func (spellsNS) DivineFavor() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "divine-favor"}
}

// FaerieFire returns a reference to the Faerie Fire spell.
func (spellsNS) FaerieFire() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "faerie-fire"}
}

// FalseLife returns a reference to the False Life spell.
func (spellsNS) FalseLife() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "false-life"}
}

// FogCloud returns a reference to the Fog Cloud spell.
func (spellsNS) FogCloud() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "fog-cloud"}
}

// RayOfSickness returns a reference to the Ray of Sickness spell.
func (spellsNS) RayOfSickness() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "ray-of-sickness"}
}

// SpeakWithAnimals returns a reference to the Speak with Animals spell.
func (spellsNS) SpeakWithAnimals() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "speak-with-animals"}
}

// ComprehendLanguages returns a reference to the Comprehend Languages spell.
func (spellsNS) ComprehendLanguages() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "comprehend-languages"}
}

// FeatherFall returns a reference to the Feather Fall spell.
func (spellsNS) FeatherFall() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "feather-fall"}
}

// Heroism returns a reference to the Heroism spell.
func (spellsNS) Heroism() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "heroism"}
}

// HideousLaughter returns a reference to the Hideous Laughter spell.
func (spellsNS) HideousLaughter() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "hideous-laughter"}
}

// IllusoryScript returns a reference to the Illusory Script spell.
func (spellsNS) IllusoryScript() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "illusory-script"}
}

// Longstrider returns a reference to the Longstrider spell.
func (spellsNS) Longstrider() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "longstrider"}
}

// SilentImage returns a reference to the Silent Image spell.
func (spellsNS) SilentImage() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "silent-image"}
}

// UnseenServant returns a reference to the Unseen Servant spell.
func (spellsNS) UnseenServant() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "unseen-servant"}
}

// AbsorbElements returns a reference to the Absorb Elements spell.
func (spellsNS) AbsorbElements() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "absorb-elements"}
}

// BeastBond returns a reference to the Beast Bond spell.
func (spellsNS) BeastBond() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "beast-bond"}
}

// Entangle returns a reference to the Entangle spell.
func (spellsNS) Entangle() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "entangle"}
}

// GoodBerry returns a reference to the Goodberry spell.
func (spellsNS) GoodBerry() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "goodberry"}
}

// JumpSpell returns a reference to the Jump spell.
func (spellsNS) JumpSpell() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "jump"}
}

// PurifyFood returns a reference to the Purify Food and Drink spell.
func (spellsNS) PurifyFood() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "purify-food-and-drink"}
}

// CatapultSpell returns a reference to the Catapult spell.
func (spellsNS) CatapultSpell() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "catapult"}
}

// CauseFear returns a reference to the Cause Fear spell.
func (spellsNS) CauseFear() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "cause-fear"}
}

// ColorSpray returns a reference to the Color Spray spell.
func (spellsNS) ColorSpray() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "color-spray"}
}

// DistortValue returns a reference to the Distort Value spell.
func (spellsNS) DistortValue() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "distort-value"}
}

// EarthTremor returns a reference to the Earth Tremor spell.
func (spellsNS) EarthTremor() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "earth-tremor"}
}

// ExpeditiousRetreat returns a reference to the Expeditious Retreat spell.
func (spellsNS) ExpeditiousRetreat() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "expeditious-retreat"}
}

// ProtectionEvil returns a reference to the Protection from Evil and Good spell.
func (spellsNS) ProtectionEvil() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "protection-from-evil-and-good"}
}

// Level 2 - Damage Spells

// ScorchingRay returns a reference to the Scorching Ray spell.
func (spellsNS) ScorchingRay() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "scorching-ray"}
}

// Shatter returns a reference to the Shatter spell.
func (spellsNS) Shatter() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "shatter"}
}

// AganazzarsScorcher returns a reference to the Aganazzar's Scorcher spell.
func (spellsNS) AganazzarsScorcher() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "aganazzars-scorcher"}
}

// CloudOfDaggers returns a reference to the Cloud of Daggers spell.
func (spellsNS) CloudOfDaggers() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "cloud-of-daggers"}
}

// MelfsAcidArrow returns a reference to the Melf's Acid Arrow spell.
func (spellsNS) MelfsAcidArrow() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "melfs-acid-arrow"}
}

// Moonbeam returns a reference to the Moonbeam spell.
func (spellsNS) Moonbeam() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "moonbeam"}
}

// SpiritualWeapon returns a reference to the Spiritual Weapon spell.
func (spellsNS) SpiritualWeapon() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "spiritual-weapon"}
}

// FlamingSphere returns a reference to the Flaming Sphere spell.
func (spellsNS) FlamingSphere() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "flaming-sphere"}
}

// GustOfWind returns a reference to the Gust of Wind spell.
func (spellsNS) GustOfWind() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "gust-of-wind"}
}

// RayOfEnfeeblement returns a reference to the Ray of Enfeeblement spell.
func (spellsNS) RayOfEnfeeblement() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "ray-of-enfeeblement"}
}

// Level 2 - Utility Spells

// Augury returns a reference to the Augury spell.
func (spellsNS) Augury() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "augury"}
}

// Barkskin returns a reference to the Barkskin spell.
func (spellsNS) Barkskin() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "barkskin"}
}

// BlindnessDeafness returns a reference to the Blindness/Deafness spell.
func (spellsNS) BlindnessDeafness() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "blindness-deafness"}
}

// LesserRestoration returns a reference to the Lesser Restoration spell.
func (spellsNS) LesserRestoration() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "lesser-restoration"}
}

// MagicWeapon returns a reference to the Magic Weapon spell.
func (spellsNS) MagicWeapon() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "magic-weapon"}
}

// MirrorImage returns a reference to the Mirror Image spell.
func (spellsNS) MirrorImage() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "mirror-image"}
}

// PassWithoutTrace returns a reference to the Pass without Trace spell.
func (spellsNS) PassWithoutTrace() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "pass-without-trace"}
}

// SpikeGrowth returns a reference to the Spike Growth spell.
func (spellsNS) SpikeGrowth() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "spike-growth"}
}

// Suggestion returns a reference to the Suggestion spell.
func (spellsNS) Suggestion() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "suggestion"}
}

// Level 3 - Damage Spells

// Fireball returns a reference to the Fireball spell.
func (spellsNS) Fireball() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "fireball"}
}

// LightningBolt returns a reference to the Lightning Bolt spell.
func (spellsNS) LightningBolt() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "lightning-bolt"}
}

// CallLightning returns a reference to the Call Lightning spell.
func (spellsNS) CallLightning() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "call-lightning"}
}

// VampiricTouch returns a reference to the Vampiric Touch spell.
func (spellsNS) VampiricTouch() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "vampiric-touch"}
}

// SleetStorm returns a reference to the Sleet Storm spell.
func (spellsNS) SleetStorm() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "sleet-storm"}
}

// SpiritGuardians returns a reference to the Spirit Guardians spell.
func (spellsNS) SpiritGuardians() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "spirit-guardians"}
}

// Level 3 - Utility Spells

// AnimateDead returns a reference to the Animate Dead spell.
func (spellsNS) AnimateDead() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "animate-dead"}
}

// BeaconOfHope returns a reference to the Beacon of Hope spell.
func (spellsNS) BeaconOfHope() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "beacon-of-hope"}
}

// Blink returns a reference to the Blink spell.
func (spellsNS) Blink() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "blink"}
}

// CrusadersMantle returns a reference to the Crusader's Mantle spell.
func (spellsNS) CrusadersMantle() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "crusaders-mantle"}
}

// Daylight returns a reference to the Daylight spell.
func (spellsNS) Daylight() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "daylight"}
}

// DispelMagic returns a reference to the Dispel Magic spell.
func (spellsNS) DispelMagic() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "dispel-magic"}
}

// Nondetection returns a reference to the Nondetection spell.
func (spellsNS) Nondetection() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "nondetection"}
}

// PlantGrowth returns a reference to the Plant Growth spell.
func (spellsNS) PlantGrowth() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "plant-growth"}
}

// Revivify returns a reference to the Revivify spell.
func (spellsNS) Revivify() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "revivify"}
}

// SpeakWithDead returns a reference to the Speak with Dead spell.
func (spellsNS) SpeakWithDead() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "speak-with-dead"}
}

// WindWall returns a reference to the Wind Wall spell.
func (spellsNS) WindWall() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "wind-wall"}
}

// Level 4 Spells

// ArcaneEye returns a reference to the Arcane Eye spell.
func (spellsNS) ArcaneEye() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "arcane-eye"}
}

// Blight returns a reference to the Blight spell.
func (spellsNS) Blight() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "blight"}
}

// Confusion returns a reference to the Confusion spell.
func (spellsNS) Confusion() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "confusion"}
}

// ControlWater returns a reference to the Control Water spell.
func (spellsNS) ControlWater() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "control-water"}
}

// DeathWard returns a reference to the Death Ward spell.
func (spellsNS) DeathWard() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "death-ward"}
}

// DimensionDoor returns a reference to the Dimension Door spell.
func (spellsNS) DimensionDoor() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "dimension-door"}
}

// DominateBeast returns a reference to the Dominate Beast spell.
func (spellsNS) DominateBeast() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "dominate-beast"}
}

// FreedomOfMovement returns a reference to the Freedom of Movement spell.
func (spellsNS) FreedomOfMovement() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "freedom-of-movement"}
}

// GraspingVine returns a reference to the Grasping Vine spell.
func (spellsNS) GraspingVine() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "grasping-vine"}
}

// GuardianOfFaith returns a reference to the Guardian of Faith spell.
func (spellsNS) GuardianOfFaith() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "guardian-of-faith"}
}

// IceStorm returns a reference to the Ice Storm spell.
func (spellsNS) IceStorm() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "ice-storm"}
}

// Polymorph returns a reference to the Polymorph spell.
func (spellsNS) Polymorph() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "polymorph"}
}

// Stoneskin returns a reference to the Stoneskin spell.
func (spellsNS) Stoneskin() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "stoneskin"}
}

// WallOfFire returns a reference to the Wall of Fire spell.
func (spellsNS) WallOfFire() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "wall-of-fire"}
}

// Level 5 Spells

// AntiLifeShell returns a reference to the Antilife Shell spell.
func (spellsNS) AntiLifeShell() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "antilife-shell"}
}

// Cloudkill returns a reference to the Cloudkill spell.
func (spellsNS) Cloudkill() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "cloudkill"}
}

// DestructiveWave returns a reference to the Destructive Wave spell.
func (spellsNS) DestructiveWave() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "destructive-wave"}
}

// DominatePerson returns a reference to the Dominate Person spell.
func (spellsNS) DominatePerson() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "dominate-person"}
}

// FlameStrike returns a reference to the Flame Strike spell.
func (spellsNS) FlameStrike() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "flame-strike"}
}

// HoldMonster returns a reference to the Hold Monster spell.
func (spellsNS) HoldMonster() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "hold-monster"}
}

// InsectPlague returns a reference to the Insect Plague spell.
func (spellsNS) InsectPlague() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "insect-plague"}
}

// LegendLore returns a reference to the Legend Lore spell.
func (spellsNS) LegendLore() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "legend-lore"}
}

// MassCureWounds returns a reference to the Mass Cure Wounds spell.
func (spellsNS) MassCureWounds() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "mass-cure-wounds"}
}

// ModifyMemory returns a reference to the Modify Memory spell.
func (spellsNS) ModifyMemory() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "modify-memory"}
}

// RaiseDead returns a reference to the Raise Dead spell.
func (spellsNS) RaiseDead() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "raise-dead"}
}

// Scrying returns a reference to the Scrying spell.
func (spellsNS) Scrying() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "scrying"}
}

// TreeStride returns a reference to the Tree Stride spell.
func (spellsNS) TreeStride() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeSpells, ID: "tree-stride"}
}
