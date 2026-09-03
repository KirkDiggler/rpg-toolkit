// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

// LifeState is a combatant's derived tabletop life state.
type LifeState string

const (
	// LifeStateUnknown is the fail-closed state for an unknown combatant kind.
	LifeStateUnknown LifeState = ""
	// LifeStateConscious can act and participate normally.
	LifeStateConscious LifeState = "conscious"
	// LifeStateDying is a down character who must make death saves.
	LifeStateDying LifeState = "dying"
	// LifeStateStabilized is a down character who no longer makes death saves.
	LifeStateStabilized LifeState = "stabilized"
	// LifeStateDead is a dead character.
	LifeStateDead LifeState = "dead"
	// LifeStateDefeated is a down monster.
	LifeStateDefeated LifeState = "defeated"
)

// LifeStateInput contains the facts needed to derive a combatant's life state.
// Down must be the canonical [IsDown] answer rather than a hit-point comparison.
type LifeStateInput struct {
	Kind       CombatantKind
	Down       bool
	Stabilized bool
	Dead       bool
}

// Participation describes how a derived life state participates in combat.
type Participation struct {
	State             LifeState
	Down              bool
	CanActNormally    bool
	NeedsDeathSave    bool
	RetainsInitiative bool
	AutoPassesTurn    bool
	AttackTarget      bool
	Conscious         bool
}

// ClassifyLifeState derives a combatant's life state from canonical facts.
func ClassifyLifeState(in LifeStateInput) LifeState {
	switch in.Kind {
	case CombatantKindCharacter:
		switch {
		case in.Dead:
			return LifeStateDead
		case in.Down && in.Stabilized:
			return LifeStateStabilized
		case in.Down:
			return LifeStateDying
		default:
			return LifeStateConscious
		}
	case CombatantKindMonster:
		if in.Down {
			return LifeStateDefeated
		}
		return LifeStateConscious
	default:
		return LifeStateUnknown
	}
}

// ParticipationFor returns the complete combat participation policy for state.
// Unknown states fail closed.
func ParticipationFor(state LifeState) Participation {
	switch state {
	case LifeStateConscious:
		return Participation{
			State:             LifeStateConscious,
			CanActNormally:    true,
			RetainsInitiative: true,
			AttackTarget:      true,
			Conscious:         true,
		}
	case LifeStateDying:
		return Participation{
			State:             LifeStateDying,
			Down:              true,
			NeedsDeathSave:    true,
			RetainsInitiative: true,
			AttackTarget:      true,
		}
	case LifeStateStabilized:
		return Participation{
			State:             LifeStateStabilized,
			Down:              true,
			RetainsInitiative: true,
			AutoPassesTurn:    true,
			AttackTarget:      true,
		}
	case LifeStateDead:
		return Participation{State: LifeStateDead, Down: true}
	case LifeStateDefeated:
		return Participation{State: LifeStateDefeated, Down: true}
	default:
		return Participation{State: LifeStateUnknown}
	}
}

// PartyState is the participation snapshot used to determine party defeat.
type PartyState struct {
	// Members contains player-character participation only. An empty party is
	// not defeated.
	Members []Participation
}

// PartyDefeated reports whether a non-empty party has no conscious member.
func PartyDefeated(party PartyState) bool {
	if len(party.Members) == 0 {
		return false
	}

	for _, member := range party.Members {
		if member.Conscious {
			return false
		}
	}
	return true
}
