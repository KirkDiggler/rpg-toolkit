// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"fmt"
	"testing"
)

// markingRoller is a distinct roller value: comparing it against a condition's
// field proves which roller the condition holds.
type markingRoller struct {
	name string
}

func (m *markingRoller) Roll(_ context.Context, _ int) (int, error) { return 1, nil }

func (m *markingRoller) RollN(ctx context.Context, count, size int) ([]int, error) {
	rolls := make([]int, count)
	for i := range count {
		roll, err := m.Roll(ctx, size)
		if err != nil {
			return nil, err
		}
		rolls[i] = roll
	}

	return rolls, nil
}

func (m *markingRoller) String() string { return fmt.Sprintf("roller(%s)", m.name) }

// TestBindRollerNilKeepsTheExplicitRoller pins the nil half of the contract
// across every rolling condition: nil means "leave the current roller as it
// is", never "erase" — so a condition constructed with an explicit roller
// keeps it through a nil binding.
func TestBindRollerNilKeepsTheExplicitRoller(t *testing.T) {
	explicit := &markingRoller{name: "explicit"}

	gwf := NewFightingStyleGreatWeaponFightingCondition("member-1", explicit)
	gwf.BindRoller(nil)
	if gwf.roller != explicit {
		t.Fatal("great weapon fighting: nil binding erased an explicit roller")
	}

	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "member-1", Level: 3, Roller: explicit})
	sneak.BindRoller(nil)
	if sneak.roller != explicit {
		t.Fatal("sneak attack: nil binding erased an explicit roller")
	}

	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: "member-1", Level: 9, Roller: explicit})
	brutal.BindRoller(nil)
	if brutal.roller != explicit {
		t.Fatal("brutal critical: nil binding erased an explicit roller")
	}

	ma := NewMartialArtsCondition(MartialArtsInput{MemberID: "member-1", Roller: explicit})
	ma.BindRoller(nil)
	if ma.roller != explicit {
		t.Fatal("martial arts: nil binding erased an explicit roller")
	}
}

// TestBindRollerBindsTheSuppliedRoller pins the positive half: a condition
// restored from persisted JSON holds no roller, and a non-nil binding gives it
// the supplied one.
func TestBindRollerBindsTheSuppliedRoller(t *testing.T) {
	bound := &markingRoller{name: "bound"}

	gwf := NewFightingStyleGreatWeaponFightingCondition("member-1", nil)
	gwf.BindRoller(bound)
	if gwf.roller != bound {
		t.Fatal("great weapon fighting: binding did not set the supplied roller")
	}

	sneak := NewSneakAttackCondition(SneakAttackInput{MemberID: "member-1", Level: 3})
	sneak.BindRoller(bound)
	if sneak.roller != bound {
		t.Fatal("sneak attack: binding did not set the supplied roller")
	}

	brutal := NewBrutalCriticalCondition(BrutalCriticalInput{MemberID: "member-1", Level: 9})
	brutal.BindRoller(bound)
	if brutal.roller != bound {
		t.Fatal("brutal critical: binding did not set the supplied roller")
	}

	ma := NewMartialArtsCondition(MartialArtsInput{MemberID: "member-1"})
	ma.BindRoller(bound)
	if ma.roller != bound {
		t.Fatal("martial arts: binding did not set the supplied roller")
	}
}
