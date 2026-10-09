package resolution

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/stretchr/testify/require"
)

func wrathHero(t *testing.T) *character.Data {
	t.Helper()
	hero := actionHero()
	f, err := features.CreateFromRef(&features.CreateFromRefInput{Ref: refs.Features.WrathOfTheStorm().String(), CharacterID: heroID})
	require.NoError(t, err)
	raw, err := f.Feature.ToJSON()
	require.NoError(t, err)
	hero.Features = []json.RawMessage{raw}
	hero.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{resources.WrathOfTheStorm: {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest}}
	hero.ActionEconomy = &character.ActionEconomyData{ReactionsRemaining: 1}
	return hero
}

func wolfStrikesHero(t *testing.T, hero *character.Data, machine Machine) (*Output, error) {
	t.Helper()
	return resolveOn(context.Background(), &Input{
		World:        actionWorld(t, 2),
		Participants: []Participant{{Monster: monsters.NewWolf(wolfID).ToData()}, {Character: hero}},
		Machine:      machine,
		Capabilities: encounter.Capabilities{Initiative: orderAsGiven{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Driver: passDriver{}, Roller: dice.NewRoller(), Actors: Actors},
	}, newSurface(events.NewEventBus()))
}

func TestPostHitChoicesKeepDescriptionAcrossFreeze(t *testing.T) {
	hero := wrathHero(t)
	hitRoller := func() *actionRoller { return &actionRoller{singles: []int{15}, damage: [][]int{{3}}} }
	out, err := wolfStrikesHero(t, hero, NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(), Roller: hitRoller()}))
	require.NoError(t, err)
	require.NotNil(t, out.Posed)
	require.NotNil(t, out.Posed.SettledStrike, "precondition: the post-hit reaction posed")

	ask := out.Posed.Ask
	require.NotEmpty(t, ask.Offer.Description, "the offer carries the feature's prose")
	require.Len(t, ask.Choices, 2)
	for _, choice := range ask.Choices {
		require.NotEmpty(t, choice.Description, choice.ID)
	}

	// Before and after the serialized freeze: the pose itself and the frozen payload.
	var frozen frozenStrike
	require.NoError(t, json.Unmarshal(out.Posed.Frozen, &frozen))
	require.NotNil(t, frozen.PostHit)
	require.Len(t, frozen.PostHit.Options, len(ask.Choices))
	require.Equal(t, frozen.PostHit.Description, ask.Offer.Description)
	for i, option := range frozen.PostHit.Options {
		require.Equal(t, option.ID, ask.Choices[i].ID)
		require.Equal(t, option.Description, ask.Choices[i].Description)
	}
	wire, err := json.Marshal(ask)
	require.NoError(t, err)
	var thawed Ask
	require.NoError(t, json.Unmarshal(wire, &thawed))
	for i := range ask.Choices {
		require.Equal(t, ask.Choices[i].Description, thawed.Choices[i].Description)
	}

	// Resuming reads frozen mechanics, not prose: rewriting every description
	// in the payload resumes to the same outcome.
	resume := func(payload []byte) string {
		machine, rerr := NewStrikeResumed(&StrikeResumeInput{Frozen: payload, Answer: OfferSpend, Option: "lightning", Roller: &actionRoller{damage: [][]int{{5, 5}}, singles: []int{20}}})
		require.NoError(t, rerr)
		resumed, rerr := wolfStrikesHero(t, wrathHero(t), machine)
		require.NoError(t, rerr)
		require.NotNil(t, resumed.Outcome.(StrikeOutcome).Retaliation, "the reaction resolved")
		encoded, rerr := json.Marshal(resumed.Outcome)
		require.NoError(t, rerr)
		return string(encoded)
	}
	original := resume(out.Posed.Frozen)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(out.Posed.Frozen, &raw))
	rewritten, err := json.Marshal(raw)
	require.NoError(t, err)
	rewritten = []byte(strings.ReplaceAll(string(rewritten), ask.Choices[0].Description, "different words"))
	require.NotEqual(t, string(out.Posed.Frozen), string(rewritten), "precondition: the prose was actually rewritten")
	// The outcome echoes the offer it resumed from, so the rewritten words
	// come back as they went in. Everything else is identical.
	after := strings.ReplaceAll(resume(rewritten), "different words", ask.Choices[0].Description)
	require.Equal(t, original, after)
}

func TestBeforeRollOfferKeepsDescription(t *testing.T) {
	roller := &actionRoller{singles: []int{15}, pairs: [][]int{{15, 2}}, damage: [][]int{{3}}}
	out, err := wolfStrikesHero(t, flareHero(t), NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(), Roller: roller}))
	require.NoError(t, err)
	require.NotNil(t, out.Posed)
	require.True(t, out.Posed.BeforeRoll)

	require.NotEmpty(t, out.Posed.Ask.Offer.Description)
	var frozen frozenStrike
	require.NoError(t, json.Unmarshal(out.Posed.Frozen, &frozen))
	require.NotNil(t, frozen.BeforeRoll)
	require.Equal(t, frozen.BeforeRoll.Description, out.Posed.Ask.Offer.Description)
	require.Empty(t, out.Posed.Ask.Choices[0].Description, "the single Use choice carries no prose of its own")
}

func TestPostRollOfferKeepsDescription(t *testing.T) {
	out, err := heroSwings(t, inspiredHero(t), &actionRoller{singles: []int{11}})
	require.NoError(t, err)
	require.NotNil(t, out.Posed)

	require.Equal(t, conditions.InspiredOfferDescription, out.Posed.Ask.Offer.Description)
	var frozen map[string]any
	require.NoError(t, json.Unmarshal(out.Posed.Frozen, &frozen))
	require.Contains(t, string(out.Posed.Frozen), conditions.InspiredOfferDescription)
}
