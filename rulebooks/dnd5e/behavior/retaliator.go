// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"cmp"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Excuse is what lets a SEEN actor off a grudge — the half of a grudge the
// clock does not own. It is asked only about an actor the mind can
// currently see: a figure it merely remembers being shot by has no hands to
// check, and Patience is what remains for one of those.
//
// The vocabulary is closed and small on purpose. Each value is a rule the
// mind applies to what it sees, and a rule nobody has named is not one a
// profile can hold.
//
// The TYPE cannot enforce that, being a string, and [NewMindedInput.Minds]
// is a door a caller's own mind comes through — so the runtime answer is
// written down here instead of left to be discovered: AN UNRECOGNIZED VALUE
// EXCUSES NOBODY. A misspelt "Unarmed" behaves exactly as [ExcuseNever] and
// reports nothing, which is fail-closed on the grudge and still a surprise
// if you expected the weapon rule.
//
// Nothing validates it, because nothing authors it: the rulebook's own
// presets are the only writers today and a table test pins every field they
// set. A use case that lets a host or a file write an Excuse is what pays
// for the check, and the same goes for [Retaliator.Room].
type Excuse string

// Excuse constants.
const (
	// ExcuseNever excuses nobody: whoever attacked it stays answerable
	// until the clock runs out, whatever they are holding now.
	//
	// It is the ZERO VALUE, and that is a claim rather than an accident. An
	// Excuse is a rule for letting somebody GO, so an author who set a
	// grudge and named no excuse wrote down no way out of it — and the
	// honest reading of an unwritten release rule is that there is none.
	// The alternative zero, [ExcuseUnarmed], would have a profile assert a
	// weapon rule its author never typed.
	ExcuseNever Excuse = ""
	// ExcuseUnarmed lets go of an actor the mind can see holding nothing
	// that could shoot back. The bow skeleton's rule, bought by #1725's
	// first walk: a skeleton that cannot be shot at any more has no reason
	// to keep answering a bow.
	ExcuseUnarmed Excuse = "unarmed"
)

// Grudge is what a mind does about a deed done to it: how long it stays
// worth answering, and what lets the actor off before then. It is the first
// half of a [Retaliator]'s profile and a plain value, so a preset is a
// literal and a mind that holds no grudge is the zero.
//
// ZERO VALUE: no grudge at all. Patience is a SPAN — how many ticks a deed
// stays worth answering, counting from the tick it was confirmed — so a
// span of zero is a deed that was never worth answering, not one answerable
// for an instant. That reading is what makes Grudge{} honest, and it is why
// Patience is a span rather than a maximum age: a maximum age of zero still
// answers a deed landed this very tick, and a mind with no grudge would
// hold one for exactly as long as anybody was looking.
type Grudge struct {
	// Patience is how many ticks a deed stays worth answering. 2 answers a
	// deed on the tick it lands and the tick after; 0 answers none.
	Patience uint64
	// Excuse is what lets a SEEN actor off before Patience runs out.
	Excuse Excuse
}

// fresh reports whether a deed confirmed at that tick is still worth
// answering at this one. A deed confirmed at or after the clock's own
// high-water is as fresh as a deed gets, which is why the age is floored
// rather than subtracted straight: the clock is the situation's and the
// testimony's counters are perception's, and nothing here is the party that
// gets to assume they agree.
func (g Grudge) fresh(at, confirmed uint64) bool {
	var age uint64
	if at > confirmed {
		age = at - confirmed
	}

	return age < g.Patience
}

// Retaliator is the rulebook's one grudge-holding mind, and the three words
// it ships under are three profiles of it rather than three types: it turns
// on whoever attacked it while that deed is still worth answering, it keeps
// whatever room it wants, and it otherwise goes for the closest. It is the
// captain's retarget-on-a-witnessed-deed from mind/behavior's proofs with a
// shot in place of a heal.
//
// Four judgments and no state. Judge attaches an attack deed to the figure
// the deed names — a claim, because the deed already says who and the mind
// still has to believe it. Name is the member's own id. Rank is the grudge,
// then the distance. Keep is [Retaliator.Room].
//
// # The profile feeds the ranking; it never reorders it
//
// Rank's order is fixed and the profile cannot touch it: a grudge first,
// then the live ahead of the remembered, then the closest, then the subject
// as a tiebreak. What [Retaliator.Grudge] decides is WHO has a grudge, not
// where a grudge sits in that list — and what [Retaliator.Room] decides is
// how much space it wants, which the ladder's rung 0 reads and this mind
// does not. Neither may reorder the ladder itself. That is the line
// `docs/ideas/mind/behavior/scenarios.md` draws between a profile and a
// claim: "how far the monster takes it" is the ladder abandoning one rung
// for another, and only a claim may say that.
//
// The grudge is kind-blind, by design: any attack deed against me counts,
// whoever landed it. A non-creature attacker — a dominated ally, a
// friendly-fire swing — therefore ranks first and is still never attacked,
// because the ladder attacks only creatures, and the mind walks toward it
// instead. No use case has paid for kind-filtering, and the dominated case
// argues the blind grudge is right.
//
// # Why a weapon, and not only a clock
//
// The first walk of #1725 found the arrow only half drawn: the fighter shot
// the skeleton with a crossbow, put it away, drew a longsword and closed —
// and the skeleton kept shooting past the nearer barbarian until the timer
// ran out. A skeleton that cannot be shot at any more has no reason to keep
// answering a bow. So the skeleton's grudge is a WEAPON rule first
// ([ExcuseUnarmed]): it holds only while the shooter is seen holding
// something ranged. Patience stays as the fallback for a shooter the mind
// cannot currently see — a ghost it remembers being shot by, whose hands it
// therefore cannot check.
//
// A berserker is the same mind with [ExcuseNever]: the walk's finding was
// about a skeleton, and a thug that keeps coming after you drop the
// crossbow is a monster, not a bug.
type Retaliator struct {
	// Space is how it knows what closest means. Geometry is the caller's
	// (mind/behavior R11); a mind may ask.
	Space behavior.Space
	// Grudge is what it does about being attacked. The zero holds none.
	Grudge Grudge
	// Room is how much space it wants between itself and a live creature,
	// in the Space's own steps — what [Retaliator.Keep] answers, and the
	// only field the ladder's rung 0 reads. 0 stands and fights; 2 backs
	// away from anything that gets within two steps.
	//
	// A NEGATIVE ROOM IS NO ROOM. Rung 0 keeps a creature it measures at
	// fewer steps than this, and nothing is ever fewer than zero steps
	// away, so a negative value skips every contact and the mind behaves
	// exactly as if it were 0. Unvalidated for the reason given on
	// [Excuse]: only the rulebook's presets write it.
	//
	// It is not called Keep because the judgment is: a mind answers
	// [behavior.Mind.Keep] with a method, and a field of that name could
	// not sit beside it.
	Room int
	// Ranged says whether an item id names a weapon that can shoot back.
	// It is the catalog knob and not a profile field — a monster that
	// answers a crossbow but shrugs off a thrown dagger is not a different
	// mind, it is really just data — and nil means the rulebook's own
	// weapon catalog. Only [ExcuseUnarmed] ever asks it.
	Ranged func(item string) bool
}

// Judge attaches every attack deed to the member it names as actor, when
// the actor is held at all. The deed's subject is qualified by channel, so
// the store never merged them; only this claim does.
func (r *Retaliator) Judge(in *behavior.JudgeInput) (*behavior.JudgeOutput, error) {
	held := func(id core.EntityID) bool {
		return slices.ContainsFunc(in.Holdings, func(h behavior.Holding) bool { return h.Subject == id })
	}

	var same []behavior.Pair

	for _, h := range in.Holdings {
		if h.Channel != deed.Channel {
			continue
		}

		if d, err := deed.Decode(h.Payload); err == nil && d.Verb == encounter.DeedAttack && d.Actor != "" && held(d.Actor) {
			same = append(same, behavior.Pair{A: h.Subject, B: d.Actor})
		}
	}

	return &behavior.JudgeOutput{Same: same}, nil
}

// Name calls a contact by the plain member id of the figure it is about. A
// monster has no words for the people it fights; the id is the encounter's
// and never parsed.
//
// Which id is not a detail. A contact's sorted-first subject is the deeds
// handle whenever the bundle holds one and the member id sorts after it, and
// a figure first met as a ghost would then be called by a handle for as long
// as the deed lives — a word matching no member the encounter offers. So the
// id comes from the testimony: sight's subject, else the actor the deed
// names. A contact it cannot put an id to gets no word at all, and the ladder
// declines to aim at what has no name.
func (r *Retaliator) Name(in *behavior.NameInput) (*behavior.NameOutput, error) {
	id, ok := memberID(in.Contact.Holdings)
	if !ok {
		return &behavior.NameOutput{}, nil
	}

	return &behavior.NameOutput{Name: behavior.Name(id), Named: true}, nil
}

// Rank puts whoever attacked ME, while the grudge still stands, first; then
// the closest. A deed against somebody else is not a grudge. Ghosts rank by
// the same distance, after everything live.
func (r *Retaliator) Rank(in *behavior.RankInput) (*behavior.RankOutput, error) {
	s := in.Situation
	ranked := slices.Clone(s.Contacts)

	distance := func(c behavior.Contact) int {
		out, err := r.Space.Distance(&behavior.DistanceInput{From: s.Self.Where, To: c.Where()})
		if err != nil || !out.Known {
			return int(^uint(0) >> 1)
		}

		return out.Steps
	}

	slices.SortStableFunc(ranked, func(a, b behavior.Contact) int {
		if ga, gb := r.grudge(a, s), r.grudge(b, s); ga != gb {
			if ga {
				return -1
			}

			return 1
		}

		if la, lb := a.Current(), b.Current(); la != lb {
			if la {
				return -1
			}

			return 1
		}

		if c := cmp.Compare(distance(a), distance(b)); c != 0 {
			return c
		}

		return cmp.Compare(a.Holdings[0].Subject, b.Holdings[0].Subject)
	})

	return &behavior.RankOutput{Ranked: ranked}, nil
}

// grudge reports whether the contact holds an attack deed against the actor
// itself that is still worth answering: inside the grudge's patience, and
// not excused.
//
// A mind may decode a payload itself (mind/behavior R2), which is why the
// hands are read here and [behavior.Reading] stays as narrow as it is: what
// a bow MEANS is this mind's business, not the driver's.
func (r *Retaliator) grudge(c behavior.Contact, s behavior.Situation) bool {
	for _, h := range c.Holdings {
		if h.Channel != deed.Channel {
			continue
		}

		d, err := deed.Decode(h.Payload)
		if err != nil || d.Verb != encounter.DeedAttack || d.Target != s.Actor {
			continue
		}

		if !r.Grudge.fresh(s.At, h.Confirmed) {
			continue
		}

		if !r.excused(c) {
			return true
		}
	}

	return false
}

// excused reports whether this profile's Excuse lets the contact go. Only a
// contact the mind can currently SEE is ever excused: an excuse is a rule
// about what a figure is doing now, and a figure it merely remembers is
// doing nothing it can observe.
func (r *Retaliator) excused(c behavior.Contact) bool {
	switch r.Grudge.Excuse {
	case ExcuseUnarmed:
		seen, armed := r.hands(c)

		return seen && !armed
	case ExcuseNever:
		return false
	default:
		// An Excuse nobody named excuses nobody, which is [Excuse]'s own
		// documented answer rather than a fallthrough nobody chose.
		return false
	}
}

// hands reads what the contact is currently SEEN holding: whether sight is
// delivering them at all, and whether either hand holds a ranged weapon.
// Seen with no equipment testimony is seen with nothing to go on — the mind
// saw the figure, not the hands — and that is not a bow.
func (r *Retaliator) hands(c behavior.Contact) (seen, armed bool) {
	for _, h := range c.Holdings {
		if h.Channel != perception.Sight || !h.CurrentOn(perception.Sight) {
			continue
		}

		seen = true

		testimony, ok := encounter.DecodeSightTestimony(h.Payload)
		if !ok || testimony.Equipment == nil {
			continue
		}

		if r.ranged(testimony.Equipment.MainHand) || r.ranged(testimony.Equipment.OffHand) {
			armed = true
		}
	}

	return seen, armed
}

// ranged asks the knob, or the catalog when there is no knob.
func (r *Retaliator) ranged(item string) bool {
	if r.Ranged != nil {
		return r.Ranged(item)
	}

	return rangedByCatalog(item)
}

// rangedByCatalog is the rulebook's own answer: an item id is ranged when
// the weapon catalog knows it and calls it ranged. An id the catalog has
// never heard of — a shield, a torch, an empty hand — is not a bow, and is
// not an error either: this is a monster looking at a pair of hands, not a
// validator.
func rangedByCatalog(item string) bool {
	w, ok := weapons.All[item]

	return ok && w.IsRanged()
}

// Keep is [Retaliator.Room]. A profile that wants none stands and fights.
func (r *Retaliator) Keep(*behavior.KeepInput) (*behavior.KeepOutput, error) {
	return &behavior.KeepOutput{Steps: r.Room}, nil
}
