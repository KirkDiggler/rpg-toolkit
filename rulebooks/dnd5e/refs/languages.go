//nolint:dupl // Namespace pattern intentional for IDE discoverability
package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// Language singletons - unexported for controlled access via methods
var (
	// Standard Languages
	languageCommon   = &core.Ref{Module: Module, Type: TypeLanguages, ID: "common"}
	languageDwarvish = &core.Ref{Module: Module, Type: TypeLanguages, ID: "dwarvish"}
	languageElvish   = &core.Ref{Module: Module, Type: TypeLanguages, ID: "elvish"}
	languageGiant    = &core.Ref{Module: Module, Type: TypeLanguages, ID: "giant"}
	languageGnomish  = &core.Ref{Module: Module, Type: TypeLanguages, ID: "gnomish"}
	languageGoblin   = &core.Ref{Module: Module, Type: TypeLanguages, ID: "goblin"}
	languageHalfling = &core.Ref{Module: Module, Type: TypeLanguages, ID: "halfling"}
	languageOrc      = &core.Ref{Module: Module, Type: TypeLanguages, ID: "orc"}

	// Exotic Languages
	languageAbyssal     = &core.Ref{Module: Module, Type: TypeLanguages, ID: "abyssal"}
	languageCelestial   = &core.Ref{Module: Module, Type: TypeLanguages, ID: "celestial"}
	languageDraconic    = &core.Ref{Module: Module, Type: TypeLanguages, ID: "draconic"}
	languageDeepSpeech  = &core.Ref{Module: Module, Type: TypeLanguages, ID: "deep-speech"}
	languageInfernal    = &core.Ref{Module: Module, Type: TypeLanguages, ID: "infernal"}
	languagePrimordial  = &core.Ref{Module: Module, Type: TypeLanguages, ID: "primordial"}
	languageSylvan      = &core.Ref{Module: Module, Type: TypeLanguages, ID: "sylvan"}
	languageUndercommon = &core.Ref{Module: Module, Type: TypeLanguages, ID: "undercommon"}
)

// Languages provides type-safe, discoverable references to D&D 5e languages.
// Use IDE autocomplete: refs.Languages.<tab> to discover available languages.
// Methods return singleton pointers enabling identity comparison (ref == refs.Languages.Common()).
var Languages = languagesNS{}

type languagesNS struct{}

// Standard Languages
func (n languagesNS) Common() *core.Ref   { return languageCommon }
func (n languagesNS) Dwarvish() *core.Ref { return languageDwarvish }
func (n languagesNS) Elvish() *core.Ref   { return languageElvish }
func (n languagesNS) Giant() *core.Ref    { return languageGiant }
func (n languagesNS) Gnomish() *core.Ref  { return languageGnomish }
func (n languagesNS) Goblin() *core.Ref   { return languageGoblin }
func (n languagesNS) Halfling() *core.Ref { return languageHalfling }
func (n languagesNS) Orc() *core.Ref      { return languageOrc }

// Exotic Languages
func (n languagesNS) Abyssal() *core.Ref     { return languageAbyssal }
func (n languagesNS) Celestial() *core.Ref   { return languageCelestial }
func (n languagesNS) Draconic() *core.Ref    { return languageDraconic }
func (n languagesNS) DeepSpeech() *core.Ref  { return languageDeepSpeech }
func (n languagesNS) Infernal() *core.Ref    { return languageInfernal }
func (n languagesNS) Primordial() *core.Ref  { return languagePrimordial }
func (n languagesNS) Sylvan() *core.Ref      { return languageSylvan }
func (n languagesNS) Undercommon() *core.Ref { return languageUndercommon }
