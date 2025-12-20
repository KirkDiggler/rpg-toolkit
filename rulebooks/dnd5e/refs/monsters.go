//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Monster singletons - unexported for controlled access via methods
var (
	// Undead (Crypt theme)
	monsterSkeleton        = &core.Ref{Module: Module, Type: TypeMonsters, ID: "skeleton"}
	monsterZombie          = &core.Ref{Module: Module, Type: TypeMonsters, ID: "zombie"}
	monsterSkeletonArcher  = &core.Ref{Module: Module, Type: TypeMonsters, ID: "skeleton-archer"}
	monsterSkeletonCaptain = &core.Ref{Module: Module, Type: TypeMonsters, ID: "skeleton-captain"}
	monsterGhoul           = &core.Ref{Module: Module, Type: TypeMonsters, ID: "ghoul"}

	// Beasts (Cave theme)
	monsterGiantRat        = &core.Ref{Module: Module, Type: TypeMonsters, ID: "giant-rat"}
	monsterGiantSpider     = &core.Ref{Module: Module, Type: TypeMonsters, ID: "giant-spider"}
	monsterGiantWolfSpider = &core.Ref{Module: Module, Type: TypeMonsters, ID: "giant-wolf-spider"}
	monsterWolf            = &core.Ref{Module: Module, Type: TypeMonsters, ID: "wolf"}
	monsterBrownBear       = &core.Ref{Module: Module, Type: TypeMonsters, ID: "brown-bear"}

	// Humanoids (Bandit Lair theme)
	monsterBandit        = &core.Ref{Module: Module, Type: TypeMonsters, ID: "bandit"}
	monsterBanditArcher  = &core.Ref{Module: Module, Type: TypeMonsters, ID: "bandit-archer"}
	monsterBanditCaptain = &core.Ref{Module: Module, Type: TypeMonsters, ID: "bandit-captain"}
	monsterThug          = &core.Ref{Module: Module, Type: TypeMonsters, ID: "thug"}
	monsterGoblin        = &core.Ref{Module: Module, Type: TypeMonsters, ID: "goblin"}
)

// Monsters provides type-safe, discoverable references to D&D 5e monsters.
// Use IDE autocomplete: refs.Monsters.<tab> to discover available monsters.
// Methods return singleton pointers enabling identity comparison (ref == refs.Monsters.Skeleton()).
var Monsters = monstersNS{}

type monstersNS struct{}

// Undead (Crypt theme)
func (n monstersNS) Skeleton() *core.Ref        { return monsterSkeleton }
func (n monstersNS) Zombie() *core.Ref          { return monsterZombie }
func (n monstersNS) SkeletonArcher() *core.Ref  { return monsterSkeletonArcher }
func (n monstersNS) SkeletonCaptain() *core.Ref { return monsterSkeletonCaptain }
func (n monstersNS) Ghoul() *core.Ref           { return monsterGhoul }

// Beasts (Cave theme)
func (n monstersNS) GiantRat() *core.Ref        { return monsterGiantRat }
func (n monstersNS) GiantSpider() *core.Ref     { return monsterGiantSpider }
func (n monstersNS) GiantWolfSpider() *core.Ref { return monsterGiantWolfSpider }
func (n monstersNS) Wolf() *core.Ref            { return monsterWolf }
func (n monstersNS) BrownBear() *core.Ref       { return monsterBrownBear }

// Humanoids (Bandit Lair theme)
func (n monstersNS) Bandit() *core.Ref        { return monsterBandit }
func (n monstersNS) BanditArcher() *core.Ref  { return monsterBanditArcher }
func (n monstersNS) BanditCaptain() *core.Ref { return monsterBanditCaptain }
func (n monstersNS) Thug() *core.Ref          { return monsterThug }
func (n monstersNS) Goblin() *core.Ref        { return monsterGoblin }
