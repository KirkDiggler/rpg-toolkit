//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Skill singletons - unexported for controlled access via methods
var (
	skillAcrobatics     = &core.Ref{Module: Module, Type: TypeSkills, ID: "acrobatics"}
	skillAnimalHandling = &core.Ref{Module: Module, Type: TypeSkills, ID: "animal-handling"}
	skillArcana         = &core.Ref{Module: Module, Type: TypeSkills, ID: "arcana"}
	skillAthletics      = &core.Ref{Module: Module, Type: TypeSkills, ID: "athletics"}
	skillDeception      = &core.Ref{Module: Module, Type: TypeSkills, ID: "deception"}
	skillHistory        = &core.Ref{Module: Module, Type: TypeSkills, ID: "history"}
	skillInsight        = &core.Ref{Module: Module, Type: TypeSkills, ID: "insight"}
	skillIntimidation   = &core.Ref{Module: Module, Type: TypeSkills, ID: "intimidation"}
	skillInvestigation  = &core.Ref{Module: Module, Type: TypeSkills, ID: "investigation"}
	skillMedicine       = &core.Ref{Module: Module, Type: TypeSkills, ID: "medicine"}
	skillNature         = &core.Ref{Module: Module, Type: TypeSkills, ID: "nature"}
	skillPerception     = &core.Ref{Module: Module, Type: TypeSkills, ID: "perception"}
	skillPerformance    = &core.Ref{Module: Module, Type: TypeSkills, ID: "performance"}
	skillPersuasion     = &core.Ref{Module: Module, Type: TypeSkills, ID: "persuasion"}
	skillReligion       = &core.Ref{Module: Module, Type: TypeSkills, ID: "religion"}
	skillSleightOfHand  = &core.Ref{Module: Module, Type: TypeSkills, ID: "sleight-of-hand"}
	skillStealth        = &core.Ref{Module: Module, Type: TypeSkills, ID: "stealth"}
	skillSurvival       = &core.Ref{Module: Module, Type: TypeSkills, ID: "survival"}
)

// Skills provides type-safe, discoverable references to D&D 5e skills.
// Use IDE autocomplete: refs.Skills.<tab> to discover available skills.
// Methods return singleton pointers enabling identity comparison (ref == refs.Skills.Stealth()).
var Skills = skillsNS{}

type skillsNS struct{}

func (n skillsNS) Acrobatics() *core.Ref     { return skillAcrobatics }
func (n skillsNS) AnimalHandling() *core.Ref { return skillAnimalHandling }
func (n skillsNS) Arcana() *core.Ref         { return skillArcana }
func (n skillsNS) Athletics() *core.Ref      { return skillAthletics }
func (n skillsNS) Deception() *core.Ref      { return skillDeception }
func (n skillsNS) History() *core.Ref        { return skillHistory }
func (n skillsNS) Insight() *core.Ref        { return skillInsight }
func (n skillsNS) Intimidation() *core.Ref   { return skillIntimidation }
func (n skillsNS) Investigation() *core.Ref  { return skillInvestigation }
func (n skillsNS) Medicine() *core.Ref       { return skillMedicine }
func (n skillsNS) Nature() *core.Ref         { return skillNature }
func (n skillsNS) Perception() *core.Ref     { return skillPerception }
func (n skillsNS) Performance() *core.Ref    { return skillPerformance }
func (n skillsNS) Persuasion() *core.Ref     { return skillPersuasion }
func (n skillsNS) Religion() *core.Ref       { return skillReligion }
func (n skillsNS) SleightOfHand() *core.Ref  { return skillSleightOfHand }
func (n skillsNS) Stealth() *core.Ref        { return skillStealth }
func (n skillsNS) Survival() *core.Ref       { return skillSurvival }
