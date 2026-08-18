// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// Standing reports which of the given members are down — out of the fight,
// however the rulebook decides that. The composition asks; the rulebook
// answers.
//
// Injected rather than held, exactly as [Decider] and [InitiativeRoller] are,
// and for the same reason [InitiativeRoller]'s doc gives with "randomness"
// swapped for "hit points": this module's go.mod cannot import the rulebook
// (law C1), so defeat is a fact it can only be TOLD. Member IDs in, member IDs
// out — nothing here learns what a hit point is, and nothing here has to.
//
// # It is a pull, and that is the whole design
//
// The composition stores no down flag, in memory or in its blob. It asks at
// every choke point where the answer could matter, and acts on the answer it
// gets. The alternative — being pushed a "this one is down" event and
// remembering it — recreates the dual state the seam reshape spent #1040
// removing: heal a character and the sheet says four hit points while the
// composition still says down. There is one source of truth for defeat and it
// is not this package.
//
// Being a pull is also what makes it COMPLETE. Every route to zero — a strike,
// a hazard, a rule nobody has written yet — is noticed at the next consult,
// without that route knowing this interface exists.
//
// # What an implementation owes
//
// Answer about the members you were asked about. The composition hands over its
// current roster, and a name in the answer that was not in the question is a
// defect it refuses (ErrNotMember) rather than ignores — a mis-wired capability
// must look like a mis-wired capability, not like a rule that silently never
// fires. Order does not matter and duplicates are harmless.
//
// Errors abort whatever verb was running, atomically (R5), the same as
// [InitiativeRoller]'s: a world that cannot find out who is standing does not
// half-act on a guess.
type Standing interface {
	// Standing reports which of the given members are down. Returning none is
	// the ordinary answer.
	Standing(members []MemberID) (down []MemberID, err error)
}
