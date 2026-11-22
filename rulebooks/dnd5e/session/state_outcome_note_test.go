// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// state_outcome_note_test.go is what a launched game carries of a creature's
// machine (rpg-project#989, rpg-project#1106): a state's `outcomes` are bound
// beside its blocking states and posts, so a creature notes the one it heard
// when its machine enters the state, and a line may require a topic only a
// machine gives.
//
// The site is a neutral creature beside alice's start. Its machine is `calm`,
// and on `heard` when the claim `failed` it goes to `wary`, which gives the
// note `lied` to itself, about the one named. The `again` line REQUIRES
// `noted: lied`, so it is offered only after the note is taken.

const stateOutcomeSite = `
version: 5
key: state-outcome-site
play: { void: transparent, lighting: bright, standing: centre-covered }
factions:
  - id: townsfolk
dispositions:
  - { between: [townsfolk, party], stance: neutral }
behaviorDefinitions:
  watcher-policy:
    recipe:
      ref: site:behaviors:watcher-policy
      initial: calm
      states:
        calm:
          transitions:
            - on: heard
              when: { fact: { name: heard.claim, op: eq, value: failed } }
              to: wary
              reason: lied to
        wary: {}
    outcomes:
      wary:
        - give:
            topic: lied
            to: itself
            about: the_one_named
room:
  version: 3
  id: room-1
  name: A Room
  coordinateFrame: { hexRadius: 1 }
  scene:
    version: 1
    id: scene-1
    name: A Room
    items:
      - { id: table, kind: prop, transform: { x: 0, y: 0, z: 2, rotationY: 0 } }
    groups: []
  room:
    implicitRegionId: room-1-region
    regions:
      - id: room-1-region
        name: A Room
        lighting: { intensity: 1 }
        cells:
          - r: 0
            q: [[0, 2]]
          - r: 1
            q: [[0, 1]]
    propDeclarations:
      table: { blocksMovement: false, blocksLineOfSight: false, footprint: { width: 1, depth: 1, offsetX: 0, offsetZ: 0 } }
    arrangementDeclarations:
      nook:
        stamped-chair: { blocksMovement: false, blocksLineOfSight: false, footprint: { width: 1, depth: 1, offsetX: 0, offsetZ: 0 } }
    partyStart: { q: 0, r: 0 }
    monsterDeclarations:
      - { id: watcher, ref: 'dnd5e:monsters:goblin', startingCell: { location: { q: 2, r: 0 } } }
    monsterBindings:
      watcher:
        faction: townsfolk
        behaviorState: policy
        behaviors: [{ id: policy, use: watcher-policy }]
        options:
          - { id: claim, say: "We're expected.", check: [{ ability: persuasion, dc: 10 }], failure: [{ say: Liar. }] }
          - { id: again, say: "Again.", requires: { noted: lied } }
`

// StateOutcomeNote is the launched site and the stores it saves into, so a
// second Manager over them is a reload of everything saved.
type stateOutcomeNote struct {
	t          *testing.T
	compiled   dungeonspec.Compiled
	characters *fakeCharacters
	sessions   *fakeSessions
	encounters *fakeEncounters
}

const stateOutcomeFailedClaim = 2 // alice's Persuasion is -1: 1 against DC 10, a miss

func newStateOutcomeNote(t *testing.T) *stateOutcomeNote {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(stateOutcomeSite))
	require.NoError(t, err, "the site compiles")
	return &stateOutcomeNote{t: t, compiled: compiled, characters: newFakeCharacters(armedFighter("alice")),
		sessions: newFakeSessions(), encounters: newFakeEncounters()}
}

func (n *stateOutcomeNote) manager(rolls ...int) *session.Manager {
	n.t.Helper()
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &d20Script{faces: rolls},
		TurnDriver: session.Pass{}, Sessions: n.sessions, Encounters: n.encounters, Characters: n.characters,
	})
	require.NoError(n.t, err)
	return mgr
}

func (n *stateOutcomeNote) launch(mgr *session.Manager) error {
	_, err := mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &n.compiled, Party: []string{"alice"}})
	return err
}

// offers is whether alice is offered the line, read through the session's own
// read of what a creature offers her.
func (n *stateOutcomeNote) offers(mgr *session.Manager, option string) bool {
	n.t.Helper()
	panel, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(n.t, err)
	for _, d := range panel.Declarations {
		for _, row := range d.SpeakOptions {
			if row.Target == "watcher" && row.OptionID == option {
				return true
			}
		}
	}
	return false
}

func (n *stateOutcomeNote) say(mgr *session.Manager, option string) {
	n.t.Helper()
	_, err := speakAsOffered(context.Background(), mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "watcher", OptionID: option})
	require.NoError(n.t, err)
}

// Catches the outcomes dropped between the compiled creature and the engine:
// the claim fails, the machine enters `wary`, and the creature never notes
// `lied`, so the line requiring it stays unoffered, here and after a reload.
func TestALaunchedCreatureNotesTheOneItHeardOnEnteringAState(t *testing.T) {
	n := newStateOutcomeNote(t)
	mgr := n.manager(stateOutcomeFailedClaim)
	require.NoError(t, n.launch(mgr))
	require.True(t, n.offers(mgr, "claim"), "precondition: the claim is offered")
	require.False(t, n.offers(mgr, "again"), "before the claim the creature has noted nothing")

	n.say(mgr, "claim")
	require.True(t, n.offers(mgr, "again"), "after the failed claim it has noted her lie")

	reloaded := n.manager()
	require.True(t, n.offers(reloaded, "again"), "the note survived the save")
}

// Catches a launch refused because a line requires a topic that only a
// machine's state gives and the engine was never told the state gives it.
func TestALaunchedSiteMayRequireATopicOnlyAMachineGives(t *testing.T) {
	n := newStateOutcomeNote(t)
	require.NoError(t, n.launch(n.manager()))
}
