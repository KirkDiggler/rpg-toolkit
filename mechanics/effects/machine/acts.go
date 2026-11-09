// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

// run performs acts on the bag in order and reports whether any changed it.
// The acts were validated at Compile, so every key they name is declared.
func (m *Machine) run(acts []Act, spec BeatSpec, ev any) bool {
	changed := false
	set := func(key string, v any) {
		if m.bag[key] != v {
			m.bag[key] = v
			changed = true
		}
	}
	round, hasRound := roundOf(spec, ev)
	for _, a := range acts {
		switch {
		case a.AnchorRound:
			set(KeySawTurnEnd, true)
			if anchor, _ := m.bag[KeyRoundActivated].(int); hasRound && anchor == 0 {
				set(KeyRoundActivated, round) // anchor late, never re-anchor
			}
		case a.DeriveTurnsActive:
			if anchor, _ := m.bag[KeyRoundActivated].(int); hasRound && anchor > 0 && round >= anchor {
				set(KeyTurnsActive, round-anchor+1)
			}
		case len(a.ClearFlags) > 0:
			for _, f := range a.ClearFlags {
				set(f, false)
			}
		case a.Set != nil:
			set(a.Set.Key, a.Set.Value)
		}
	}
	return changed
}

// roundOf returns the beat's round when it carries one above zero.
func roundOf(spec BeatSpec, ev any) (int, bool) {
	if spec.Round == nil {
		return 0, false
	}
	round, ok := spec.Round(ev)
	return round, ok && round > 0
}
