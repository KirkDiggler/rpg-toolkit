package core_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		module  string
		idType  string
		wantErr bool
	}{
		{
			name:    "valid identifier",
			value:   testDarkvision,
			module:  testModuleCore,
			idType:  testTypeFeature,
			wantErr: false,
		},
		{
			name:    "empty value",
			value:   "",
			module:  testModuleCore,
			idType:  testTypeFeature,
			wantErr: true,
		},
		{
			name:    "empty module",
			value:   testDarkvision,
			module:  "",
			idType:  testTypeFeature,
			wantErr: true,
		},
		{
			name:    "empty type",
			value:   testDarkvision,
			module:  testModuleCore,
			idType:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := core.NewRef(core.RefInput{
				Module: tt.module,
				Type:   tt.idType,
				ID:     tt.value,
			})
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.value, id.ID)
			assert.Equal(t, tt.module, id.Module)
			assert.Equal(t, tt.idType, id.Type)
		})
	}
}

func TestID_String(t *testing.T) {
	id := core.MustNewRef(core.RefInput{Module: testModuleCore, Type: testTypeFeature, ID: testDarkvision})
	assert.Equal(t, "core:feature:darkvision", id.String())
}

func TestID_Equals(t *testing.T) {
	id1 := core.MustNewRef(core.RefInput{Module: testModuleCore, Type: testTypeFeature, ID: testDarkvision})
	id2 := core.MustNewRef(core.RefInput{Module: testModuleCore, Type: testTypeFeature, ID: testDarkvision})
	id3 := core.MustNewRef(core.RefInput{Module: testModuleCore, Type: "proficiency", ID: testDarkvision})
	id4 := core.MustNewRef(core.RefInput{Module: testModuleCore, Type: testTypeFeature, ID: "keen_senses"})

	assert.True(t, id1.Equals(id2), "identical IDs should be equal")
	assert.False(t, id1.Equals(id3), "different types should not be equal")
	assert.False(t, id1.Equals(id4), "different values should not be equal")

	// Test nil handling
	var nilID *core.Ref
	var nilID2 *core.Ref
	assert.False(t, id1.Equals(nilID), "non-nil should not equal nil")
	assert.True(t, nilID.Equals(nilID2), "nil should equal nil")
}

func TestID_JSONMarshaling(t *testing.T) {
	original := core.MustNewRef(core.RefInput{Module: testModuleCore, Type: "skill", ID: "athletics"})

	// Marshal to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Equal(t, `"core:skill:athletics"`, string(data))

	// Unmarshal back
	var unmarshaled core.Ref
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.True(t, original.Equals(&unmarshaled))
}

func TestID_JSONUnmarshal_BackwardCompatibility(t *testing.T) {
	// Test that we can unmarshal the object format
	objectFormat := `{"module":"core","type":"feature","id":"darkvision"}`

	var id core.Ref
	err := json.Unmarshal([]byte(objectFormat), &id)
	require.NoError(t, err)

	assert.Equal(t, testDarkvision, id.ID)
	assert.Equal(t, "core", id.Module)
	assert.Equal(t, "feature", id.Type)
}

func TestWithSource(t *testing.T) {
	id := core.MustNewRef(core.RefInput{Module: testModuleCore, Type: testTypeFeature, ID: "second_wind"})
	withSource := core.NewWithSourcedRef(id, &core.Source{
		Category: core.SourceClass,
		Name:     "fighter",
	})

	assert.Equal(t, id, withSource.ID)
	assert.Equal(t, "class:fighter", withSource.Source.String())

	// Test JSON marshaling
	data, err := json.Marshal(withSource)
	require.NoError(t, err)

	var unmarshaled core.WithSourcedRef
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.True(t, withSource.ID.Equals(unmarshaled.ID))
	assert.Equal(t, withSource.Source.String(), unmarshaled.Source.String())
}

func TestMustNew_Panics(t *testing.T) {
	assert.Panics(t, func() {
		core.MustNewRef(core.RefInput{Module: testModuleCore, Type: testTypeFeature, ID: ""})
	}, "MustNewRef should panic with invalid input")
}

func TestParseString(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		want         *core.Ref
		wantErr      error
		wantErrMsg   string
		checkErrType bool
	}{
		{
			name:  "valid identifier",
			input: "core:feature:rage",
			want:  core.MustNewRef(core.RefInput{Module: testModuleCore, Type: testTypeFeature, ID: testRage}),
		},
		{
			name:  "valid with underscores",
			input: "core:feature:sneak_attack",
			want:  core.MustNewRef(core.RefInput{Module: testModuleCore, Type: testTypeFeature, ID: "sneak_attack"}),
		},
		{
			name:  "valid with dashes",
			input: "third-party:feature:custom-ability",
			want:  core.MustNewRef(core.RefInput{Module: "third-party", Type: testTypeFeature, ID: "custom-ability"}),
		},
		{
			name:         "empty string",
			input:        "",
			wantErr:      core.ErrEmptyString,
			checkErrType: true,
		},
		{
			name:         "missing parts",
			input:        "core:feature",
			wantErr:      core.ErrTooFewSegments,
			wantErrMsg:   "expected 3 segments, got 2",
			checkErrType: true,
		},
		{
			name:  "an id with two parts",
			input: "dnd5e:props:plushie:skeleton-dog",
			want: core.MustNewRef(core.RefInput{
				Module: "dnd5e", Type: "props", ID: "plushie:skeleton-dog"}),
		},
		{
			name:  "an id with three parts",
			input: "dnd5e:props:plushie:skeleton-dog:chewed",
			want: core.MustNewRef(core.RefInput{
				Module: "dnd5e", Type: "props", ID: "plushie:skeleton-dog:chewed"}),
		},
		{
			name:         "an id that is only a separator",
			input:        "a:b::c",
			wantErr:      core.ErrEmptyComponent,
			wantErrMsg:   "id part 1",
			checkErrType: true,
		},
		{
			name:         "an id ending in a separator",
			input:        "a:b:c:",
			wantErr:      core.ErrEmptyComponent,
			wantErrMsg:   testIDPart2,
			checkErrType: true,
		},
		{
			name:         "a later id part with invalid characters",
			input:        "dnd5e:props:plushie:skeleton dog",
			wantErr:      core.ErrInvalidCharacters,
			wantErrMsg:   testIDPart2,
			checkErrType: true,
		},
		{
			name:         "empty module",
			input:        ":feature:rage",
			wantErr:      core.ErrEmptyComponent,
			wantErrMsg:   "module",
			checkErrType: true,
		},
		{
			name:         "empty type",
			input:        "core::rage",
			wantErr:      core.ErrEmptyComponent,
			wantErrMsg:   "type",
			checkErrType: true,
		},
		{
			name:         "empty id",
			input:        "core:feature:",
			wantErr:      core.ErrEmptyComponent,
			wantErrMsg:   "id",
			checkErrType: true,
		},
		{
			name:         "empty id, the issue's own spelling",
			input:        "a:b:",
			wantErr:      core.ErrEmptyComponent,
			wantErrMsg:   "id",
			checkErrType: true,
		},
		{
			name:         "invalid characters - spaces",
			input:        "core:feature:rage bonus",
			wantErr:      core.ErrInvalidCharacters,
			wantErrMsg:   testErrInvalidChar,
			checkErrType: true,
		},
		{
			name:         "invalid characters - special chars",
			input:        "core:feature:rage!",
			wantErr:      core.ErrInvalidCharacters,
			wantErrMsg:   testErrInvalidChar,
			checkErrType: true,
		},
		{
			name:         "invalid characters - dots",
			input:        "core:feature:rage.bonus",
			wantErr:      core.ErrInvalidCharacters,
			wantErrMsg:   testErrInvalidChar,
			checkErrType: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := core.ParseString(tt.input)

			if tt.wantErr != nil {
				assert.Error(t, err)

				// Check for specific error type if requested
				if tt.checkErrType {
					assert.ErrorIs(t, err, tt.wantErr, "should match expected error type")
				}

				// Check error message contains expected text
				if tt.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tt.wantErrMsg)
				}

				// Verify it's a ParseError or ValidationError
				if core.IsParseError(err) {
					var parseErr *core.ParseError
					errors.As(err, &parseErr)
					assert.Equal(t, tt.input, parseErr.Input)
				} else if core.IsValidationError(err) {
					var valErr *core.ValidationError
					errors.As(err, &valErr)
					assert.NotEmpty(t, valErr.Field)
				}

				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.True(t, got.Equals(tt.want), "parsed Ref should equal expected")
			}
		})
	}
}

// TestParseString_IDParts is the grammar rule stated on its own: the id is
// everything after the second separator, and String puts it back verbatim.
//
// The round trip is what makes the rule safe to adopt. A ref with a two-part
// id crosses the wire as a string, is parsed by whoever receives it, and is
// printed again on the way out; if any of those steps counted parts or
// re-joined them differently, the ref that came back would not be the ref that
// went in. Depth is the thing being varied, because depth is what the old
// grammar refused.
func TestParseString_IDParts(t *testing.T) {
	depths := []struct {
		name string
		ref  string
		id   string
	}{
		{"three parts", "dnd5e:props:brazier", "brazier"},
		{"four parts", "dnd5e:props:plushie:skeleton-dog", "plushie:skeleton-dog"},
		{"five parts", "dnd5e:props:plushie:skeleton-dog:chewed", "plushie:skeleton-dog:chewed"},
	}

	for _, d := range depths {
		t.Run(d.name, func(t *testing.T) {
			parsed, err := core.ParseString(d.ref)
			require.NoError(t, err)

			assert.Equal(t, "dnd5e", parsed.Module)
			assert.Equal(t, "props", parsed.Type)
			assert.Equal(t, d.id, parsed.ID,
				"the id is everything after the second separator, joined as authored")
			assert.Equal(t, d.ref, parsed.String(),
				"and printing it gives back exactly the ref that was parsed")
		})
	}
}

// TestParseString_NamesTheEmptyPart — a refusal has to say WHICH part was
// empty. These three strings are three different author mistakes, and telling
// them apart is the whole reason the id's parts are validated one at a time
// instead of as one string.
func TestParseString_NamesTheEmptyPart(t *testing.T) {
	tests := []struct {
		name  string
		input string
		names string
	}{
		{"nothing after the second separator", "a:b:", "id"},
		{"a gap at the front of the id", "a:b::c", "id part 1"},
		{"a gap at the end of the id", "a:b:c:", testIDPart2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := core.ParseString(tt.input)

			require.Error(t, err)
			assert.Nil(t, got)
			assert.ErrorIs(t, err, core.ErrEmptyComponent)
			assert.Contains(t, err.Error(), tt.names,
				"the refusal names the part the author has to go fix")
		})
	}
}
