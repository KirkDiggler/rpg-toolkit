// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"errors"
	"fmt"
	"maps"
	"math"
)

// ErrInvalidAct marks a runtime act that cannot complete without invalid state,
// such as integer overflow. No writes in that transition are committed.
var ErrInvalidAct = errors.New("invalid machine act")

// run stages acts in order and commits only after every act succeeds. The
// existing bag retains its identity; readers never observe a partly applied list.
func (m *Machine) run(acts []Act, spec BeatSpec, ev any) (bool, error) {
	if len(acts) == 0 {
		return false, nil
	}
	next := m.bag.Clone()
	changed := false
	set := func(key string, v any) {
		if next[key] != v {
			next[key] = v
			changed = true
		}
	}
	round, hasRound := roundOf(spec, ev)
	for _, a := range acts {
		switch {
		case a.AnchorRound:
			set(KeySawTurnEnd, true)
			if anchor, _ := next[KeyRoundActivated].(int); hasRound && anchor == 0 {
				set(KeyRoundActivated, round)
			}
		case a.DeriveTurnsActive:
			if anchor, _ := next[KeyRoundActivated].(int); hasRound && anchor > 0 && round >= anchor {
				set(KeyTurnsActive, round-anchor+1)
			}
		case len(a.ClearFlags) > 0:
			for _, flag := range a.ClearFlags {
				set(flag, false)
			}
		case a.Set != nil:
			set(a.Set.Key, a.Set.Value)
		case a.Add != nil:
			value, ok := next[a.Add.Key].(int)
			if !ok {
				return false, fmt.Errorf("%w: add key %q has no integer value", ErrInvalidAct, a.Add.Key)
			}
			delta := a.Add.Delta
			if (delta > 0 && value > math.MaxInt-delta) || (delta < 0 && value < math.MinInt-delta) {
				return false, fmt.Errorf("%w: integer overflow adding %d to %q", ErrInvalidAct, delta, a.Add.Key)
			}
			set(a.Add.Key, value+delta)
		}
	}
	if changed {
		maps.Copy(m.bag, next)
	}
	return changed, nil
}

// roundOf returns the beat's round when it carries one above zero.
func roundOf(spec BeatSpec, ev any) (int, bool) {
	if spec.Round == nil {
		return 0, false
	}
	round, ok := spec.Round(ev)
	return round, ok && round > 0
}
