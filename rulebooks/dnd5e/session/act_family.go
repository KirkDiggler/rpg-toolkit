// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// act_family.go is R16's TABLE: every beat the encounter writes has exactly
// one family, or is one of R9's record-only beats (Law 8). The beat decides,
// never the verb: a struck beat carrying a reaction ref is a reaction under
// whatever act it sits in.
//
// TestEveryBeatHasAFamily reads every exported beat name out of the
// encounter's source and fails for one in neither list, so a new beat cannot
// reach a recipient unfolded.

// recordOnly are R9's beats: written to the record, folded into no act —
// trading's beats and prop interaction, until trading's use case brings
// their act (Law 8) — and a creature's described table (rpg-project#674),
// which is the record's debug symbols and never anything a player is shown.
var recordOnly = map[string]bool{
	encounter.BeatTable:             true,
	encounter.BeatTransferred:       true,
	encounter.BeatInteracted:        true,
	string(encounter.OutcomeBought): true,
	string(encounter.OutcomeSold):   true,
}

// familyOf is a beat name's family, and false for a beat with none — a
// record-only beat or one this build does not know.
func familyOf(beat string) (Family, bool) {
	switch beat {
	case encounter.BeatMoved, encounter.BeatCast, encounter.BeatCastMissed, encounter.BeatCastWarded,
		encounter.BeatSaved, encounter.BeatActivated, encounter.BeatActivationResult,
		encounter.BeatSpoke, string(encounter.OutcomeDeathSave), encounter.BeatLooted,
		encounter.BeatHeld, encounter.BeatDropped, encounter.BeatDoor, encounter.BeatEquipmentChanged,
		encounter.BeatStayed, encounter.BeatSnuck, encounter.BeatRested:
		return FamilyAct, true
	case string(encounter.OutcomeStruck), string(encounter.OutcomeMissed), string(encounter.OutcomeWarded):
		return FamilyAct, true // FamilyReaction when the beat carries a reaction ref
	case encounter.BeatAnswered, encounter.BeatTempered:
		return FamilyTableAnswer, true // a speak's answer folds into the SpeakAct
	case string(encounter.OutcomeDown), encounter.BeatStance, encounter.BeatConcentrationEnded,
		string(encounter.OutcomeExperienceGained):
		return FamilyState, true
	case encounter.BeatSighted, encounter.BeatHeard, encounter.BeatDiscoveryChecked,
		encounter.BeatConcealmentRevealed, encounter.BeatCellsRevealed,
		encounter.BeatRoomRevealed: //nolint:staticcheck // SA1019: stored stories still name it
		return FamilyPerception, true
	case encounter.BeatTick, encounter.BeatTurnEnded, encounter.BeatFightStarted, encounter.BeatFightEnded,
		encounter.BeatJoined, encounter.BeatExited, encounter.BeatArrived, encounter.BeatSceneOpened,
		encounter.BeatEnded, encounter.BeatWindowOpened, encounter.BeatRollWindowOpened:
		return FamilyClock, true
	default:
		return "", false
	}
}

// piecesOf is what one entry contributes to its recipient's acts: usually one
// piece, none for a record-only beat, and one more state piece per
// concentration or condition a rest, a departure or an ending lifted.
func piecesOf(recipient, tag string, payload []byte) []piece {
	beat := peekBeat(payload)
	if recordOnly[beat] {
		return nil
	}
	family, ok := familyOf(beat)
	if !ok {
		return []piece{{family: FamilyClock, undecoded: fmt.Sprintf("beat %q has no family", beat)}}
	}
	pieces, err := decodePieces(recipient, tag, beat, payload)
	if err != nil {
		return []piece{{family: family, undecoded: fmt.Sprintf("%s: %v", beat, err)}}
	}
	return pieces
}

// one is a single-piece answer.
func one(p piece) ([]piece, error) { return []piece{p}, nil }

// decodePieces reads one beat's payload into its pieces.
//
//nolint:gocyclo // one arm per beat is the table, and splitting it hides it
func decodePieces(recipient, tag, beat string, payload []byte) ([]piece, error) {
	switch beat {
	case encounter.BeatMoved:
		return walkPiece(payload)
	case string(encounter.OutcomeStruck), string(encounter.OutcomeMissed):
		return attackPiece(beat, payload)
	case string(encounter.OutcomeWarded):
		return wardedPiece(payload)
	case encounter.BeatCast:
		return castPiece(payload)
	case encounter.BeatCastMissed, encounter.BeatCastWarded:
		return castResultPiece(beat, payload)
	case encounter.BeatSaved:
		return savedPiece(payload)
	case encounter.BeatActivated:
		return activatedPiece(payload)
	case encounter.BeatActivationResult:
		return activationResultPiece(payload)
	case encounter.BeatSpoke:
		return spokePiece(payload)
	case encounter.BeatAnswered:
		return answeredPiece(tag, payload)
	case encounter.BeatTempered:
		return temperedPiece(payload)
	case string(encounter.OutcomeDeathSave):
		return deathSavePiece(payload)
	case encounter.BeatLooted:
		var p struct {
			Member string `json:"member"`
			Target string `json:"target"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, err
		}
		return one(actPiece(LootAct{By: member(p.Member), From: member(p.Target)}))
	case encounter.BeatHeld:
		var p struct {
			Holder string `json:"holder"`
			Prop   string `json:"prop"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, err
		}
		return one(actPiece(HoldAct{By: member(p.Holder), Prop: prop(p.Prop)}))
	case encounter.BeatDropped:
		var p struct {
			Member   string           `json:"member"`
			Prop     string           `json:"prop"`
			Position spatial.Position `json:"position"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, err
		}
		return one(actPiece(DropAct{By: member(p.Member), Prop: prop(p.Prop), Cell: p.Position}))
	case encounter.BeatDoor:
		return doorPiece(payload)
	case encounter.BeatEquipmentChanged:
		var p struct {
			Member string `json:"member"`
			Slot   string `json:"slot"`
			Item   string `json:"item"`
			Change string `json:"change"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, err
		}
		return one(actPiece(EquipAct{By: member(p.Member), Slot: p.Slot, Item: p.Item, Change: p.Change}))
	case encounter.BeatStayed:
		var p struct {
			Member string `json:"member"`
			Cause  string `json:"cause"`
			Why    string `json:"why"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, err
		}
		return one(actPiece(StayAct{Member: member(p.Member), Cause: p.Cause, Why: p.Why}))
	case encounter.BeatSnuck:
		return snuckPiece(payload)
	case encounter.BeatRested:
		return restedPieces(payload)
	case string(encounter.OutcomeDown):
		var p struct {
			Member string `json:"member"`
			Kind   string `json:"kind"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, err
		}
		return one(statePiece(StateChange{Party: member(p.Member), Change: StateDowned, Downed: &Downed{Kind: p.Kind}}))
	case encounter.BeatStance:
		return stancePiece(payload)
	case encounter.BeatConcentrationEnded:
		state, err := concentrationState(payload)
		if err != nil {
			return nil, err
		}
		return one(statePiece(state))
	case string(encounter.OutcomeExperienceGained):
		return experiencePiece(payload)
	case encounter.BeatSighted, encounter.BeatHeard, encounter.BeatDiscoveryChecked,
		encounter.BeatConcealmentRevealed, encounter.BeatCellsRevealed,
		encounter.BeatRoomRevealed: //nolint:staticcheck // SA1019: stored stories still name it
		return perceptionPiece(recipient, beat, payload)
	default:
		return clockPieces(beat, payload)
	}
}

// actPiece is a FamilyAct piece that stands alone.
func actPiece(body ActBody) piece { return piece{family: FamilyAct, body: body} }

// statePiece is a FamilyState piece.
func statePiece(body StateChange) piece { return piece{family: FamilyState, body: body} }

// walkPiece is one step. It extends an open walk of the same mover and cause,
// and a new segment of a walk starts where the last one left the mover.
func walkPiece(payload []byte) ([]piece, error) {
	var p struct {
		Member   string           `json:"member"`
		Position spatial.Position `json:"position"`
		Doors    []string         `json:"doors"`
		Cause    string           `json:"cause"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	step := WalkStep{Cell: p.Position, Doors: p.Doors}
	walk := WalkAct{Mover: member(p.Member), Path: []WalkStep{step}, Cause: p.Cause}
	return one(piece{
		family: FamilyAct, body: walk,
		absorb: func(open ActBody) (ActBody, bool) {
			w, ok := open.(WalkAct)
			if !ok || w.Mover != walk.Mover || w.Cause != walk.Cause {
				return nil, false
			}
			w.Path = append(append([]WalkStep(nil), w.Path...), step)
			return w, true
		},
		continues: func(prior ActBody) ActBody {
			w, ok := prior.(WalkAct)
			if !ok || w.Mover != walk.Mover || len(w.Path) == 0 {
				return walk
			}
			from := WalkStep{Cell: w.Path[len(w.Path)-1].Cell}
			return WalkAct{Mover: walk.Mover, Path: []WalkStep{from, step}, Cause: walk.Cause}
		},
	})
}

// outcomeFields are the keys every attack outcome beat shares.
type outcomeFields struct {
	Actor          string          `json:"actor"`
	Targets        []string        `json:"targets"`
	Roll           int             `json:"roll"`
	Total          int             `json:"total"`
	Against        int             `json:"against"`
	Amount         int             `json:"amount"`
	Critical       bool            `json:"critical"`
	Attack         beatAttack      `json:"attack"`
	Reaction       *beatReaction   `json:"reaction"`
	Sequence       *beatReaction   `json:"sequence"`
	PresentationID string          `json:"presentation_id"`
	Calculation    json.RawMessage `json:"calculation"`
	Components     json.RawMessage `json:"damage_components"`
	// AdvantageSources and DisadvantageSources are the attack fold's own
	// ordered sources (encounter.AttackModifierSource). RETIRED by the dice
	// keep (rpg-project#462 R1): read only for a beat with no keep.
	AdvantageSources    []modifierSource `json:"advantage_sources"`
	DisadvantageSources []modifierSource `json:"disadvantage_sources"`
}

// modifierSource is one advantage or disadvantage source a swing names.
type modifierSource struct {
	SourceRef string `json:"source_ref"`
	SourceID  string `json:"source_id"`
}

// attackInfluences are a swing's retired advantage and disadvantage source
// keys, in that order, for a beat with no keep; nil when it named none.
func attackInfluences(p outcomeFields) []RollInfluence {
	var out []RollInfluence
	for _, src := range p.AdvantageSources {
		out = append(out, RollInfluence{Kind: InfluenceAdvantage, SourceRef: src.SourceRef, SourceID: src.SourceID})
	}
	for _, src := range p.DisadvantageSources {
		out = append(out, RollInfluence{Kind: InfluenceDisadvantage, SourceRef: src.SourceRef, SourceID: src.SourceID})
	}
	return out
}

// attackBase is the AttackAct the shared keys fill, and the piece's family:
// a reaction when the beat carries a reaction ref (R16).
func attackBase(p outcomeFields) (AttackAct, Family, error) {
	if len(p.Targets) == 0 {
		return AttackAct{}, "", fmt.Errorf("names no target")
	}
	reaction, ok := p.Reaction.toRef()
	if !ok {
		return AttackAct{}, "", fmt.Errorf("reaction names no ref")
	}
	sequence, ok := p.Sequence.toSequence()
	if !ok {
		return AttackAct{}, "", fmt.Errorf("sequence names no ref")
	}
	family := FamilyAct
	if reaction != nil {
		family = FamilyReaction
	}
	return AttackAct{
		By: member(p.Actor), To: member(p.Targets[0]), Attack: p.Attack.toRef(),
		Sequence: sequence, Reaction: reaction, PresentationID: p.PresentationID,
	}, family, nil
}

// attackPiece is a struck or missed swing, its outcome named by the beat.
func attackPiece(beat string, payload []byte) ([]piece, error) {
	var p outcomeFields
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	act, family, err := attackBase(p)
	if err != nil {
		return nil, err
	}
	hit := beat == string(encounter.OutcomeStruck)
	act.Outcome = AttackMiss
	if hit {
		act.Outcome, act.Critical, act.Damage = AttackHit, p.Critical, p.Amount
		components, ok := decodeDamageComponents(p.Components)
		if !ok {
			return nil, fmt.Errorf("damage components do not replay")
		}
		act.DamageComponents = components
	}
	roll, err := rollOf(p.Calculation, facesOf(p.Roll), p.Total, intp(p.Against), boolp(hit))
	if err != nil {
		return nil, err
	}
	// THE KEEP IS THE RECORD (rpg-project#462 R1). The swing's
	// advantage_sources/disadvantage_sources keys are retired; they are read
	// only for a beat whose calculation recorded no keep.
	if len(roll.Influences) == 0 {
		roll.Influences = attackInfluences(p)
	}
	act.Roll = roll
	return one(piece{family: family, body: act})
}

// wardedPiece is a swing a ward stopped, and the save it forced.
func wardedPiece(payload []byte) ([]piece, error) {
	var p struct {
		outcomeFields
		Warded struct {
			Source string `json:"source"`
			Save   struct {
				Ability     string          `json:"ability"`
				Roll        int             `json:"roll"`
				Total       int             `json:"total"`
				DC          int             `json:"dc"`
				Succeeded   bool            `json:"succeeded"`
				Calculation json.RawMessage `json:"calculation"`
			} `json:"save"`
		} `json:"warded"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	act, family, err := attackBase(p.outcomeFields)
	if err != nil {
		return nil, err
	}
	save := p.Warded.Save
	roll, err := rollOf(save.Calculation, facesOf(save.Roll), save.Total, intp(save.DC), boolp(save.Succeeded))
	if err != nil {
		return nil, err
	}
	act.Outcome = AttackWarded
	act.Ward = &WardSave{Source: member(p.Warded.Source), Ability: save.Ability, Roll: *roll}
	return one(piece{family: family, body: act})
}

// castPiece opens a cast its result beats fold into.
func castPiece(payload []byte) ([]piece, error) {
	var p struct {
		Actor   string   `json:"actor"`
		Spell   SpellRef `json:"spell"`
		Target  string   `json:"target"`
		Targets []string `json:"targets"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	targets := p.Targets
	if p.Target != "" {
		targets = append([]string{p.Target}, targets...)
	}
	cast := CastAct{By: member(p.Actor), Spell: p.Spell}
	for _, t := range targets {
		cast.Targets = append(cast.Targets, member(t))
	}
	return one(piece{family: FamilyAct, body: cast, opens: true, continues: restartCast})
}

// restartCast is a cast's later segment: the same caster, spell and targets,
// with the results that follow.
func restartCast(prior ActBody) ActBody {
	c, ok := prior.(CastAct)
	if !ok {
		return CastAct{}
	}
	return CastAct{By: c.By, Spell: c.Spell, Targets: c.Targets}
}

// castResultPiece is a cast's miss or ward against one target.
func castResultPiece(beat string, payload []byte) ([]piece, error) {
	var p struct {
		Actor       string          `json:"actor"`
		Target      string          `json:"target"`
		Source      string          `json:"source"`
		Spell       SpellRef        `json:"spell"`
		Ability     string          `json:"ability"`
		Roll        int             `json:"roll"`
		Total       int             `json:"total"`
		DC          int             `json:"dc"`
		Calculation json.RawMessage `json:"calculation"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	result := CastResult{Target: member(p.Target), Outcome: CastMissed}
	if beat == encounter.BeatCastWarded {
		// The ward's save is the caster's, and the beat writes only a failed
		// one: a caster who saved would have cast.
		roll, err := rollOf(p.Calculation, facesOf(p.Roll), p.Total, intp(p.DC), boolp(false))
		if err != nil {
			return nil, err
		}
		result = CastResult{Target: member(p.Target), Outcome: CastWarded, Ability: p.Ability, Save: roll, Source: member(p.Source)}
	}
	alone := CastAct{By: member(p.Actor), Spell: p.Spell, Results: []CastResult{result}}
	return one(castResult(alone, result))
}

// castResult is a result piece: folded into the open cast, or a cast of its
// own when none is open.
func castResult(alone CastAct, result CastResult) piece {
	return piece{
		family: FamilyAct, body: alone, opens: true,
		absorb: func(open ActBody) (ActBody, bool) {
			c, ok := open.(CastAct)
			if !ok {
				return nil, false
			}
			c.Results = append(append([]CastResult(nil), c.Results...), result)
			return c, true
		},
		continues: func(prior ActBody) ActBody {
			c, ok := restartCast(prior).(CastAct)
			if !ok || c.By.ID == "" {
				return alone
			}
			c.Results = []CastResult{result}
			return c
		},
	}
}

// savedPiece is a save a cast or an activation forced.
func savedPiece(payload []byte) ([]piece, error) {
	var p struct {
		Saver       string          `json:"saver"`
		Ability     string          `json:"ability"`
		Roll        int             `json:"roll"`
		Total       int             `json:"total"`
		DC          int             `json:"dc"`
		Succeeded   bool            `json:"succeeded"`
		Calculation json.RawMessage `json:"calculation"`
		Source      SpellRef        `json:"source"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	roll, err := rollOf(p.Calculation, facesOf(p.Roll), p.Total, intp(p.DC), boolp(p.Succeeded))
	if err != nil {
		return nil, err
	}
	outcome := CastFailed
	if p.Succeeded {
		outcome = CastSaved
	}
	result := CastResult{Target: member(p.Saver), Outcome: outcome, Ability: p.Ability, Save: roll}
	save := SaveResult{Saver: member(p.Saver), Ability: p.Ability, Source: p.Source, Roll: *roll}
	cast := castResult(CastAct{Spell: p.Source, Results: []CastResult{result}}, result)
	castAbsorb := cast.absorb
	cast.absorb = func(open ActBody) (ActBody, bool) {
		if a, ok := open.(ActivateAct); ok {
			a.Saves = append(append([]SaveResult(nil), a.Saves...), save)
			return a, true
		}
		return castAbsorb(open)
	}
	return one(cast)
}

// activatedPiece opens an activation its results fold into.
func activatedPiece(payload []byte) ([]piece, error) {
	var p struct {
		Actor   string     `json:"actor"`
		Ability AbilityRef `json:"ability"`
		Target  string     `json:"target"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	act := ActivateAct{By: member(p.Actor), Ability: p.Ability, Target: member(p.Target)}
	return one(piece{family: FamilyAct, body: act, opens: true, continues: restartActivation})
}

// restartActivation is an activation's later segment: the same activation,
// with the results that follow.
func restartActivation(prior ActBody) ActBody {
	a, ok := prior.(ActivateAct)
	if !ok {
		return ActivateAct{}
	}
	return ActivateAct{By: a.By, Ability: a.Ability, Target: a.Target}
}

// resultFields are an activation result's keys (activation-result `result`,
// and the removals a rest, a departure or an ending carries).
type resultFields struct {
	Kind          string          `json:"kind"`
	Target        string          `json:"target"`
	Ref           string          `json:"ref"`
	Name          string          `json:"name"`
	SourceID      string          `json:"source_id"`
	Amount        int             `json:"amount"`
	Requested     int             `json:"requested"`
	Before        json.RawMessage `json:"before"`
	After         json.RawMessage `json:"after"`
	Calculation   json.RawMessage `json:"calculation"`
	DamageType    string          `json:"damage_type"`
	Detail        string          `json:"detail"`
	Description   string          `json:"description"`
	Reason        string          `json:"reason"`
	Moved         int             `json:"moved"`
	StoppedBy     string          `json:"stopped_by"`
	Stabilization *struct {
		Before            string `json:"before"`
		After             string `json:"after"`
		HitPoints         int    `json:"hit_points"`
		Successes         int    `json:"successes"`
		Failures          int    `json:"failures"`
		SuccessesNeeded   int    `json:"successes_needed"`
		FailuresRemaining int    `json:"failures_remaining"`
		Stabilized        bool   `json:"stabilized"`
		Dead              bool   `json:"dead"`
	} `json:"stabilization"`
}

// removal is a condition-removed result as a state entry.
func (r resultFields) removal() StateChange {
	return StateChange{
		Party: member(r.Target), Change: StateConditionRemoved,
		Condition: &ConditionLifted{Ref: r.Ref, Name: r.Name, SourceID: r.SourceID, Reason: r.Reason},
	}
}

// activationResultPiece is one result. A removed condition is a state entry;
// every other result folds into the open activation.
func activationResultPiece(payload []byte) ([]piece, error) {
	var p struct {
		Actor  string       `json:"actor"`
		Result resultFields `json:"result"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	r := p.Result
	if r.Kind == string(encounter.ResultConditionRemoved) {
		return one(statePiece(r.removal()))
	}
	result := ActivationResult{
		Kind: r.Kind, Target: member(r.Target), Ref: r.Ref, Name: r.Name, SourceID: r.SourceID,
		Amount: r.Amount, Requested: r.Requested, DamageType: r.DamageType, Detail: r.Detail,
		Description: r.Description, Moved: r.Moved, StoppedBy: r.StoppedBy,
	}
	// before and after are numbers on a healing or a damage; a stabilization
	// writes its own inside its detail.
	_ = json.Unmarshal(r.Before, &result.Before)
	_ = json.Unmarshal(r.After, &result.After)
	if len(r.Calculation) > 0 {
		roll, err := rollOf(r.Calculation, nil, r.Requested, nil, nil)
		if err != nil {
			return nil, err
		}
		result.Roll = roll
	}
	if s := r.Stabilization; s != nil {
		result.Stabilization = &Stabilization{
			Before: s.Before, After: s.After, HitPoints: s.HitPoints,
			Successes: s.Successes, Failures: s.Failures,
			SuccessesNeeded: s.SuccessesNeeded, FailuresRemaining: s.FailuresRemaining,
			Stabilized: s.Stabilized, Dead: s.Dead,
		}
	}
	alone := ActivateAct{By: member(p.Actor), Results: []ActivationResult{result}}
	return one(piece{
		family: FamilyAct, body: alone, opens: true,
		absorb: func(open ActBody) (ActBody, bool) {
			a, ok := open.(ActivateAct)
			if !ok {
				return nil, false
			}
			a.Results = append(append([]ActivationResult(nil), a.Results...), result)
			return a, true
		},
		continues: func(prior ActBody) ActBody {
			// A cast's results (Bane's Baned) follow its cast under the cast's
			// act id: the activation is the spell's (#643 F, rpg-toolkit#2100).
			if c, ok := prior.(CastAct); ok {
				return ActivateAct{
					By: c.By, Ability: AbilityRef{Ref: c.Spell.Ref, Name: c.Spell.Name},
					Results: []ActivationResult{result},
				}
			}
			a, ok := restartActivation(prior).(ActivateAct)
			if !ok || a.By.ID == "" {
				return alone
			}
			a.Results = []ActivationResult{result}
			return a
		},
	})
}

// spokePiece opens a line its answer folds into.
func spokePiece(payload []byte) ([]piece, error) {
	var p struct {
		Actor       string          `json:"actor"`
		Target      string          `json:"target"`
		Prop        string          `json:"prop"`
		Option      string          `json:"option"`
		Line        string          `json:"line"`
		Skill       string          `json:"skill"`
		DC          int             `json:"dc"`
		Total       int             `json:"total"`
		Beaten      bool            `json:"beaten"`
		Calculation json.RawMessage `json:"calculation"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	to := member(p.Target)
	if p.Prop != "" {
		to = prop(p.Prop)
	}
	roll, err := rollOf(p.Calculation, nil, p.Total, intp(p.DC), boolp(p.Beaten))
	if err != nil {
		return nil, err
	}
	act := SpeakAct{By: member(p.Actor), To: to, Option: p.Option, Line: p.Line, Skill: p.Skill, Roll: roll}
	return one(piece{family: FamilyAct, body: act, opens: true})
}

// answeredPiece is a table's answer: folded into the line it answers when it
// sits under a speak act with the line open, a table answer otherwise.
func answeredPiece(tag string, payload []byte) ([]piece, error) {
	var p struct {
		Creature   string `json:"creature"`
		Prop       string `json:"prop"`
		Option     string `json:"option"`
		Key        string `json:"key"`
		Word       string `json:"word"`
		Selector   string `json:"selector"`
		Say        string `json:"say"`
		Temper     string `json:"temper"`
		Roll       int    `json:"roll"`
		Of         int    `json:"of"`
		Entry      int    `json:"entry"`
		Candidates []struct {
			Entry   int `json:"entry"`
			Weight  int `json:"weight"`
			Percent int `json:"percent"`
			Loaded  int `json:"loaded"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	who := member(p.Creature)
	if p.Prop != "" {
		who = prop(p.Prop)
	}
	answer := TableAnswer{
		Who: who, Option: p.Option, Key: p.Key, Word: p.Word, Selector: p.Selector,
		Say: p.Say, Temperament: p.Temper, Entry: p.Entry,
	}
	if p.Of > 0 {
		answer.Roll = &Roll{Dice: facesOf(p.Roll), Total: p.Roll, Of: p.Of}
	}
	for _, c := range p.Candidates {
		answer.Candidates = append(answer.Candidates, TableCandidate{
			Entry: c.Entry, Weight: c.Weight, Percent: c.Percent, Loaded: c.Loaded,
		})
	}
	pc := piece{family: FamilyTableAnswer, body: answer}
	if strings.HasPrefix(tag, string(VerbSpeak)+":") {
		pc.into = FamilyAct
		pc.absorb = func(open ActBody) (ActBody, bool) {
			s, ok := open.(SpeakAct)
			if !ok || s.Answer != nil {
				return nil, false
			}
			s.Answer = &answer
			return s, true
		}
	}
	return one(pc)
}

// temperedPiece is a temperament deal: a table roll of its own, dealt when
// the creature entered the run.
func temperedPiece(payload []byte) ([]piece, error) {
	var p struct {
		Member  string `json:"member"`
		Faction string `json:"faction"`
		Temper  string `json:"temper"`
		Roll    int    `json:"roll"`
		Of      int    `json:"of"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	faction := Party{}
	if p.Faction != "" {
		faction = Party{Kind: PartyFaction, ID: p.Faction}
	}
	return one(piece{family: FamilyTableAnswer, body: TableAnswer{
		Who: member(p.Member),
		Dealt: &Temper{
			Faction: faction, Temper: p.Temper,
			Roll: Roll{Dice: facesOf(p.Roll), Total: p.Roll, Of: p.Of},
		},
	}})
}

// deathSavePiece is one death save.
func deathSavePiece(payload []byte) ([]piece, error) {
	var p struct {
		Actor     string `json:"actor"`
		DeathSave struct {
			Roll              int             `json:"roll"`
			Outcome           string          `json:"outcome"`
			SuccessesAdded    int             `json:"successes_added"`
			FailuresAdded     int             `json:"failures_added"`
			Successes         int             `json:"successes"`
			Failures          int             `json:"failures"`
			SuccessesNeeded   int             `json:"successes_needed"`
			FailuresRemaining int             `json:"failures_remaining"`
			Stabilized        bool            `json:"stabilized"`
			Dead              bool            `json:"dead"`
			Recovered         bool            `json:"recovered"`
			HPRestored        int             `json:"hp_restored"`
			Continuation      string          `json:"continuation"`
			PresentationID    string          `json:"presentation_id"`
			Calculation       json.RawMessage `json:"calculation"`
		} `json:"death_save"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	d := p.DeathSave
	roll, err := rollOf(d.Calculation, facesOf(d.Roll), d.Roll, nil, nil)
	if err != nil {
		return nil, err
	}
	return one(actPiece(DeathSaveAct{
		By: member(p.Actor), Roll: roll, Outcome: d.Outcome,
		SuccessesAdded: d.SuccessesAdded, FailuresAdded: d.FailuresAdded,
		Successes: d.Successes, Failures: d.Failures,
		SuccessesNeeded: d.SuccessesNeeded, FailuresRemaining: d.FailuresRemaining,
		Stabilized: d.Stabilized, Dead: d.Dead, Recovered: d.Recovered,
		HPRestored: d.HPRestored, Continuation: d.Continuation, PresentationID: d.PresentationID,
	}))
}

// doorPiece is a door changing, by whoever the beat names.
func doorPiece(payload []byte) ([]piece, error) {
	var p struct {
		Door        string          `json:"door"`
		State       string          `json:"state"`
		Actor       string          `json:"actor"`
		DC          *int            `json:"dc"`
		Total       int             `json:"total"`
		Beaten      bool            `json:"beaten"`
		Calculation json.RawMessage `json:"calculation"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	act := DoorAct{By: member(p.Actor), Door: p.Door, State: p.State}
	if p.DC != nil {
		roll, err := rollOf(p.Calculation, nil, p.Total, p.DC, boolp(p.Beaten))
		if err != nil {
			return nil, err
		}
		act.Roll = roll
	}
	return one(actPiece(act))
}

// snuckPiece is a Stealth roll against one watcher.
func snuckPiece(payload []byte) ([]piece, error) {
	var p struct {
		Member      string          `json:"member"`
		Watcher     string          `json:"watcher"`
		DC          int             `json:"dc"`
		Total       int             `json:"total"`
		Beaten      bool            `json:"beaten"`
		Calculation json.RawMessage `json:"calculation"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	roll, err := rollOf(p.Calculation, nil, p.Total, intp(p.DC), boolp(p.Beaten))
	if err != nil {
		return nil, err
	}
	return one(actPiece(SneakAct{By: member(p.Member), Watcher: member(p.Watcher), Roll: roll}))
}

// restedPieces is a rest, then a state entry for each concentration it ended
// and each condition it lifted.
func restedPieces(payload []byte) ([]piece, error) {
	var p struct {
		Member             string            `json:"member"`
		Kind               string            `json:"kind"`
		HitPointsRestored  int               `json:"hit_points_restored"`
		HitPoints          int               `json:"hit_points"`
		HitDiceSpent       int               `json:"hit_dice_spent"`
		HitDiceReturned    int               `json:"hit_dice_returned"`
		HitDiceRemaining   int               `json:"hit_dice_remaining"`
		ResourcesRefilled  []string          `json:"resources_refilled"`
		Calculation        json.RawMessage   `json:"calculation"`
		ConcentrationEnded []json.RawMessage `json:"concentration_ended"`
		Ended              []resultFields    `json:"ended"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	rest := RestAct{
		By: member(p.Member), Kind: p.Kind,
		HitPointsRestored: p.HitPointsRestored, HitPoints: p.HitPoints,
		HitDiceSpent: p.HitDiceSpent, HitDiceReturned: p.HitDiceReturned, HitDiceRemaining: p.HitDiceRemaining,
		ResourcesRefilled: p.ResourcesRefilled,
	}
	if len(p.Calculation) > 0 {
		roll, err := rollOf(p.Calculation, nil, p.HitPointsRestored, nil, nil)
		if err != nil {
			return nil, err
		}
		rest.Roll = roll
	}
	pieces := []piece{actPiece(rest)}
	for _, raw := range p.ConcentrationEnded {
		state, err := concentrationState(raw)
		if err != nil {
			return nil, err
		}
		pieces = append(pieces, statePiece(state))
		var held struct {
			Removed []resultFields `json:"removed"`
		}
		if err := json.Unmarshal(raw, &held); err != nil {
			return nil, err
		}
		for _, r := range held.Removed {
			pieces = append(pieces, statePiece(r.removal()))
		}
	}
	for _, r := range p.Ended {
		if r.Target == "" {
			r.Target = p.Member
		}
		pieces = append(pieces, statePiece(r.removal()))
	}
	return pieces, nil
}

// concentrationState is a concentration's end (concentration_ended, and each
// of a rest's `concentration_ended`).
func concentrationState(payload []byte) (StateChange, error) {
	var p struct {
		Caster string   `json:"caster"`
		Spell  SpellRef `json:"spell"`
		Reason string   `json:"reason"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return StateChange{}, err
	}
	return StateChange{
		Party: member(p.Caster), Change: StateConcentrationEnded,
		Concentration: &ConcentrationEnded{Spell: p.Spell, Reason: p.Reason},
	}, nil
}

// stancePiece is two factions' stance turning.
func stancePiece(payload []byte) ([]piece, error) {
	var p struct {
		Between []string `json:"between"`
		Stance  string   `json:"stance"`
		Cause   string   `json:"cause"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	turn := &StanceTurn{Stance: p.Stance, Cause: p.Cause}
	for _, f := range p.Between {
		turn.Between = append(turn.Between, Party{Kind: PartyFaction, ID: f})
	}
	return one(statePiece(StateChange{Change: StateStance, Stance: turn}))
}

// experiencePiece is experience paid out, named by the grant's source.
func experiencePiece(payload []byte) ([]piece, error) {
	var p struct {
		Experience struct {
			Member string `json:"member"`
			Grants []struct {
				Character string `json:"character"`
				Amount    int    `json:"amount"`
				Total     int    `json:"total"`
			} `json:"grants"`
		} `json:"experience"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	state := StateChange{Party: member(p.Experience.Member), Change: StateExperience}
	for _, g := range p.Experience.Grants {
		state.Experience = append(state.Experience, ExperienceGrant{Character: g.Character, Amount: g.Amount, Total: g.Total})
	}
	return one(statePiece(state))
}

// perceptionPiece is what the observer — the recipient — perceived.
func perceptionPiece(recipient, beat string, payload []byte) ([]piece, error) {
	out := Perception{Observer: member(recipient)}
	switch beat {
	case encounter.BeatSighted:
		var p SightChange
		var keys struct {
			Gained           []string `json:"gained"`
			Lost             []string `json:"lost"`
			Changed          []string `json:"changed"`
			KnowledgeChanged bool     `json:"knowledge_changed"`
		}
		if err := json.Unmarshal(payload, &keys); err != nil {
			return nil, err
		}
		p = SightChange{Gained: keys.Gained, Lost: keys.Lost, Changed: keys.Changed, KnowledgeChanged: keys.KnowledgeChanged}
		out.Sense, out.Sighted = SenseSight, &p
	case encounter.BeatHeard:
		var p struct {
			Kind     string           `json:"kind"`
			Position spatial.Position `json:"position"`
			Hops     int              `json:"hops"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, err
		}
		out.Sense, out.Heard = SenseHearing, &Sound{Kind: p.Kind, Cell: p.Position, Hops: p.Hops}
	case encounter.BeatDiscoveryChecked:
		var p struct {
			Ability     string          `json:"ability"`
			Beaten      bool            `json:"beaten"`
			Total       int             `json:"total"`
			Calculation json.RawMessage `json:"calculation"`
		}
		if err := json.Unmarshal(payload, &p); err != nil {
			return nil, err
		}
		roll, err := rollOf(p.Calculation, nil, p.Total, nil, nil)
		if err != nil {
			return nil, err
		}
		// The check is against the secrets' DCs, which the beat does not
		// name; whether it found anything is its own `beaten`.
		roll.Beaten = boolp(p.Beaten)
		out.Sense, out.Discovery = SenseDiscovery, &DiscoveryCheck{Ability: p.Ability, Roll: roll}
	case encounter.BeatConcealmentRevealed:
		revealed, err := secretRevealed(payload)
		if err != nil {
			return nil, err
		}
		out.Sense, out.Revealed = SenseReveal, revealed
	default:
		cells, err := cellsLearned(payload)
		if err != nil {
			return nil, err
		}
		out.Sense, out.Cells = SenseCells, cells
	}
	return one(piece{family: FamilyPerception, body: out})
}

// secretRevealed reads a concealment's contents.
func secretRevealed(payload []byte) (*SecretRevealed, error) {
	var p struct {
		Concealment string             `json:"concealment"`
		Cells       []spatial.Position `json:"cells"`
		Props       []AtlasProp        `json:"props"`
		Doors       []RevealedDoor     `json:"doors"`
		Regions     []AtlasRegion      `json:"regions"`
		Boundaries  []AtlasBoundary    `json:"boundaries"`
		Segments    []AtlasSegment     `json:"segments"`
		Sealed      []spatial.Position `json:"sealed"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	rows, ok := structuralRowsFromPayload(payload)
	if !ok {
		return nil, fmt.Errorf("structural rows do not read")
	}
	presentations, ok := propPresentationsFromPayload(payload)
	if !ok {
		return nil, fmt.Errorf("prop presentations do not read")
	}
	// The beat hangs each door's doorways under the door and names only the
	// crossing; the act names the door on every doorway, as v1's projection
	// did (#643 E). A doorway that names its own door keeps it.
	for i := range p.Doors {
		for j := range p.Doors[i].Doorways {
			if p.Doors[i].Doorways[j].Door == "" {
				p.Doors[i].Doorways[j].Door = p.Doors[i].Door
			}
		}
	}
	return &SecretRevealed{
		Concealment: p.Concealment, Cells: p.Cells, Props: p.Props, Doors: p.Doors,
		Regions: p.Regions, Boundaries: p.Boundaries, Segments: p.Segments, Sealed: p.Sealed,
		StructuralWalls: rows.Walls, StructuralDoors: rows.Doors,
		StructuralWallOpeningsReplacements: rows.Replacements, PropPresentations: presentations,
	}, nil
}

// cellsLearned reads layout newly learned, a legacy room reveal included.
func cellsLearned(payload []byte) (*CellsLearned, error) {
	var p struct {
		Regions    []AtlasRegion      `json:"regions"`
		Region     *AtlasRegion       `json:"region"`
		Scenery    []spatial.Position `json:"scenery"`
		Props      []AtlasProp        `json:"props"`
		Boundaries []AtlasBoundary    `json:"boundaries"`
		Segments   []AtlasSegment     `json:"segments"`
		Sealed     []spatial.Position `json:"sealed"`
		Doorways   []AtlasDoorway     `json:"doorways"`
		Exits      []AtlasExit        `json:"exits"`
		Placed     json.RawMessage    `json:"placed"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	fixed, ok := revealedFixedRows(payload)
	if !ok {
		return nil, fmt.Errorf("fixed rows do not read")
	}
	regions := p.Regions
	if p.Region != nil {
		regions = append(regions, *p.Region)
	}
	return &CellsLearned{
		Regions: regions, Scenery: p.Scenery, Props: p.Props, Boundaries: p.Boundaries,
		Segments: p.Segments, Sealed: p.Sealed, Doorways: p.Doorways, Placed: fixed.placed, Exits: p.Exits,
		StructuralWalls: fixed.rows.Walls, StructuralDoors: fixed.rows.Doors,
		StructuralWallOpeningsReplacements: fixed.rows.Replacements, PropPresentations: fixed.presentations,
	}, nil
}

// clockPieces is the table's bookkeeping, plus a state entry for each
// condition a departure or an ending lifted.
//
//nolint:gocyclo // one arm per clock beat
func clockPieces(beat string, payload []byte) ([]piece, error) {
	var p struct {
		Tick      uint64           `json:"tick"`
		Member    string           `json:"member"`
		Next      string           `json:"next"`
		Order     []string         `json:"order"`
		Surprised []string         `json:"surprised"`
		Members   []string         `json:"members"`
		Cause     string           `json:"cause"`
		Holding   []string         `json:"holding"`
		Exit      string           `json:"exit"`
		ID        string           `json:"id"`
		Kind      string           `json:"kind"`
		Cell      spatial.Position `json:"cell"`
		From      spatial.Position `json:"from"`
		To        spatial.Position `json:"to"`
		Ending    string           `json:"ending"`
		Outcome   string           `json:"outcome"`
		Ended     json.RawMessage  `json:"ended"`
		Windows   []struct {
			Audience string       `json:"audience"`
			Reaction beatReaction `json:"reaction"`
		} `json:"windows"`
		PresentationID string          `json:"presentation_id"`
		Audience       string          `json:"audience"`
		Offer          beatReaction    `json:"offer"`
		Roll           int             `json:"roll"`
		Total          int             `json:"total"`
		Calculation    json.RawMessage `json:"calculation"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, err
	}
	parties := func(ids []string) []Party {
		var out []Party
		for _, id := range ids {
			out = append(out, member(id))
		}
		return out
	}
	entry := ClockEntry{}
	var lifted []piece
	switch beat {
	case encounter.BeatTick:
		entry.Mark, entry.Tick = ClockTick, p.Tick
	case encounter.BeatTurnEnded:
		entry.Mark, entry.Turn = ClockTurnEnded, &TurnPassed{Member: member(p.Member), Next: member(p.Next)}
	case encounter.BeatFightStarted:
		entry.Mark, entry.Fight = ClockFightFormed, &FightFormed{Order: parties(p.Order), Surprised: parties(p.Surprised)}
	case encounter.BeatFightEnded:
		entry.Mark, entry.Dissolved = ClockFightDissolved, &FightDissolved{Members: parties(p.Members), Cause: p.Cause}
	case encounter.BeatJoined:
		entry.Mark, entry.Member = ClockJoined, member(p.Member)
	case encounter.BeatExited:
		entry.Mark = ClockExited
		entry.Departure = &Departure{Member: member(p.Member), Holding: p.Holding, Exit: p.Exit}
		var ended []resultFields
		if len(p.Ended) > 0 {
			if err := json.Unmarshal(p.Ended, &ended); err != nil {
				return nil, err
			}
		}
		lifted = removals(p.Member, ended)
	case encounter.BeatArrived:
		entry.Mark, entry.Arrival = ClockArrived, &Arrival{ID: p.ID, Kind: p.Kind, Cell: p.Cell}
	case encounter.BeatSceneOpened:
		entry.Mark = ClockSceneOpened
	case encounter.BeatEnded:
		entry.Mark, entry.Ending = ClockEnded, p.Ending
		// The ending's declared result rides the beat; absent on an ending
		// that declares none, so the zero value carries "no result".
		entry.Outcome = EndingResult(p.Outcome)
		var ended map[string][]resultFields
		if len(p.Ended) > 0 {
			if err := json.Unmarshal(p.Ended, &ended); err != nil {
				return nil, err
			}
		}
		for _, holder := range sortedKeys(ended) {
			lifted = append(lifted, removals(holder, ended[holder])...)
		}
	case encounter.BeatWindowOpened:
		w := &WindowOpened{Member: member(p.Member), From: p.From, To: p.To}
		for _, o := range p.Windows {
			w.Windows = append(w.Windows, WindowOffer{
				Audience: member(o.Audience), Reaction: ReactionRef{Ref: o.Reaction.Ref, Name: o.Reaction.Name},
			})
		}
		entry.Mark, entry.Window = ClockWindowOpened, w
	case encounter.BeatRollWindowOpened:
		roll, err := rollOf(p.Calculation, facesOf(p.Roll), p.Total, nil, nil)
		if err != nil {
			return nil, err
		}
		entry.Mark = ClockRollWindowOpened
		entry.RollWindow = &RollWindowOpened{
			PresentationID: p.PresentationID, Audience: member(p.Audience),
			Offer: ReactionRef{Ref: p.Offer.Ref, Name: p.Offer.Name}, Roll: roll,
		}
	default:
		return nil, fmt.Errorf("no clock mark")
	}
	return append([]piece{{family: FamilyClock, body: entry}}, lifted...), nil
}

// removals are conditions lifted from one holder, each a state entry.
func removals(holder string, ended []resultFields) []piece {
	var out []piece
	for _, r := range ended {
		if r.Target == "" {
			r.Target = holder
		}
		out = append(out, statePiece(r.removal()))
	}
	return out
}

// sortedKeys is a map's keys in order, so a fold is the same twice.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
