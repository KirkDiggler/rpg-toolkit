// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package banditcamp

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
)

// VerbName names a declared action — "sneak", "impersonate".
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
)

// Subjecting declares which side of the act the fact is about.
type Subjecting int

const (
	// SubjectTarget makes the fact about whoever was acted on. Almost always
	// what you want.
	SubjectTarget Subjecting = iota

	// SubjectActor makes the fact about the actor. A disguise coming apart is
	// about the person wearing it, not about the camp that was fooled — and
	// that is what lets one generic Vacate handle both an assassinated leader
	// and an unmasked impostor.
	SubjectActor
)

// Emission declares the fact a verb writes for one branch of its outcome.
type Emission struct {
	// Kind is the fact kind written. An empty Kind means the branch writes
	// nothing.
	Kind journal.Kind

	// Subject chooses whom the fact is about.
	Subject Subjecting

	// Witness chooses who sees it.
	Witness Witnessing
}

// Verb is a declared action: an approach, a difficulty, and the facts each
// branch writes.
//
// There are no prerequisites here and there is nowhere to put one. A verb does
// not name a class, a feat, a proficiency, or a condition, and the executor
// consults none. The barbarian may sneak. Being bad at it changes the roll and
// nothing else, and the roll is somebody else's business entirely.
type Verb struct {
	// Name identifies the verb.
	Name VerbName

	// Approach is handed to the resolver. An empty Approach makes the verb
	// uncontested: no resolver call, no dice, the success branch always.
	Approach journal.Approach

	// Difficulty is handed to the resolver untouched. What a 13 means is the
	// resolver's opinion.
	Difficulty int

	// OnSuccess is the fact written when the attempt lands, and the only fact
	// an uncontested verb ever writes.
	OnSuccess Emission

	// OnFailure is the fact written when it does not. Required for a contested
	// verb, because a failed attempt that leaves no trace is a fact the world
	// silently forgot.
	OnFailure Emission
}

// Act is one attempt at a verb.
type Act struct {
	// Verb names what is being tried.
	Verb VerbName

	// Actor is who is trying it.
	Actor journal.EntityID

	// Target is who it is aimed at.
	Target journal.EntityID

	// Bystanders are the ids a [WitnessBystanders] emission is audienced to.
	//
	// The spike takes them from the caller. A finished world would derive them
	// from sight and position, and that is a seam this example does not have.
	Bystanders []journal.EntityID
}

// ErrUnknownVerb reports an act naming a verb the executor was not given.
var ErrUnknownVerb = errors.New("banditcamp: unknown verb")

// ErrNoResolver reports an executor built without one.
var ErrNoResolver = errors.New("banditcamp: resolver is required")

// ErrNoJournal reports an executor built without one.
var ErrNoJournal = errors.New("banditcamp: journal is required")

// ErrIncompleteVerb reports a verb declaration that could write nothing.
var ErrIncompleteVerb = errors.New("banditcamp: verb writes no fact")

// Executor runs declared verbs: it asks the resolver, picks the branch, and
// appends the fact.
//
// It is the one piece of this example that is not data and not rulebook. It
// imports journal and nothing else — no graph, no dnd5e — so it never reads
// world state and cannot gate an attempt on it.
type Executor struct {
	verbs    map[VerbName]Verb
	resolver journal.Resolver
	log      *journal.Journal
}

// ExecutorConfig supplies an executor's parts. All three are required; none is
// defaulted.
type ExecutorConfig struct {
	Journal  *journal.Journal
	Resolver journal.Resolver
	Verbs    []Verb
}

// NewExecutor validates the verb declarations and returns an executor.
//
// Returns [ErrNoJournal], [ErrNoResolver], or [ErrIncompleteVerb].
func NewExecutor(cfg ExecutorConfig) (*Executor, error) {
	if cfg.Journal == nil {
		return nil, ErrNoJournal
	}
	if cfg.Resolver == nil {
		return nil, ErrNoResolver
	}

	e := &Executor{
		verbs:    make(map[VerbName]Verb, len(cfg.Verbs)),
		resolver: cfg.Resolver,
		log:      cfg.Journal,
	}
	for _, v := range cfg.Verbs {
		if v.OnSuccess.Kind == "" {
			return nil, fmt.Errorf("%w: %q has no success fact", ErrIncompleteVerb, v.Name)
		}
		if v.Approach != "" && v.OnFailure.Kind == "" {
			return nil, fmt.Errorf("%w: contested %q has no failure fact", ErrIncompleteVerb, v.Name)
		}
		e.verbs[v.Name] = v
	}

	return e, nil
}

// Do attempts a verb and returns the fact it wrote.
//
// The whole of it: look the verb up, ask the resolver if there is anything to
// ask, choose the branch, work out who saw it, append. Nothing is checked about
// the actor beyond its existence, because there is nothing to check.
//
// Returns [ErrUnknownVerb], or whatever the resolver returned — a resolver error
// means the attempt could not be judged, which is a wiring fault and leaves the
// journal untouched.
func (e *Executor) Do(ctx context.Context, act Act) (journal.Fact, error) {
	verb, ok := e.verbs[act.Verb]
	if !ok {
		return journal.Fact{}, fmt.Errorf("%w: %q", ErrUnknownVerb, act.Verb)
	}

	outcome, err := e.resolve(ctx, verb, act)
	if err != nil {
		return journal.Fact{}, fmt.Errorf("resolving %q: %w", act.Verb, err)
	}

	emission := verb.OnSuccess
	if outcome.Contested && !outcome.Succeeded {
		emission = verb.OnFailure
	}
	if emission.Kind == "" {
		return journal.Fact{}, nil
	}

	return e.log.Append(journal.Fact{
		Kind:     emission.Kind,
		Actor:    act.Actor,
		Subject:  subjectOf(emission, act),
		Audience: audienceOf(emission, act),
		Outcome:  outcome,
	})
}

func (e *Executor) resolve(ctx context.Context, verb Verb, act Act) (journal.Outcome, error) {
	if verb.Approach == "" {
		return journal.Outcome{Detail: string(verb.Name) + ": uncontested"}, nil
	}

	return e.resolver.Resolve(ctx, journal.Attempt{
		Actor:      act.Actor,
		Approach:   verb.Approach,
		Target:     act.Target,
		Difficulty: verb.Difficulty,
	})
}

func subjectOf(emission Emission, act Act) journal.EntityID {
	if emission.Subject == SubjectActor {
		return act.Actor
	}

	return act.Target
}

// audienceOf always includes the actor: whatever else happened, you saw yourself
// do it. A camp-audience of nobody is still nobody.
func audienceOf(emission Emission, act Act) journal.Audience {
	audience := journal.Audience{act.Actor}

	switch emission.Witness {
	case WitnessTarget:
		if act.Target != "" && act.Target != act.Actor {
			audience = append(audience, act.Target)
		}
	case WitnessBystanders:
		for _, id := range act.Bystanders {
			if !slices.Contains(audience, id) {
				audience = append(audience, id)
			}
		}
	case WitnessNobody:
	}

	return audience
}
