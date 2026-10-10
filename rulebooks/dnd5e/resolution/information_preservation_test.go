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
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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
	require.Equal(t, PausePostHit, out.Posed.Kind, "precondition: the post-hit reaction posed")

	ask := out.Posed.Ask
	require.NotEmpty(t, ask.Offer.Description, "the offer carries the feature's prose")
	require.Len(t, ask.Offer.Choices, 2)
	for _, choice := range ask.Offer.Choices {
		require.NotEmpty(t, choice.Description, choice.ID)
	}

	// Before and after the serialized freeze: the pause itself and the frozen payload.
	h, err := readFrozen(out.Posed.Frozen)
	require.NoError(t, err)
	var frozen frozenPostHit
	require.NoError(t, decodeState(h, &frozen))
	require.Len(t, frozen.Offer.Options, len(ask.Offer.Choices))
	require.Equal(t, frozen.Offer.Description, ask.Offer.Description)
	for i, option := range frozen.Offer.Options {
		require.Equal(t, option.ID, ask.Offer.Choices[i].ID)
		require.Equal(t, option.Description, ask.Offer.Choices[i].Description)
	}
	wire, err := json.Marshal(ask)
	require.NoError(t, err)
	var thawed Ask
	require.NoError(t, json.Unmarshal(wire, &thawed))
	for i := range ask.Offer.Choices {
		require.Equal(t, ask.Offer.Choices[i].Description, thawed.Offer.Choices[i].Description)
	}

	// Resuming reads frozen mechanics, not prose: rewriting every description
	// in the payload (the offer's own and each option's) resumes to the same
	// outcome. Only the echoed offer, Retaliation.Offer, may differ; it is
	// neutralised and everything else is compared as it came back.
	resume := func(payload []byte) string {
		pause := *out.Posed
		pause.Frozen = payload
		machine, rerr := Resume(&ResumeInput{Pause: pause, Answer: Take("lightning"), Roller: &actionRoller{damage: [][]int{{5, 5}}, singles: []int{20}}})
		require.NoError(t, rerr)
		resumed, rerr := wolfStrikesHero(t, wrathHero(t), machine)
		require.NoError(t, rerr)
		outcome := resumed.Outcome.(StrikeOutcome)
		require.NotNil(t, outcome.Retaliation, "the reaction resolved")
		retaliation := *outcome.Retaliation
		retaliation.Offer = dnd5eEvents.PostHitOffer{}
		outcome.Retaliation = &retaliation
		encoded, rerr := json.Marshal(outcome)
		require.NoError(t, rerr)
		return string(encoded)
	}
	original := resume(out.Posed.Frozen)
	rewritten := string(out.Posed.Frozen)
	descriptions := []string{ask.Offer.Description}
	for _, choice := range ask.Offer.Choices {
		descriptions = append(descriptions, choice.Description)
	}
	for i, words := range descriptions {
		quoted, merr := json.Marshal(words)
		require.NoError(t, merr)
		replacement, merr := json.Marshal("rewritten prose " + string(rune('a'+i)))
		require.NoError(t, merr)
		require.Contains(t, rewritten, string(quoted), "precondition: the prose is in the frozen payload")
		rewritten = strings.ReplaceAll(rewritten, string(quoted), string(replacement))
	}
	require.Equal(t, original, resume([]byte(rewritten)))
}

func TestBeforeRollOfferKeepsDescription(t *testing.T) {
	roller := &actionRoller{singles: []int{15}, pairs: [][]int{{15, 2}}, damage: [][]int{{3}}}
	out, err := wolfStrikesHero(t, flareHero(t), NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(), Roller: roller}))
	require.NoError(t, err)
	require.NotNil(t, out.Posed)
	require.Equal(t, PauseBeforeRoll, out.Posed.Kind)

	require.NotEmpty(t, out.Posed.Ask.Offer.Description)
	h, err := readFrozen(out.Posed.Frozen)
	require.NoError(t, err)
	var frozen frozenBeforeRoll
	require.NoError(t, decodeState(h, &frozen))
	require.Equal(t, frozen.Offer.Description, out.Posed.Ask.Offer.Description)
	require.Empty(t, out.Posed.Ask.Offer.Choices[0].Description, "the single Use choice carries no prose of its own")
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
