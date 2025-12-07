//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Spells provides type-safe, discoverable references to D&D 5e spells.
// Use IDE autocomplete: refs.Spells.<tab> to discover available spells.
var Spells = spellsNS{ns{TypeSpells}}

type spellsNS struct{ ns }

// Cantrips - Damage
func (n spellsNS) FireBolt() *core.Ref        { return n.ref("fire-bolt") }
func (n spellsNS) RayOfFrost() *core.Ref      { return n.ref("ray-of-frost") }
func (n spellsNS) ShockingGrasp() *core.Ref   { return n.ref("shocking-grasp") }
func (n spellsNS) AcidSplash() *core.Ref      { return n.ref("acid-splash") }
func (n spellsNS) PoisonSpray() *core.Ref     { return n.ref("poison-spray") }
func (n spellsNS) ChillTouch() *core.Ref      { return n.ref("chill-touch") }
func (n spellsNS) SacredFlame() *core.Ref     { return n.ref("sacred-flame") }
func (n spellsNS) TollTheDead() *core.Ref     { return n.ref("toll-the-dead") }
func (n spellsNS) WordOfRadiance() *core.Ref  { return n.ref("word-of-radiance") }
func (n spellsNS) EldritchBlast() *core.Ref   { return n.ref("eldritch-blast") }
func (n spellsNS) Frostbite() *core.Ref       { return n.ref("frostbite") }
func (n spellsNS) PrimalSavagery() *core.Ref  { return n.ref("primal-savagery") }
func (n spellsNS) Thornwhip() *core.Ref       { return n.ref("thornwhip") }
func (n spellsNS) CreateBonfire() *core.Ref   { return n.ref("create-bonfire") }
func (n spellsNS) Druidcraft() *core.Ref      { return n.ref("druidcraft") }
func (n spellsNS) Infestation() *core.Ref     { return n.ref("infestation") }
func (n spellsNS) MagicStone() *core.Ref      { return n.ref("magic-stone") }
func (n spellsNS) MoldEarth() *core.Ref       { return n.ref("mold-earth") }
func (n spellsNS) ShapeWater() *core.Ref      { return n.ref("shape-water") }
func (n spellsNS) BoomingBlade() *core.Ref    { return n.ref("booming-blade") }
func (n spellsNS) ControlFlames() *core.Ref   { return n.ref("control-flames") }
func (n spellsNS) GreenFlameBlade() *core.Ref { return n.ref("green-flame-blade") }
func (n spellsNS) GustWind() *core.Ref        { return n.ref("gust") }
func (n spellsNS) SwordBurst() *core.Ref      { return n.ref("sword-burst") }

// Cantrips - Utility
func (n spellsNS) MageHand() *core.Ref         { return n.ref("mage-hand") }
func (n spellsNS) MinorIllusion() *core.Ref    { return n.ref("minor-illusion") }
func (n spellsNS) Prestidigitation() *core.Ref { return n.ref("prestidigitation") }
func (n spellsNS) Light() *core.Ref            { return n.ref("light") }
func (n spellsNS) Guidance() *core.Ref         { return n.ref("guidance") }
func (n spellsNS) Resistance() *core.Ref       { return n.ref("resistance") }
func (n spellsNS) Thaumaturgy() *core.Ref      { return n.ref("thaumaturgy") }
func (n spellsNS) SpareTheDying() *core.Ref    { return n.ref("spare-the-dying") }
func (n spellsNS) BladeWard() *core.Ref        { return n.ref("blade-ward") }
func (n spellsNS) DancingLights() *core.Ref    { return n.ref("dancing-lights") }
func (n spellsNS) Friends() *core.Ref          { return n.ref("friends") }
func (n spellsNS) Mending() *core.Ref          { return n.ref("mending") }
func (n spellsNS) Message() *core.Ref          { return n.ref("message") }
func (n spellsNS) TrueStrike() *core.Ref       { return n.ref("true-strike") }
func (n spellsNS) ViciousMockery() *core.Ref   { return n.ref("vicious-mockery") }

// Level 1 - Damage
func (n spellsNS) MagicMissile() *core.Ref    { return n.ref("magic-missile") }
func (n spellsNS) BurningHands() *core.Ref    { return n.ref("burning-hands") }
func (n spellsNS) ChromaticOrb() *core.Ref    { return n.ref("chromatic-orb") }
func (n spellsNS) Thunderwave() *core.Ref     { return n.ref("thunderwave") }
func (n spellsNS) IceKnife() *core.Ref        { return n.ref("ice-knife") }
func (n spellsNS) WitchBolt() *core.Ref       { return n.ref("witch-bolt") }
func (n spellsNS) GuidingBolt() *core.Ref     { return n.ref("guiding-bolt") }
func (n spellsNS) InflictWounds() *core.Ref   { return n.ref("inflict-wounds") }
func (n spellsNS) HailOfThorns() *core.Ref    { return n.ref("hail-of-thorns") }
func (n spellsNS) EnsnaringStrike() *core.Ref { return n.ref("ensnaring-strike") }
func (n spellsNS) HellishRebuke() *core.Ref   { return n.ref("hellish-rebuke") }
func (n spellsNS) ArmsOfHadar() *core.Ref     { return n.ref("arms-of-hadar") }
func (n spellsNS) Hex() *core.Ref             { return n.ref("hex") }
func (n spellsNS) SearingSmite() *core.Ref    { return n.ref("searing-smite") }
func (n spellsNS) ThunderousSmite() *core.Ref { return n.ref("thunderous-smite") }
func (n spellsNS) WrathfulSmite() *core.Ref   { return n.ref("wrathful-smite") }

// Level 1 - Utility
func (n spellsNS) Shield() *core.Ref              { return n.ref("shield") }
func (n spellsNS) Sleep() *core.Ref               { return n.ref("sleep") }
func (n spellsNS) CharmPerson() *core.Ref         { return n.ref("charm-person") }
func (n spellsNS) DetectMagic() *core.Ref         { return n.ref("detect-magic") }
func (n spellsNS) Identify() *core.Ref            { return n.ref("identify") }
func (n spellsNS) CureWounds() *core.Ref          { return n.ref("cure-wounds") }
func (n spellsNS) HealingWord() *core.Ref         { return n.ref("healing-word") }
func (n spellsNS) Bless() *core.Ref               { return n.ref("bless") }
func (n spellsNS) Bane() *core.Ref                { return n.ref("bane") }
func (n spellsNS) ShieldOfFaith() *core.Ref       { return n.ref("shield-of-faith") }
func (n spellsNS) AnimalFriendship() *core.Ref    { return n.ref("animal-friendship") }
func (n spellsNS) Command() *core.Ref             { return n.ref("command") }
func (n spellsNS) DisguiseSelf() *core.Ref        { return n.ref("disguise-self") }
func (n spellsNS) DivineFavor() *core.Ref         { return n.ref("divine-favor") }
func (n spellsNS) FaerieFire() *core.Ref          { return n.ref("faerie-fire") }
func (n spellsNS) FalseLife() *core.Ref           { return n.ref("false-life") }
func (n spellsNS) FogCloud() *core.Ref            { return n.ref("fog-cloud") }
func (n spellsNS) RayOfSickness() *core.Ref       { return n.ref("ray-of-sickness") }
func (n spellsNS) SpeakWithAnimals() *core.Ref    { return n.ref("speak-with-animals") }
func (n spellsNS) ComprehendLanguages() *core.Ref { return n.ref("comprehend-languages") }
func (n spellsNS) FeatherFall() *core.Ref         { return n.ref("feather-fall") }
func (n spellsNS) Heroism() *core.Ref             { return n.ref("heroism") }
func (n spellsNS) HideousLaughter() *core.Ref     { return n.ref("hideous-laughter") }
func (n spellsNS) IllusoryScript() *core.Ref      { return n.ref("illusory-script") }
func (n spellsNS) Longstrider() *core.Ref         { return n.ref("longstrider") }
func (n spellsNS) SilentImage() *core.Ref         { return n.ref("silent-image") }
func (n spellsNS) UnseenServant() *core.Ref       { return n.ref("unseen-servant") }
func (n spellsNS) AbsorbElements() *core.Ref      { return n.ref("absorb-elements") }
func (n spellsNS) BeastBond() *core.Ref           { return n.ref("beast-bond") }
func (n spellsNS) Entangle() *core.Ref            { return n.ref("entangle") }
func (n spellsNS) GoodBerry() *core.Ref           { return n.ref("goodberry") }
func (n spellsNS) JumpSpell() *core.Ref           { return n.ref("jump") }
func (n spellsNS) PurifyFood() *core.Ref          { return n.ref("purify-food-and-drink") }
func (n spellsNS) CatapultSpell() *core.Ref       { return n.ref("catapult") }
func (n spellsNS) CauseFear() *core.Ref           { return n.ref("cause-fear") }
func (n spellsNS) ColorSpray() *core.Ref          { return n.ref("color-spray") }
func (n spellsNS) DistortValue() *core.Ref        { return n.ref("distort-value") }
func (n spellsNS) EarthTremor() *core.Ref         { return n.ref("earth-tremor") }
func (n spellsNS) ExpeditiousRetreat() *core.Ref  { return n.ref("expeditious-retreat") }
func (n spellsNS) ProtectionEvil() *core.Ref      { return n.ref("protection-from-evil-and-good") }

// Level 2 - Damage
func (n spellsNS) ScorchingRay() *core.Ref       { return n.ref("scorching-ray") }
func (n spellsNS) Shatter() *core.Ref            { return n.ref("shatter") }
func (n spellsNS) AganazzarsScorcher() *core.Ref { return n.ref("aganazzars-scorcher") }
func (n spellsNS) CloudOfDaggers() *core.Ref     { return n.ref("cloud-of-daggers") }
func (n spellsNS) MelfsAcidArrow() *core.Ref     { return n.ref("melfs-acid-arrow") }
func (n spellsNS) Moonbeam() *core.Ref           { return n.ref("moonbeam") }
func (n spellsNS) SpiritualWeapon() *core.Ref    { return n.ref("spiritual-weapon") }
func (n spellsNS) FlamingSphere() *core.Ref      { return n.ref("flaming-sphere") }
func (n spellsNS) GustOfWind() *core.Ref         { return n.ref("gust-of-wind") }
func (n spellsNS) RayOfEnfeeblement() *core.Ref  { return n.ref("ray-of-enfeeblement") }

// Level 2 - Utility
func (n spellsNS) Augury() *core.Ref            { return n.ref("augury") }
func (n spellsNS) Barkskin() *core.Ref          { return n.ref("barkskin") }
func (n spellsNS) BlindnessDeafness() *core.Ref { return n.ref("blindness-deafness") }
func (n spellsNS) LesserRestoration() *core.Ref { return n.ref("lesser-restoration") }
func (n spellsNS) MagicWeapon() *core.Ref       { return n.ref("magic-weapon") }
func (n spellsNS) MirrorImage() *core.Ref       { return n.ref("mirror-image") }
func (n spellsNS) PassWithoutTrace() *core.Ref  { return n.ref("pass-without-trace") }
func (n spellsNS) SpikeGrowth() *core.Ref       { return n.ref("spike-growth") }
func (n spellsNS) Suggestion() *core.Ref        { return n.ref("suggestion") }

// Level 3 - Damage
func (n spellsNS) Fireball() *core.Ref        { return n.ref("fireball") }
func (n spellsNS) LightningBolt() *core.Ref   { return n.ref("lightning-bolt") }
func (n spellsNS) CallLightning() *core.Ref   { return n.ref("call-lightning") }
func (n spellsNS) VampiricTouch() *core.Ref   { return n.ref("vampiric-touch") }
func (n spellsNS) SleetStorm() *core.Ref      { return n.ref("sleet-storm") }
func (n spellsNS) SpiritGuardians() *core.Ref { return n.ref("spirit-guardians") }

// Level 3 - Utility
func (n spellsNS) AnimateDead() *core.Ref     { return n.ref("animate-dead") }
func (n spellsNS) BeaconOfHope() *core.Ref    { return n.ref("beacon-of-hope") }
func (n spellsNS) Blink() *core.Ref           { return n.ref("blink") }
func (n spellsNS) CrusadersMantle() *core.Ref { return n.ref("crusaders-mantle") }
func (n spellsNS) Daylight() *core.Ref        { return n.ref("daylight") }
func (n spellsNS) DispelMagic() *core.Ref     { return n.ref("dispel-magic") }
func (n spellsNS) Nondetection() *core.Ref    { return n.ref("nondetection") }
func (n spellsNS) PlantGrowth() *core.Ref     { return n.ref("plant-growth") }
func (n spellsNS) Revivify() *core.Ref        { return n.ref("revivify") }
func (n spellsNS) SpeakWithDead() *core.Ref   { return n.ref("speak-with-dead") }
func (n spellsNS) WindWall() *core.Ref        { return n.ref("wind-wall") }

// Level 4
func (n spellsNS) ArcaneEye() *core.Ref         { return n.ref("arcane-eye") }
func (n spellsNS) Blight() *core.Ref            { return n.ref("blight") }
func (n spellsNS) Confusion() *core.Ref         { return n.ref("confusion") }
func (n spellsNS) ControlWater() *core.Ref      { return n.ref("control-water") }
func (n spellsNS) DeathWard() *core.Ref         { return n.ref("death-ward") }
func (n spellsNS) DimensionDoor() *core.Ref     { return n.ref("dimension-door") }
func (n spellsNS) DominateBeast() *core.Ref     { return n.ref("dominate-beast") }
func (n spellsNS) FreedomOfMovement() *core.Ref { return n.ref("freedom-of-movement") }
func (n spellsNS) GraspingVine() *core.Ref      { return n.ref("grasping-vine") }
func (n spellsNS) GuardianOfFaith() *core.Ref   { return n.ref("guardian-of-faith") }
func (n spellsNS) IceStorm() *core.Ref          { return n.ref("ice-storm") }
func (n spellsNS) Polymorph() *core.Ref         { return n.ref("polymorph") }
func (n spellsNS) Stoneskin() *core.Ref         { return n.ref("stoneskin") }
func (n spellsNS) WallOfFire() *core.Ref        { return n.ref("wall-of-fire") }

// Level 5
func (n spellsNS) AntiLifeShell() *core.Ref   { return n.ref("antilife-shell") }
func (n spellsNS) Cloudkill() *core.Ref       { return n.ref("cloudkill") }
func (n spellsNS) DestructiveWave() *core.Ref { return n.ref("destructive-wave") }
func (n spellsNS) DominatePerson() *core.Ref  { return n.ref("dominate-person") }
func (n spellsNS) FlameStrike() *core.Ref     { return n.ref("flame-strike") }
func (n spellsNS) HoldMonster() *core.Ref     { return n.ref("hold-monster") }
func (n spellsNS) InsectPlague() *core.Ref    { return n.ref("insect-plague") }
func (n spellsNS) LegendLore() *core.Ref      { return n.ref("legend-lore") }
func (n spellsNS) MassCureWounds() *core.Ref  { return n.ref("mass-cure-wounds") }
func (n spellsNS) ModifyMemory() *core.Ref    { return n.ref("modify-memory") }
func (n spellsNS) RaiseDead() *core.Ref       { return n.ref("raise-dead") }
func (n spellsNS) Scrying() *core.Ref         { return n.ref("scrying") }
func (n spellsNS) TreeStride() *core.Ref      { return n.ref("tree-stride") }
