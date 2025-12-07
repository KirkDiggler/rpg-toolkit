//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Skills provides type-safe, discoverable references to D&D 5e skills.
// Use IDE autocomplete: refs.Skills.<tab> to discover available skills.
var Skills = skillsNS{ns{TypeSkills}}

type skillsNS struct{ ns }

func (n skillsNS) Acrobatics() *core.Ref     { return n.ref("acrobatics") }
func (n skillsNS) AnimalHandling() *core.Ref { return n.ref("animal-handling") }
func (n skillsNS) Arcana() *core.Ref         { return n.ref("arcana") }
func (n skillsNS) Athletics() *core.Ref      { return n.ref("athletics") }
func (n skillsNS) Deception() *core.Ref      { return n.ref("deception") }
func (n skillsNS) History() *core.Ref        { return n.ref("history") }
func (n skillsNS) Insight() *core.Ref        { return n.ref("insight") }
func (n skillsNS) Intimidation() *core.Ref   { return n.ref("intimidation") }
func (n skillsNS) Investigation() *core.Ref  { return n.ref("investigation") }
func (n skillsNS) Medicine() *core.Ref       { return n.ref("medicine") }
func (n skillsNS) Nature() *core.Ref         { return n.ref("nature") }
func (n skillsNS) Perception() *core.Ref     { return n.ref("perception") }
func (n skillsNS) Performance() *core.Ref    { return n.ref("performance") }
func (n skillsNS) Persuasion() *core.Ref     { return n.ref("persuasion") }
func (n skillsNS) Religion() *core.Ref       { return n.ref("religion") }
func (n skillsNS) SleightOfHand() *core.Ref  { return n.ref("sleight-of-hand") }
func (n skillsNS) Stealth() *core.Ref        { return n.ref("stealth") }
func (n skillsNS) Survival() *core.Ref       { return n.ref("survival") }
