package dungeonspec_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/encounter/dungeonspec"
)

const roomChainContractSource = `version: 1
key: contract
name: Contract
height: 7
rooms:
  - { id: entrance, archetype: entrance, width: 7, pattern: empty }
  - id: boss
    archetype: boss
    width: 7
    pattern: empty
    boss: { ref: "dnd5e:monsters:skeleton", at: [1,1] }
connectors: [{ from: entrance, to: boss }]
`

const (
	contractOutsideFloor = "outside_floor"
	contractStartPath    = "start"
)

func TestCompileDungeonSupportsYAMLMergeKeysWithExactMergedPaths(t *testing.T) {
	valid := []string{
		`version: 1
key: anchor-merge
name: Anchor Merge
height: 7
rooms:
  - &base { id: entrance, archetype: entrance, width: 7, pattern: empty }
  - <<: *base
    id: boss
    archetype: boss
    boss: { ref: "dnd5e:monsters:skeleton", at: [1,1] }
connectors: [{ from: entrance, to: boss }]
`,
		`version: 1
key: sequence-merge
name: Sequence Merge
height: 7
rooms:
  - &entrance { id: entrance, archetype: entrance, width: 7, pattern: empty }
  - &chamber { id: chamber, archetype: chamber, width: 7, pattern: empty }
  - <<: [*entrance, *chamber]
    id: boss
    archetype: boss
    boss: { ref: "dnd5e:monsters:skeleton", at: [1,1] }
connectors:
  - { from: entrance, to: chamber }
  - { from: chamber, to: boss }
`,
	}
	for _, source := range valid {
		out := compileCandidate(t, source, dungeonspec.CompileModeStrict, 1)
		require.Empty(t, out.FieldErrors)
		require.NotNil(t, out.FloorPlan)
	}

	invalid := []string{
		`version: 1
key: bad-merge
name: Bad Merge
height: 7
rooms:
  - <<: &bad { id: entrance, archetype: entrance, width: 7, pattern: empty, mystery: true }
  - { id: boss, archetype: boss, width: 7, pattern: empty, boss: { ref: "dnd5e:monsters:skeleton", at: [1,1] } }
connectors: [{ from: entrance, to: boss }]
`,
		`version: 1
key: bad-sequence-merge
name: Bad Sequence Merge
height: 7
rooms:
  - <<: [ &base { id: entrance, archetype: entrance, width: 7, pattern: empty }, &bad { mystery: true } ]
  - { id: boss, archetype: boss, width: 7, pattern: empty, boss: { ref: "dnd5e:monsters:skeleton", at: [1,1] } }
connectors: [{ from: entrance, to: boss }]
`,
	}
	for _, source := range invalid {
		out := compileCandidate(t, source, dungeonspec.CompileModeDraft, 1)
		require.Equal(t, "rooms[0].mystery", out.FieldErrors[0].Field)
		require.Equal(t, "unknown_field", out.FieldErrors[0].Code)
	}
}

func TestCompileDungeonRoomChainErrorsAreTypedAtSource(t *testing.T) {
	cases := []struct{ name, source, field, code string }{
		{
			name:   "room width",
			source: strings.Replace(roomChainContractSource, "width: 7, pattern: empty", "width: 3, pattern: empty", 1),
			field:  "rooms[0].width", code: "invalid_dimension",
		},
		{
			name:   "connector",
			source: strings.Replace(roomChainContractSource, "from: entrance", "from: boss", 1),
			field:  "connectors[0]", code: "invalid_chain",
		},
		{
			name:   "boss ref",
			source: strings.Replace(roomChainContractSource, "dnd5e:monsters:skeleton", "dnd5e:props:pillar", 1),
			field:  "rooms[1].boss.ref", code: "invalid_ref",
		},
		{
			name: "nested placed at",
			source: strings.Replace(roomChainContractSource,
				"  - { id: entrance, archetype: entrance, width: 7, pattern: empty }",
				"  - { id: entrance, archetype: entrance, width: 7, pattern: empty, "+
					"place: [{ ref: 'dnd5e:props:pillar', at: [99,0] }] }", 1),
			field: "rooms[0].place[0].at", code: contractOutsideFloor,
		},
		{
			name:   "wall endpoint",
			source: roomChainContractSource + "walls: [{ from: [99,0], to: [1,0], kind: solid }]\n",
			field:  "walls[0].from", code: contractOutsideFloor,
		},
		{
			name:   "start connector",
			source: roomChainContractSource + "start: [7,3]\n",
			field:  contractStartPath, code: contractOutsideFloor,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := compileCandidate(t, tc.source, dungeonspec.CompileModeDraft, 1)
			require.Equal(t, tc.field, out.FieldErrors[0].Field)
			require.Equal(t, tc.code, out.FieldErrors[0].Code)
		})
	}
}
