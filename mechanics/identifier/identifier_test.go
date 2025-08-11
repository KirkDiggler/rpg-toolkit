package identifier_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/mechanics/identifier"
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
			id, err := identifier.New(tt.value, tt.module, tt.idType)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.value, id.Value)
			assert.Equal(t, tt.module, id.Module)
			assert.Equal(t, tt.idType, id.Type)
		})
	}
}

func TestID_String(t *testing.T) {
	id := identifier.MustNew("darkvision", "core", "feature")
	assert.Equal(t, "core:feature:darkvision", id.String())
}

func TestID_Equals(t *testing.T) {
	id1 := identifier.MustNew("darkvision", "core", "feature")
	id2 := identifier.MustNew("darkvision", "core", "feature")
	id3 := identifier.MustNew("darkvision", "core", "proficiency")
	id4 := identifier.MustNew("keen_senses", "core", "feature")

	assert.True(t, id1.Equals(id2), "identical IDs should be equal")
	assert.False(t, id1.Equals(id3), "different types should not be equal")
	assert.False(t, id1.Equals(id4), "different values should not be equal")
	
	// Test nil handling
	var nilID *identifier.ID
	assert.False(t, id1.Equals(nilID), "non-nil should not equal nil")
	assert.True(t, nilID.Equals(nilID), "nil should equal nil")
}

func TestID_JSONMarshaling(t *testing.T) {
	original := identifier.MustNew("athletics", "core", "skill")

	// Marshal to JSON
	data, err := json.Marshal(original)
	require.NoError(t, err)
	assert.Equal(t, `"core:skill:athletics"`, string(data))

	// Unmarshal back
	var unmarshaled identifier.ID
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	assert.True(t, original.Equals(&unmarshaled))
}

func TestID_JSONUnmarshal_BackwardCompatibility(t *testing.T) {
	// Test that we can unmarshal the old object format
	oldFormat := `{"value":"darkvision","module":"core","type":"feature"}`

	var id identifier.ID
	err := json.Unmarshal([]byte(oldFormat), &id)
	require.NoError(t, err)

	assert.Equal(t, "darkvision", id.Value)
	assert.Equal(t, "core", id.Module)
	assert.Equal(t, "feature", id.Type)
}

func TestWithSource(t *testing.T) {
	id := identifier.MustNew("second_wind", "core", "feature")
	withSource := identifier.NewWithSource(id, "class:fighter")

	assert.Equal(t, id, withSource.ID)
	assert.Equal(t, "class:fighter", withSource.Source)

	// Test JSON marshaling
	data, err := json.Marshal(withSource)
	require.NoError(t, err)

	var unmarshaled identifier.WithSource
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	assert.True(t, withSource.ID.Equals(unmarshaled.ID))
	assert.Equal(t, withSource.Source, unmarshaled.Source)
}

func TestMustNew_Panics(t *testing.T) {
	assert.Panics(t, func() {
		identifier.MustNew("", "core", "feature")
	}, "MustNew should panic with invalid input")
}

func TestParseString(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		want         *identifier.ID
		wantErr      error
		wantErrMsg   string
		checkErrType bool
	}{
		{
			name:  "valid identifier",
			input: "core:feature:rage",
			want:  identifier.MustNew("rage", "core", "feature"),
		},
		{
			name:  "valid with underscores",
			input: "core:feature:sneak_attack",
			want:  identifier.MustNew("sneak_attack", "core", "feature"),
		},
		{
			name:  "valid with dashes",
			input: "third-party:feature:custom-ability",
			want:  identifier.MustNew("custom-ability", "third-party", "feature"),
		},
		{
			name:         "empty string",
			input:        "",
			wantErr:      identifier.ErrEmptyString,
			checkErrType: true,
		},
		{
			name:         "missing parts",
			input:        "core:feature",
			wantErr:      identifier.ErrTooFewSegments,
			wantErrMsg:   "expected 3 segments, got 2",
			checkErrType: true,
		},
		{
			name:         "too many parts",
			input:        "core:feature:rage:extra",
			wantErr:      identifier.ErrTooManySegments,
			wantErrMsg:   "expected 3 segments, got 4",
			checkErrType: true,
		},
		{
			name:         "empty module",
			input:        ":feature:rage",
			wantErr:      identifier.ErrEmptyComponent,
			wantErrMsg:   "module",
			checkErrType: true,
		},
		{
			name:         "empty type",
			input:        "core::rage",
			wantErr:      identifier.ErrEmptyComponent,
			wantErrMsg:   "type",
			checkErrType: true,
		},
		{
			name:         "empty value",
			input:        "core:feature:",
			wantErr:      identifier.ErrEmptyComponent,
			wantErrMsg:   "value",
			checkErrType: true,
		},
		{
			name:         "invalid characters - spaces",
			input:        "core:feature:rage bonus",
			wantErr:      identifier.ErrInvalidCharacters,
			wantErrMsg:   "invalid characters",
			checkErrType: true,
		},
		{
			name:         "invalid characters - special chars",
			input:        "core:feature:rage!",
			wantErr:      identifier.ErrInvalidCharacters,
			wantErrMsg:   "invalid characters",
			checkErrType: true,
		},
		{
			name:         "invalid characters - dots",
			input:        "core:feature:rage.bonus",
			wantErr:      identifier.ErrInvalidCharacters,
			wantErrMsg:   "invalid characters",
			checkErrType: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := identifier.ParseString(tt.input)
			
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
				if identifier.IsParseError(err) {
					var parseErr *identifier.ParseError
					errors.As(err, &parseErr)
					assert.Equal(t, tt.input, parseErr.Input)
				} else if identifier.IsValidationError(err) {
					var valErr *identifier.ValidationError
					errors.As(err, &valErr)
					assert.NotEmpty(t, valErr.Field)
				}
				
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.True(t, got.Equals(tt.want), "parsed ID should equal expected")
			}
		})
	}
}
