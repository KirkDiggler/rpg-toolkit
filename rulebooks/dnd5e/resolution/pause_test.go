// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// postRollPause is a live post-roll pause: the inspired hero's swing, stopped
// on its own die.
func postRollPause(t *testing.T) Pause {
	t.Helper()
	out, err := heroSwings(t, inspiredHero(t), &actionRoller{singles: []int{11}})
	require.NoError(t, err)
	require.NotNil(t, out.Posed)
	return *out.Posed
}

// TestAnAnswerTheOfferDoesNotAcceptIsRefused: the zero Answer, Decline with an
// option, Take with an unlisted option, and Take with an option on a
// choiceless offer are each ErrNotOffered — through the answer's own check and
// through Resume, before anything is charged.
func TestAnAnswerTheOfferDoesNotAcceptIsRefused(t *testing.T) {
	choiceless := Offer{Name: "Bardic Inspiration"}
	listed := Offer{Name: "Wrath of the Storm", Choices: []Choice{{ID: "lightning"}, {ID: "thunder"}}}

	for name, tc := range map[string]struct {
		answer Answer
		offer  Offer
	}{
		"the zero Answer":                       {Answer{}, choiceless},
		"Decline with an option":                {Answer{given: true, option: "thunder"}, listed},
		"Take with an unlisted option":          {Take("acid"), listed},
		"Take with no option on a listed offer": {Take(""), listed},
		"Take with an option on a choiceless":   {Take("lightning"), choiceless},
	} {
		t.Run(name, func(t *testing.T) {
			require.ErrorIs(t, tc.answer.accepts(tc.offer), ErrNotOffered)
		})
	}

	for name, answer := range map[string]Answer{
		"Take on its choice":         Take("lightning"),
		"Decline":                    Decline(),
		"Take on a choiceless offer": Take(""),
	} {
		t.Run("accepted: "+name, func(t *testing.T) {
			offer := listed
			if answer.Option() == "" {
				offer = choiceless
			}
			require.NoError(t, answer.accepts(offer))
		})
	}

	pause := postRollPause(t)
	for name, answer := range map[string]Answer{
		"the zero Answer":                     {},
		"Take with an option on a choiceless": Take("counterspell"),
	} {
		t.Run("through Resume: "+name, func(t *testing.T) {
			machine, err := Resume(&ResumeInput{Pause: pause, Answer: answer, Roller: dice.NewRoller()})
			require.ErrorIs(t, err, ErrNotOffered)
			require.Nil(t, machine)
		})
	}
}

// TestAStaleHeaderIsRefusedBeforeAnythingLoads: a header written with an
// earlier version, and a verbatim pre-E frozen strike with no header at all,
// each refuse with ErrStalePause from Resume. No machine comes back, so no
// world is loaded and no sheet can be dirtied.
func TestAStaleHeaderIsRefusedBeforeAnythingLoads(t *testing.T) {
	pause := postRollPause(t)
	h, err := readFrozen(pause.Frozen)
	require.NoError(t, err)

	versionThree, err := json.Marshal(frozenHeader{V: 3, Machine: h.Machine, Kind: h.Kind, State: h.State})
	require.NoError(t, err)

	// The shape the strike wrote before the envelope: its own kind and
	// version fields inline, no header.
	preE := []byte(`{"kind":"strike.post_roll","version":3,"attacker_id":"hero","target_id":"wolf",` +
		`"roll":11,"total":15,"offer":{"Audience":"hero","Die":"1d6"}}`)

	for name, frozen := range map[string][]byte{"a version-3 header": versionThree, "a pre-E frozen strike": preE} {
		t.Run(name, func(t *testing.T) {
			stale := pause
			stale.Frozen = frozen
			machine, err := Resume(&ResumeInput{Pause: stale, Answer: Take(""), Roller: dice.NewRoller()})
			require.ErrorIs(t, err, ErrStalePause)
			require.Nil(t, machine, "nothing to resolve, so nothing loads and nothing is charged")
		})
	}
}

// monsterCast is a cast holding one loaded wolf, for the door alone.
func monsterCast(t *testing.T) (*Participants, *monster.Monster) {
	t.Helper()
	wolf, err := monster.LoadFromData(context.Background(), monsters.NewWolf(wolfID).ToData(), events.NewEventBus())
	require.NoError(t, err)
	return &Participants{
		characters: map[string]*character.Character{},
		monsters:   map[string]*monster.Monster{wolfID: wolf},
		order:      []string{wolfID},
	}, wolf
}

// TestTheDoorRefusesAMonsterPriceThatIsNotOneReaction: a monster can be
// charged its one reaction and nothing else. Any other price is ErrNoPayer,
// and the reaction is left where it was.
func TestTheDoorRefusesAMonsterPriceThatIsNotOneReaction(t *testing.T) {
	for name, profile := range map[string]*combat.SpendProfile{
		"an action":                   {Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}},
		"a reaction and a pool point": reactionCost(wolfID, resources.WardingFlare).Profile,
		"two reactions":               {Slots: map[coreCombat.ActionType]int{coreCombat.ActionReaction: 2}},
		"nothing at all":              nil,
	} {
		t.Run(name, func(t *testing.T) {
			cast, wolf := monsterCast(t)
			err := payAtTheDoor(context.Background(), &Cost{PayerID: wolfID, Profile: profile}, cast)
			require.ErrorIs(t, err, ErrNoPayer)
			require.True(t, wolf.CanReact(), "a refused price spends nothing")
		})
	}
}

// TestASecondBillAgainstASpentMonsterIsRefused: the door charges a monster
// once. The second bill wraps the monster's own refusal under ErrCannotPay, so
// a caller can match either, and the sheet is exactly what the first bill left.
func TestASecondBillAgainstASpentMonsterIsRefused(t *testing.T) {
	cast, wolf := monsterCast(t)
	require.NoError(t, payAtTheDoor(context.Background(), reactionCost(wolfID, ""), cast))
	require.False(t, wolf.CanReact(), "the first bill spends the one reaction")
	before, err := json.Marshal(wolf.ToData())
	require.NoError(t, err)

	err = payAtTheDoor(context.Background(), reactionCost(wolfID, ""), cast)
	require.ErrorIs(t, err, ErrCannotPay)
	require.ErrorIs(t, err, monster.ErrReactionSpent)
	after, err := json.Marshal(wolf.ToData())
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after), "the refused bill leaves the sheet unchanged")
}

// TestTheReactionPriceIsOneTable: one reaction, plus one point of the offer's
// pool when it names one — and the JSON a host stores carries the price and
// never a turn.
func TestTheReactionPriceIsOneTable(t *testing.T) {
	bare := reactionCost(heroID, "")
	require.Equal(t, map[coreCombat.ActionType]int{coreCombat.ActionReaction: 1}, bare.Profile.Slots)
	require.Empty(t, bare.Profile.Pools)

	pooled := reactionCost(heroID, resources.WardingFlare)
	require.Equal(t, map[coreResources.ResourceKey]int{resources.WardingFlare: 1}, pooled.Profile.Pools)

	pooled.Turn = &Turn{Number: 3, Speed: 30}
	pooled.SpellTurn = "turn-3"
	raw, err := json.Marshal(pooled)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"payer_id":"hero"`)
	require.NotContains(t, string(raw), "turn", "a pause's price never refreshes a turn")
}

// TestTakingAPostHitReactionChargesTheFrozenPrice: the pause states one
// reaction and one Wrath point; taking it spends exactly that, once, at the
// resume. A price edited on the pause before resuming is a blob nobody should
// act on.
func TestTakingAPostHitReactionChargesTheFrozenPrice(t *testing.T) {
	out, err := wolfStrikesHero(t, wrathHero(t), NewStrike(&StrikeInput{
		AttackerID: wolfID, TargetID: heroID, Definition: validMeleeDefinition(),
		Roller: &actionRoller{singles: []int{15}, damage: [][]int{{3}}},
	}))
	require.NoError(t, err)
	require.NotNil(t, out.Posed)
	require.Equal(t, PausePostHit, out.Posed.Kind)
	require.Equal(t, reactionCost(heroID, resources.WrathOfTheStorm), out.Posed.Cost)
	for _, sheet := range out.DirtyCharacters {
		if sheet.ID == heroID && sheet.ActionEconomy != nil {
			require.Equal(t, 1, sheet.ActionEconomy.ReactionsRemaining, "nothing is charged at the pause")
		}
	}

	t.Run("an edited price is refused", func(t *testing.T) {
		for name, cost := range map[string]*Cost{
			"free":         nil,
			"cheaper":      reactionCost(heroID, ""),
			"someone else": reactionCost(wolfID, resources.WrathOfTheStorm),
		} {
			t.Run(name, func(t *testing.T) {
				edited := *out.Posed
				edited.Cost = cost
				_, err := Resume(&ResumeInput{Pause: edited, Answer: Take("lightning"), Roller: dice.NewRoller()})
				require.ErrorIs(t, err, ErrBadFrozen)
			})
		}
	})

	machine, err := Resume(&ResumeInput{
		Pause: *out.Posed, Answer: Take("lightning"),
		Roller: &actionRoller{damage: [][]int{{5, 5}}, singles: []int{20}},
	})
	require.NoError(t, err)
	resumed, err := wolfStrikesHero(t, wrathHero(t), machine)
	require.NoError(t, err)

	var paid *character.Data
	for _, sheet := range resumed.DirtyCharacters {
		if sheet.ID == heroID {
			paid = sheet
		}
	}
	require.NotNil(t, paid, "the reactor's sheet was charged")
	require.Equal(t, 0, paid.ActionEconomy.ReactionsRemaining, "one reaction")
	require.Equal(t, 1, paid.Resources[resources.WrathOfTheStorm].Current, "one Wrath point, of two")

	struck := resumed.Outcome.(StrikeOutcome)
	require.True(t, struck.Continued, "the resume tells the retaliation, not the hit again")
	require.NotNil(t, struck.Retaliation)
	require.False(t, struck.Hit)
	require.Zero(t, struck.Damage)
	require.Zero(t, struck.Roll)
}
