// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package combat

import (
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

// Ledger is the account an action is paid out of.
//
// Both halves of a fight's cast hold one, and neither holds it the same way: a
// character's is persisted on the sheet and reaches storage only if the sheet
// reports itself dirty, while a monster's is a plain [ActionEconomy] its caller
// hands to the turn and throws away after. The gate is not allowed to tell them
// apart. Monsters take no gated action in v1 — but a signature that could not
// accept one would have to be replaced rather than extended on the day they do.
//
// Every method is over state the holder already has. Nothing here takes a
// context, a bus or a repository: paying is arithmetic on a sheet the runner is
// already holding, which is what lets the gate stay somewhere a machine cannot
// reach it.
type Ledger interface {
	// InCombat reports whether there is an economy to spend from at all.
	//
	// A holder that is not in combat refuses every payment, including one that
	// only grants. A grant with nowhere to land is worse than a refusal: the
	// caller is told the action was paid for and the capacity is simply gone.
	InCombat() bool

	// SlotsLeft reports how many of a per-turn slot remain. Only the three
	// slots a turn holds — action, bonus action, reaction — can answer;
	// anything else reports nothing left rather than unlimited, so a cost
	// keyed to it is refused instead of granted for free.
	SlotsLeft(slot coreCombat.ActionType) int

	// CapacityLeft reports how much of a keyed capacity remains. Every member
	// of [CapacityTypes] must answer, and must answer about the same state the
	// holder persists — a keyed view over a private copy is a spend that the
	// next write-back discards.
	CapacityLeft(key CapacityType) int

	// PoolLeft reports the points remaining in a keyed pool. A pool the holder
	// does not have reports zero, which refuses a cost in it; a monster with no
	// ki must not be able to pay for Flurry of Blows by not paying.
	PoolLeft(key coreResources.ResourceKey) int

	// SpendSlots debits per-turn slots. The gate calls it only after a
	// complete affordability check, so it takes no error return and must not
	// refuse — see [Pay] on why that is what makes a multi-currency debit
	// atomic.
	SpendSlots(slot coreCombat.ActionType, n int)

	// SpendCapacity debits keyed capacity, under the same contract as
	// [Ledger.SpendSlots].
	SpendCapacity(key CapacityType, n int)

	// SpendPool debits a keyed pool, under the same contract as
	// [Ledger.SpendSlots].
	SpendPool(key coreResources.ResourceKey, n int)

	// BankCapacity ADDS keyed capacity rather than assigning it, so an action
	// taken twice in one turn banks twice.
	BankCapacity(key CapacityType, n int)
}

// CanPay reports whether this ledger could pay this profile right now, without
// paying it.
//
// It is a pure read — no debit, no mark, nothing the caller has to undo — which
// is what makes it usable at a reaction moment: a wizard who has already spent
// their reaction has no Shield window to be offered, and asking must not be the
// thing that spends it. It answers from the same check [Pay] runs, so there is
// no state in which one says yes and the other refuses.
//
// A nil profile is a free action and is always payable.
func CanPay(l Ledger, p *SpendProfile) bool {
	return check(l, p) == nil
}

// Pay debits every currency the profile names, or none of them.
//
// ATOMIC IS THE POINT. The whole affordability check runs first, over every
// currency, and only then does anything move — so a bonus strike that needs a
// bonus action AND a ki point cannot take the bonus action and then discover
// there is no ki. That is why [Ledger]'s debit methods return no error: by the
// time one is called the check has already passed, and a debit that could
// refuse would put a partial payment back on the table.
//
// A refused payment is indistinguishable from one that was never attempted. On
// a character's sheet that includes the dirty flag: nothing moved, so there is
// nothing to save, and the sheet stays exactly as clean as it was found.
//
// What Pay does NOT do is decide whether the action should happen. It charges a
// price. Whether there was a target, whether the machine can run, whether the
// swing hits — all of that is somebody else's, and all of it happens after.
func Pay(l Ledger, p *SpendProfile) error {
	if err := check(l, p); err != nil {
		return err
	}

	if p == nil {
		return nil
	}

	for slot, amount := range p.Slots {
		l.SpendSlots(slot, amount)
	}
	for key, amount := range p.Capacity {
		l.SpendCapacity(key, amount)
	}
	for key, amount := range p.Pools {
		l.SpendPool(key, amount)
	}

	// Grants land last so a profile that spends and banks the same capacity —
	// a hypothetical action that trades one attack for two — nets out the same
	// way whichever order the maps happen to iterate in.
	for key, amount := range p.Grants {
		l.BankCapacity(key, amount)
	}

	return nil
}

// check is the affordability question, asked in full. Both gate functions go
// through it, so they cannot drift apart, and [Pay] runs it to completion
// before touching anything.
func check(l Ledger, p *SpendProfile) error {
	if l == nil {
		return rpgerr.New(rpgerr.CodeNil, "no ledger to pay from")
	}
	if p == nil {
		return nil
	}
	if err := p.Validate(); err != nil {
		return err
	}
	if p.movesNothing() {
		return nil
	}
	if !l.InCombat() {
		return rpgerr.New(rpgerr.CodeInvalidState,
			"no action economy to pay from: not in combat")
	}

	for slot, amount := range p.Slots {
		if l.SlotsLeft(slot) < amount {
			return rpgerr.ResourceExhaustedf("%s: %d needed, %d left",
				slot, amount, l.SlotsLeft(slot))
		}
	}
	for key, amount := range p.Capacity {
		if l.CapacityLeft(key) < amount {
			return rpgerr.ResourceExhaustedf("%s: %d needed, %d left",
				key, amount, l.CapacityLeft(key))
		}
	}
	for key, amount := range p.Pools {
		if l.PoolLeft(key) < amount {
			return rpgerr.ResourceExhaustedf("%s: %d needed, %d left",
				key, amount, l.PoolLeft(key))
		}
	}

	// Requirements are read against the same state the costs were, before
	// anything is spent. A precondition asks what was true when the action was
	// declared, not what is left after paying for it.
	for key, amount := range p.Requires {
		if l.CapacityLeft(key) < amount {
			return rpgerr.PrerequisiteNotMetf("%s: %d needed, %d present",
				key, amount, l.CapacityLeft(key))
		}
	}

	return nil
}
