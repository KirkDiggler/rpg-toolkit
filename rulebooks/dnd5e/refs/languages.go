package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Languages provides type-safe, discoverable references to D&D 5e languages.
// Use IDE autocomplete: refs.Languages.<tab> to discover available languages.
var Languages = languagesNS{ns{TypeLanguages}}

type languagesNS struct{ ns }

// Standard Languages
func (n languagesNS) Common() *core.Ref   { return n.ref("common") }
func (n languagesNS) Dwarvish() *core.Ref { return n.ref("dwarvish") }
func (n languagesNS) Elvish() *core.Ref   { return n.ref("elvish") }
func (n languagesNS) Giant() *core.Ref    { return n.ref("giant") }
func (n languagesNS) Gnomish() *core.Ref  { return n.ref("gnomish") }
func (n languagesNS) Goblin() *core.Ref   { return n.ref("goblin") }
func (n languagesNS) Halfling() *core.Ref { return n.ref("halfling") }
func (n languagesNS) Orc() *core.Ref      { return n.ref("orc") }

// Exotic Languages
func (n languagesNS) Abyssal() *core.Ref     { return n.ref("abyssal") }
func (n languagesNS) Celestial() *core.Ref   { return n.ref("celestial") }
func (n languagesNS) Draconic() *core.Ref    { return n.ref("draconic") }
func (n languagesNS) DeepSpeech() *core.Ref  { return n.ref("deep-speech") }
func (n languagesNS) Infernal() *core.Ref    { return n.ref("infernal") }
func (n languagesNS) Primordial() *core.Ref  { return n.ref("primordial") }
func (n languagesNS) Sylvan() *core.Ref      { return n.ref("sylvan") }
func (n languagesNS) Undercommon() *core.Ref { return n.ref("undercommon") }
