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

// frozenContest is a contest stopped mid-save, in enough detail to finish it
// and in no more detail than that — the contest sibling of [frozenPostRoll].
//
// THE FOLD IS STORED RATHER THAN RECOMPUTED for Ability and DC, [frozenPostRoll]'s
// own reason: a sheet edited during the pause must not change what the saver
// was asked about after they answered. Everything else here is exactly what
// [contestMachine.Start] already worked out from content BEFORE the save ever
// rolled — the gate's own consequence, what a failure costs, what a success
// buys — so it is carried rather than re-derived, on the same "store what
// content already decided" footing as [frozenPostRoll.Definition].
type frozenContest struct {
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

	// Save is the save machine's own whole frozen header, opaque here
	// exactly as [Pause.Frozen] is opaque to everyone outside the machine that
	// wrote it.
	Save json.RawMessage `json:"save"`
}

// poseContest turns a save's own pause into the contest's, freezing what
// [contestMachine.resolve] needs to finish once the save is answered. Kind and
// price are the save's.
func poseContest(m *contestMachine, ability abilities.Ability, dc int, save Pause) (Step, error) {
	frozen, err := writeFrozen(machineContest, save.Kind, frozenContest{
		SaverID: m.in.SaverID, Ability: ability, DC: dc,
		OnSuccess: m.in.Gate.OnSuccess, Application: m.in.Application, HasCondition: m.hasCondition,
		Damage: m.in.Damage, SourceName: m.in.SourceName, Removal: m.in.Removal, Move: m.in.Move,
		Cause: m.in.Cause, DamageTaken: m.in.DamageTaken,
		Save: json.RawMessage(save.Frozen),
	})
	if err != nil {
		return nil, err
	}

	return Pause{Kind: save.Kind, Ask: save.Ask, Cost: save.Cost, Frozen: frozen}, nil
}

// resumeContest builds the machine that finishes a contest somebody answered
// the save on. It is never a top-level resume: a contest is always held by a
// cast or a retaliation, which resume it through [resumeInner].
func resumeContest(h frozenHeader, in *ResumeInput) (Machine, error) {
	var frozen frozenContest
	if err := decodeState(h, &frozen); err != nil {
		return nil, err
	}
	if err := samePrice(nil, in.Pause.Cost); err != nil {
		return nil, err
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
			Roller:      in.Roller,
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
		contest: m, ability: frozen.Ability, dc: frozen.DC, save: frozen.Save, answer: in.Answer, roller: in.Roller,
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
	answer  Answer
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
