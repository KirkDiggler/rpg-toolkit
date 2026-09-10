package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// inspiredHero is the fighter with a bardic die in hand: the same sheet
// actionHero builds, carrying the condition that answers the offer chain.
func inspiredHero(t *testing.T) *character.Data {
	t.Helper()
	hero := actionHero()
	stored, err := conditions.NewInspiredCondition(heroID, "bard-1", conditions.InspiredDie).ToJSON()
	require.NoError(t, err)
	hero.Conditions = []json.RawMessage{stored}
	return hero
}

func banedInspiredHero(t *testing.T) *character.Data {
	t.Helper()
	hero := inspiredHero(t)
	bane, err := conditions.NewBanedCondition(conditions.NewBanedConditionInput{
		MemberID: heroID, SourceID: "bane-caster", SourceRef: refs.Spells.Bane(),
	})
	require.NoError(t, err)
	stored, err := bane.ToJSON()
	require.NoError(t, err)
	hero.Conditions = append([]json.RawMessage{stored}, hero.Conditions...)
	return hero
}

// heroSwings resolves one attack by the hero on the wolf, with the hero's
// sheet supplied by the caller so a test can choose whether a die is offered.
func heroSwings(t *testing.T, hero *character.Data, roller dice.Roller) (*Output, error) {
	t.Helper()
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: heroID, TargetID: wolfID, Roller: roller,
	})
	require.NoError(t, err)
	return resolveHeroStrike(t, hero, machine)
}

func resolveHeroStrike(t *testing.T, hero *character.Data, machine Machine) (*Output, error) {
	t.Helper()
	return resolveHeroStrikeOn(t, hero, machine, newSurface(events.NewEventBus()))
}

func resolveHeroStrikeOn(
	t *testing.T, hero *character.Data, machine Machine, surf *surface,
) (*Output, error) {
	t.Helper()
	return resolveOn(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Character: hero},
		},
		Machine: machine, Initiative: orderAsGiven{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, TurnDriver: passDriver{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
	}, surf)
}

// TestAStrikeNobodyOffersAnythingOnIsUnchanged is the regression that matters:
// the offer chain folds on every attack now, and a roll with no subscriber
// must produce the same outcome and no pose at all.
func TestAStrikeNobodyOffersAnythingOnIsUnchanged(t *testing.T) {
	roller := &actionRoller{singles: []int{15}, damage: [][]int{{3}}}

	out, err := heroSwings(t, actionHero(), roller)

	require.NoError(t, err)
	require.Nil(t, out.Posed, "nobody offered anything, so nothing was asked")
	outcome := out.Outcome.(StrikeOutcome)
	require.Equal(t, 15, outcome.Roll)
	require.Equal(t, 19, outcome.Total)
	require.True(t, outcome.Hit)
	require.Equal(t, 5, outcome.Damage)
}

// TestAnOfferPosesAndProducesNoOutcome — the machine stops, and the zero value
// does not lie: Outcome is nil rather than a strike that missed for nothing.
func TestAnOfferPosesAndProducesNoOutcome(t *testing.T) {
	roller := &actionRoller{singles: []int{11}}

	out, err := heroSwings(t, inspiredHero(t), roller)

	require.NoError(t, err)
	require.Nil(t, out.Outcome, "a posed machine produced nothing yet")
	require.NotNil(t, out.Posed)
	require.Equal(t, heroID, out.Posed.Ask.Audience)
	require.Equal(t, 11, out.Posed.Ask.Roll)
	require.Equal(t, 15, out.Posed.Ask.Total)
	require.Equal(t, conditions.InspiredName, out.Posed.Ask.Offer.Name)
	require.Equal(t, refs.Conditions.Inspired().String(), out.Posed.Ask.Offer.Ref.String())
	require.Equal(t, []string{"spend", "keep"}, out.Posed.Ask.Options)
	require.NotEmpty(t, out.Posed.Frozen)
	require.Equal(t, 1, roller.calls, "the d20 and nothing else")
}

// TestTheAskDoesNotLeakTheAC pins the one number deliberately left off it:
// a player who could see the AC would be deciding whether a d6 closes the gap
// rather than whether it is worth spending.
func TestTheAskDoesNotLeakTheAC(t *testing.T) {
	out, err := heroSwings(t, inspiredHero(t), &actionRoller{singles: []int{11}})
	require.NoError(t, err)

	raw, err := json.Marshal(out.Posed.Ask)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "target_ac")
	require.NotContains(t, string(raw), "TargetAC")
}

// poseThenAnswer runs the pose and resumes it with one answer, returning the
// finished strike.
func poseThenAnswer(
	t *testing.T, d20 int, answer OfferAnswer, resumeRoller *actionRoller,
) StrikeOutcome {
	t.Helper()
	posed, err := heroSwings(t, inspiredHero(t), &actionRoller{singles: []int{d20}})
	require.NoError(t, err)
	require.NotNil(t, posed.Posed)

	machine, err := NewStrikeResumed(&StrikeResumeInput{
		Frozen: posed.Posed.Frozen, Answer: answer, Roller: resumeRoller,
	})
	require.NoError(t, err)

	out, err := resolveHeroStrike(t, inspiredHero(t), machine)
	require.NoError(t, err)
	require.Nil(t, out.Posed, "a resumed strike finishes")
	return out.Outcome.(StrikeOutcome)
}

func TestBaneCalculationIsFrozenAcrossInspirationAnswers(t *testing.T) {
	for _, tc := range []struct {
		name           string
		answer         OfferAnswer
		resume         *actionRoller
		wantComponents int
	}{
		{name: "keep reuses exact Bane", answer: OfferKeep, resume: &actionRoller{}, wantComponents: 3},
		{name: "spend appends only Inspiration", answer: OfferSpend, resume: &actionRoller{singles: []int{4}, damage: [][]int{{3}}}, wantComponents: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posing := &actionRoller{singles: []int{8}, damage: [][]int{{3}}}
			posed, err := heroSwings(t, banedInspiredHero(t), posing)
			require.NoError(t, err)
			require.NotNil(t, posed.Posed)
			require.Equal(t, 2, posing.calls, "one d20 and one Bane d4 before the pose")

			machine, err := NewStrikeResumed(&StrikeResumeInput{
				Frozen: append([]byte(nil), posed.Posed.Frozen...), Answer: tc.answer, Roller: tc.resume,
			})
			require.NoError(t, err)
			out, err := resolveHeroStrike(t, banedInspiredHero(t), machine)
			require.NoError(t, err)
			calculation := out.Outcome.(StrikeOutcome).Calculation
			require.NotNil(t, calculation)
			require.Len(t, calculation.Components, tc.wantComponents)
			bane := calculation.Components[2]
			require.Equal(t, "bane-caster", bane.Source.SourceID)
			require.Equal(t, []int{3}, bane.Dice.FinalRolls)
			require.True(t, bane.SubtractDice)
			if tc.answer == OfferKeep {
				require.Zero(t, tc.resume.calls, "keep neither rerolls nor appends")
			} else {
				require.Equal(t, refs.Conditions.Inspired().String(), calculation.Components[3].Source.Ref.String())
			}
		})
	}
}

// TestSpendingAddsOneFaceAndCanTurnAMissIntoAHit is the walk's whole point.
// The wolf's AC is 13; an 11 plus 4 is 15, so this hits either way — the case
// that matters is the arithmetic, checked below on the total.
func TestSpendingAddsOneFaceAndCanTurnAMissIntoAHit(t *testing.T) {
	// d20 of 8 plus a bonus of 4 is 12, one short of the wolf's 13. A d6 of 4
	// makes it 16.
	outcome := poseThenAnswer(t, 8, OfferSpend, &actionRoller{singles: []int{4}, damage: [][]int{{3}}})

	require.Equal(t, 8, outcome.Roll, "the d20 is not re-rolled")
	require.Equal(t, 16, outcome.Total, "roll plus bonus plus the face")
	require.True(t, outcome.Hit)
	require.Equal(t, 5, outcome.Damage)
}

// TestKeepingLeavesTheTotalAlone — declining costs nothing and changes nothing.
func TestKeepingLeavesTheTotalAlone(t *testing.T) {
	outcome := poseThenAnswer(t, 8, OfferKeep, &actionRoller{})

	require.Equal(t, 8, outcome.Roll)
	require.Equal(t, 12, outcome.Total, "no face joined it")
	require.False(t, outcome.Hit, "12 is short of the wolf's 13")
	require.Zero(t, outcome.Damage)
}

// TestANaturalOneStillMissesWithTheDie is the arithmetic branch being the ONLY
// one a resume touches. Twenty added to a 1 is still a miss.
func TestANaturalOneStillMissesWithTheDie(t *testing.T) {
	outcome := poseThenAnswer(t, 1, OfferSpend, &actionRoller{singles: []int{6}})

	require.Equal(t, 1, outcome.Roll)
	require.Equal(t, 11, outcome.Total, "the face still joins the total")
	require.False(t, outcome.Hit, "a natural 1 always misses")
	require.False(t, outcome.Critical)
}

// TestANaturalTwentyIsUnmoved — the crit is the face of the d20, not the total.
func TestANaturalTwentyIsUnmoved(t *testing.T) {
	outcome := poseThenAnswer(t, 20, OfferSpend, &actionRoller{singles: []int{3}, damage: [][]int{{3}, {5}}})

	require.Equal(t, 20, outcome.Roll)
	require.Equal(t, 27, outcome.Total)
	require.True(t, outcome.Hit)
	require.True(t, outcome.Critical)
}

// TestThePostRollChainIsPublishedExactlyOnceAcrossThePair is R2's mechanical
// half. Shield reads WouldHit off that chain; two publications would answer it
// twice with two different numbers, once without the die and once with it.
func TestThePostRollChainIsPublishedExactlyOnceAcrossThePair(t *testing.T) {
	var seen []*dnd5eEvents.PostAttackRollEvent
	watch := func(bus events.EventBus) {
		_, err := dnd5eEvents.PostAttackRollChain.On(bus).SubscribeWithChain(context.Background(),
			func(
				_ context.Context, event *dnd5eEvents.PostAttackRollEvent,
				c chain.Chain[*dnd5eEvents.PostAttackRollEvent],
			) (chain.Chain[*dnd5eEvents.PostAttackRollEvent], error) {
				seen = append(seen, event)
				return c, nil
			})
		require.NoError(t, err)
	}

	posingBus := events.NewEventBus()
	watch(posingBus)
	posed, err := resolveHeroStrikeOn(t, inspiredHero(t), strikeFor(t, &actionRoller{singles: []int{8}}),
		newSurface(posingBus))
	require.NoError(t, err)
	require.NotNil(t, posed.Posed)
	require.Empty(t, seen, "the posing half publishes none: it stopped before the chain")

	machine, err := NewStrikeResumed(&StrikeResumeInput{
		Frozen: posed.Posed.Frozen, Answer: OfferSpend,
		Roller: &actionRoller{singles: []int{4}, damage: [][]int{{3}}},
	})
	require.NoError(t, err)

	resumeBus := events.NewEventBus()
	watch(resumeBus)
	out, err := resolveHeroStrikeOn(t, inspiredHero(t), machine, newSurface(resumeBus))
	require.NoError(t, err)
	require.Nil(t, out.Posed)

	require.Len(t, seen, 1, "exactly one across the pair")
	require.Equal(t, 8, seen[0].AttackRoll, "the d20 the player was asked about")
	require.Equal(t, 16, seen[0].TotalAttack, "with the face already joined")
	require.True(t, seen[0].WouldHit, "and the answer Shield reads is the final one")
}

// strikeFor is one attack by the hero, ready to hand to a resolve.
func strikeFor(t *testing.T, roller dice.Roller) Machine {
	t.Helper()
	machine, err := NewAction(&ActionInput{
		Definition: validMeleeDefinition(), AttackerID: heroID, TargetID: wolfID, Roller: roller,
	})
	require.NoError(t, err)
	return machine
}

// TestSpendingConsumesTheDieAndKeepingDoesNot is "spent when TAKEN" observed
// on the sheet that comes back, which is the only place it matters.
func TestSpendingConsumesTheDieAndKeepingDoesNot(t *testing.T) {
	for _, tc := range []struct {
		name   string
		answer OfferAnswer
		roller *actionRoller
		want   string
	}{
		{"spending takes the die off the sheet", OfferSpend,
			&actionRoller{singles: []int{4}, damage: [][]int{{3}}}, "gone"},
		{"keeping leaves it in hand", OfferKeep, &actionRoller{}, "held"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posed, err := heroSwings(t, inspiredHero(t), &actionRoller{singles: []int{8}})
			require.NoError(t, err)

			machine, err := NewStrikeResumed(&StrikeResumeInput{
				Frozen: posed.Posed.Frozen, Answer: tc.answer, Roller: tc.roller,
			})
			require.NoError(t, err)
			out, err := resolveHeroStrike(t, inspiredHero(t), machine)
			require.NoError(t, err)

			require.Equal(t, tc.want, heroDieState(t, out))
		})
	}
}

// heroDieState reads the hero's persisted conditions off the sheets the
// resolution handed back: "gone" when the sheet came back without the die,
// "held" when it still carries one or when the sheet was never dirtied at all
// — which is itself the statement that nothing was spent.
func heroDieState(t *testing.T, out *Output) string {
	t.Helper()
	for _, sheet := range out.DirtyCharacters {
		if sheet.ID != heroID {
			continue
		}
		for _, raw := range sheet.Conditions {
			var peek struct {
				Ref struct {
					ID string `json:"id"`
				} `json:"ref"`
			}
			require.NoError(t, json.Unmarshal(raw, &peek))
			if peek.Ref.ID == refs.Conditions.Inspired().ID {
				return "held"
			}
		}
		return "gone"
	}
	return "held"
}

// TestATamperedFrozenBlobIsRefused — every way of lying about what was rolled.
func TestATamperedFrozenBlobIsRefused(t *testing.T) {
	posed, err := heroSwings(t, inspiredHero(t), &actionRoller{singles: []int{11}})
	require.NoError(t, err)

	var frozen frozenStrike
	require.NoError(t, json.Unmarshal(posed.Posed.Frozen, &frozen))

	t.Run("a d20 outside 1-20", func(t *testing.T) {
		bad := frozen
		bad.Roll = 21
		bad.Total = 21 + bad.Folded.AttackBonus
		_, err := NewStrikeResumed(resumeOf(t, bad, OfferSpend))
		require.ErrorIs(t, err, ErrBadFrozen)
	})

	t.Run("a total that disagrees with the frozen calculation", func(t *testing.T) {
		bad := frozen
		bad.Total = frozen.Total + 7
		_, err := NewStrikeResumed(resumeOf(t, bad, OfferSpend))
		require.ErrorIs(t, err, ErrBadFrozen)
	})

	t.Run("calculation bonus disagrees with folded attack", func(t *testing.T) {
		bad := frozen
		bad.Calculation = dnd5eEvents.CloneRollCalculation(frozen.Calculation)
		*bad.Calculation.Components[1].Modifier++
		bad.Calculation.Total++
		bad.Total++
		_, err := NewStrikeResumed(resumeOf(t, bad, OfferSpend))
		require.ErrorIs(t, err, ErrBadFrozen)
	})

	t.Run("a kind this build did not write", func(t *testing.T) {
		bad := frozen
		bad.Kind = "walk.paused"
		_, err := NewStrikeResumed(resumeOf(t, bad, OfferSpend))
		require.ErrorIs(t, err, ErrBadFrozen)
	})

	t.Run("version one fails closed", func(t *testing.T) {
		bad := frozen
		bad.Version = 1
		bad.Calculation = nil
		_, err := NewStrikeResumed(resumeOf(t, bad, OfferSpend))
		require.ErrorIs(t, err, ErrBadFrozen)
	})

	t.Run("a version this build did not write", func(t *testing.T) {
		bad := frozen
		bad.Version = 99
		_, err := NewStrikeResumed(resumeOf(t, bad, OfferSpend))
		require.ErrorIs(t, err, ErrBadFrozen)
	})

	t.Run("an offer posed to somebody other than the roller", func(t *testing.T) {
		bad := frozen
		bad.Offer.Audience = "somebody-else"
		_, err := NewStrikeResumed(resumeOf(t, bad, OfferSpend))
		require.ErrorIs(t, err, ErrNotOffered)
	})

	t.Run("an answer this machine did not pose", func(t *testing.T) {
		_, err := NewStrikeResumed(resumeOf(t, frozen, OfferAnswer("counterspell")))
		require.ErrorIs(t, err, ErrNotOffered)
	})

	t.Run("no roller", func(t *testing.T) {
		in := resumeOf(t, frozen, OfferKeep)
		in.Roller = nil
		_, err := NewStrikeResumed(in)
		require.ErrorIs(t, err, ErrNoRoller)
	})

	t.Run("nothing at all", func(t *testing.T) {
		_, err := NewStrikeResumed(nil)
		require.ErrorIs(t, err, ErrNilInput)
		_, err = NewStrikeResumed(&StrikeResumeInput{Answer: OfferKeep, Roller: dice.NewRoller()})
		require.ErrorIs(t, err, ErrBadFrozen)
	})
}

func resumeOf(t *testing.T, frozen frozenStrike, answer OfferAnswer) *StrikeResumeInput {
	t.Helper()
	raw, err := json.Marshal(frozen)
	require.NoError(t, err)
	return &StrikeResumeInput{Frozen: raw, Answer: answer, Roller: dice.NewRoller()}
}

// TestAnOfferToSomebodyElseIsRefusedRatherThanPosed is R5 failing closed. The
// chain is already wide enough for Cutting Words; the freeze is not, and the
// machine says so instead of posing a window nobody designed.
func TestAnOfferToSomebodyElseIsRefusedRatherThanPosed(t *testing.T) {
	machine := newStrikeMachine(&StrikeInput{
		AttackerID: heroID, TargetID: wolfID, Definition: validMeleeDefinition(),
	})
	machine.outcome = StrikeOutcome{AttackerID: heroID, TargetID: wolfID, Total: 15}

	_, err := machine.pose(dnd5eEvents.AttackChainEvent{AttackBonus: 4}, 11, []dnd5eEvents.Offer{
		{Ref: refs.Conditions.Inspired(), Name: "Cutting Words", Audience: "hostile-bard", Die: "1d6"},
	})

	require.ErrorIs(t, err, ErrNotOffered)
}

// TestTwoOffersOnOneRollAreRefused is the same shelf item from the other side:
// one Pose per run is what this slice drives.
func TestTwoOffersOnOneRollAreRefused(t *testing.T) {
	machine := newStrikeMachine(&StrikeInput{
		AttackerID: heroID, TargetID: wolfID, Definition: validMeleeDefinition(),
	})
	machine.outcome = StrikeOutcome{AttackerID: heroID, TargetID: wolfID, Total: 15}

	_, err := machine.pose(dnd5eEvents.AttackChainEvent{AttackBonus: 4}, 11, []dnd5eEvents.Offer{
		{Ref: refs.Conditions.Inspired(), Audience: heroID, Die: "1d6"},
		{Ref: refs.Conditions.Helped(), Audience: heroID, Die: "1d4"},
	})

	require.ErrorIs(t, err, ErrNotOffered)
}

// TestARequestedMachineCannotPose: a sub-machine's requester is a Go closure
// on the stack, and nothing serializes it. Refused by name rather than
// silently dropping the question.
func TestARequestedMachineCannotPose(t *testing.T) {
	_, err := drive(context.Background(), nil, posingMachine{}, &Participants{})
	require.ErrorIs(t, err, ErrBadStep)
}

type posingMachine struct{}

func (posingMachine) Start(_ context.Context, _ *Participants) (Step, error) {
	return Pose{Ask: Ask{Audience: "somebody"}, Frozen: []byte("{}")}, nil
}
