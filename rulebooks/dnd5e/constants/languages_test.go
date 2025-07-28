package constants_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/constants"
)

type LanguagesTestSuite struct {
	suite.Suite
}

func (s *LanguagesTestSuite) TestLanguageConstants() {
	tests := []struct {
		name     string
		language constants.Language
		expected string
		display  string
		standard bool
	}{
		{
			name:     "common language",
			language: constants.LanguageCommon,
			expected: "common",
			display:  "Common",
			standard: true,
		},
		{
			name:     "elvish language",
			language: constants.LanguageElvish,
			expected: "elvish",
			display:  "Elvish",
			standard: true,
		},
		{
			name:     "draconic language",
			language: constants.LanguageDraconic,
			expected: "draconic",
			display:  "Draconic",
			standard: false,
		},
		{
			name:     "deep speech language",
			language: constants.LanguageDeepSpeech,
			expected: "deep speech",
			display:  "Deep Speech",
			standard: false,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.Equal(tc.expected, string(tc.language))
			s.Equal(tc.display, tc.language.Display())
			s.Equal(tc.standard, tc.language.IsStandard())
		})
	}
}

func (s *LanguagesTestSuite) TestLanguageDisplay_UnknownLanguage() {
	unknown := constants.Language("unknown")
	s.Equal("Unknown", unknown.Display()) // Should capitalize first letter
}

func (s *LanguagesTestSuite) TestLanguageDisplay_EmptyLanguage() {
	empty := constants.Language("")
	s.Equal("", empty.Display())
}

func (s *LanguagesTestSuite) TestStandardLanguages() {
	languages := constants.StandardLanguages()
	s.Len(languages, 8)

	// Verify all are standard
	for _, lang := range languages {
		s.True(lang.IsStandard())
	}
}

func (s *LanguagesTestSuite) TestExoticLanguages() {
	languages := constants.ExoticLanguages()
	s.Len(languages, 8)

	// Verify none are standard
	for _, lang := range languages {
		s.False(lang.IsStandard())
	}
}

func TestLanguagesTestSuite(t *testing.T) {
	suite.Run(t, new(LanguagesTestSuite))
}
