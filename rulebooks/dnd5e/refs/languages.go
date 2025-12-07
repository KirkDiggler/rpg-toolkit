package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Languages provides type-safe, discoverable references to D&D 5e languages.
// Use IDE autocomplete: refs.Languages.<tab> to discover available languages.
var Languages = languagesNS{}

type languagesNS struct{}

// Common returns a reference to the Common language.
func (languagesNS) Common() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "common"}
}

// Dwarvish returns a reference to the Dwarvish language.
func (languagesNS) Dwarvish() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "dwarvish"}
}

// Elvish returns a reference to the Elvish language.
func (languagesNS) Elvish() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "elvish"}
}

// Giant returns a reference to the Giant language.
func (languagesNS) Giant() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "giant"}
}

// Gnomish returns a reference to the Gnomish language.
func (languagesNS) Gnomish() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "gnomish"}
}

// Goblin returns a reference to the Goblin language.
func (languagesNS) Goblin() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "goblin"}
}

// Halfling returns a reference to the Halfling language.
func (languagesNS) Halfling() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "halfling"}
}

// Orc returns a reference to the Orc language.
func (languagesNS) Orc() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "orc"}
}

// Abyssal returns a reference to the Abyssal language.
func (languagesNS) Abyssal() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "abyssal"}
}

// Celestial returns a reference to the Celestial language.
func (languagesNS) Celestial() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "celestial"}
}

// Draconic returns a reference to the Draconic language.
func (languagesNS) Draconic() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "draconic"}
}

// DeepSpeech returns a reference to the Deep Speech language.
func (languagesNS) DeepSpeech() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "deep-speech"}
}

// Infernal returns a reference to the Infernal language.
func (languagesNS) Infernal() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "infernal"}
}

// Primordial returns a reference to the Primordial language.
func (languagesNS) Primordial() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "primordial"}
}

// Sylvan returns a reference to the Sylvan language.
func (languagesNS) Sylvan() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "sylvan"}
}

// Undercommon returns a reference to the Undercommon language.
func (languagesNS) Undercommon() *core.Ref {
	return &core.Ref{Module: Module, Type: TypeLanguages, ID: "undercommon"}
}
