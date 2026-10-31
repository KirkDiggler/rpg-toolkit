// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// beat_decode.go is the strict payload decoding the act fold shares: roll
// traces, damage components, an attack's and a reaction's references, and
// the JSON shape guards. It outlived the v1alpha1 projection it was written
// for (rpg-project#631), whose bodies decoded through the same functions.

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// strictJSONObject walks one JSON object key by key, retaining each field's
// raw value and refusing the ambiguities encoding/json resolves silently:
// a repeated key at the depth it walks, a payload that is not exactly one
// JSON object, and any trailing content after it. It runs at the OUTER beat
// payload boundary and over the nested bodies this package routes through
// strict decoding — activation results, damage components, and their roll
// graphs. Bodies no decoder routes through it (the shared attack-scalar
// pass, advantage/disadvantage source entries) keep encoding/json's tolerant
// decoding, because a body that says a field twice has no lawful reading
// only where this package vouches for the shape.
func strictJSONObject(payload []byte) (map[string]json.RawMessage, bool) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return nil, false
	}

	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		token, tokenErr := decoder.Token()
		if tokenErr != nil {
			return nil, false
		}
		key, isString := token.(string)
		if !isString {
			return nil, false
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, false
		}

		var value json.RawMessage
		if decodeErr := decoder.Decode(&value); decodeErr != nil {
			return nil, false
		}
		fields[key] = value
	}

	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') {
		return nil, false
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, false
	}
	return fields, true
}

// isJSONNull reports whether a retained raw value is the null literal —
// present, but carrying no value. Inside a decoded roll body null is never an
// answer: the composition writes absence by omitting a key, so a null is a
// malformed payload, not a zero.
func isJSONNull(raw json.RawMessage) bool {
	return string(raw) == "null"
}

// decodeActivityCue reads the captured two-field cue without consulting live
// content. Present malformed data is refused rather than converted to absence.
func decodeActivityCue(raw json.RawMessage) (*ActivityCue, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok || len(fields) != 2 {
		return nil, false
	}
	var cue ActivityCue
	if err := json.Unmarshal(fields["ref"], &cue.Ref); err != nil || cue.Ref == "" {
		return nil, false
	}
	if err := json.Unmarshal(fields["cycles"], &cue.Cycles); err != nil || cue.Cycles == 0 {
		return nil, false
	}
	return &cue, true
}

// decodeRollCalculation decodes one persisted roll calculation into the SDK's
// string-ref shape, refusing unknown keys, nulls, duplicate keys at every
// nested depth, and any structural or arithmetic inconsistency the
// composition's own validator replays: face ranges, ordered rerolls, kept
// indices, subtotals, and the total.
func decodeRollCalculation(raw json.RawMessage) (*RollCalculation, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return nil, false
	}
	for key := range fields {
		switch key {
		case "components", "total":
		default:
			return nil, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return nil, false
		}
	}
	componentsRaw, componentsPresent := fields["components"]
	totalRaw, totalPresent := fields["total"]
	if !componentsPresent || !totalPresent {
		return nil, false
	}

	var elements []json.RawMessage
	if json.Unmarshal(componentsRaw, &elements) != nil {
		return nil, false
	}
	calculation := &RollCalculation{}
	for _, element := range elements {
		component, ok := decodeRollComponent(element)
		if !ok {
			return nil, false
		}
		calculation.Components = append(calculation.Components, component)
	}
	if json.Unmarshal(totalRaw, &calculation.Total) != nil {
		return nil, false
	}
	if encounter.ValidateRollCalculation(validationCalculationFor(calculation)) != nil {
		return nil, false
	}
	return calculation, true
}

// decodeRollComponent decodes one persisted roll component, refusing unknown
// keys, nulls, and duplicate keys, and — when the component rolls dice — every
// structural and arithmetic inconsistency the composition's validator can
// replay. A component that contributes no dice and no modifier (the
// multiplier-only trait component's shape) carries only its sourced identity,
// and there is no arithmetic to replay.
func decodeRollComponent(raw json.RawMessage) (RollComponent, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return RollComponent{}, false
	}
	for key := range fields {
		switch key {
		case "source", "dice", "modifier", "subtract_dice":
		default:
			return RollComponent{}, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return RollComponent{}, false
		}
	}
	sourceRaw, sourcePresent := fields["source"]
	if !sourcePresent {
		return RollComponent{}, false
	}
	source, ok := decodeRollSource(sourceRaw)
	if !ok {
		return RollComponent{}, false
	}
	component := RollComponent{Source: source}
	if diceRaw, present := fields["dice"]; present {
		trace, ok := decodeDiceTrace(diceRaw)
		if !ok {
			return RollComponent{}, false
		}
		component.Dice = trace
	}
	if modifierRaw, present := fields["modifier"]; present {
		var modifier int
		if json.Unmarshal(modifierRaw, &modifier) != nil {
			return RollComponent{}, false
		}
		component.Modifier = &modifier
	}
	if subtractRaw, present := fields["subtract_dice"]; present {
		if json.Unmarshal(subtractRaw, &component.SubtractDice) != nil {
			return RollComponent{}, false
		}
	}
	if component.Dice == nil && component.Modifier == nil {
		// No rollable fact: nothing to replay. The source identity is all this
		// decoder can vouch for, and the damage facts beside the roll — a
		// multiplier, on the damage carrier — say whether the component
		// contributed at all.
		return component, true
	}
	if encounter.ValidateRollCalculation(&encounter.RollCalculation{
		Components: []encounter.RollComponent{validationComponentFor(component)},
		Total:      rollComponentTotal(component),
	}) != nil {
		return RollComponent{}, false
	}
	return component, true
}

// decodeRollSource decodes one sourced identity, requiring the canonical ref
// and display name and refusing nulls, duplicates, and unknown keys.
func decodeRollSource(raw json.RawMessage) (RollSource, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return RollSource{}, false
	}
	for key := range fields {
		switch key {
		case "ref", "name", "label", "source_id":
		default:
			return RollSource{}, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return RollSource{}, false
		}
	}
	var source RollSource
	refRaw, refPresent := fields["ref"]
	if !refPresent || json.Unmarshal(refRaw, &source.Ref) != nil || source.Ref == "" {
		return RollSource{}, false
	}
	// Canonical ref syntax, the same law the composition's own write-time
	// validation applies to every persisted roll source: a ref this package
	// cannot parse is a corrupted record, not a fact to pass through. The
	// dice-bearing paths replay it again through the composition's
	// validator; a multiplier-only roll's identity is checked HERE, because
	// there is no trace for that validator to reach it through.
	if _, err := core.ParseString(source.Ref); err != nil {
		return RollSource{}, false
	}
	nameRaw, namePresent := fields["name"]
	if !namePresent || json.Unmarshal(nameRaw, &source.Name) != nil || strings.TrimSpace(source.Name) == "" {
		return RollSource{}, false
	}
	if labelRaw, labelPresent := fields["label"]; labelPresent {
		if json.Unmarshal(labelRaw, &source.Label) != nil {
			return RollSource{}, false
		}
	}
	if sourceIDRaw, sourceIDPresent := fields["source_id"]; sourceIDPresent {
		if json.Unmarshal(sourceIDRaw, &source.SourceID) != nil {
			return RollSource{}, false
		}
	}
	return source, true
}

// decodeDiceTrace decodes one persisted dice pool trace. Faces, rerolls, and
// kept indices are retained exactly as written; their arithmetic is replayed
// by decodeRollComponent's validator, which owns the refusal.
func decodeDiceTrace(raw json.RawMessage) (*DiceTrace, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return nil, false
	}
	for key := range fields {
		switch key {
		case "notation", "die_size", "original_rolls", "rerolls", "final_rolls",
			"kept_indices", "subtotal", "keep":
		default:
			return nil, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return nil, false
		}
	}
	trace := &DiceTrace{}
	notationRaw, notationPresent := fields["notation"]
	if !notationPresent || json.Unmarshal(notationRaw, &trace.Notation) != nil {
		return nil, false
	}
	dieSizeRaw, dieSizePresent := fields["die_size"]
	if !dieSizePresent || json.Unmarshal(dieSizeRaw, &trace.DieSize) != nil {
		return nil, false
	}
	originalRaw, originalPresent := fields["original_rolls"]
	if !originalPresent || json.Unmarshal(originalRaw, &trace.OriginalRolls) != nil ||
		len(trace.OriginalRolls) == 0 {
		return nil, false
	}
	finalRaw, finalPresent := fields["final_rolls"]
	if !finalPresent || json.Unmarshal(finalRaw, &trace.FinalRolls) != nil {
		return nil, false
	}
	subtotalRaw, subtotalPresent := fields["subtotal"]
	if !subtotalPresent || json.Unmarshal(subtotalRaw, &trace.Subtotal) != nil {
		return nil, false
	}
	if rerollsRaw, rerollsPresent := fields["rerolls"]; rerollsPresent {
		var elements []json.RawMessage
		if json.Unmarshal(rerollsRaw, &elements) != nil {
			return nil, false
		}
		for _, element := range elements {
			reroll, ok := decodeDiceReroll(element)
			if !ok {
				return nil, false
			}
			trace.Rerolls = append(trace.Rerolls, reroll)
		}
	}
	if keptRaw, keptPresent := fields["kept_indices"]; keptPresent {
		if json.Unmarshal(keptRaw, &trace.KeptIndices) != nil {
			return nil, false
		}
	}
	if keepRaw, keepPresent := fields["keep"]; keepPresent {
		keep, ok := decodeDiceKeep(keepRaw)
		if !ok {
			return nil, false
		}
		trace.Keep = keep
	}
	return trace, true
}

// decodeDiceKeep decodes the record that says why one face counted, refusing
// unknown keys, nulls and duplicates like every other decoder here. Whether
// the record DESCRIBES the pool it sits on — advantage kept the highest of at
// least two faces, a cancellation kept none and names both sides — is replayed
// by the composition's validator once the whole calculation is assembled.
func decodeDiceKeep(raw json.RawMessage) (*DiceKeep, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return nil, false
	}
	for key := range fields {
		switch key {
		case "rule", "granted", "imposed":
		default:
			return nil, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return nil, false
		}
	}
	ruleRaw, rulePresent := fields["rule"]
	if !rulePresent {
		return nil, false
	}
	keep := &DiceKeep{}
	if json.Unmarshal(ruleRaw, &keep.Rule) != nil {
		return nil, false
	}
	granted, ok := decodeKeepSources(fields["granted"])
	if !ok {
		return nil, false
	}
	imposed, ok := decodeKeepSources(fields["imposed"])
	if !ok {
		return nil, false
	}
	keep.Granted, keep.Imposed = granted, imposed
	return keep, true
}

// decodeKeepSources decodes one side of a keep record. An absent key is an
// empty side, which is a real answer: advantage has nothing imposed.
func decodeKeepSources(raw json.RawMessage) ([]RollSource, bool) {
	if raw == nil {
		return nil, true
	}
	var elements []json.RawMessage
	if json.Unmarshal(raw, &elements) != nil {
		return nil, false
	}
	sources := make([]RollSource, 0, len(elements))
	for _, element := range elements {
		source, ok := decodeRollSource(element)
		if !ok {
			return nil, false
		}
		sources = append(sources, source)
	}
	return sources, true
}

// decodeDiceReroll decodes one ordered die replacement, requiring its index,
// faces, and source; the replay that checks the faces against the pool is
// decodeRollComponent's.
func decodeDiceReroll(raw json.RawMessage) (DiceReroll, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return DiceReroll{}, false
	}
	for key := range fields {
		switch key {
		case "die_index", "before", "after", "source":
		default:
			return DiceReroll{}, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return DiceReroll{}, false
		}
	}
	reroll := DiceReroll{}
	dieIndexRaw, dieIndexPresent := fields["die_index"]
	if !dieIndexPresent || json.Unmarshal(dieIndexRaw, &reroll.DieIndex) != nil {
		return DiceReroll{}, false
	}
	beforeRaw, beforePresent := fields["before"]
	if !beforePresent || json.Unmarshal(beforeRaw, &reroll.Before) != nil {
		return DiceReroll{}, false
	}
	afterRaw, afterPresent := fields["after"]
	if !afterPresent || json.Unmarshal(afterRaw, &reroll.After) != nil {
		return DiceReroll{}, false
	}
	sourceRaw, sourcePresent := fields["source"]
	if !sourcePresent {
		return DiceReroll{}, false
	}
	source, ok := decodeRollSource(sourceRaw)
	if !ok {
		return DiceReroll{}, false
	}
	reroll.Source = source
	return reroll, true
}

// rollComponentTotal totals one component's roll facts the way the
// composition's validator does: the dice trace's authoritative subtotal plus
// any present modifier. It never resumms recorded faces.
func rollComponentTotal(component RollComponent) int {
	total := 0
	if component.Dice != nil {
		if component.SubtractDice {
			total -= component.Dice.Subtotal
		} else {
			total += component.Dice.Subtotal
		}
	}
	if component.Modifier != nil {
		total += *component.Modifier
	}
	return total
}

// validationComponentFor projects the SDK's string-ref roll component onto the
// composition's validator shape. The two carry identical fields, so the
// projection is mechanical — and one-directional: it exists so replay
// arithmetic lives in exactly one module instead of a copy here.
func validationComponentFor(component RollComponent) encounter.RollComponent {
	return encounter.RollComponent{
		Source: encounter.RollSource{
			Ref: component.Source.Ref, Name: component.Source.Name, Label: component.Source.Label,
			SourceID: component.Source.SourceID,
		},
		Dice:         validationDiceTraceFor(component.Dice),
		Modifier:     cloneInt(component.Modifier),
		SubtractDice: component.SubtractDice,
	}
}

// validationCalculationFor projects the SDK's string-ref calculation onto the
// composition's validator shape, components in order.
func validationCalculationFor(calculation *RollCalculation) *encounter.RollCalculation {
	if calculation == nil {
		return nil
	}
	clone := &encounter.RollCalculation{Total: calculation.Total}
	if calculation.Components != nil {
		clone.Components = make([]encounter.RollComponent, len(calculation.Components))
		for i, component := range calculation.Components {
			clone.Components[i] = validationComponentFor(component)
		}
	}
	return clone
}

// validationDiceTraceFor projects the SDK's dice trace onto the composition's
// validator shape, or nil when there is none.
func validationDiceTraceFor(trace *DiceTrace) *encounter.DiceTrace {
	if trace == nil {
		return nil
	}
	clone := &encounter.DiceTrace{
		Notation:      trace.Notation,
		DieSize:       trace.DieSize,
		OriginalRolls: append([]int(nil), trace.OriginalRolls...),
		FinalRolls:    append([]int(nil), trace.FinalRolls...),
		KeptIndices:   append([]int(nil), trace.KeptIndices...),
		Subtotal:      trace.Subtotal,
		Keep:          validationDiceKeepFor(trace.Keep),
	}
	if trace.Rerolls != nil {
		clone.Rerolls = make([]encounter.DiceReroll, len(trace.Rerolls))
		for i, reroll := range trace.Rerolls {
			clone.Rerolls[i] = encounter.DiceReroll{
				DieIndex: reroll.DieIndex, Before: reroll.Before, After: reroll.After,
				Source: encounter.RollSource{
					Ref: reroll.Source.Ref, Name: reroll.Source.Name, Label: reroll.Source.Label,
					SourceID: reroll.Source.SourceID,
				},
			}
		}
	}
	return clone
}

// validationDiceKeepFor projects the keep record onto the composition's
// validator shape, so a decoded record that does not describe its own dice is
// refused by the same replay every other trace fact goes through.
func validationDiceKeepFor(keep *DiceKeep) *encounter.DiceKeep {
	if keep == nil {
		return nil
	}

	clone := &encounter.DiceKeep{Rule: encounter.KeepRule(keep.Rule)}
	for _, source := range keep.Granted {
		clone.Granted = append(clone.Granted, validationRollSourceFor(source))
	}
	for _, source := range keep.Imposed {
		clone.Imposed = append(clone.Imposed, validationRollSourceFor(source))
	}
	return clone
}

func validationRollSourceFor(source RollSource) encounter.RollSource {
	return encounter.RollSource{
		Ref: source.Ref, Name: source.Name, Label: source.Label, SourceID: source.SourceID,
	}
}

// cloneInt returns an independently owned copy of value, nil-safe.
func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

// beatAttack is the wire shape Record's payload writes an attack identity
// as (attack.go's recordFor) — decoded here rather than re-derived, since
// this package is the one that wrote it in the first place.
type beatAttack struct {
	Ref        string `json:"ref"`
	Name       string `json:"name"`
	DamageType string `json:"damage_type"`
}

func (a beatAttack) toRef() AttackRef {
	return AttackRef{Ref: a.Ref, Name: a.Name, DamageType: DamageType(a.DamageType)}
}

// beatReaction is the wire shape the composition writes a reaction identity
// as (encounter's Record) — the key that has been on every reaction beat since
// rung 2 and that nothing read.
type beatReaction struct {
	Ref  string `json:"ref"`
	Name string `json:"name"`
}

// toRef converts a decoded reaction identity, reporting false for a present
// but incomplete one. Nil converts to nil and true: an ordinary swing carries
// no reaction and that is an answer, not a defect.
func (r *beatReaction) toRef() (*ReactionRef, bool) {
	if r == nil {
		return nil, true
	}
	if r.Ref == "" || r.Name == "" {
		return nil, false
	}
	return &ReactionRef{Ref: r.Ref, Name: r.Name}, true
}

// toSequence converts a decoded sequence identity with toRef's discipline: nil
// is the lone swing and converts to nil and true, a present but incomplete one
// reports false.
func (r *beatReaction) toSequence() (*SequenceRef, bool) {
	if r == nil {
		return nil, true
	}
	if r.Ref == "" || r.Name == "" {
		return nil, false
	}
	return &SequenceRef{Ref: r.Ref, Name: r.Name}, true
}

// decodeDamageComponents decodes a struck beat's ordered damage components.
// nil means the key was absent — the composition omits it when a swing dealt
// no componented damage — and absence is legal.
func decodeDamageComponents(raw json.RawMessage) ([]DamageComponent, bool) {
	if raw == nil {
		return nil, true
	}
	var elements []json.RawMessage
	if json.Unmarshal(raw, &elements) != nil {
		return nil, false
	}
	if len(elements) == 0 {
		return nil, true
	}
	components := make([]DamageComponent, 0, len(elements))
	for _, element := range elements {
		component, ok := decodeDamageComponent(element)
		if !ok {
			return nil, false
		}
		components = append(components, component)
	}
	return components, true
}

// damageComponentKeys are the field names one persisted damage component may
// carry: the trace representation's four, the multiplier beside them, and the
// legacy scalars that a pre-trace payload reads back through.
var damageComponentKeys = map[string]struct{}{
	"source": {}, "roll": {}, "damage_type": {}, "multiplier": {},
	"source_ref": {}, "dice": {}, "final_rolls": {}, "flat_bonus": {},
}

// decodeDamageComponent decodes one damage component in exactly one of its two
// representations. A component carrying the roll trace AND any legacy scalar
// is mixing two shapes and is refused; so is one carrying neither. The legacy
// scalars are read as written — the decoder never fabricates a trace from
// them.
func decodeDamageComponent(raw json.RawMessage) (DamageComponent, bool) {
	fields, ok := strictJSONObject(raw)
	if !ok {
		return DamageComponent{}, false
	}
	for key := range fields {
		if _, known := damageComponentKeys[key]; !known {
			return DamageComponent{}, false
		}
	}
	for _, value := range fields {
		if isJSONNull(value) {
			return DamageComponent{}, false
		}
	}

	_, rollPresent := fields["roll"]
	legacyPresent := false
	for _, key := range []string{"source_ref", "dice", "final_rolls", "flat_bonus", "multiplier"} {
		if _, present := fields[key]; present {
			legacyPresent = true
		}
	}
	if rollPresent {
		// The NEW representation. A multiplier rides BESIDE the roll as a
		// damage fact and the two are one shape together; the four scalar
		// roll facts are the OLD shape's representation of the same question
		// and cannot coexist with a trace.
		for _, key := range []string{"source_ref", "dice", "final_rolls", "flat_bonus"} {
			if _, present := fields[key]; present {
				return DamageComponent{}, false
			}
		}
	} else if !legacyPresent {
		// Neither representation — with no roll and no legacy scalar fact,
		// the multiplier among them, there is nothing to read.
		return DamageComponent{}, false
	}

	var scalar struct {
		Source     string   `json:"source"`
		SourceRef  string   `json:"source_ref"`
		Dice       string   `json:"dice"`
		FinalRolls []int    `json:"final_rolls"`
		FlatBonus  int      `json:"flat_bonus"`
		DamageType string   `json:"damage_type"`
		Multiplier *float64 `json:"multiplier"`
	}
	if json.Unmarshal(raw, &scalar) != nil || scalar.Source == "" {
		return DamageComponent{}, false
	}

	component := DamageComponent{
		Source: scalar.Source, DamageType: DamageType(scalar.DamageType), Multiplier: scalar.Multiplier,
	}
	if rollPresent {
		roll, ok := decodeRollComponent(fields["roll"])
		if !ok {
			return DamageComponent{}, false
		}
		component.Roll = roll
		// A sourced identity alone contributes nothing — the composition
		// refuses a component with neither dice, a modifier, nor a multiplier
		// at write time, and the multiplier-only shape is the one way a
		// component with neither rollable fact has a story to tell.
		if roll.Dice == nil && roll.Modifier == nil && component.Multiplier == nil {
			return DamageComponent{}, false
		}
		return component, true
	}
	component.SourceRef = scalar.SourceRef
	component.Dice = scalar.Dice
	component.FinalRolls = scalar.FinalRolls
	component.FlatBonus = scalar.FlatBonus
	return component, true
}

// revealedFixed is what a room or cells reveal decodes beyond its plain
// fields: placed props from the composition's placement shape, and the shared
// structural rows and prop presentations.
type revealedFixed struct {
	placed        []AtlasPlacedProp
	presentations []PropPresentation
	rows          structuralRows
}

// revealedFixedRows decodes the shared rows of a reveal payload. False when
// any row is malformed or missing its identity; an absent key leaves its list
// empty.
func revealedFixedRows(payload []byte) (revealedFixed, bool) {
	var p struct {
		Placed []struct {
			ID                string                  `json:"id"`
			Placement         encounter.PlacementData `json:"placement"`
			Cells             []spatial.Position      `json:"cells"`
			BlocksMovement    bool                    `json:"blocks_movement"`
			BlocksLineOfSight bool                    `json:"blocks_line_of_sight"`
		} `json:"placed"`
	}
	if json.Unmarshal(payload, &p) != nil {
		return revealedFixed{}, false
	}
	// THE STRUCTURAL ROWS ARE DECODED ONCE, SHARED WITH THE CONCEALMENT BEAT.
	rows, ok := structuralRowsFromPayload(payload)
	if !ok {
		return revealedFixed{}, false
	}
	presentations, ok := propPresentationsFromPayload(payload)
	if !ok {
		return revealedFixed{}, false
	}
	out := revealedFixed{presentations: presentations, rows: rows}
	for _, prop := range p.Placed {
		if prop.ID == "" {
			return revealedFixed{}, false
		}
		d := prop.Placement
		out.placed = append(out.placed, AtlasPlacedProp{
			ID: prop.ID, Cells: prop.Cells, BlocksMovement: prop.BlocksMovement, BlocksLineOfSight: prop.BlocksLineOfSight,
			Placement: FootprintPlacement{
				Width: d.Footprint.W, Depth: d.Footprint.D, Origin: FootprintPoint{X: d.Origin.X, Y: d.Origin.Y},
				Facing: d.Facing, LocalOffset: FootprintPoint{X: d.LocalOffset.X, Y: d.LocalOffset.Y},
			},
		})
	}
	return out, true
}
