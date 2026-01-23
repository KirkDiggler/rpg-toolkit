//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Spell singletons - unexported for controlled access via methods
var (
	// Cantrips - Damage
	spellFireBolt        = &core.Ref{Module: Module, Type: TypeSpells, ID: "fire-bolt"}
	spellRayOfFrost      = &core.Ref{Module: Module, Type: TypeSpells, ID: "ray-of-frost"}
	spellShockingGrasp   = &core.Ref{Module: Module, Type: TypeSpells, ID: "shocking-grasp"}
	spellAcidSplash      = &core.Ref{Module: Module, Type: TypeSpells, ID: "acid-splash"}
	spellPoisonSpray     = &core.Ref{Module: Module, Type: TypeSpells, ID: "poison-spray"}
	spellChillTouch      = &core.Ref{Module: Module, Type: TypeSpells, ID: "chill-touch"}
	spellSacredFlame     = &core.Ref{Module: Module, Type: TypeSpells, ID: "sacred-flame"}
	spellTollTheDead     = &core.Ref{Module: Module, Type: TypeSpells, ID: "toll-the-dead"}
	spellWordOfRadiance  = &core.Ref{Module: Module, Type: TypeSpells, ID: "word-of-radiance"}
	spellEldritchBlast   = &core.Ref{Module: Module, Type: TypeSpells, ID: "eldritch-blast"}
	spellFrostbite       = &core.Ref{Module: Module, Type: TypeSpells, ID: "frostbite"}
	spellPrimalSavagery  = &core.Ref{Module: Module, Type: TypeSpells, ID: "primal-savagery"}
	spellThornWhip       = &core.Ref{Module: Module, Type: TypeSpells, ID: "thorn-whip"}
	spellCreateBonfire   = &core.Ref{Module: Module, Type: TypeSpells, ID: "create-bonfire"}
	spellDruidcraft      = &core.Ref{Module: Module, Type: TypeSpells, ID: "druidcraft"}
	spellInfestation     = &core.Ref{Module: Module, Type: TypeSpells, ID: "infestation"}
	spellMagicStone      = &core.Ref{Module: Module, Type: TypeSpells, ID: "magic-stone"}
	spellMoldEarth       = &core.Ref{Module: Module, Type: TypeSpells, ID: "mold-earth"}
	spellShapeWater      = &core.Ref{Module: Module, Type: TypeSpells, ID: "shape-water"}
	spellBoomingBlade    = &core.Ref{Module: Module, Type: TypeSpells, ID: "booming-blade"}
	spellControlFlames   = &core.Ref{Module: Module, Type: TypeSpells, ID: "control-flames"}
	spellGreenFlameBlade = &core.Ref{Module: Module, Type: TypeSpells, ID: "green-flame-blade"}
	spellGust            = &core.Ref{Module: Module, Type: TypeSpells, ID: "gust"}
	spellSwordBurst      = &core.Ref{Module: Module, Type: TypeSpells, ID: "sword-burst"}

	// Cantrips - Utility
	spellMageHand         = &core.Ref{Module: Module, Type: TypeSpells, ID: "mage-hand"}
	spellMinorIllusion    = &core.Ref{Module: Module, Type: TypeSpells, ID: "minor-illusion"}
	spellPrestidigitation = &core.Ref{Module: Module, Type: TypeSpells, ID: "prestidigitation"}
	spellLight            = &core.Ref{Module: Module, Type: TypeSpells, ID: "light"}
	spellGuidance         = &core.Ref{Module: Module, Type: TypeSpells, ID: "guidance"}
	spellResistance       = &core.Ref{Module: Module, Type: TypeSpells, ID: "resistance"}
	spellThaumaturgy      = &core.Ref{Module: Module, Type: TypeSpells, ID: "thaumaturgy"}
	spellSpareTheDying    = &core.Ref{Module: Module, Type: TypeSpells, ID: "spare-the-dying"}
	spellBladeWard        = &core.Ref{Module: Module, Type: TypeSpells, ID: "blade-ward"}
	spellDancingLights    = &core.Ref{Module: Module, Type: TypeSpells, ID: "dancing-lights"}
	spellFriends          = &core.Ref{Module: Module, Type: TypeSpells, ID: "friends"}
	spellMending          = &core.Ref{Module: Module, Type: TypeSpells, ID: "mending"}
	spellMessage          = &core.Ref{Module: Module, Type: TypeSpells, ID: "message"}
	spellTrueStrike       = &core.Ref{Module: Module, Type: TypeSpells, ID: "true-strike"}
	spellViciousMockery   = &core.Ref{Module: Module, Type: TypeSpells, ID: "vicious-mockery"}

	// Level 1 - Damage
	spellMagicMissile    = &core.Ref{Module: Module, Type: TypeSpells, ID: "magic-missile"}
	spellBurningHands    = &core.Ref{Module: Module, Type: TypeSpells, ID: "burning-hands"}
	spellChromaticOrb    = &core.Ref{Module: Module, Type: TypeSpells, ID: "chromatic-orb"}
	spellThunderwave     = &core.Ref{Module: Module, Type: TypeSpells, ID: "thunderwave"}
	spellIceKnife        = &core.Ref{Module: Module, Type: TypeSpells, ID: "ice-knife"}
	spellWitchBolt       = &core.Ref{Module: Module, Type: TypeSpells, ID: "witch-bolt"}
	spellGuidingBolt     = &core.Ref{Module: Module, Type: TypeSpells, ID: "guiding-bolt"}
	spellInflictWounds   = &core.Ref{Module: Module, Type: TypeSpells, ID: "inflict-wounds"}
	spellHailOfThorns    = &core.Ref{Module: Module, Type: TypeSpells, ID: "hail-of-thorns"}
	spellEnsnaringStrike = &core.Ref{Module: Module, Type: TypeSpells, ID: "ensnaring-strike"}
	spellHellishRebuke   = &core.Ref{Module: Module, Type: TypeSpells, ID: "hellish-rebuke"}
	spellArmsOfHadar     = &core.Ref{Module: Module, Type: TypeSpells, ID: "arms-of-hadar"}
	spellHex             = &core.Ref{Module: Module, Type: TypeSpells, ID: "hex"}
	spellSearingSmite    = &core.Ref{Module: Module, Type: TypeSpells, ID: "searing-smite"}
	spellThunderousSmite = &core.Ref{Module: Module, Type: TypeSpells, ID: "thunderous-smite"}
	spellWrathfulSmite   = &core.Ref{Module: Module, Type: TypeSpells, ID: "wrathful-smite"}

	// Level 1 - Utility
	spellShield              = &core.Ref{Module: Module, Type: TypeSpells, ID: "shield"}
	spellSleep               = &core.Ref{Module: Module, Type: TypeSpells, ID: "sleep"}
	spellCharmPerson         = &core.Ref{Module: Module, Type: TypeSpells, ID: "charm-person"}
	spellDetectMagic         = &core.Ref{Module: Module, Type: TypeSpells, ID: "detect-magic"}
	spellIdentify            = &core.Ref{Module: Module, Type: TypeSpells, ID: "identify"}
	spellCureWounds          = &core.Ref{Module: Module, Type: TypeSpells, ID: "cure-wounds"}
	spellHealingWord         = &core.Ref{Module: Module, Type: TypeSpells, ID: "healing-word"}
	spellBless               = &core.Ref{Module: Module, Type: TypeSpells, ID: "bless"}
	spellBane                = &core.Ref{Module: Module, Type: TypeSpells, ID: "bane"}
	spellShieldOfFaith       = &core.Ref{Module: Module, Type: TypeSpells, ID: "shield-of-faith"}
	spellAnimalFriendship    = &core.Ref{Module: Module, Type: TypeSpells, ID: "animal-friendship"}
	spellCommand             = &core.Ref{Module: Module, Type: TypeSpells, ID: "command"}
	spellDisguiseSelf        = &core.Ref{Module: Module, Type: TypeSpells, ID: "disguise-self"}
	spellDivineFavor         = &core.Ref{Module: Module, Type: TypeSpells, ID: "divine-favor"}
	spellFaerieFire          = &core.Ref{Module: Module, Type: TypeSpells, ID: "faerie-fire"}
	spellFalseLife           = &core.Ref{Module: Module, Type: TypeSpells, ID: "false-life"}
	spellFogCloud            = &core.Ref{Module: Module, Type: TypeSpells, ID: "fog-cloud"}
	spellRayOfSickness       = &core.Ref{Module: Module, Type: TypeSpells, ID: "ray-of-sickness"}
	spellSpeakWithAnimals    = &core.Ref{Module: Module, Type: TypeSpells, ID: "speak-with-animals"}
	spellComprehendLanguages = &core.Ref{Module: Module, Type: TypeSpells, ID: "comprehend-languages"}
	spellFeatherFall         = &core.Ref{Module: Module, Type: TypeSpells, ID: "feather-fall"}
	spellHeroism             = &core.Ref{Module: Module, Type: TypeSpells, ID: "heroism"}
	spellHideousLaughter     = &core.Ref{Module: Module, Type: TypeSpells, ID: "hideous-laughter"}
	spellIllusoryScript      = &core.Ref{Module: Module, Type: TypeSpells, ID: "illusory-script"}
	spellLongstrider         = &core.Ref{Module: Module, Type: TypeSpells, ID: "longstrider"}
	spellSilentImage         = &core.Ref{Module: Module, Type: TypeSpells, ID: "silent-image"}
	spellUnseenServant       = &core.Ref{Module: Module, Type: TypeSpells, ID: "unseen-servant"}
	spellAbsorbElements      = &core.Ref{Module: Module, Type: TypeSpells, ID: "absorb-elements"}
	spellBeastBond           = &core.Ref{Module: Module, Type: TypeSpells, ID: "beast-bond"}
	spellEntangle            = &core.Ref{Module: Module, Type: TypeSpells, ID: "entangle"}
	spellGoodberry           = &core.Ref{Module: Module, Type: TypeSpells, ID: "goodberry"}
	spellJump                = &core.Ref{Module: Module, Type: TypeSpells, ID: "jump"}
	spellPurifyFood          = &core.Ref{Module: Module, Type: TypeSpells, ID: "purify-food-and-drink"}
	spellCatapult            = &core.Ref{Module: Module, Type: TypeSpells, ID: "catapult"}
	spellCauseFear           = &core.Ref{Module: Module, Type: TypeSpells, ID: "cause-fear"}
	spellColorSpray          = &core.Ref{Module: Module, Type: TypeSpells, ID: "color-spray"}
	spellDistortValue        = &core.Ref{Module: Module, Type: TypeSpells, ID: "distort-value"}
	spellEarthTremor         = &core.Ref{Module: Module, Type: TypeSpells, ID: "earth-tremor"}
	spellExpeditiousRetreat  = &core.Ref{Module: Module, Type: TypeSpells, ID: "expeditious-retreat"}
	spellProtectionEvil      = &core.Ref{Module: Module, Type: TypeSpells, ID: "protection-from-evil-and-good"}

	// Level 2 - Damage
	spellScorchingRay       = &core.Ref{Module: Module, Type: TypeSpells, ID: "scorching-ray"}
	spellShatter            = &core.Ref{Module: Module, Type: TypeSpells, ID: "shatter"}
	spellAganazzarsScorcher = &core.Ref{Module: Module, Type: TypeSpells, ID: "aganazzars-scorcher"}
	spellCloudOfDaggers     = &core.Ref{Module: Module, Type: TypeSpells, ID: "cloud-of-daggers"}
	spellMelfsAcidArrow     = &core.Ref{Module: Module, Type: TypeSpells, ID: "melfs-acid-arrow"}
	spellMoonbeam           = &core.Ref{Module: Module, Type: TypeSpells, ID: "moonbeam"}
	spellSpiritualWeapon    = &core.Ref{Module: Module, Type: TypeSpells, ID: "spiritual-weapon"}
	spellFlamingSphere      = &core.Ref{Module: Module, Type: TypeSpells, ID: "flaming-sphere"}
	spellGustOfWind         = &core.Ref{Module: Module, Type: TypeSpells, ID: "gust-of-wind"}
	spellRayOfEnfeeblement  = &core.Ref{Module: Module, Type: TypeSpells, ID: "ray-of-enfeeblement"}

	// Level 2 - Utility
	spellAugury            = &core.Ref{Module: Module, Type: TypeSpells, ID: "augury"}
	spellBarkskin          = &core.Ref{Module: Module, Type: TypeSpells, ID: "barkskin"}
	spellBlindnessDeafness = &core.Ref{Module: Module, Type: TypeSpells, ID: "blindness-deafness"}
	spellLesserRestoration = &core.Ref{Module: Module, Type: TypeSpells, ID: "lesser-restoration"}
	spellMagicWeapon       = &core.Ref{Module: Module, Type: TypeSpells, ID: "magic-weapon"}
	spellMirrorImage       = &core.Ref{Module: Module, Type: TypeSpells, ID: "mirror-image"}
	spellPassWithoutTrace  = &core.Ref{Module: Module, Type: TypeSpells, ID: "pass-without-trace"}
	spellSpikeGrowth       = &core.Ref{Module: Module, Type: TypeSpells, ID: "spike-growth"}
	spellSuggestion        = &core.Ref{Module: Module, Type: TypeSpells, ID: "suggestion"}

	// Level 3 - Damage
	spellFireball        = &core.Ref{Module: Module, Type: TypeSpells, ID: "fireball"}
	spellLightningBolt   = &core.Ref{Module: Module, Type: TypeSpells, ID: "lightning-bolt"}
	spellCallLightning   = &core.Ref{Module: Module, Type: TypeSpells, ID: "call-lightning"}
	spellVampiricTouch   = &core.Ref{Module: Module, Type: TypeSpells, ID: "vampiric-touch"}
	spellSleetStorm      = &core.Ref{Module: Module, Type: TypeSpells, ID: "sleet-storm"}
	spellSpiritGuardians = &core.Ref{Module: Module, Type: TypeSpells, ID: "spirit-guardians"}

	// Level 3 - Utility
	spellAnimateDead     = &core.Ref{Module: Module, Type: TypeSpells, ID: "animate-dead"}
	spellBeaconOfHope    = &core.Ref{Module: Module, Type: TypeSpells, ID: "beacon-of-hope"}
	spellBlink           = &core.Ref{Module: Module, Type: TypeSpells, ID: "blink"}
	spellCrusadersMantle = &core.Ref{Module: Module, Type: TypeSpells, ID: "crusaders-mantle"}
	spellDaylight        = &core.Ref{Module: Module, Type: TypeSpells, ID: "daylight"}
	spellDispelMagic     = &core.Ref{Module: Module, Type: TypeSpells, ID: "dispel-magic"}
	spellNondetection    = &core.Ref{Module: Module, Type: TypeSpells, ID: "nondetection"}
	spellPlantGrowth     = &core.Ref{Module: Module, Type: TypeSpells, ID: "plant-growth"}
	spellRevivify        = &core.Ref{Module: Module, Type: TypeSpells, ID: "revivify"}
	spellSpeakWithDead   = &core.Ref{Module: Module, Type: TypeSpells, ID: "speak-with-dead"}
	spellWindWall        = &core.Ref{Module: Module, Type: TypeSpells, ID: "wind-wall"}

	// Level 4
	spellArcaneEye         = &core.Ref{Module: Module, Type: TypeSpells, ID: "arcane-eye"}
	spellBlight            = &core.Ref{Module: Module, Type: TypeSpells, ID: "blight"}
	spellConfusion         = &core.Ref{Module: Module, Type: TypeSpells, ID: "confusion"}
	spellControlWater      = &core.Ref{Module: Module, Type: TypeSpells, ID: "control-water"}
	spellDeathWard         = &core.Ref{Module: Module, Type: TypeSpells, ID: "death-ward"}
	spellDimensionDoor     = &core.Ref{Module: Module, Type: TypeSpells, ID: "dimension-door"}
	spellDominateBeast     = &core.Ref{Module: Module, Type: TypeSpells, ID: "dominate-beast"}
	spellFreedomOfMovement = &core.Ref{Module: Module, Type: TypeSpells, ID: "freedom-of-movement"}
	spellGraspingVine      = &core.Ref{Module: Module, Type: TypeSpells, ID: "grasping-vine"}
	spellGuardianOfFaith   = &core.Ref{Module: Module, Type: TypeSpells, ID: "guardian-of-faith"}
	spellIceStorm          = &core.Ref{Module: Module, Type: TypeSpells, ID: "ice-storm"}
	spellPolymorph         = &core.Ref{Module: Module, Type: TypeSpells, ID: "polymorph"}
	spellStoneskin         = &core.Ref{Module: Module, Type: TypeSpells, ID: "stoneskin"}
	spellWallOfFire        = &core.Ref{Module: Module, Type: TypeSpells, ID: "wall-of-fire"}

	// Level 5
	spellAntiLifeShell   = &core.Ref{Module: Module, Type: TypeSpells, ID: "antilife-shell"}
	spellCloudkill       = &core.Ref{Module: Module, Type: TypeSpells, ID: "cloudkill"}
	spellDestructiveWave = &core.Ref{Module: Module, Type: TypeSpells, ID: "destructive-wave"}
	spellDominatePerson  = &core.Ref{Module: Module, Type: TypeSpells, ID: "dominate-person"}
	spellFlameStrike     = &core.Ref{Module: Module, Type: TypeSpells, ID: "flame-strike"}
	spellHoldMonster     = &core.Ref{Module: Module, Type: TypeSpells, ID: "hold-monster"}
	spellInsectPlague    = &core.Ref{Module: Module, Type: TypeSpells, ID: "insect-plague"}
	spellLegendLore      = &core.Ref{Module: Module, Type: TypeSpells, ID: "legend-lore"}
	spellMassCureWounds  = &core.Ref{Module: Module, Type: TypeSpells, ID: "mass-cure-wounds"}
	spellModifyMemory    = &core.Ref{Module: Module, Type: TypeSpells, ID: "modify-memory"}
	spellRaiseDead       = &core.Ref{Module: Module, Type: TypeSpells, ID: "raise-dead"}
	spellScrying         = &core.Ref{Module: Module, Type: TypeSpells, ID: "scrying"}
	spellTreeStride      = &core.Ref{Module: Module, Type: TypeSpells, ID: "tree-stride"}
)

// Spells provides type-safe, discoverable references to D&D 5e spells.
// Use IDE autocomplete: refs.Spells.<tab> to discover available spells.
// Methods return singleton pointers enabling identity comparison (ref == refs.Spells.Fireball()).
var Spells = spellsNS{}

type spellsNS struct{}

// Cantrips - Damage
func (n spellsNS) FireBolt() *core.Ref        { return spellFireBolt }
func (n spellsNS) RayOfFrost() *core.Ref      { return spellRayOfFrost }
func (n spellsNS) ShockingGrasp() *core.Ref   { return spellShockingGrasp }
func (n spellsNS) AcidSplash() *core.Ref      { return spellAcidSplash }
func (n spellsNS) PoisonSpray() *core.Ref     { return spellPoisonSpray }
func (n spellsNS) ChillTouch() *core.Ref      { return spellChillTouch }
func (n spellsNS) SacredFlame() *core.Ref     { return spellSacredFlame }
func (n spellsNS) TollTheDead() *core.Ref     { return spellTollTheDead }
func (n spellsNS) WordOfRadiance() *core.Ref  { return spellWordOfRadiance }
func (n spellsNS) EldritchBlast() *core.Ref   { return spellEldritchBlast }
func (n spellsNS) Frostbite() *core.Ref       { return spellFrostbite }
func (n spellsNS) PrimalSavagery() *core.Ref  { return spellPrimalSavagery }
func (n spellsNS) ThornWhip() *core.Ref       { return spellThornWhip }
func (n spellsNS) CreateBonfire() *core.Ref   { return spellCreateBonfire }
func (n spellsNS) Druidcraft() *core.Ref      { return spellDruidcraft }
func (n spellsNS) Infestation() *core.Ref     { return spellInfestation }
func (n spellsNS) MagicStone() *core.Ref      { return spellMagicStone }
func (n spellsNS) MoldEarth() *core.Ref       { return spellMoldEarth }
func (n spellsNS) ShapeWater() *core.Ref      { return spellShapeWater }
func (n spellsNS) BoomingBlade() *core.Ref    { return spellBoomingBlade }
func (n spellsNS) ControlFlames() *core.Ref   { return spellControlFlames }
func (n spellsNS) GreenFlameBlade() *core.Ref { return spellGreenFlameBlade }
func (n spellsNS) Gust() *core.Ref            { return spellGust }
func (n spellsNS) SwordBurst() *core.Ref      { return spellSwordBurst }

// Cantrips - Utility
func (n spellsNS) MageHand() *core.Ref         { return spellMageHand }
func (n spellsNS) MinorIllusion() *core.Ref    { return spellMinorIllusion }
func (n spellsNS) Prestidigitation() *core.Ref { return spellPrestidigitation }
func (n spellsNS) Light() *core.Ref            { return spellLight }
func (n spellsNS) Guidance() *core.Ref         { return spellGuidance }
func (n spellsNS) Resistance() *core.Ref       { return spellResistance }
func (n spellsNS) Thaumaturgy() *core.Ref      { return spellThaumaturgy }
func (n spellsNS) SpareTheDying() *core.Ref    { return spellSpareTheDying }
func (n spellsNS) BladeWard() *core.Ref        { return spellBladeWard }
func (n spellsNS) DancingLights() *core.Ref    { return spellDancingLights }
func (n spellsNS) Friends() *core.Ref          { return spellFriends }
func (n spellsNS) Mending() *core.Ref          { return spellMending }
func (n spellsNS) Message() *core.Ref          { return spellMessage }
func (n spellsNS) TrueStrike() *core.Ref       { return spellTrueStrike }
func (n spellsNS) ViciousMockery() *core.Ref   { return spellViciousMockery }

// Level 1 - Damage
func (n spellsNS) MagicMissile() *core.Ref    { return spellMagicMissile }
func (n spellsNS) BurningHands() *core.Ref    { return spellBurningHands }
func (n spellsNS) ChromaticOrb() *core.Ref    { return spellChromaticOrb }
func (n spellsNS) Thunderwave() *core.Ref     { return spellThunderwave }
func (n spellsNS) IceKnife() *core.Ref        { return spellIceKnife }
func (n spellsNS) WitchBolt() *core.Ref       { return spellWitchBolt }
func (n spellsNS) GuidingBolt() *core.Ref     { return spellGuidingBolt }
func (n spellsNS) InflictWounds() *core.Ref   { return spellInflictWounds }
func (n spellsNS) HailOfThorns() *core.Ref    { return spellHailOfThorns }
func (n spellsNS) EnsnaringStrike() *core.Ref { return spellEnsnaringStrike }
func (n spellsNS) HellishRebuke() *core.Ref   { return spellHellishRebuke }
func (n spellsNS) ArmsOfHadar() *core.Ref     { return spellArmsOfHadar }
func (n spellsNS) Hex() *core.Ref             { return spellHex }
func (n spellsNS) SearingSmite() *core.Ref    { return spellSearingSmite }
func (n spellsNS) ThunderousSmite() *core.Ref { return spellThunderousSmite }
func (n spellsNS) WrathfulSmite() *core.Ref   { return spellWrathfulSmite }

// Level 1 - Utility
func (n spellsNS) Shield() *core.Ref              { return spellShield }
func (n spellsNS) Sleep() *core.Ref               { return spellSleep }
func (n spellsNS) CharmPerson() *core.Ref         { return spellCharmPerson }
func (n spellsNS) DetectMagic() *core.Ref         { return spellDetectMagic }
func (n spellsNS) Identify() *core.Ref            { return spellIdentify }
func (n spellsNS) CureWounds() *core.Ref          { return spellCureWounds }
func (n spellsNS) HealingWord() *core.Ref         { return spellHealingWord }
func (n spellsNS) Bless() *core.Ref               { return spellBless }
func (n spellsNS) Bane() *core.Ref                { return spellBane }
func (n spellsNS) ShieldOfFaith() *core.Ref       { return spellShieldOfFaith }
func (n spellsNS) AnimalFriendship() *core.Ref    { return spellAnimalFriendship }
func (n spellsNS) Command() *core.Ref             { return spellCommand }
func (n spellsNS) DisguiseSelf() *core.Ref        { return spellDisguiseSelf }
func (n spellsNS) DivineFavor() *core.Ref         { return spellDivineFavor }
func (n spellsNS) FaerieFire() *core.Ref          { return spellFaerieFire }
func (n spellsNS) FalseLife() *core.Ref           { return spellFalseLife }
func (n spellsNS) FogCloud() *core.Ref            { return spellFogCloud }
func (n spellsNS) RayOfSickness() *core.Ref       { return spellRayOfSickness }
func (n spellsNS) SpeakWithAnimals() *core.Ref    { return spellSpeakWithAnimals }
func (n spellsNS) ComprehendLanguages() *core.Ref { return spellComprehendLanguages }
func (n spellsNS) FeatherFall() *core.Ref         { return spellFeatherFall }
func (n spellsNS) Heroism() *core.Ref             { return spellHeroism }
func (n spellsNS) HideousLaughter() *core.Ref     { return spellHideousLaughter }
func (n spellsNS) IllusoryScript() *core.Ref      { return spellIllusoryScript }
func (n spellsNS) Longstrider() *core.Ref         { return spellLongstrider }
func (n spellsNS) SilentImage() *core.Ref         { return spellSilentImage }
func (n spellsNS) UnseenServant() *core.Ref       { return spellUnseenServant }
func (n spellsNS) AbsorbElements() *core.Ref      { return spellAbsorbElements }
func (n spellsNS) BeastBond() *core.Ref           { return spellBeastBond }
func (n spellsNS) Entangle() *core.Ref            { return spellEntangle }
func (n spellsNS) Goodberry() *core.Ref           { return spellGoodberry }
func (n spellsNS) Jump() *core.Ref                { return spellJump }
func (n spellsNS) PurifyFood() *core.Ref          { return spellPurifyFood }
func (n spellsNS) Catapult() *core.Ref            { return spellCatapult }
func (n spellsNS) CauseFear() *core.Ref           { return spellCauseFear }
func (n spellsNS) ColorSpray() *core.Ref          { return spellColorSpray }
func (n spellsNS) DistortValue() *core.Ref        { return spellDistortValue }
func (n spellsNS) EarthTremor() *core.Ref         { return spellEarthTremor }
func (n spellsNS) ExpeditiousRetreat() *core.Ref  { return spellExpeditiousRetreat }
func (n spellsNS) ProtectionEvil() *core.Ref      { return spellProtectionEvil }

// Level 2 - Damage
func (n spellsNS) ScorchingRay() *core.Ref       { return spellScorchingRay }
func (n spellsNS) Shatter() *core.Ref            { return spellShatter }
func (n spellsNS) AganazzarsScorcher() *core.Ref { return spellAganazzarsScorcher }
func (n spellsNS) CloudOfDaggers() *core.Ref     { return spellCloudOfDaggers }
func (n spellsNS) MelfsAcidArrow() *core.Ref     { return spellMelfsAcidArrow }
func (n spellsNS) Moonbeam() *core.Ref           { return spellMoonbeam }
func (n spellsNS) SpiritualWeapon() *core.Ref    { return spellSpiritualWeapon }
func (n spellsNS) FlamingSphere() *core.Ref      { return spellFlamingSphere }
func (n spellsNS) GustOfWind() *core.Ref         { return spellGustOfWind }
func (n spellsNS) RayOfEnfeeblement() *core.Ref  { return spellRayOfEnfeeblement }

// Level 2 - Utility
func (n spellsNS) Augury() *core.Ref            { return spellAugury }
func (n spellsNS) Barkskin() *core.Ref          { return spellBarkskin }
func (n spellsNS) BlindnessDeafness() *core.Ref { return spellBlindnessDeafness }
func (n spellsNS) LesserRestoration() *core.Ref { return spellLesserRestoration }
func (n spellsNS) MagicWeapon() *core.Ref       { return spellMagicWeapon }
func (n spellsNS) MirrorImage() *core.Ref       { return spellMirrorImage }
func (n spellsNS) PassWithoutTrace() *core.Ref  { return spellPassWithoutTrace }
func (n spellsNS) SpikeGrowth() *core.Ref       { return spellSpikeGrowth }
func (n spellsNS) Suggestion() *core.Ref        { return spellSuggestion }

// Level 3 - Damage
func (n spellsNS) Fireball() *core.Ref        { return spellFireball }
func (n spellsNS) LightningBolt() *core.Ref   { return spellLightningBolt }
func (n spellsNS) CallLightning() *core.Ref   { return spellCallLightning }
func (n spellsNS) VampiricTouch() *core.Ref   { return spellVampiricTouch }
func (n spellsNS) SleetStorm() *core.Ref      { return spellSleetStorm }
func (n spellsNS) SpiritGuardians() *core.Ref { return spellSpiritGuardians }

// Level 3 - Utility
func (n spellsNS) AnimateDead() *core.Ref     { return spellAnimateDead }
func (n spellsNS) BeaconOfHope() *core.Ref    { return spellBeaconOfHope }
func (n spellsNS) Blink() *core.Ref           { return spellBlink }
func (n spellsNS) CrusadersMantle() *core.Ref { return spellCrusadersMantle }
func (n spellsNS) Daylight() *core.Ref        { return spellDaylight }
func (n spellsNS) DispelMagic() *core.Ref     { return spellDispelMagic }
func (n spellsNS) Nondetection() *core.Ref    { return spellNondetection }
func (n spellsNS) PlantGrowth() *core.Ref     { return spellPlantGrowth }
func (n spellsNS) Revivify() *core.Ref        { return spellRevivify }
func (n spellsNS) SpeakWithDead() *core.Ref   { return spellSpeakWithDead }
func (n spellsNS) WindWall() *core.Ref        { return spellWindWall }

// Level 4
func (n spellsNS) ArcaneEye() *core.Ref         { return spellArcaneEye }
func (n spellsNS) Blight() *core.Ref            { return spellBlight }
func (n spellsNS) Confusion() *core.Ref         { return spellConfusion }
func (n spellsNS) ControlWater() *core.Ref      { return spellControlWater }
func (n spellsNS) DeathWard() *core.Ref         { return spellDeathWard }
func (n spellsNS) DimensionDoor() *core.Ref     { return spellDimensionDoor }
func (n spellsNS) DominateBeast() *core.Ref     { return spellDominateBeast }
func (n spellsNS) FreedomOfMovement() *core.Ref { return spellFreedomOfMovement }
func (n spellsNS) GraspingVine() *core.Ref      { return spellGraspingVine }
func (n spellsNS) GuardianOfFaith() *core.Ref   { return spellGuardianOfFaith }
func (n spellsNS) IceStorm() *core.Ref          { return spellIceStorm }
func (n spellsNS) Polymorph() *core.Ref         { return spellPolymorph }
func (n spellsNS) Stoneskin() *core.Ref         { return spellStoneskin }
func (n spellsNS) WallOfFire() *core.Ref        { return spellWallOfFire }

// Level 5
func (n spellsNS) AntiLifeShell() *core.Ref   { return spellAntiLifeShell }
func (n spellsNS) Cloudkill() *core.Ref       { return spellCloudkill }
func (n spellsNS) DestructiveWave() *core.Ref { return spellDestructiveWave }
func (n spellsNS) DominatePerson() *core.Ref  { return spellDominatePerson }
func (n spellsNS) FlameStrike() *core.Ref     { return spellFlameStrike }
func (n spellsNS) HoldMonster() *core.Ref     { return spellHoldMonster }
func (n spellsNS) InsectPlague() *core.Ref    { return spellInsectPlague }
func (n spellsNS) LegendLore() *core.Ref      { return spellLegendLore }
func (n spellsNS) MassCureWounds() *core.Ref  { return spellMassCureWounds }
func (n spellsNS) ModifyMemory() *core.Ref    { return spellModifyMemory }
func (n spellsNS) RaiseDead() *core.Ref       { return spellRaiseDead }
func (n spellsNS) Scrying() *core.Ref         { return spellScrying }
func (n spellsNS) TreeStride() *core.Ref      { return spellTreeStride }
