// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// heirloom_test.go is the recover-the-artifact authoring slice
// (rpg-project#368, design §3.1 and §3.3): placement ids, knowledge links,
// holdable props, exits, and scenario bindings — what each compiles to, and
// every way the file can get one wrong.
//
// The refusals are the deliverable as much as the compile is. Each scene
// below starts from the heirloom tomb, changes ONE line, and asserts the
// author is told about that line — the form-filler at the builder is the
// reader, and "somewhere in your dungeon" is not an answer they can act on.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

const heirloomPath = "testdata/reference-tomb-heirloom.yaml"

// heirloomSource is the fixture's bytes, read once per scene so a scene that
// edits them cannot leak into the next.
func heirloomSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(heirloomPath)
	require.NoError(t, err)
	return string(raw)
}

// defectsIn decodes and validates a spec, returning every defect as
// "path: message" so a scene can assert on the path AND the sentence.
func defectsIn(t *testing.T, source string) []string {
	t.Helper()
	spec, err := dungeonspec.Decode([]byte(source))
	require.NoError(t, err, "these scenes are about validation, so the file must still decode")
	out := make([]string, 0)
	for _, e := range dungeonspec.Validate(spec) {
		out = append(out, e.Path+": "+e.Message)
	}
	return out
}

// requireDefect asserts exactly one defect matches, and reports every defect
// when none does — a scene that silently matched the wrong one would be a
// test that cannot fail.
func requireDefect(t *testing.T, defects []string, substrings ...string) {
	t.Helper()
	var matched []string
	for _, d := range defects {
		hit := true
		for _, want := range substrings {
			if !strings.Contains(d, want) {
				hit = false
				break
			}
		}
		if hit {
			matched = append(matched, d)
		}
	}
	require.Len(t, matched, 1, "wanted exactly one defect containing %q, got all of: %v", substrings, defects)
}

// TestTheHeirloomTombCompiles is the fixture's own gate: the dungeon the rest
// of this slice is authored against is a legal dungeon, and it carries
// exactly what the design says it carries.
func TestTheHeirloomTombCompiles(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(heirloomSource(t)))
	require.NoError(t, err)

	t.Run("the artifact is a holdable prop with an id", func(t *testing.T) {
		var artifact encounter.PropInput
		named := map[encounter.PropID]bool{}
		for _, p := range compiled.Field.Props {
			if p.ID != "" {
				named[p.ID] = true
			}
			if p.ID == "heirloom" {
				artifact = p
			}
		}
		// Two named props now: the artifact the scenario binds, and the
		// scroll that carries the second record (R6).
		require.Equal(t, map[encounter.PropID]bool{"heirloom": true, "hall-scroll": true}, named)
		require.Equal(t, "heirloom", artifact.ID)
		require.True(t, artifact.Holdable)
		require.Equal(t, "dnd5e:props:reliquary", artifact.Ref)
		require.Empty(t, artifact.Holds, "the artifact is the prize, not a source of intel")
	})

	t.Run("the hall scroll carries the second record (R6)", func(t *testing.T) {
		var scroll encounter.PropInput
		for _, p := range compiled.Field.Props {
			if p.ID == "hall-scroll" {
				scroll = p
			}
		}
		require.Equal(t, "hall-scroll", scroll.ID, "the fixture ships a scroll")
		require.True(t, scroll.Holdable, "you have to be able to pick it up to read it")
		require.Equal(t, []encounter.IntelID{"reference-tomb-heirloom/hall-notes"}, scroll.Holds)
	})

	t.Run("two records may reveal one door", func(t *testing.T) {
		require.Len(t, compiled.Intel, 2)
		for _, rec := range compiled.Intel {
			require.Equal(t, encounter.DoorID("reference-tomb-heirloom/vault"), rec.Reveals.Door,
				"knowledge is not scarce: the captain knows the way and so does the scroll")
		}
	})

	t.Run("the captain holds the vault map and carries no boss flag", func(t *testing.T) {
		var captain dungeonspec.MonsterPlacement
		for _, m := range compiled.Monsters {
			if m.ID == "captain" {
				captain = m
			}
		}
		require.Equal(t, "captain", captain.ID, "the fixture names its captain")
		require.False(t, captain.Boss,
			"design R8: this dungeon ends because a scenario says so, not because a monster has a flag")
		// THE COMPILED RECORD ID, not the author's. Every other id on this
		// half is the author's own word; a holder is the exception, because
		// intelOf mints <key>/<id> and a holder carrying the raw authored id
		// would name a record the composition does not have.
		require.Equal(t, []string{"reference-tomb-heirloom/vault-map"}, captain.Holds)
	})

	t.Run("the exit is the cell the party starts on, named", func(t *testing.T) {
		require.Len(t, compiled.Field.Exits, 1)
		require.Equal(t, "entrance", compiled.Field.Exits[0].ID)
		require.Equal(t, compiled.PartyStart[0].At, compiled.Field.Exits[0].At,
			"the fixture authors its entrance as its way out — deliberately, since start is never implicitly one")
	})

	t.Run("the scenario bindings are carried opaquely", func(t *testing.T) {
		require.Equal(t, map[string]map[string]string{
			"recover-the-artifact": {"artifact": "heirloom", "exit": "entrance"},
		}, compiled.Scenarios)
	})

	t.Run("the vault is concealed behind a concealed door", func(t *testing.T) {
		var vault encounter.RegionInput
		for _, r := range compiled.Field.Regions {
			if r.ID == "vault" {
				vault = r
			}
		}
		require.Equal(t, "vault", vault.ID)
		require.True(t, vault.Concealed)

		var door encounter.DoorInput
		for _, d := range compiled.Field.Doors {
			if strings.HasSuffix(string(d.ID), "/vault") {
				door = d
			}
		}
		require.Equal(t, encounter.DoorID("reference-tomb-heirloom/vault"), door.ID)
		require.Len(t, door.Concealed, 2, "spotted or reasoned out — a check is beaten by any listed route")
	})
}

// TestTheScenarioBindingsAreCarriedAndNothingElse pins design law C1 at this
// package's own boundary: dungeonspec validates that a binding names
// something that exists and NOTHING about what the scenario means.
func TestTheScenarioBindingsAreCarriedAndNothingElse(t *testing.T) {
	source := heirloomSource(t)

	t.Run("a scenario id this build never heard of compiles", func(t *testing.T) {
		// The scenario package owns which ids exist. If this package refused
		// an unknown one, adding a scenario would mean shipping a new
		// dungeonspec — which is the coupling C1 exists to prevent.
		edited := strings.Replace(source, "  recover-the-artifact:", "  polish-the-brass:", 1)
		require.Empty(t, defectsIn(t, edited))
	})

	t.Run("a key this build never heard of compiles", func(t *testing.T) {
		edited := strings.Replace(source, "    artifact: heirloom", "    trinket: heirloom", 1)
		require.Empty(t, defectsIn(t, edited))
	})

	t.Run("a binding naming a monster compiles, because kind is not this layer's question", func(t *testing.T) {
		edited := strings.Replace(source, "    artifact: heirloom", "    artifact: captain", 1)
		require.Empty(t, defectsIn(t, edited),
			"whether an artifact must be a holdable prop is the scenario's refusal, in form-filler words")
	})

	t.Run("a binding naming nothing is refused, at the binding", func(t *testing.T) {
		edited := strings.Replace(source, "    artifact: heirloom", "    artifact: chalice", 1)
		requireDefect(t, defectsIn(t, edited),
			"scenarios.recover-the-artifact.artifact", `binds artifact to "chalice"`, "has that id")
	})

	t.Run("a binding naming an exit that does not exist is refused", func(t *testing.T) {
		edited := strings.Replace(source, "    exit: entrance", "    exit: back-door", 1)
		requireDefect(t, defectsIn(t, edited), "scenarios.recover-the-artifact.exit", `"back-door"`)
	})

	t.Run("a binding naming nothing at all is refused", func(t *testing.T) {
		edited := strings.Replace(source, "    exit: entrance", `    exit: ""`, 1)
		requireDefect(t, defectsIn(t, edited), "scenarios.recover-the-artifact.exit", "binds exit to nothing")
	})
}

// TestPlacementIDRefusals covers design §3.3's id rules.
func TestPlacementIDRefusals(t *testing.T) {
	source := heirloomSource(t)

	t.Run("a duplicate id is refused, naming both lines", func(t *testing.T) {
		edited := strings.Replace(source,
			`  - { ref: "dnd5e:props:candles", at: [26,5]`,
			`  - { id: heirloom, ref: "dnd5e:props:candles", at: [26,5]`, 1)
		defects := defectsIn(t, edited)
		requireDefect(t, defects, ".id", `id "heirloom" is already declared`, "place[")
	})

	t.Run("an id is optional", func(t *testing.T) {
		edited := strings.Replace(source, "  - { id: captain, ref:", "  - { ref:", 1)
		require.Empty(t, defectsIn(t, edited), "a monster nothing binds to needs no name")
	})
}

// TestIntelRefusals covers the intel record's own rules (design §2).
func TestIntelRefusals(t *testing.T) {
	source := heirloomSource(t)

	t.Run("a record revealing a door that does not exist is refused by name", func(t *testing.T) {
		edited := strings.Replace(source, "reveals: { door: vault }", "reveals: { door: cellar }", 1)
		requireDefect(t, defectsIn(t, edited), "intel[0].reveals.door", `reveals door "cellar"`, "has that id")
	})

	t.Run("a record that reveals nothing is refused", func(t *testing.T) {
		// Nothing is defaulted: there is no "reveals the nearest door".
		edited := strings.Replace(source, "    reveals: { door: vault }", "    reveals: {}", 1)
		requireDefect(t, defectsIn(t, edited), "intel[0].reveals", "does not say what it reveals")
	})

	t.Run("a record with no id is refused", func(t *testing.T) {
		edited := strings.Replace(source,
			"  - id: vault-map\n    reveals: { door: vault }",
			`  - id: ""`+"\n    reveals: { door: vault }", 1)
		requireDefect(t, defectsIn(t, edited), "intel[0].id", "the intel record has no id")
	})

	t.Run("a duplicate record id is refused, naming the earlier line", func(t *testing.T) {
		edited := strings.Replace(source,
			"intel:\n  - id: vault-map\n    reveals: { door: vault }",
			"intel:\n  - id: vault-map\n    reveals: { door: vault }\n"+
				"  - id: vault-map\n    reveals: { door: hall-tomb }", 1)
		requireDefect(t, defectsIn(t, edited), "intel[1].id", `intel "vault-map" is already declared at intel[0]`)
	})

	t.Run("revealing an ORDINARY door is legal and inert", func(t *testing.T) {
		// Not an error, deliberately: refusing it would make this record's
		// legality depend on a fact about a different declaration.
		edited := strings.Replace(source, "reveals: { door: vault }", "reveals: { door: hall-tomb }", 1)
		require.Empty(t, defectsIn(t, edited))
	})
}

// TestHoldsRefusals covers what a placement may carry (design §2).
func TestHoldsRefusals(t *testing.T) {
	source := heirloomSource(t)

	t.Run("a record that does not exist is refused by name", func(t *testing.T) {
		edited := strings.Replace(source, "holds: [vault-map] }", "holds: [cellar-map] }", 1)
		requireDefect(t, defectsIn(t, edited), ".holds[0]",
			`holds intel "cellar-map"`, "no record in this dungeon")
	})

	t.Run("holds on a PROP is legal (R6)", func(t *testing.T) {
		// Kirk, walking: "tech could get intel by holding something too."
		// A scroll on a table is the shape, and the shipped fixture uses it
		// so the tool can be tested without a fight.
		edited := strings.Replace(source,
			`  - { ref: "dnd5e:props:candles", at: [26,5], blocks_movement: false, blocks_los: false }`,
			`  - { ref: "dnd5e:props:candles", at: [26,5], blocks_movement: false, blocks_los: false,`+
				"\n      holds: [vault-map] }", 1)
		require.Empty(t, defectsIn(t, edited))
	})

	t.Run("a prop naming a record that does not exist is still refused", func(t *testing.T) {
		edited := strings.Replace(source, "holds: [hall-notes] }", "holds: [no-such-record] }", 1)
		requireDefect(t, defectsIn(t, edited), ".holds[0]",
			`holds intel "no-such-record"`, "no record in this dungeon")
	})

	t.Run("the SAME record may be held by two monsters — intel copies", func(t *testing.T) {
		edited := strings.Replace(source,
			`  - { ref: "dnd5e:monsters:skeleton", at: [11,3], targeting: lowest-health }`,
			`  - { ref: "dnd5e:monsters:skeleton", at: [11,3], targeting: lowest-health,`+
				"\n      holds: [vault-map] }", 1)
		require.Empty(t, defectsIn(t, edited),
			"two guards may both know the way in; looting either teaches it")
	})
}

// TestKnowsIsGone is R1: the deleted key is refused BY NAME, pointing at what
// replaced it — never as a bare unknown field, because the author who wrote
// it had a meaning and deserves to be told where it went.
func TestKnowsIsGone(t *testing.T) {
	source := heirloomSource(t)
	edited := strings.Replace(source, "holds: [vault-map] }", "knows: [vault] }", 1)

	_, err := dungeonspec.Decode([]byte(edited))
	require.Error(t, err)
	require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
	require.Contains(t, err.Error(), "`knows` is gone")
	require.Contains(t, err.Error(), "intel:", "the refusal names the replacement")
	require.Contains(t, err.Error(), "holds:")
}

// TestHoldableRefusals covers design §5's authoring rules.
func TestHoldableRefusals(t *testing.T) {
	source := heirloomSource(t)

	t.Run("a holdable prop with no id is refused", func(t *testing.T) {
		edited := strings.Replace(source, "  - { id: heirloom, ref:", "  - { ref:", 1)
		requireDefect(t, defectsIn(t, edited), ".id", "is holdable and has no id", "has to be nameable")
	})

	t.Run("holdable on a monster is refused", func(t *testing.T) {
		edited := strings.Replace(source, "    holds: [vault-map] }", "    holds: [vault-map], holdable: true }", 1)
		requireDefect(t, defectsIn(t, edited), ".holdable", "is not a prop and cannot be held")
	})

	t.Run("holdable FALSE on a monster is refused too", func(t *testing.T) {
		// The pointer is what makes this reachable: an authored `false` and
		// an omitted key are the same fact for a prop, and a monster that
		// wrote either has still declared something it cannot declare.
		edited := strings.Replace(source, "    holds: [vault-map] }", "    holds: [vault-map], holdable: false }", 1)
		requireDefect(t, defectsIn(t, edited), ".holdable", "is not a prop and cannot be held")
	})

	t.Run("a prop with an id and no holdable is ordinary scenery", func(t *testing.T) {
		edited := strings.Replace(source,
			`  - { ref: "dnd5e:props:candles", at: [26,5]`,
			`  - { id: candles, ref: "dnd5e:props:candles", at: [26,5]`, 1)
		require.Empty(t, defectsIn(t, edited))
		compiled, err := dungeonspec.Load([]byte(edited))
		require.NoError(t, err)
		for _, p := range compiled.Field.Props {
			if p.ID == "candles" {
				require.False(t, p.Holdable, "a thing nobody declared holdable stays scenery")
			}
		}
	})
}

// TestExitRefusals covers design §3.3's exit rules.
func TestExitRefusals(t *testing.T) {
	source := heirloomSource(t)

	t.Run("an exit off the floor is refused", func(t *testing.T) {
		edited := strings.Replace(source, "  - { id: entrance, at: [1, 3] }", "  - { id: entrance, at: [99, 99] }", 1)
		requireDefect(t, defectsIn(t, edited), "exits[0].at", "which is not floor")
	})

	t.Run("a duplicate exit id is refused, naming the earlier line", func(t *testing.T) {
		edited := strings.Replace(source, "  - { id: entrance, at: [1, 3] }",
			"  - { id: entrance, at: [1, 3] }\n  - { id: entrance, at: [2, 3] }", 1)
		requireDefect(t, defectsIn(t, edited), "exits[1].id", `exit "entrance" is already declared at exits[0]`)
	})

	t.Run("an exit with no id is refused", func(t *testing.T) {
		edited := strings.Replace(source, "  - { id: entrance, at: [1, 3] }", "  - { at: [1, 3] }", 1)
		requireDefect(t, defectsIn(t, edited), "exits[0].id", "the exit has no id")
	})

	t.Run("start is NOT implicitly an exit", func(t *testing.T) {
		// Nothing is defaulted. A dungeon that authors no exits compiles to
		// a dungeon with no way out declared, and says so by having none.
		edited := strings.Replace(source, "exits:\n  - { id: entrance, at: [1, 3] }\n", "", 1)
		edited = strings.Replace(edited, "    exit: entrance\n", "", 1)
		compiled, err := dungeonspec.Load([]byte(edited))
		require.NoError(t, err)
		require.Empty(t, compiled.Field.Exits)
	})
}

// TestTheUnchangedTombIsUnchanged is the claim the whole slice rests on: the
// plain reference tomb authors none of this and compiles to a dungeon
// carrying none of it. The heirloom fixture is a SECOND file, not an edit.
func TestTheUnchangedTombIsUnchanged(t *testing.T) {
	raw, err := os.ReadFile(tombPath)
	require.NoError(t, err)
	compiled, err := dungeonspec.Load(raw)
	require.NoError(t, err)

	require.Empty(t, compiled.Field.Exits)
	require.Nil(t, compiled.Scenarios)
	for _, p := range compiled.Field.Props {
		require.Empty(t, p.ID)
		require.False(t, p.Holdable)
	}
	for _, m := range compiled.Monsters {
		require.Empty(t, m.ID)
		require.Empty(t, m.Holds)
	}

	var bosses int
	for _, m := range compiled.Monsters {
		if m.Boss {
			bosses++
		}
	}
	require.Equal(t, 1, bosses, "design R8: the boss flag is untouched by this slice")
}
