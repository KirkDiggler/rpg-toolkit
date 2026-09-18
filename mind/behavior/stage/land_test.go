// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package stage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/stage"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// land_test.go is [stage.Land]'s own coverage, written when the ladder this
// package used to serve was retired (rpg-project#465).
//
// Land WAS exercised, thoroughly, by the module's scene tests — but always as
// the setup half of a scene whose assertions were about what a mind then
// decided. Those scenes went with the ladder, and the landing they leaned on
// would have gone untested with them. It is the one rule the deeds channel
// keeps (R9), and every `when: { <deed>: { within: N } }` condition an author
// writes reads what it wrote, so it is tested here on its own terms.

// eyesOn is the Reach a sight pass is measured by: a map of who can see whom.
// It is the caller's own physics, which is what perception leaves to it.
type eyesOn map[core.EntityID][]core.EntityID

func (e eyesOn) Reaches(_ perception.Channel, observer, subject core.EntityID) bool {
	for _, seen := range e[observer] {
		if seen == subject {
			return true
		}
	}

	return false
}

// look runs one complete sight pass, which is what makes a holding CURRENT —
// Report lands testimony and sustains nothing, so a sighting set up that way
// would be a ghost and would name nobody.
func look(t *testing.T, store *perception.Perception, who eyesOn, present []core.EntityID, at uint64) {
	t.Helper()

	presences := make([]perception.Presence, 0, len(present))
	for _, id := range present {
		presences = append(presences, perception.Presence{ID: id, Payload: []byte(`{"where":"yard"}`)})
	}
	observers := make([]core.EntityID, 0, len(who))
	for observer := range who {
		observers = append(observers, observer)
	}

	_, err := store.Observe(perception.Pass{
		At: at, Channel: perception.Sight, Presences: presences, Observers: observers, Reach: who,
	})
	require.NoError(t, err)
}

// heldDeed is the deed one witness holds about one actor, and whether it holds
// any at all.
func heldDeed(t *testing.T, store *perception.Perception, witness, actor core.EntityID) (deed.Deed, bool) {
	t.Helper()

	held, err := store.Held(witness)
	require.NoError(t, err)

	for _, h := range held {
		if h.Channel != deed.Channel || h.Subject != deed.Subject(actor) {
			continue
		}
		got, derr := deed.Decode(h.Payload)
		require.NoError(t, derr)

		return got, true
	}

	return deed.Deed{}, false
}

// A DEED LANDS IN EACH WITNESS'S OWN TERMS, and that is the whole rule.
//
// Three witnesses see one swing. The bystander can see both figures and learns
// the whole sentence. The one who can see nobody learns that an attack
// happened and not who did it — which is what keeps a dumb monster dumb and an
// illusion an illusion. And the target learns it was the target even though
// nobody perceives themselves, because you know when you have been shot at.
func TestALandedDeedNamesOnlyWhatEachWitnessCouldSee(t *testing.T) {
	store, err := perception.New()
	require.NoError(t, err)

	look(t, store, eyesOn{
		"bystander":   {"raider", "alice"},
		"blindfolded": nil,
		"alice":       {"raider"},
	}, []core.EntityID{"raider", "alice"}, 1)

	require.NoError(t, stage.Land(&stage.LandInput{
		Store:     store,
		Deed:      deed.Deed{Verb: "attack", Actor: "raider", Target: "alice", Where: "yard"},
		Witnesses: []core.EntityID{"bystander", "blindfolded", "alice"},
		At:        2,
	}))

	tests := []struct {
		name    string
		witness core.EntityID
		want    deed.Deed
		why     string
	}{
		{
			name: "a witness who could see both learns the whole sentence", witness: "bystander",
			want: deed.Deed{Verb: "attack", Actor: "raider", Target: "alice", Where: "yard"},
			why:  "it held both figures on sight when the blow landed",
		},
		{
			name: "a witness who could see neither learns only that it happened", witness: "blindfolded",
			want: deed.Deed{Verb: "attack", Where: "yard"},
			why: "the verb and the place are what reached it; naming the figures anyway is the " +
				"shortcut R9 exists to refuse",
		},
		{
			name: "the target knows it was the target", witness: "alice",
			want: deed.Deed{Verb: "attack", Actor: "raider", Target: "alice", Where: "yard"},
			why: "an observer never perceives itself, so alice holds no sight of alice — and " +
				"you know when you have been shot at",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, held := heldDeed(t, store, tc.witness, "raider")

			require.True(t, held, "every witness holds the deed; what differs is what it says")
			require.Equal(t, tc.want, got, tc.why)
		})
	}
}

// A deed is held and NEVER CURRENT: it is in the past the moment it exists,
// which is what lets a `within: N` condition age it instead of reading it as a
// thing still going on.
func TestALandedDeedIsHeldAndNeverCurrent(t *testing.T) {
	store, err := perception.New()
	require.NoError(t, err)

	require.NoError(t, stage.Land(&stage.LandInput{
		Store:     store,
		Deed:      deed.Deed{Verb: "intimidate", Actor: "alice", Target: "goblin", Where: "front"},
		Witnesses: []core.EntityID{"goblin"},
		At:        7,
	}))

	held, err := store.Held("goblin")
	require.NoError(t, err)
	require.Len(t, held, 1)
	require.Equal(t, deed.Channel, held[0].Channel)
	require.Equal(t, deed.Subject("alice"), held[0].Subject,
		"one subject per FIGURE, qualified by channel so it can never collide with a sight holding")
	require.Empty(t, held[0].CurrentVia, "nothing sustains a deed; it happened and it is over")
	require.Equal(t, uint64(7), held[0].Confirmed, "stamped when it happened, which is what ages it")
}

// A deed nobody did is a wiring fault, refused rather than landed: the actor
// keys the subject it is held under, so an actorless deed would land every
// such deed on one subject and overwrite the last.
func TestADeedWithNoActorIsRefused(t *testing.T) {
	store, err := perception.New()
	require.NoError(t, err)

	err = stage.Land(&stage.LandInput{
		Store:     store,
		Deed:      deed.Deed{Verb: "attack", Target: "alice", Where: "yard"},
		Witnesses: []core.EntityID{"alice"},
		At:        1,
	})

	require.ErrorIs(t, err, stage.ErrNoActor)

	held, herr := store.Held("alice")
	require.NoError(t, herr)
	require.Empty(t, held, "a refused landing writes nothing at all")
}

// Nobody saw it, so nobody holds it — and that is not an error. A deed with no
// witnesses is a thing that happened in an empty room.
func TestADeedNobodyWitnessedLandsNowhere(t *testing.T) {
	store, err := perception.New()
	require.NoError(t, err)

	require.NoError(t, stage.Land(&stage.LandInput{
		Store:     store,
		Deed:      deed.Deed{Verb: "attack", Actor: "raider", Target: "alice", Where: "yard"},
		Witnesses: nil,
		At:        1,
	}))

	held, herr := store.Held("alice")
	require.NoError(t, herr)
	require.Empty(t, held)
}
