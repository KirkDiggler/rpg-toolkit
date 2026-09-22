//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// TypeMonsterActions is the type identifier for monster action content.
const TypeMonsterActions core.Type = "monster_actions"

var (
	monsterActionAnimatedArmorSlam        = monsterActionRef("animated-armor-slam")
	monsterActionBrownBearBite            = monsterActionRef("brown-bear-bite")
	monsterActionBrownBearClaw            = monsterActionRef("brown-bear-claw")
	monsterActionGhoulBite                = monsterActionRef("ghoul-bite")
	monsterActionGhoulClaw                = monsterActionRef("ghoul-claw")
	monsterActionGiantRatBite             = monsterActionRef("giant-rat-bite")
	monsterActionSkeletonCaptainLongsword = monsterActionRef("skeleton-captain-longsword")
	monsterActionWolfBite                 = monsterActionRef("wolf-bite")
	monsterActionZombieSlam               = monsterActionRef("zombie-slam")

	// The multiattack scripts. A sequence is AUTHORED CONTENT even when
	// every step it names is a catalog weapon: "two scimitar attacks, the
	// second at disadvantage" is the goblin boss's own line and no weapon
	// entry describes it. That is why these have members here while the
	// scimitar and the mace they name do not (rpg-project#448).
	monsterActionAnimatedArmorMultiattack   = monsterActionRef("animated-armor-multiattack")
	monsterActionBrownBearMultiattack       = monsterActionRef("brown-bear-multiattack")
	monsterActionGhoulMultiattack           = monsterActionRef("ghoul-multiattack")
	monsterActionGoblinBossMultiattack      = monsterActionRef("goblin-boss-multiattack")
	monsterActionSkeletonCaptainMultiattack = monsterActionRef("skeleton-captain-multiattack")
	monsterActionThugMultiattack            = monsterActionRef("thug-multiattack")
)

func monsterActionRef(id string) *core.Ref {
	return &core.Ref{Module: Module, Type: TypeMonsterActions, ID: id}
}

// MonsterActions provides type-safe, discoverable references to authored
// monster action content. The refs identify definitions, never implementations.
//
// AUTHORED means authored: a claw, a bite, a slam — a thing a creature does
// that no catalog entry describes. A WEAPON IS NOT AUTHORED CONTENT AND HAS
// NO MEMBER HERE (rpg-project#448). A monster's shortbow attack carries the
// weapon's own ref, `dnd5e:weapons:shortbow`, exactly as a character's does,
// because there is nothing different about a goblin's shortbow and a
// skeleton's: both are +4 for 1d6+2 because both wielders have DEX 14 and a
// +2 proficiency bonus. A per-monster member for each would be a catalog of
// copies, and the six that existed — skeleton-shortsword, skeleton-shortbow,
// goblin-scimitar, thug-mace, bandit-scimitar, bandit-light-crossbow — are
// gone.
//
// One weapon-shaped member survives: skeleton-captain-longsword. The captain
// was outside this slice's named scope and is re-authored whenever somebody
// asks; its "+5, 1d8+3" derives from STR 16 and a +2 bonus exactly as the
// rest did.
var MonsterActions = monsterActionsNS{}

type monsterActionsNS struct{}

// AnimatedArmorSlam returns the animated armor's slam definition ref.
func (monsterActionsNS) AnimatedArmorSlam() *core.Ref { return monsterActionAnimatedArmorSlam }

// BrownBearBite returns the brown bear's bite definition ref.
func (monsterActionsNS) BrownBearBite() *core.Ref { return monsterActionBrownBearBite }

// BrownBearClaw returns the brown bear's claw definition ref.
func (monsterActionsNS) BrownBearClaw() *core.Ref { return monsterActionBrownBearClaw }

// GhoulBite returns the ghoul's bite definition ref.
func (monsterActionsNS) GhoulBite() *core.Ref { return monsterActionGhoulBite }

// GhoulClaw returns the ghoul's claw definition ref.
func (monsterActionsNS) GhoulClaw() *core.Ref { return monsterActionGhoulClaw }

// GiantRatBite returns the giant rat's bite definition ref.
func (monsterActionsNS) GiantRatBite() *core.Ref { return monsterActionGiantRatBite }

// SkeletonCaptainLongsword returns the skeleton captain's longsword definition ref.
func (monsterActionsNS) SkeletonCaptainLongsword() *core.Ref {
	return monsterActionSkeletonCaptainLongsword
}

// WolfBite returns the wolf's bite definition ref.
func (monsterActionsNS) WolfBite() *core.Ref { return monsterActionWolfBite }

// ZombieSlam returns the zombie's slam definition ref.
func (monsterActionsNS) ZombieSlam() *core.Ref { return monsterActionZombieSlam }

// AnimatedArmorMultiattack returns the animated armor's two-slam sequence ref.
func (monsterActionsNS) AnimatedArmorMultiattack() *core.Ref {
	return monsterActionAnimatedArmorMultiattack
}

// BrownBearMultiattack returns the brown bear's bite-and-claw sequence ref.
func (monsterActionsNS) BrownBearMultiattack() *core.Ref { return monsterActionBrownBearMultiattack }

// GhoulMultiattack returns the ghoul's bite-and-claw sequence ref.
func (monsterActionsNS) GhoulMultiattack() *core.Ref { return monsterActionGhoulMultiattack }

// GoblinBossMultiattack returns the goblin boss's two-scimitar sequence ref.
func (monsterActionsNS) GoblinBossMultiattack() *core.Ref {
	return monsterActionGoblinBossMultiattack
}

// SkeletonCaptainMultiattack returns the skeleton captain's two-longsword sequence ref.
func (monsterActionsNS) SkeletonCaptainMultiattack() *core.Ref {
	return monsterActionSkeletonCaptainMultiattack
}

// ThugMultiattack returns the thug's two-mace sequence ref.
func (monsterActionsNS) ThugMultiattack() *core.Ref { return monsterActionThugMultiattack }
