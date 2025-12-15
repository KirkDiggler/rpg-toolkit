//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Class singletons - unexported for controlled access via methods
var (
	classBarbarian = &core.Ref{Module: Module, Type: TypeClasses, ID: "barbarian"}
	classBard      = &core.Ref{Module: Module, Type: TypeClasses, ID: "bard"}
	classCleric    = &core.Ref{Module: Module, Type: TypeClasses, ID: "cleric"}
	classDruid     = &core.Ref{Module: Module, Type: TypeClasses, ID: "druid"}
	classFighter   = &core.Ref{Module: Module, Type: TypeClasses, ID: "fighter"}
	classMonk      = &core.Ref{Module: Module, Type: TypeClasses, ID: "monk"}
	classPaladin   = &core.Ref{Module: Module, Type: TypeClasses, ID: "paladin"}
	classRanger    = &core.Ref{Module: Module, Type: TypeClasses, ID: "ranger"}
	classRogue     = &core.Ref{Module: Module, Type: TypeClasses, ID: "rogue"}
	classSorcerer  = &core.Ref{Module: Module, Type: TypeClasses, ID: "sorcerer"}
	classWarlock   = &core.Ref{Module: Module, Type: TypeClasses, ID: "warlock"}
	classWizard    = &core.Ref{Module: Module, Type: TypeClasses, ID: "wizard"}
)

// Classes provides type-safe, discoverable references to D&D 5e classes.
// Use IDE autocomplete: refs.Classes.<tab> to discover available classes.
// Methods return singleton pointers enabling identity comparison (ref == refs.Classes.Fighter()).
var Classes = classesNS{}

type classesNS struct{}

func (n classesNS) Barbarian() *core.Ref { return classBarbarian }
func (n classesNS) Bard() *core.Ref      { return classBard }
func (n classesNS) Cleric() *core.Ref    { return classCleric }
func (n classesNS) Druid() *core.Ref     { return classDruid }
func (n classesNS) Fighter() *core.Ref   { return classFighter }
func (n classesNS) Monk() *core.Ref      { return classMonk }
func (n classesNS) Paladin() *core.Ref   { return classPaladin }
func (n classesNS) Ranger() *core.Ref    { return classRanger }
func (n classesNS) Rogue() *core.Ref     { return classRogue }
func (n classesNS) Sorcerer() *core.Ref  { return classSorcerer }
func (n classesNS) Warlock() *core.Ref   { return classWarlock }
func (n classesNS) Wizard() *core.Ref    { return classWizard }
