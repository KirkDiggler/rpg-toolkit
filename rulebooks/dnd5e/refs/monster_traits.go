//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Monster trait singletons - unexported for controlled access via methods
var (
	monsterTraitImmunity        = &core.Ref{Module: Module, Type: TypeMonsterTraits, ID: "immunity"}
	monsterTraitVulnerability   = &core.Ref{Module: Module, Type: TypeMonsterTraits, ID: "vulnerability"}
	monsterTraitPackTactics     = &core.Ref{Module: Module, Type: TypeMonsterTraits, ID: "pack_tactics"}
	monsterTraitUndeadFortitude = &core.Ref{Module: Module, Type: TypeMonsterTraits, ID: "undead_fortitude"}
)

// MonsterTraits provides type-safe, discoverable references to monster trait behaviors.
// Use IDE autocomplete: refs.MonsterTraits.<tab> to discover available traits.
// Methods return singleton pointers enabling identity comparison (ref == refs.MonsterTraits.Immunity()).
var MonsterTraits = monsterTraitsNS{}

type monsterTraitsNS struct{}

// Immunity returns the ref for damage immunity trait
func (n monsterTraitsNS) Immunity() *core.Ref { return monsterTraitImmunity }

// Vulnerability returns the ref for damage vulnerability trait
func (n monsterTraitsNS) Vulnerability() *core.Ref { return monsterTraitVulnerability }

// PackTactics returns the ref for pack tactics trait
func (n monsterTraitsNS) PackTactics() *core.Ref { return monsterTraitPackTactics }

// UndeadFortitude returns the ref for undead fortitude trait
func (n monsterTraitsNS) UndeadFortitude() *core.Ref { return monsterTraitUndeadFortitude }
