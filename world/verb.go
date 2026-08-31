// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package world

import (
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// VerbName names a declared action — "sneak", "rescue".
type VerbName string

// Witnessing declares who a verb's fact reaches.
//
// This is where stealth lives. A sneak that works and a sneak that fails write
// the same kind of fact about the same subject; all that differs is who saw it,
// and every consequence follows from that one difference.
type Witnessing int

const (
	// WitnessNobody audiences the fact to the actor alone. The quiet kill.
	WitnessNobody Witnessing = iota

	// WitnessTarget audiences it to whoever the act was aimed at, which reaches
	// every member of a group target.
	WitnessTarget

	// WitnessBystanders audiences it to the ids the caller supplied — whoever
	// happened to be looking. Individual grain enters here: name one lieutenant
	// and the fact reaches that lieutenant and nobody else.
	WitnessBystanders

	// WitnessTargetAndBystanders audiences it to both. Some things are done in
	// front of the person they are done to and a crowd besides.
	WitnessTargetAndBystanders
)

// Subjecting declares which side of the act the fact is about.
type Subjecting int

const (
	// SubjectTarget makes the fact about whoever was acted on. Almost always
	// what you want.
	SubjectTarget Subjecting = iota

	// SubjectActor makes the fact about the actor. A disguise coming apart is
	// about the person wearing it, not about the camp that was fooled — and
	// that is what lets one generic derivation handle both an assassinated
	// leader and an unmasked impostor.
	SubjectActor
)

// Emission declares the fact a verb writes when one of its outcomes is reached.
type Emission struct {
	// Kind is the fact kind written.
	Kind journal.Kind

	// Subject chooses whom the fact is about.
	Subject Subjecting

	// Witness chooses who sees it.
	Witness Witnessing
}

// Band is one graded outcome of a verb: how well the attempt had to go, and
// what gets written down when it went that well.
//
// Grading by margin rather than by pass and fail is what lets a rescue that
// went beautifully leave a different mark from one that barely worked.
type Band struct {
	// AtLeast is the margin this outcome needs — how far past the difficulty
	// the attempt landed. Zero means "succeeded at all".
	AtLeast int

	// Emission is what gets written when the margin reaches the bar.
	Emission Emission
}

// Verb is a declared action: an approach, a difficulty, and what gets written
// down depending on how it went.
//
// There are no prerequisites here and there is nowhere to put one. A verb does
// not name a class, a feat, a proficiency, or a condition, and the composer
// consults none. The barbarian may sneak. Being bad at it changes the roll and
// nothing else, and the roll is somebody else's business entirely.
//
// Verbs are content: the scenario declares what its people can try, and the
// composer runs verbs it never defined.
type Verb struct {
	// Name identifies the verb.
	Name VerbName

	// Approach is handed to the resolver. An empty Approach makes the verb
	// uncontested: no resolver call, no dice, and Otherwise is what happens.
	Approach journal.Approach

	// Difficulty is handed to the resolver untouched. What a 13 means is the
	// resolver's opinion.
	Difficulty int

	// Outcomes are the graded results, best first: the first band whose bar the
	// margin clears is the one that happens. Leave it empty for a verb with one
	// result.
	Outcomes []Band

	// Otherwise is what gets written when no band was cleared — the failure, or
	// for an uncontested verb, simply the thing that happens. Always required:
	// an attempt that can leave no trace at all is a fact the world forgot.
	Otherwise Emission
}

// Act is one attempt at a verb.
type Act struct {
	// Verb names what is being tried.
	Verb VerbName

	// Actor is who is trying it.
	Actor journal.EntityID

	// Target is who it is aimed at.
	Target journal.EntityID

	// Bystanders are the ids a bystander-witnessed emission is audienced to.
	//
	// The spike takes them from the caller. A finished world would derive them
	// from sight and position, and that is a seam this example does not have.
	Bystanders []journal.EntityID
}

// ErrUnknownVerb reports an act naming a verb the world was not given.
var ErrUnknownVerb = errors.New("nobody here knows how to do that")

// ErrNoVerbName reports a verb declared without a name.
var ErrNoVerbName = errors.New("this action needs a name — it is how the rest of the content asks for it")

// ErrNoOtherwise reports a verb with no fallback outcome.
var ErrNoOtherwise = errors.New(
	"this action needs a result for when it does not go well — every attempt leaves a mark, " +
		"even the ones that fail")

// ErrEmptyBand reports an outcome band that writes nothing.
var ErrEmptyBand = errors.New("this outcome needs something to write down — name the fact it records")

// ErrBandsOutOfOrder reports outcome bands that are not best-first.
var ErrBandsOutOfOrder = errors.New(
	"put an action's outcomes best first — each one needs a higher bar than the one below it, " +
		"because the first bar the roll clears is the result that happens")

// ErrGradedButUncontested reports graded outcomes on a verb nobody rolls for.
var ErrGradedButUncontested = errors.New(
	"this action is never rolled for, so it cannot have graded outcomes — either give it an approach " +
		"to be judged by, or give it only the one result")

// ErrDuplicateVerb reports the same verb declared twice.
var ErrDuplicateVerb = errors.New("this action is declared twice — each one needs its own name")

func validateVerb(v Verb) error {
	if v.Name == "" {
		return ErrNoVerbName
	}
	if v.Otherwise.Kind == "" {
		return fmt.Errorf("%q: %w", v.Name, ErrNoOtherwise)
	}
	if v.Approach == "" && len(v.Outcomes) > 0 {
		return fmt.Errorf("%q: %w", v.Name, ErrGradedButUncontested)
	}

	for i, band := range v.Outcomes {
		if band.Emission.Kind == "" {
			return fmt.Errorf("%q outcome %d: %w", v.Name, i+1, ErrEmptyBand)
		}
		if i > 0 && band.AtLeast >= v.Outcomes[i-1].AtLeast {
			return fmt.Errorf("%q outcome %d: %w", v.Name, i+1, ErrBandsOutOfOrder)
		}
	}

	return nil
}

// emissionFor picks the outcome a margin reached.
func (v Verb) emissionFor(margin int) Emission {
	for _, band := range v.Outcomes {
		if margin >= band.AtLeast {
			return band.Emission
		}
	}

	return v.Otherwise
}

func subjectOf(emission Emission, act Act) journal.EntityID {
	if emission.Subject == SubjectActor {
		return act.Actor
	}

	return act.Target
}

// audienceOf always includes the actor: whatever else happened, you saw
// yourself do it. An audience of nobody else is still nobody else.
func audienceOf(emission Emission, act Act) journal.Audience {
	audience := journal.Audience{act.Actor}

	add := func(id journal.EntityID) {
		if id != "" && !slices.Contains(audience, id) {
			audience = append(audience, id)
		}
	}

	switch emission.Witness {
	case WitnessTarget:
		add(act.Target)
	case WitnessBystanders:
		for _, id := range act.Bystanders {
			add(id)
		}
	case WitnessTargetAndBystanders:
		add(act.Target)
		for _, id := range act.Bystanders {
			add(id)
		}
	case WitnessNobody:
	}

	return audience
}
