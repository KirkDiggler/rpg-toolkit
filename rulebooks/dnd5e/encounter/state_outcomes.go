// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// state_outcomes.go is A CREATURE'S MACHINE GIVES A NOTE ON ENTERING A STATE
// (rpg-project#989): [MemberInput.Outcomes] binds, by state of the body
// machine, consequences done once each time a step enters that state. The one
// kind is [Consequence.Recognize]: the creature itself notes a topic about the
// one the beat names.

// validateStateOutcomes refuses state outcomes nobody could author on
// purpose, in state order, each naming its state: outcomes on a member with
// no recipe, a state the recipe lacks, a state with no consequence, a state no
// transition enters, a consequence of any kind but recognize, a recognize with
// no topic or with words this place does not allow, and a note about the one
// the beat names where a beat entering the state names nobody.
func validateStateOutcomes(r *machine.Recipe, outcomes map[string][]Consequence) error {
	if len(outcomes) == 0 {
		return nil
	}
	if r == nil {
		return fmt.Errorf("state outcomes name states of no recipe: a machine's note needs authored states")
	}
	for _, state := range slices.Sorted(maps.Keys(outcomes)) {
		if _, ok := r.States[state]; !ok {
			return fmt.Errorf("state outcome %q is not a state of its recipe", state)
		}
		if len(outcomes[state]) == 0 {
			return fmt.Errorf("state outcome %q names no consequence", state)
		}
		entering := beatsEntering(r, state)
		if len(entering) == 0 {
			return fmt.Errorf("state outcome %q is entered by no transition of its recipe, so its outcomes could never run", state)
		}
		for i, c := range outcomes[state] {
			what := fmt.Sprintf("state outcome %q consequence %d", state, i)
			if err := validateStateConsequence(what, state, c, entering); err != nil {
				return err
			}
		}
	}
	return nil
}

// beatsEntering is the beat of every transition of r that steps into state,
// in state and transition order.
func beatsEntering(r *machine.Recipe, state string) []machine.Beat {
	var out []machine.Beat
	for _, name := range slices.Sorted(maps.Keys(r.States)) {
		for _, t := range r.States[name].Transitions {
			if t.To == state {
				out = append(out, t.On)
			}
		}
	}
	return out
}

// validateStateConsequence refuses one consequence a machine's state may not
// give, saying where it belongs.
func validateStateConsequence(what, state string, c Consequence, entering []machine.Beat) error {
	kinds := 0
	for _, set := range []bool{c.Recognize != nil, c.Arrest != nil, c.Jail != nil,
		c.Tells != "", c.Relocate != nil, c.ApplyStatus != nil} {
		if set {
			kinds++
		}
	}
	switch {
	case kinds > 1:
		return fmt.Errorf("%s is of two kinds: one consequence is one kind", what)
	case c.ApplyStatus != nil:
		return fmt.Errorf("%s gives a status: a line's row gives a status; a machine's state gives nobody one yet", what)
	case c.Tells != "":
		return fmt.Errorf("%s tells a record: a line's row tells whoever caught its answer; a state has no answer", what)
	case c.Relocate != nil:
		return fmt.Errorf("%s relocates: a line's row moves the speaker; a state moves nobody yet", what)
	case c.Arrest != nil:
		return fmt.Errorf("%s arrests: an arrest follows a conversation entering a state, never a machine's", what)
	case c.Jail != nil:
		return fmt.Errorf("%s jails: a jail follows a conversation entering a state, never a machine's", what)
	case c.Recognize == nil:
		return fmt.Errorf("%s is of no kind: the one kind is recognize", what)
	}
	r := c.Recognize
	switch {
	case r.Topic == "":
		return fmt.Errorf("%s recognizes on no topic", what)
	case r.As != "" && isEngineTopic(r.Topic):
		return errEngineTopicValue(what, r.Topic, r.As)
	case string(r.Holder) != WhoItself:
		return fmt.Errorf("%s holder %q: the holder is %q", what, r.Holder, WhoItself)
	case !slices.Contains(WhoWordsIn(WhoInStateGiveAbout), string(r.About)):
		return fmt.Errorf("%s about %q: the subject is %s", what, r.About, joinWords(quotedWords(WhoWordsIn(WhoInStateGiveAbout)), "or"))
	case r.HeardBy != "":
		return fmt.Errorf("%s heard_by %q: a state's note has no line, so nobody else hears it", what, r.HeardBy)
	}
	for _, on := range entering {
		if b, ok := BeatWordOf(string(on)); !ok || !b.Names {
			return fmt.Errorf("%s notes about `%s`, and the recipe enters %q on `%s`, which names nobody: "+
				"it is read on %s", what, r.About, state, on, joinWords(familyNaming.beats(), "and"))
		}
	}
	return nil
}

// useStateOutcomes gives a member its own copy of its state outcomes, refused
// as [validateStateOutcomes] refuses them.
//
// Errors: the refusal, wrapped in [ErrBadMember].
func (e *Encounter) useStateOutcomes(m *memberRecord, outcomes map[string][]Consequence) error {
	if err := validateStateOutcomes(m.behaviors.bodyRecipe(), outcomes); err != nil {
		return fmt.Errorf("member %s: %w: %w", m.ID, err, ErrBadMember)
	}
	m.Outcomes = cloneOutcomes(outcomes)
	return nil
}

// enteredState does the outcomes bound to a state the member's body machine
// just stepped into, once, after its state beat: the creature itself notes
// each topic about named, the one the beat names (nobody writes nothing).
// The note is told to the creature alone, and its testimony names the state
// entered, with no interaction and no option.
func (e *Encounter) enteredState(m *memberRecord, to string, named MemberID, now uint64) error {
	for _, c := range m.Outcomes[to] {
		if c.Recognize == nil {
			continue
		}
		line := recognitionLine{state: to, spokenTo: m.ID, named: named, at: now, holdersOnly: true}
		if err := e.grantRecognitionAt(*c.Recognize, line, WhoInStateGiveTo, WhoInStateGiveAbout); err != nil {
			return fmt.Errorf("member %s state %q outcome: %w", m.ID, to, err)
		}
	}
	return nil
}

// quotedWords is each word in double quotes, in order.
func quotedWords(words []string) []string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		out = append(out, fmt.Sprintf("%q", w))
	}
	return out
}
