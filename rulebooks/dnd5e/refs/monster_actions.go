//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// TypeMonsterActions is the type identifier for monster action content.
const TypeMonsterActions core.Type = "monster_actions"

var (
	monsterActionBanditScimitar           = monsterActionRef("bandit-scimitar")
	monsterActionBanditLightCrossbow      = monsterActionRef("bandit-light-crossbow")
	monsterActionBrownBearBite            = monsterActionRef("brown-bear-bite")
	monsterActionBrownBearClaw            = monsterActionRef("brown-bear-claw")
	monsterActionGhoulBite                = monsterActionRef("ghoul-bite")
	monsterActionGhoulClaw                = monsterActionRef("ghoul-claw")
	monsterActionGiantRatBite             = monsterActionRef("giant-rat-bite")
	monsterActionGoblinScimitar           = monsterActionRef("goblin-scimitar")
	monsterActionSkeletonCaptainLongsword = monsterActionRef("skeleton-captain-longsword")
	monsterActionSkeletonShortsword       = monsterActionRef("skeleton-shortsword")
	monsterActionSkeletonShortbow         = monsterActionRef("skeleton-shortbow")
	monsterActionThugMace                 = monsterActionRef("thug-mace")
	monsterActionWolfBite                 = monsterActionRef("wolf-bite")
	monsterActionZombieSlam               = monsterActionRef("zombie-slam")
)

func monsterActionRef(id string) *core.Ref {
	return &core.Ref{Module: Module, Type: TypeMonsterActions, ID: id}
}

// MonsterActions provides type-safe, discoverable references to authored
// monster action content. The refs identify definitions, never implementations.
var MonsterActions = monsterActionsNS{}

type monsterActionsNS struct{}

// BanditScimitar returns the bandit's scimitar definition ref.
func (monsterActionsNS) BanditScimitar() *core.Ref { return monsterActionBanditScimitar }

// BanditLightCrossbow returns the bandit's light-crossbow definition ref.
func (monsterActionsNS) BanditLightCrossbow() *core.Ref { return monsterActionBanditLightCrossbow }

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

// GoblinScimitar returns the goblin's scimitar definition ref.
func (monsterActionsNS) GoblinScimitar() *core.Ref { return monsterActionGoblinScimitar }

// SkeletonCaptainLongsword returns the skeleton captain's longsword definition ref.
func (monsterActionsNS) SkeletonCaptainLongsword() *core.Ref {
	return monsterActionSkeletonCaptainLongsword
}

// SkeletonShortsword returns the skeleton's shortsword definition ref.
func (monsterActionsNS) SkeletonShortsword() *core.Ref { return monsterActionSkeletonShortsword }

// SkeletonShortbow returns the skeleton's shortbow definition ref.
func (monsterActionsNS) SkeletonShortbow() *core.Ref { return monsterActionSkeletonShortbow }

// ThugMace returns the thug's mace definition ref.
func (monsterActionsNS) ThugMace() *core.Ref { return monsterActionThugMace }

// WolfBite returns the wolf's bite definition ref.
func (monsterActionsNS) WolfBite() *core.Ref { return monsterActionWolfBite }

// ZombieSlam returns the zombie's slam definition ref.
func (monsterActionsNS) ZombieSlam() *core.Ref { return monsterActionZombieSlam }
