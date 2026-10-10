// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// goodWindow is a window this build writes: a post-hit pause asked of the
// fighter, behind an attack story.
func goodWindow() pendingWindow {
	return pendingWindow{
		Version:  windowVersion,
		Kind:     resolution.PausePostHit,
		Audience: "fighter",
		Pause: resolution.Pause{
			Kind: resolution.PausePostHit,
			Ask: resolution.Ask{Audience: "fighter", Offer: resolution.Offer{
				Ref: *refs.Features.WrathOfTheStorm(), Name: "Wrath of the Storm",
				Choices: []resolution.Choice{{ID: "thunder", Label: "Thunder"}},
			}},
			Frozen: []byte(`{"v":4}`),
		},
		Story: windowStory{
			Kind: storyAttack, Attacker: "goblin", Target: "fighter",
			Definition: combatActions.Definition{Ref: core.Ref{Module: "dnd5e", Type: "weapons", ID: "scimitar"}},
		},
	}
}

// TestThawWindowRefusesWhatThisBuildDidNotWrite holds thawWindow's trust
// boundary, one refusal per row, each against an otherwise good window.
func TestThawWindowRefusesWhatThisBuildDidNotWrite(t *testing.T) {
	raw, err := json.Marshal(goodWindow())
	require.NoError(t, err)
	_, err = thawWindow(raw, "fighter")
	require.NoError(t, err, "control: the good window thaws")

	for _, tc := range []struct {
		name   string
		edit   func(*pendingWindow)
		posed  string
		refuse error
	}{
		{"another version", func(w *pendingWindow) { w.Version = 2 }, "fighter", ErrStalePause},
		{"posed to somebody else", func(*pendingWindow) {}, "cleric", ErrInvalidSession},
		{"asks somebody the window does not", func(w *pendingWindow) { w.Pause.Ask.Audience = "cleric" }, "fighter", ErrInvalidSession},
		{"a kind that is not the pause's", func(w *pendingWindow) { w.Kind = resolution.PausePostRoll }, "fighter", ErrInvalidSession},
		{"an offer with no ref", func(w *pendingWindow) { w.Pause.Ask.Offer.Ref = core.Ref{} }, "fighter", ErrInvalidSession},
		{"an offer with no name", func(w *pendingWindow) { w.Pause.Ask.Offer.Name = "" }, "fighter", ErrInvalidSession},
		{"an empty choice id", func(w *pendingWindow) { w.Pause.Ask.Offer.Choices = []resolution.Choice{{ID: ""}} }, "fighter", ErrInvalidSession},
		{"a repeated choice id", func(w *pendingWindow) {
			w.Pause.Ask.Offer.Choices = []resolution.Choice{{ID: "a"}, {ID: "a"}}
		}, "fighter", ErrInvalidSession},
		{"no frozen machine", func(w *pendingWindow) { w.Pause.Frozen = nil }, "fighter", ErrInvalidSession},
		{"an attack story with no target", func(w *pendingWindow) { w.Story.Target = "" }, "fighter", ErrInvalidSession},
		{"a step story with no mover", func(w *pendingWindow) { w.Story = windowStory{Kind: storyStep} }, "fighter", ErrInvalidSession},
		{"a cast story with no spell", func(w *pendingWindow) { w.Story = windowStory{Kind: storyCast, Caster: "bard"} }, "fighter", ErrInvalidSession},
		{"a check story finishing nothing", func(w *pendingWindow) { w.Story = windowStory{Kind: storyCheck} }, "fighter", ErrInvalidSession},
		{"a check story on a target with no social verb", func(w *pendingWindow) {
			w.Story = windowStory{Kind: storyCheck, Target: "goblin"}
		}, "fighter", ErrInvalidSession},
		{"a check story naming a door and a verb", func(w *pendingWindow) {
			w.Story = windowStory{Kind: storyCheck, Door: "gate", Verb: VerbPersuade}
		}, "fighter", ErrInvalidSession},
		{"a story kind nobody tells", func(w *pendingWindow) { w.Story.Kind = "dance" }, "fighter", ErrInvalidSession},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := goodWindow()
			tc.edit(&w)
			raw, err := json.Marshal(w)
			require.NoError(t, err)
			_, err = thawWindow(raw, tc.posed)
			require.ErrorIs(t, err, tc.refuse)
		})
	}

	_, err = thawWindow([]byte(`not json`), "fighter")
	require.ErrorIs(t, err, ErrInvalidSession, "an undecodable payload")
}

// TestPoseWindowWritesNothingItWouldRefuse: the only writer reads its own
// output back, so a story missing what its kind needs is refused before
// anything reaches the ledger.
func (s *LandSuite) TestPoseWindowWritesNothingItWouldRefuse() {
	w := goodWindow()
	w.Story.Target = ""
	err := poseWindow(s.scope, w.Pause, w.Story)
	s.Require().ErrorIs(err, ErrInvalidSession)
	open, err := s.scope.ledger.Open()
	s.Require().NoError(err)
	s.Empty(open, "nothing was posed")

	good := goodWindow()
	s.Require().NoError(poseWindow(s.scope, good.Pause, good.Story), "control: a good window poses")
	open, err = s.scope.ledger.Open()
	s.Require().NoError(err)
	s.Len(open, 1)
}

// TestAPausedCastThatToldSomethingIsRefused: a cast is one told unit and tells
// nothing at its pause, so a paused cast output carrying an outcome is a
// provider defect, refused at the record step rather than told twice.
func (s *LandSuite) TestAPausedCastThatToldSomethingIsRefused() {
	good := goodWindow()
	pause := good.Pause
	pause.Kind = resolution.PauseSaveRoll
	out := &resolution.Output{World: s.scope.enc.ToData(), Outcome: resolution.CastOutcome{}, Posed: &pause}
	_, err := s.mgr.poseCastWindow(context.Background(), s.scope, "fighter",
		SpellRef{Ref: refs.Spells.Bane().String(), Name: "Bane"}, nil, out)
	s.Require().ErrorIs(err, ErrInvalidWorld)
	s.Zero(s.encounters.saves, "nothing commits")
}
