// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// states_test.go pins A CREATURE'S STATES (rpg-project#766 S4, states.go):
// the words compile to the machine's recipe, layer nearest-wins like a
// table, make the keys they pick legal table keys, and are refused at the
// author's own path when this build could not run them.

import (
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// statesRoot is a root with the garrison's states and a table with a `march`
// key beside `time`.
const statesRoot = `tables:
  drill:
    time:
      - { hold: {} }
    march:
      - { when: { enemy: none }, toward: { at: { q: 0, r: 1 } } }
factions:
  - id: garrison
    states:
      initial: waiting
      waiting:
        transitions:
          - { on: time, do: [{ pick: time }] }
          - { on: clock:fight, to: marching, do: [{ pick: march }] }
      marching:
        transitions:
          - { on: clock:world, to: waiting }`

// statesBindings binds both goblins to the garrison; goblin-1 writes its own
// orders and states (and so does not take the drill, whose `march` its
// states never pick), goblin-2 takes the drill and inherits the faction's
// states, which pick `march`.
const statesBindings = `    monsterBindings:
      goblin-1:
        faction: garrison
        on: { time: [{ hold: {} }] }
        states:
          initial: posted
          posted:
            transitions:
              - { on: heard, within: 6, to: alert, do: [{ anchor: true }], reason: heard something }
          alert:
            transitions:
              - { on: time, rounds: 3, to: posted }
              - { on: time, when: { enemy: seen }, do: [{ pick: time }] }
              - { on: deed:attacked, regions: 1, do: [{ wake_in: 2 }] }
      goblin-2:
        faction: garrison
        table: drill`

func ref(t *testing.T, s string) core.Ref {
	t.Helper()
	r, err := core.ParseString(s)
	require.NoError(t, err)

	return *r
}

// TestDungeonspec_StatesCompile: each creature's states compile to the recipe
// the encounter runs — a scope, a rounds span and a `when` folded into one
// guard, the acts verbatim — the binding's own winning wholesale over its
// faction's, and the faction's recipe for the creature that wrote none. The
// `march` key the faction's states pick is a legal table key, and the
// encounter accepts both recipes at its own door.
func TestDungeonspec_StatesCompile(t *testing.T) {
	compiled := loadSource(t, v4With(statesRoot, statesBindings))

	own := monsterNamed(t, compiled, "goblin-1")
	require.Equal(t, &machine.Recipe{
		Ref: ref(t, "dnd5e:machines:goblin-1"), Initial: "posted",
		States: map[string]machine.State{
			"posted": {Transitions: []machine.Transition{{
				On: "heard", When: &machine.Guard{Scope: &machine.ScopeLeaf{Within: 6}},
				To: "alert", Do: []machine.Act{{Anchor: true}}, Reason: "heard something",
			}}},
			"alert": {Transitions: []machine.Transition{
				{On: "time", When: &machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: 3}}, To: "posted"},
				{On: "time", When: &machine.Guard{Fact: &machine.FactLeaf{
					Name: encounter.MachineEnemyFact, Op: machine.OpEq, Value: "seen",
				}}, Do: []machine.Act{{Pick: "time"}}},
				{On: "deed:attacked", When: &machine.Guard{Scope: &machine.ScopeLeaf{Regions: 1}}, Do: []machine.Act{{WakeIn: 2}}},
			}},
		},
	}, own.Machine, "the binding's own states, compiled")

	inherited := monsterNamed(t, compiled, "goblin-2")
	require.NotNil(t, inherited.Machine)
	require.Equal(t, ref(t, "dnd5e:machines:garrison"), inherited.Machine.Ref, "the faction's states, for the creature that wrote none")
	require.Equal(t, "waiting", inherited.Machine.Initial)
	require.Contains(t, inherited.Table, encounter.AnswerKey("march"), "the key its states pick is on its table")

	// THROUGH THE DOOR: what the dialect accepts, the encounter seats — each
	// compiled table with its compiled machine, judged per member.
	members := []encounter.MemberInput{{ID: "hero", Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Machine: m.Machine,
		})
	}
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        compiled.Field,
		Members:      members,
		Endings:      append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention:    encounter.RetentionUnbounded,
		Capabilities: facingCapabilities(),
	})
	require.NoError(t, err, "the door seats every creature the dialect compiled")
}

// TestDungeonspec_StatesRefusals: each defect at its own path, in a sentence
// that says what there is.
func TestDungeonspec_StatesRefusals(t *testing.T) {
	binding := func(states string) string {
		return "    monsterBindings:\n      goblin-1:\n        states:\n" + states
	}
	at := "room.room.monsterBindings.goblin-1.states"
	for _, tc := range []struct {
		name, states, path string
		words              []string
	}{
		{
			name:   "an unknown beat",
			states: "          initial: a\n          a: { transitions: [{ on: hear, to: a }] }\n",
			path:   at + ".a.transitions[0].on", words: []string{`"hear" is not a beat a state listens for`, "heard"},
		},
		{
			name:   "an unknown state",
			states: "          initial: a\n          a: { transitions: [{ on: time, to: b }] }\n",
			path:   at + ".a.transitions[0].to", words: []string{`"b" is not a state this block declares`},
		},
		{
			name:   "an initial it lacks",
			states: "          initial: b\n          a: { transitions: [{ on: time }] }\n",
			path:   at + ".initial", words: []string{`"b" is not a state this block declares`},
		},
		{
			name:   "within on a fact",
			states: "          initial: a\n          a: { transitions: [{ on: fact, within: 2, to: a }] }\n",
			path:   at + ".a.transitions[0].within", words: []string{"`fact` happened nowhere"},
		},
		{
			name:   "a deed as a state's when",
			states: "          initial: a\n          a: { transitions: [{ on: time, when: { attacked: { within: 2 } }, to: a }] }\n",
			path:   at + ".a.transitions[0].when", words: []string{"a state's `when` is one of { enemy }, { fact } or { not }"},
		},
		{
			name:   "an act that is two",
			states: "          initial: a\n          a: { transitions: [{ on: time, do: [{ pick: time, wake_in: 2 }] }] }\n",
			path:   at + ".a.transitions[0].do[0]", words: []string{"an act is one of"},
		},
		{
			name:   "holds on a terminal state",
			states: "          initial: a\n          a: { transitions: [{ on: time, to: b }] }\n          b: { terminal: true, holds: { door: gate } }\n",
			path:   at + ".b.holds", words: []string{"a terminal state ends the machine and holds no post"},
		},
		{
			name:   "holds a door the document lacks",
			states: "          initial: a\n          a: { holds: { door: postern }, transitions: [{ on: time, do: [{ pick: time }] }] }\n",
			path:   at + ".a.holds.door", words: []string{`"postern" is not a door this document declares`},
		},
		{
			name:   "holds naming no door",
			states: "          initial: a\n          a: { holds: { at: [{ q: 0, r: 0 }] }, transitions: [{ on: time, do: [{ pick: time }] }] }\n",
			path:   at + ".a.holds.door", words: []string{"a post names the door whose opening it holds"},
		},
		{
			name:   "holds from a cell that is not floor",
			states: "          initial: a\n          a: { holds: { door: postern, at: [{ q: 9, r: 9 }] }, transitions: [{ on: time, do: [{ pick: time }] }] }\n",
			path:   at + ".a.holds.at[0]", words: []string{"{q: 9, r: 9}", "not floor"},
		},
		{
			name:   "blocks on a terminal state",
			states: "          initial: a\n          a: { transitions: [{ on: time, to: b }] }\n          b: { terminal: true, blocks: true }\n",
			path:   at + ".b.blocks", words: []string{"a terminal state ends the machine"},
		},
		{
			name:   "a pick of a verdict",
			states: "          initial: a\n          a: { transitions: [{ on: time, do: [{ pick: success }] }] }\n",
			path:   at, words: []string{`pick "success" is not a key`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireRefusedAt(t, refusals(t, v4With("", binding(tc.states))), tc.path, tc.words...)
		})
	}

	// PER CREATURE, AT ITS OWN PATH: the drill's `march` is picked by the
	// faction's states, and goblin-1's own states win and never pick it, so
	// goblin-1 taking the drill is refused where it took it — the door would
	// refuse to seat it.
	t.Run("a table key this creature's states never pick", func(t *testing.T) {
		requireExactDefect(t, refusals(t, v4With(statesRoot, `    monsterBindings:
      goblin-1:
        faction: garrison
        table: drill
        states:
          initial: posted
          posted:
            transitions:
              - { on: time, do: [{ pick: time }] }
      goblin-2:
        faction: garrison
        table: drill`)),
			"room.room.monsterBindings.goblin-1",
			`this creature's table carries "march" and the states it runs never pick it: they pick time`)
	})

	t.Run("a key no state picks is still not a trigger", func(t *testing.T) {
		requireExactDefect(t, refusals(t, v4With("tables:\n  drill:\n    march:\n      - { hold: {} }", "")),
			"tables.drill.on.march", `"march" is not a trigger this build rolls: they are time`)
	})
}

// TestDungeonspec_BlocksCompilesToBlockingStates (rpg-project#836, C1): a
// state written `blocks: true` is one the creature's body blocks in. The
// compile collects them, in name order, beside the recipe they name, for the
// creature whose states win; a creature with no blocking state carries none,
// and the encounter's door seats what the dialect compiled.
func TestDungeonspec_BlocksCompilesToBlockingStates(t *testing.T) {
	compiled := loadSource(t, v4With("", `    monsterBindings:
      goblin-1:
        on: { time: [{ hold: {} }] }
        states:
          initial: posted
          posted:
            blocks: true
            transitions:
              - { on: fact, when: { fact: gate-cowed }, to: cowed }
              - { on: heard, within: 6, to: alert }
          alert:
            blocks: true
            transitions:
              - { on: time, rounds: 3, to: posted }
          cowed:
            transitions:
              - { on: time, do: [{ pick: time }] }`))

	posted := monsterNamed(t, compiled, "goblin-1")
	require.Equal(t, []string{"alert", "posted"}, posted.BlockingStates, "every state written blocks, in name order")
	require.Empty(t, monsterNamed(t, compiled, "goblin-2").BlockingStates, "a creature with no states blocks as it is declared")

	members := []encounter.MemberInput{{ID: "hero", Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Machine: m.Machine, BlockingStates: m.BlockingStates, Posts: m.Posts,
		})
	}
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        compiled.Field,
		Members:      members,
		Endings:      append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention:    encounter.RetentionUnbounded,
		Capabilities: facingCapabilities(),
	})
	require.NoError(t, err, "the door seats the blocking states the dialect compiled")
}

// TestDungeonspec_HoldsCompilesToPosts (rpg-project#852): a state written
// `holds: { door, at }` compiles to the encounter's post, its door qualified
// to `<key>/<id>` and its cells the axial cells the author wrote. The
// castle's gate post holds the gate door from both gatehouse-side cells of
// the opening while posted or alert; cowed and turned hold nothing, and the
// other gate guard holds no post.
func TestDungeonspec_HoldsCompilesToPosts(t *testing.T) {
	raw, err := os.ReadFile("testdata/world-builder-v4-castle.yaml")
	require.NoError(t, err)
	compiled := loadSource(t, string(raw))

	gate := encounter.Post{
		Door:  "the-mad-king/gate-wall-door",
		Cells: []spatial.Position{{X: -3, Y: 7}, {X: -4, Y: 7}},
	}
	require.Equal(t, map[string]encounter.Post{"posted": gate, "alert": gate}, monsterNamed(t, compiled, "gate-1").Posts,
		"posted and alert hold the gate; cowed and turned hold nothing")
	require.Nil(t, monsterNamed(t, compiled, "gate-2").Posts, "the other gate guard holds no post")
	require.True(t, slices.ContainsFunc(compiled.Field.Doors, func(d encounter.DoorInput) bool { return d.ID == gate.Door }),
		"the qualified id is the door the field compiles")
}
