// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// frozenContestKind and frozenContestVersion discriminate what a stored blob
// is, the same trust boundary [frozenStrikeKind]/[frozenCheckKind] keep.
const (
	frozenContestKind    = "contest.post_save_roll"
	frozenContestVersion = 1
)

// frozenContest is a contest stopped mid-save, in enough detail to finish it
// and in no more detail than that — the contest sibling of [frozenStrike].
//
// THE FOLD IS STORED RATHER THAN RECOMPUTED for Ability and DC, [frozenStrike]'s
// own reason: a sheet edited during the pause must not change what the saver
// was asked about after they answered. Everything else here is exactly what
// [contestMachine.Start] already worked out from content BEFORE the save ever
// rolled — the gate's own consequence, what a failure costs, what a success
// buys — so it is carried rather than re-derived, on the same "store what
// content already decided" footing as [frozenStrike.Definition].
type frozenContest struct {
	Kind    string `json:"kind"`
	Version int    `json:"version"`

	SaverID string            `json:"saver_id"`
	Ability abilities.Ability `json:"ability"`
	DC      int               `json:"dc"`

	OnSuccess    saves.SaveEffect                   `json:"on_success"`
	Application  combatActions.ConditionApplication `json:"application"`
	HasCondition bool                               `json:"has_condition"`
	Damage       []damage.Damage                    `json:"damage,omitempty"`
	SourceName   string                             `json:"source_name,omitempty"`
	Removal      *ConditionRemoval                  `json:"removal,omitempty"`
	Move         *MoveDirective                     `json:"move,omitempty"`
	Cause        dnd5eEvents.SaveCause              `json:"cause"`
	DamageTaken  int                                `json:"damage_taken,omitempty"`

	// Save is the save machine's own frozen bytes, opaque here exactly as
	// [Pose.Frozen] is opaque to everyone outside the machine that wrote it —
	// this package wrote both halves, so nesting one inside the other costs
	// nothing more than [json.RawMessage] already buys.
	Save json.RawMessage `json:"save"`
}

// poseContest turns a save's own pose into the contest's, freezing what
// [contestMachine.resolve] needs to finish once the save is answered.
func poseContest(m *contestMachine, ability abilities.Ability, dc int, save Pose) (Step, error) {
	frozen, err := json.Marshal(frozenContest{
		Kind: frozenContestKind, Version: frozenContestVersion,
		SaverID: m.in.SaverID, Ability: ability, DC: dc,
		OnSuccess: m.in.Gate.OnSuccess, Application: m.in.Application, HasCondition: m.hasCondition,
		Damage: m.in.Damage, SourceName: m.in.SourceName, Removal: m.in.Removal, Move: m.in.Move,
		Cause: m.in.Cause, DamageTaken: m.in.DamageTaken,
		Save: json.RawMessage(save.Frozen),
	})
	if err != nil {
		return nil, fmt.Errorf("%w: freeze contest: %v", ErrBadFrozen, err)
	}

	return Pose{Ask: save.Ask, Frozen: frozen}, nil
}

// newContestResumed builds the machine that finishes a contest somebody
// answered the save on. Unexported for [newSaveResumed]'s reason: a contest
// is always reached through [castMachine.resolveTarget], so resuming one is
// internal to that resume, not a public entry on its own.
func newContestResumed(raw json.RawMessage, answer OfferAnswer, roller dice.Roller) (Machine, error) {
	var frozen frozenContest
	if err := json.Unmarshal(raw, &frozen); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadFrozen, err)
	}
	if frozen.Kind != frozenContestKind || frozen.Version != frozenContestVersion {
		return nil, fmt.Errorf("%w: kind %q version %d is not one this build froze",
			ErrBadFrozen, frozen.Kind, frozen.Version)
	}

	source := frozen.Application.Ref.String()
	if frozen.Cause.EffectRef != nil {
		source = frozen.Cause.EffectRef.String()
	}

	m := &contestMachine{
		in: &ContestInput{
			Gate:        &saves.SaveGate{OnSuccess: frozen.OnSuccess},
			SaverID:     frozen.SaverID,
			Application: frozen.Application,
			Damage:      frozen.Damage,
			SourceName:  frozen.SourceName,
			Removal:     frozen.Removal,
			Move:        frozen.Move,
			Cause:       frozen.Cause,
			DamageTaken: frozen.DamageTaken,
			Roller:      roller,
		},
		hasCondition: frozen.HasCondition,
	}
	if frozen.HasCondition {
		prepared, err := prepareCondition(frozen.Application, frozen.SaverID, source)
		if err != nil {
			return nil, err
		}
		m.prepared = prepared
	}

	return &contestResumeMachine{
		contest: m, ability: frozen.Ability, dc: frozen.DC, save: frozen.Save, answer: answer, roller: roller,
	}, nil
}

// contestResumeMachine is a contest whose save already posed, resumed with
// its answer. It is a distinct type from [contestMachine] rather than a
// field on it, because a resumed contest's Start has exactly one thing to
// do — resume the save and call [contestMachine.resolve] — and giving that
// its own small Start keeps [contestMachine.Start]'s preflight from growing
// a branch for a case it never reaches (a fresh contest always calls
// [requestSave]; a resumed one never does, because a resumed save cannot
// pose again — an offer is spent or kept, not re-offered).
type contestResumeMachine struct {
	contest *contestMachine
	ability abilities.Ability
	dc      int
	save    json.RawMessage
	answer  OfferAnswer
	roller  dice.Roller
}

func (m *contestResumeMachine) Start(_ context.Context, cast *Participants) (Step, error) {
	m.contest.cast = cast

	saveMachine, err := newSaveResumedFromJSON(m.save, m.answer, m.roller)
	if err != nil {
		return nil, err
	}

	return Request{
		name:    "resume saving throw",
		machine: saveMachine,
		next: func(_ context.Context, out Outcome) (Step, error) {
			save, ok := out.(SaveOutcome)
			if !ok {
				return nil, fmt.Errorf("%w: saving throw returned %T", ErrBadStep, out)
			}
			return m.contest.resolve(m.ability, m.dc, save)
		},
	}, nil
}
