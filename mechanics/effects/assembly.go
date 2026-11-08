// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ErrInvalidAssembly marks an invalid local assembly definition or operation.
var ErrInvalidAssembly = errors.New("invalid effect assembly")

// EndScope states whether a part's terminal state completes that part or asks
// the application's owner to end its shared lifetime.
type EndScope string

const (
	// EndPart leaves other parts running and persists this part as completed.
	EndPart EndScope = "part"
	// EndApplication asks the owner to end, then detaches all local parts.
	EndApplication EndScope = "application"
)

// PartInput binds a stable local key to one existing composed runtime. Its
// vocabulary supplies no End/Changed hooks: the assembly owns that routing.
type PartInput struct {
	Key        string
	Compose    ComposeInput
	OnTerminal EndScope
}

// AssemblyEnd describes the bound part that ended. It carries no rulebook data
// or permission to mutate another entity; the consumer delivers application ends.
type AssemblyEnd struct {
	AssemblyKey string
	PartKey     string
	Scope       EndScope
	Reason      string
}

// AssemblyChange names the part whose saved state needs recording.
type AssemblyChange struct {
	AssemblyKey string
	PartKey     string
}

// AssemblyLifecycle supplies lifecycle transport, not another state runner.
// End is required for application-ending parts. Changed reports local state
// changes and local completion. Callbacks run on the attached interaction bus.
type AssemblyLifecycle struct {
	End     func(context.Context, events.EventBus, AssemblyEnd) error
	Changed func(context.Context, events.EventBus, AssemblyChange) error
}

// AssemblyInput declares one entity-local attachment. Key is the instance's
// output namespace; Parts are ordered and have unique nonblank keys. All parts
// name the same nonblank holder and declare an explicit terminal scope.
type AssemblyInput struct {
	Key       string
	Parts     []PartInput
	Lifecycle AssemblyLifecycle
}

// AssemblySnapshot owns one snapshot per declared part, including completed
// local parts. Missing entries never mean fresh construction or a forgotten end.
type AssemblySnapshot struct {
	Parts map[string]Snapshot `json:"parts"`
}

// RestoreAssemblyInput restores exact saved coverage against its definition.
type RestoreAssemblyInput struct {
	Definition AssemblyInput
	Snapshot   *AssemblySnapshot
}

// PartOutput identifies a current observational output without exposing its
// machine. Returned slices are owned; the Bound uses the existing live-query API.
type PartOutput struct {
	PartKey string
	Output  *Bound
}

type assemblyPart struct {
	key     string
	runtime *Composed
}

// Assembly attaches independently reacting Composed parts to one interaction.
// It owns local subscription custody, not the shared Lifetime's membership. It
// is not safe for concurrent use, like the runtimes it contains.
type Assembly struct {
	key       string
	parts     []assemblyPart
	lifecycle AssemblyLifecycle
	bus       events.EventBus
	ending    bool
}

// Assemble validates and constructs all parts without attachment. It refuses
// duplicate identities, mixed holders, invalid scopes, nested lifecycle hooks
// and invalid composed recipes. An empty ordered assembly is valid for an owner
// whose only role is explicit lifetime custody.
func Assemble(input AssemblyInput) (*Assembly, error) {
	if strings.TrimSpace(input.Key) == "" {
		return nil, fmt.Errorf("%w: key is required", ErrInvalidAssembly)
	}
	a := &Assembly{key: input.Key, lifecycle: input.Lifecycle}
	seen := make(map[string]struct{}, len(input.Parts))
	var holder string
	for _, part := range input.Parts {
		if strings.TrimSpace(part.Key) == "" {
			return nil, fmt.Errorf("%w: part key is required", ErrInvalidAssembly)
		}
		if _, duplicate := seen[part.Key]; duplicate {
			return nil, fmt.Errorf("%w: duplicate part %q", ErrInvalidAssembly, part.Key)
		}
		seen[part.Key] = struct{}{}
		if part.Compose.InstanceKey != "" {
			return nil, fmt.Errorf("%w: part %q supplies a second instance namespace", ErrInvalidAssembly, part.Key)
		}
		if strings.TrimSpace(part.Compose.HolderID) == "" || (holder != "" && holder != part.Compose.HolderID) {
			return nil, fmt.Errorf("%w: parts must name one nonblank holder", ErrInvalidAssembly)
		}
		holder = part.Compose.HolderID
		if part.Compose.OutputVocabulary.End != nil || part.Compose.OutputVocabulary.Changed != nil {
			return nil, fmt.Errorf("%w: part %q supplies nested lifecycle hooks", ErrInvalidAssembly, part.Key)
		}
		switch part.OnTerminal {
		case EndPart:
		case EndApplication:
			if a.lifecycle.End == nil {
				return nil, fmt.Errorf("%w: part %q requires application end transport", ErrInvalidAssembly, part.Key)
			}
		default:
			return nil, fmt.Errorf("%w: part %q has unknown terminal scope %q", ErrInvalidAssembly, part.Key, part.OnTerminal)
		}
		bound := part.Compose
		bound.InstanceKey = fmt.Sprintf("%d:%s/%s", len(input.Key), input.Key, part.Key)
		bound.OutputVocabulary.End = func(ctx context.Context, bus events.EventBus, _ Host, reason string) error {
			return a.partEnded(ctx, bus, AssemblyEnd{AssemblyKey: a.key, PartKey: part.Key, Scope: part.OnTerminal, Reason: reason})
		}
		bound.OutputVocabulary.Changed = func(ctx context.Context, bus events.EventBus, _ Host) error {
			return a.partChanged(ctx, bus, part.Key)
		}
		composed, err := Compose(bound)
		if err != nil {
			return nil, fmt.Errorf("%w: part %q: %w", ErrInvalidAssembly, part.Key, err)
		}
		if part.OnTerminal == EndApplication && composed.m.Program().IsTerminal(composed.State()) {
			return nil, fmt.Errorf("%w: part %q is already application-terminal", ErrInvalidAssembly, part.Key)
		}
		a.parts = append(a.parts, assemblyPart{key: part.Key, runtime: composed})
	}
	return a, nil
}

// RestoreAssembly requires every part's explicit state and complete declared
// bag. Completed local parts remain completed; terminal application-maintenance
// state cannot be restored as a live application. Inputs are not retained.
func RestoreAssembly(input RestoreAssemblyInput) (*Assembly, error) {
	if input.Snapshot == nil || len(input.Snapshot.Parts) != len(input.Definition.Parts) {
		return nil, fmt.Errorf("%w: snapshot must name every part exactly once", ErrInvalidAssembly)
	}
	definition := input.Definition
	definition.Parts = slices.Clone(definition.Parts)
	for i := range definition.Parts {
		part := &definition.Parts[i]
		saved, found := input.Snapshot.Parts[part.Key]
		if !found || saved.State == "" || saved.Bag == nil || len(saved.Bag) != len(part.Compose.Recipe.Bag) {
			return nil, fmt.Errorf("%w: part %q requires complete saved state and bag", ErrInvalidAssembly, part.Key)
		}
		state, known := part.Compose.Recipe.States[saved.State]
		if !known || (state.Terminal && part.OnTerminal == EndApplication) {
			return nil, fmt.Errorf("%w: part %q cannot restore state %q", ErrInvalidAssembly, part.Key, saved.State)
		}
		for key := range part.Compose.Recipe.Bag {
			if _, present := saved.Bag[key]; !present {
				return nil, fmt.Errorf("%w: part %q missing saved key %q", ErrInvalidAssembly, part.Key, key)
			}
		}
		part.Compose.State = saved.State
		part.Compose.Parameters = saved.Bag.Clone()
	}
	return Assemble(definition)
}

// Apply attaches live parts in declared order, unwinding on failure. It refuses
// nil buses, double attachment and reattachment after a gameplay application end.
func (a *Assembly) Apply(ctx context.Context, bus events.EventBus) error {
	if bus == nil || a.bus != nil || a.ending {
		return fmt.Errorf("%w: Apply requires a bus and a detached live assembly", ErrInvalidAssembly)
	}
	a.bus = bus
	for _, part := range a.parts {
		if part.runtime.m.Program().IsTerminal(part.runtime.State()) {
			continue
		}
		if err := part.runtime.Apply(ctx, bus); err != nil {
			return errors.Join(err, a.Remove(ctx, bus))
		}
	}
	return nil
}

// Remove tears down all local parts in reverse order without publishing an end.
// Repetition is harmless. A gameplay end remains ended; ordinary teardown leaves
// the saved state available for information or another interaction.
func (a *Assembly) Remove(ctx context.Context, bus events.EventBus) error {
	a.bus = nil
	var err error
	for i := len(a.parts) - 1; i >= 0; i-- {
		err = errors.Join(err, a.parts[i].runtime.Remove(ctx, bus))
	}
	return err
}

// IsApplied reports local interaction attachment, not application membership.
func (a *Assembly) IsApplied() bool { return a.bus != nil }

// Snapshot returns owned state for every part, including local completed parts.
func (a *Assembly) Snapshot() AssemblySnapshot {
	parts := make(map[string]Snapshot, len(a.parts))
	for _, part := range a.parts {
		parts[part.key] = part.runtime.Snapshot()
	}
	return AssemblySnapshot{Parts: parts}
}

// CurrentOutputs returns live local outputs in declared part/output order.
// Ordinary detachment does not erase information; a gameplay application end does.
func (a *Assembly) CurrentOutputs() []PartOutput {
	var outputs []PartOutput
	if a.ending {
		return outputs
	}
	for _, part := range a.parts {
		if part.runtime.m.Program().IsTerminal(part.runtime.State()) {
			continue
		}
		for _, output := range part.runtime.CurrentOutputs() {
			outputs = append(outputs, PartOutput{PartKey: part.key, Output: output})
		}
	}
	return outputs
}

func (a *Assembly) partChanged(ctx context.Context, bus events.EventBus, key string) error {
	if a.ending || a.lifecycle.Changed == nil {
		return nil
	}
	return a.lifecycle.Changed(ctx, bus, AssemblyChange{AssemblyKey: a.key, PartKey: key})
}

func (a *Assembly) partEnded(ctx context.Context, bus events.EventBus, end AssemblyEnd) error {
	if a.ending {
		return nil
	}
	if end.Scope == EndApplication {
		a.ending = true
		err := a.lifecycle.End(ctx, bus, end)
		return errors.Join(err, a.Remove(ctx, bus))
	}
	if a.lifecycle.End != nil {
		if err := a.lifecycle.End(ctx, bus, end); err != nil {
			return err
		}
	}
	return a.partChanged(ctx, bus, end.PartKey)
}
