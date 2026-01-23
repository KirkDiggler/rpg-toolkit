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
			value:   "darkvision",
			module:  "core",
			idType:  "feature",
			wantErr: false,
		},
		{
			name:    "empty value",
			value:   "",
			module:  "core",
			idType:  "feature",
			wantErr: true,
		},
		{
			name:    "empty module",
			value:   "darkvision",
			module:  "",
			idType:  "feature",
			wantErr: true,
		},
		{
			name:    "empty type",
			value:   "darkvision",
			module:  "core",
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
	id := core.MustNewRef(core.RefInput{Module: "core", Type: "feature", ID: "darkvision"})
	assert.Equal(t, "core:feature:darkvision", id.String())
}

func TestID_Equals(t *testing.T) {
	id1 := core.MustNewRef(core.RefInput{Module: "core", Type: "feature", ID: "darkvision"})
	id2 := core.MustNewRef(core.RefInput{Module: "core", Type: "feature", ID: "darkvision"})
	id3 := core.MustNewRef(core.RefInput{Module: "core", Type: "proficiency", ID: "darkvision"})
	id4 := core.MustNewRef(core.RefInput{Module: "core", Type: "feature", ID: "keen_senses"})

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
	original := core.MustNewRef(core.RefInput{Module: "core", Type: "skill", ID: "athletics"})

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

	assert.Equal(t, "darkvision", id.ID)
	assert.Equal(t, "core", id.Module)
	assert.Equal(t, "feature", id.Type)
}

func TestWithSource(t *testing.T) {
	id := core.MustNewRef(core.RefInput{Module: "core", Type: "feature", ID: "second_wind"})
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
		core.MustNewRef(core.RefInput{Module: "core", Type: "feature", ID: ""})
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
			want:  core.MustNewRef(core.RefInput{Module: "core", Type: "feature", ID: "rage"}),
		},
		{
			name:  "valid with underscores",
			input: "core:feature:sneak_attack",
			want:  core.MustNewRef(core.RefInput{Module: "core", Type: "feature", ID: "sneak_attack"}),
		},
		{
			name:  "valid with dashes",
			input: "third-party:feature:custom-ability",
			want:  core.MustNewRef(core.RefInput{Module: "third-party", Type: "feature", ID: "custom-ability"}),
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
			name:         "too many parts",
			input:        "core:feature:rage:extra",
			wantErr:      core.ErrTooManySegments,
			wantErrMsg:   "expected 3 segments, got 4",
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
			name:         "invalid characters - spaces",
			input:        "core:feature:rage bonus",
			wantErr:      core.ErrInvalidCharacters,
			wantErrMsg:   "invalid characters",
			checkErrType: true,
		},
		{
			name:         "invalid characters - special chars",
			input:        "core:feature:rage!",
			wantErr:      core.ErrInvalidCharacters,
			wantErrMsg:   "invalid characters",
			checkErrType: true,
		},
		{
			name:         "invalid characters - dots",
			input:        "core:feature:rage.bonus",
			wantErr:      core.ErrInvalidCharacters,
			wantErrMsg:   "invalid characters",
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
