// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/selection"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// option.go is THE ONE SOCIAL VERB (rpg-project#569, the mad king use case
// §2 and rulings R1–R3, R9): a line a character can SAY to a creature or a
// prop, the check that line rolls, what it waits on, and what is answered on
// either verdict.
//
// # It replaced two verbs and four table keys
//
// Intimidate and Persuade were one body ([Encounter.social], as it was) under
// two names, and what a creature answered was keyed by the VERB on its table
// (`intimidated`, `persuade_failed`, …). That could say one thing per verb per
// verdict and nothing else: a creature answered a threat the same way however
// it was phrased, and a line that rolled Performance or Religion had nowhere to
// live at all. An option is the line itself — flat, authored, gated by facts
// (R1) — and it carries its own answer rows (R2), so the next line is a row in
// the file and never a field here.
//
// # The deed is the skill, not the line
//
// A beaten check lands the deed of the SKILL it rolled ([deedForSkill], R3):
// Intimidation lands `intimidate` on every witness whatever the line said,
// Persuasion lands `persuade`, and every other skill lands nothing. What a
// held deed MEANS is still the mind's (rulebooks/dnd5e/behavior) — this verb
// lands testimony and forms no opinion.
//
// # The audience is the witnesses, and the target must be among them
//
// Who learns is [Encounter.witnessesOf] — sight and line of sight from the
// ACTOR's cell, the set [Encounter.landAttack] computes. A creature spoken to
// must be able to see who is speaking ([ErrUnwitnessed]); a PROP sees nothing,
// so speaking to one is gated by the actor seeing the prop (the probe law,
// [ErrNoProp]) and its audience is still whoever sees the actor — the fact an
// altar teaches lands on everyone who watched the character read it.
//
// # `requires` is read, never fired
//
// An option's [Option.Requires] is judged against the SPEAKER's own holdings
// (R9): a fact that speaker knows, a prop that speaker carries. It is asked at
// the offer ([Encounter.OptionsFor]) and again at the door ([Encounter.Speak]),
// and nowhere else — which is why [TriggerHolds] exists only here and is
// refused as an ending, an arrival or an until. [TriggerInteraction] is the
// one requirement that reads the RUN rather than the speaker (rpg-project#922,
// #916 R2: the entrance remembers for the run), and is refused in the same
// three places. [TriggerRecognized] reads the CREATURE SPOKEN TO — what he
// holds about the speaker (rpg-project#928) — and is refused there too.

// Option is one line a character can say to a creature or a prop: what is
// said, the check it rolls, what it waits on, and what is answered on each
// verdict.
type Option struct {
	// ID names the line on its target. REQUIRED non-empty and unique among
	// one target's options — the session offers it by this id and
	// [SpeakInput.OptionID] names it back.
	ID string

	// Say is the character's line, carried VERBATIM onto the [BeatSpeech]
	// versions of whoever catches its words. REQUIRED non-empty: an option is
	// something said.
	Say string

	// Loudness is how loudly the line is said (rpg-project#933, R10): one of
	// [Loudnesses], empty for [LoudnessSpeak]. A whispered line is admitted
	// only from beside the member it is said to. Omitted when empty, so a
	// world compiled before the field pictures exactly as it did.
	Loudness string `json:"Loudness,omitempty"`

	// Check is the approaches through the line's check — the same list a
	// lock carries, priced per route, each with a DC of at least 1. CARRIED,
	// NEVER COMPARED, as every check is here: the session rolls it, and this
	// composition is told whether it was beaten.
	//
	// NIL IS A PLAIN LINE (rpg-project#785 R5: "the check is the action…
	// speaking with no check would be a free action"). A plain line rolls
	// nothing, lands no deed, is always answered from [Option.Success], and
	// costs nothing on either clock: the world pays no round for it here, and
	// the session spends no action slot for it. [Option.Checked] is the one
	// question a caller asks to tell the two apart. A plain line carries no
	// [Option.Failure] rows — it cannot be missed — and the door refuses them.
	Check []CheckApproach

	// Requires is what must hold before the line is offered at all, EVERY
	// ONE OF THEM (rpg-project#922): nil or empty for always, else a list of
	// requirements, each a [TriggerFact] (the speaker knows the fact), a
	// [TriggerHolds] (the speaker carries the prop), a [TriggerInteraction]
	// (the run's interaction is in one of the named states — judged per run,
	// not per speaker, #916 R2), a [TriggerRecognized] (the creature spoken
	// to holds the topic about the speaker, rpg-project#928; refused on a
	// prop's line), or a `not` of one. Each
	// is one named requirement in the pool [Encounter.evaluateOptions]
	// builds, and the line is offered only when all of them are Met. No
	// other trigger is a requirement, and a nil item names nothing; the door
	// refuses both ([ErrBadMember] on a member, [ErrNoField] on a prop).
	Requires []Trigger `json:"Requires,omitempty"`

	// Advance is the interaction this line's verdict steps, and on which
	// beat for each verdict (rpg-project#922). Nil advances nothing.
	Advance *Advance `json:"Advance,omitempty"`

	// Success is the rows answered when the check was beaten, rolled under
	// [AnswerSuccess]. Nil answers nothing.
	Success []Answer `json:"Success,omitempty"`

	// Failure is the rows answered when the check was missed, rolled under
	// [AnswerFailure]. Nil answers nothing.
	Failure []Answer `json:"Failure,omitempty"`

	// Grants is what a row lands when it is the row answered (recognition.go,
	// rpg-project#929): the consequence an interaction's outcome uses, on a
	// reply. A success row's grant lands when the line lands — a plain line
	// always, a checked one when beaten; a failure row's (rpg-project#932)
	// when a checked line is missed. Nil grants nothing.
	Grants []RowGrant `json:"Grants,omitempty"`
}

// Checked reports whether the line rolls a check — whether it carries any
// [Option.Check] approach. A checked line is an action: the action slot on
// the turn clock, one round of the world in free roam. A plain line is free
// on both (rpg-project#785 R5). The session prices a Speak offer per option
// by this answer.
func (o Option) Checked() bool { return len(o.Check) != 0 }

// TriggerHolds is a requirement that the speaker CARRIES a prop
// (`requires: { holds: <prop id> }`, rpg-project#569).
//
// READ, NEVER EDGE-FIRED. It is asked of one member at the moment an option
// is offered or spoken, and of no one at any other time: nothing waits for it,
// so it is refused as an ending, an arrival and an until. The prop must be one
// this field declares holdable — a requirement nobody can ever meet is an
// option nobody can ever be offered, refused at the door it came in through.
type TriggerHolds struct {
	// Prop is the prop the speaker must be carrying. REQUIRED non-empty.
	Prop PropID
}

// isTrigger marks TriggerHolds as a Trigger.
func (t TriggerHolds) isTrigger() {}

// TriggerInteraction is a requirement that one of the run's interactions is
// in one of the named states (`requires: { interaction: { id, in: [...] } }`,
// rpg-project#922).
//
// JUDGED PER RUN, NOT PER SPEAKER — the one requirement that ignores who is
// speaking, by design: #916 R2, "the entrance remembers for the run" —
// changing speaker or guard, backing away or reloading does not create a
// fresh attempt. Read, never edge-fired, as [TriggerHolds] is: an ending, an
// arrival and an until refuse it. The interaction must be one the run has
// and every state one its recipe declares, refused at the door otherwise.
type TriggerInteraction struct {
	// Interaction is the interaction read. REQUIRED non-empty.
	Interaction InteractionID

	// In is the states any one of which satisfies the requirement.
	// REQUIRED non-empty.
	In []string
}

// isTrigger marks TriggerInteraction as a Trigger.
func (t TriggerInteraction) isTrigger() {}

// Advance is what a line's SETTLED VERDICT lands on one of the run's
// interactions (rpg-project#922, #916 R3): the beat stepped on a beaten
// check, and the beat stepped on a missed one. Which flavour row answered
// does not advance anything; the verdict does.
//
// Stepped synchronously inside [Encounter.Speak], after the answer has
// landed, so the very next offer reads the new state. A beat the current
// state does not listen for steps nothing and writes no beat.
type Advance struct {
	// Interaction is the interaction stepped. REQUIRED, one the run has.
	Interaction InteractionID

	// Success is the beat stepped when the check was beaten (and for a
	// plain line, which always lands). Empty steps nothing.
	Success machine.Beat

	// Failure is the beat stepped when the check was missed. Empty steps
	// nothing.
	Failure machine.Beat
}

// BeatInteraction is the "beat" value of the story beat this composition
// appends when a line's verdict moves one of the run's interactions: its id,
// the state it left and entered, the transition's reason, the option and
// the verdict. Audienced to the line's witnesses. EXPORTED BECAUSE A DECODER
// READS IT, as [BeatSpoke] is.
const BeatInteraction = "interaction"

// BeatSpoke is the "beat" value of the story beat this composition appends
// when somebody speaks an option — landed or not.
//
// EXPORTED BECAUSE A DECODER READS IT: a session-side decoder is written
// against it in the same wave, so a rename fails to compile there instead of
// quietly producing a beat nobody renders.
//
// IT IS THE SPEAKER'S OWN RECORD OF THE VERB (rpg-project#933, R9): the line,
// the check and the deed, written to the speaker alone. Every other member is
// told the line, and its roll, only by their own [BeatSpeech] version — a
// member who sees the speaker but is not reached by the words learns neither.
const BeatSpoke = "spoke"

// verbSpeak is the bare verb name the answered beat carries for a spoken
// answer, and the tag the spoke beat goes down under.
const verbSpeak = "speak"

// Witnesses is every member whose senses reach a member's own cell: who would
// learn what that member does where they stand.
//
// A READ, and the one the caller needs BEFORE it does anything expensive. The
// session prices a line in the actor's action and must know the target can
// see them before it charges — a verb that takes the action and then refuses
// is one nobody would forgive — and the same list is what an action panel
// offers as the people you can talk to. [Encounter.Speak] asks again for
// itself and does not trust the answer it handed out: this is a read of a
// moment, and a door may close between the two calls.
//
// Sorted, and includes the member themselves whenever they can see their own
// cell — which is every placed member, and is why the audience of a `spoke`
// beat has the actor in it.
//
// Errors: ErrNotMember (no such member), ErrBadPlacement (not placed).
func (e *Encounter) Witnesses(of MemberID) ([]MemberID, error) {
	_, witnesses, err := e.audienceOf(of)
	if err != nil {
		return nil, fmt.Errorf("witnesses: %w", err)
	}

	return witnesses, nil
}

// OptionsFor is the options a target creature offers one speaker: every
// option on the target whose [Option.Requires] holds for the SPEAKER (R9), in
// authored order, deep-copied, each with the interaction it touches and that
// interaction's revision now, and PENDING ([OfferedOption.Pending]) when
// the interaction waits on a reserved roll (interaction_hold.go, rpg-project#924). A
// target that is DOWN offers none (rpg-project#718): [Encounter.Speak]
// refuses a line said to it.
//
// A CREATURE WITH NO OPTIONS OFFERS NONE, and that is an empty list and a nil
// error — what the absence means (the shortfall a client is shown) is the
// offer side's to decide. Whether the target can SEE the speaker is not asked
// here: that is [Encounter.Witnesses]' question, asked by whoever prices the
// line, and asked again by [Encounter.Speak] at the door.
//
// Errors: ErrNotMember (no such speaker or target), and the participation
// capability's own error when who is down cannot be asked.
func (e *Encounter) OptionsFor(actor, target MemberID) ([]OfferedOption, error) {
	if _, ok := e.members[actor]; !ok {
		return nil, fmt.Errorf("options for: actor %q: %w", actor, ErrNotMember)
	}
	record, ok := e.members[target]
	if !ok {
		return nil, fmt.Errorf("options for: target %q: %w", target, ErrNotMember)
	}
	// A DOWN CREATURE OFFERS NONE (rpg-project#718), so the offer and
	// [Encounter.Speak]'s door agree: a line it would refuse is never shown.
	down, err := e.isDown(target)
	if err != nil {
		return nil, fmt.Errorf("options for: %w", err)
	}
	if down {
		return []OfferedOption{}, nil
	}

	offered, err := e.offered(actor, target, record.Options)
	if err != nil {
		return nil, fmt.Errorf("options for: %w", err)
	}

	return offered, nil
}

// PropOptionsFor is the options a prop offers one speaker: every option on
// the prop whose [Option.Requires] holds for the speaker, in authored order,
// deep-copied, offered as [Encounter.OptionsFor] offers a creature's.
//
// THE PROBE LAW FIRST. A prop the speaker cannot be shown — one that names
// nothing, is hidden from them, waits in reserve, or is in somebody else's
// hands — is refused exactly as an id that names nothing ([ErrNoProp]), so an
// offer cannot confirm a thing no map shows. A prop the speaker carries is
// always shown to them.
//
// Errors: ErrNotMember (no such speaker), ErrNoProp (no prop this speaker can
// be shown).
func (e *Encounter) PropOptionsFor(actor MemberID, prop PropID) ([]OfferedOption, error) {
	if _, ok := e.members[actor]; !ok {
		return nil, fmt.Errorf("prop options for: actor %q: %w", actor, ErrNotMember)
	}
	options, _, err := e.speakableProp(actor, prop)
	if err != nil {
		return nil, fmt.Errorf("prop options for: %w", err)
	}

	offered, err := e.offered(actor, "", options)
	if err != nil {
		return nil, fmt.Errorf("prop options for: %w", err)
	}

	return offered, nil
}

// evaluateOptions judges every option for one speaker as ONE POOL
// (rpg-project#921), the judgment the offer ([Encounter.offered]) and the
// door ([Encounter.lineDoor]) both read, so the two cannot disagree.
// One candidate per option, in authored order: its id is the option's id,
// its weight is zero — a line is chosen, never drawn (#917 R5) — and it
// carries ONE NAMED REQUIREMENT PER `requires` ITEM (rpg-project#922, #917
// R2), `requires:<n>:<kind>`, Met or Unmet as [Encounter.requirementHolds]
// answers for THIS speaker (R9), with a reason naming only the
// requirement's kind. A line with no requirement carries none and is
// eligible; a line with several is eligible only when every one is Met.
//
// PENDING IS UNKNOWN (rpg-project#924): a line touching an interaction that
// holds a reservation carries one more requirement, `held:<interaction>`,
// Unknown — for every speaker, the holder included — so it is never
// eligible while the roll waits. The one exception is settle, the
// reservation a Speak settles: its own reserved line is judged as it is.
//
// Errors: a blank or duplicate option id is a construction defect the field
// validator already refuses; one that reaches here is returned wrapping
// [selection.ErrInvalidCandidate], never swallowed.
func (e *Encounter) evaluateOptions(
	actor, other MemberID, options []Option, settle *InteractionReservation,
) (*selection.Evaluation, error) {
	candidates := make([]selection.Candidate, 0, len(options))
	for _, o := range options {
		var requirements []selection.Requirement
		for n, requires := range o.Requires {
			verdict := selection.Unmet
			if e.requirementHolds(actor, other, requires) {
				verdict = selection.Met
			}
			kind := requirementKind(requires)
			requirements = append(requirements, selection.Requirement{
				ID: fmt.Sprintf("requires:%d:%s", n, kind), Verdict: verdict, Reason: kind,
			})
		}
		if hold := e.holdOf(o); hold != nil && (hold != settle || o.ID != settle.Option) {
			id, _ := optionInteraction(o)
			requirements = append(requirements, heldRequirement(id))
		}
		candidates = append(candidates, selection.Candidate{ID: o.ID, Requirements: requirements})
	}
	evaluation, err := selection.Evaluate(&selection.EvaluateInput{Candidates: candidates})
	if err != nil {
		return nil, fmt.Errorf("evaluate options: %w", err)
	}

	return evaluation, nil
}

// requirementKind names the kind of an option's `requires` and nothing of
// its content, so a row's reason cannot leak the fact or prop it waits on.
func requirementKind(requires Trigger) string {
	switch requires.(type) {
	case nil:
		return "none"
	case TriggerNot:
		return "not"
	case TriggerFact:
		return "fact"
	case TriggerHolds:
		return "holds"
	case TriggerInteraction:
		return "interaction"
	case TriggerRecognized:
		return "recognized"
	default:
		return "unsupported"
	}
}

// offered is the eligible and the pending rows of
// [Encounter.evaluateOptions]' pool — the options whose requirements hold
// for the speaker, and those whose requirements hold but whose interaction
// waits on a reserved roll — in authored order, deep-copied so a caller's
// edit cannot reach the field. A gated line is absent, never greyed (the
// probe law); a pending line names who it waits on.
func (e *Encounter) offered(actor, other MemberID, options []Option) ([]OfferedOption, error) {
	evaluation, err := e.evaluateOptions(actor, other, options, nil)
	if err != nil {
		return nil, err
	}
	inspection, err := evaluation.Inspect()
	if err != nil {
		return nil, fmt.Errorf("inspect options: %w", err)
	}
	out := make([]OfferedOption, 0, inspection.Eligible)
	for i, row := range inspection.Rows {
		switch {
		case row.Eligible:
			out = append(out, e.offerOf(options[i], false, ""))
		case !hasUnmet(row):
			hold := e.holdOf(options[i])
			offered := e.offerOf(options[i], true, e.pendingOnFor(actor, hold.Holder))
			// AN ABANDONED ROLL IS TOLD TO ITS HOLDER ALONE, on the reserved
			// line (rpg-project#938): theirs to resume declined and settle.
			offered.Abandoned = hold.Abandoned && hold.Holder == actor && hold.Option == options[i].ID
			out = append(out, offered)
		}
	}

	return out, nil
}

// requirementHolds answers one requirement for "me" (actor) facing "the
// other party" (other): against MY own holdings (R9) — the facts I know, the
// props I carry — or, for [TriggerRecognized], against what the OTHER party
// holds about me (rpg-project#928). On a line, I am the speaker and the
// other party the creature spoken to ("" for a prop); on a post's pass, I am
// the walker and the other party the post's holder. Nil holds always. A
// trigger of any other kind holds never — the door refuses one at
// construction, so this arm is the fail-closed answer for a value that could
// only arrive by hand.
func (e *Encounter) requirementHolds(actor, other MemberID, requires Trigger) bool {
	switch t := requires.(type) {
	case nil:
		return true
	case TriggerNot:
		// The speaker's own read, negated (rpg-project#719, D3): withheld
		// from whoever learned the fact, still offered to whoever did not.
		// The door refuses a nil inner, so nil's "holds always" is never
		// what is negated.
		if t.Inner == nil {
			return false
		}
		return !e.requirementHolds(actor, other, t.Inner)
	case TriggerFact:
		return e.world.knowsFact(actor, t.Fact)
	case TriggerHolds:
		return slices.Contains(e.heldPropsOf(actor), t.Prop)
	case TriggerInteraction:
		// THE RUN'S, NOT THE SPEAKER'S (#916 R2): actor is not read.
		return e.interactionIn(t.Interaction, t.In)
	case TriggerRecognized:
		// THE OTHER PARTY'S HOLDING ABOUT ME (rpg-project#928): a line said
		// to a prop has no other party, and never holds.
		return e.recognizes(other, actor, t.Topic)
	default:
		return false
	}
}

// speakableProp answers a prop's options when the speaker may address it, and
// ErrNoProp — the one refusal an id that names nothing earns — when they may
// not: no such prop, hidden from them, in reserve, held by somebody else, or
// standing nowhere they are shown.
//
// THE SAME QUESTIONS [Encounter.Hold] ASKS, minus reach: whether a line can
// be said from where the speaker stands is [Encounter.Speak]'s question,
// asked after this one, so the cells the prop stands on are returned beside
// its options — nil for a prop the speaker carries, which is wherever they
// are.
//
// The probe law is [Encounter.shownTo] under the speaker's own mask, built
// once here: no atlas is projected to answer it.
func (e *Encounter) speakableProp(actor MemberID, prop PropID) ([]Option, []spatial.Position, error) {
	refused := fmt.Errorf("prop %q: %w", prop, ErrNoProp)
	if prop == "" || e.hiddenPropTo(actor, prop) {
		return nil, nil, refused
	}
	if index := e.field.propIndexOf(prop); index >= 0 {
		p := e.field.props[index]
		if slices.Contains(e.heldPropsOf(actor), prop) {
			return p.Options, nil, nil
		}
		cell := e.field.cellAt(p.At)
		placement, moved := e.holdings.propPlacements()[prop]
		if p.Arrives != nil && !placement.arrived {
			return nil, nil, refused
		}
		if moved && placement.gone {
			return nil, nil, refused
		}
		if moved {
			cell = placement.at
		}
		cells := []spatial.Position{cell}
		if !e.shownTo(e.undiscoveredMask(actor), cells) {
			return nil, nil, refused
		}

		return p.Options, cells, nil
	}
	if index := e.field.placedIndexOf(prop); index >= 0 {
		p := &e.field.placed[index]
		if slices.Contains(e.heldPropsOf(actor), prop) {
			return p.options, nil, nil
		}
		now := e.field.placedNow()
		placement, standing := now.stands(p)
		if !standing {
			return nil, nil, refused
		}
		cells := e.field.placedCellsOf(p.id, placement)
		if !e.shownTo(e.undiscoveredMask(actor), cells) {
			return nil, nil, refused
		}

		return p.options, cells, nil
	}

	return nil, nil, refused
}

// SpeakInput names who spoke, to what, which line, and the check the session
// rolled for it.
type SpeakInput struct {
	// Actor is the member speaking. Must be a member (ErrNotMember) and
	// placed (ErrBadPlacement).
	Actor MemberID

	// Target is the creature spoken to, or empty when the line is said to a
	// prop. EXACTLY ONE of Target and Prop (ErrNoMember otherwise). A target
	// must be able to SEE the actor ([ErrUnwitnessed]).
	Target MemberID

	// Prop is the prop spoken to, or empty when the line is said to a
	// creature. The actor must be shown it ([ErrNoProp]).
	Prop PropID

	// Range is how many cells away the target may stand, measured by
	// [Encounter.Distance] as every reach is; ZERO MEANS 1 (adjacent), as
	// it does for Hold and Interact. The session passes its interaction
	// reach (rpg-project#753, R4), the same one its offer was read within,
	// so the offer and this door measure alike. Asked after the shown and
	// witness checks ([ErrOutOfRange]); a prop the actor carries is always
	// within reach.
	Range int

	// OptionID names the line, on the target. It must be one the target
	// carries AND whose requirement holds for this actor ([ErrNoOption]) —
	// the offer and the door agree.
	OptionID string

	// Revision is the [OfferedOption.Revision] the line was offered at
	// (rpg-project#924). A line that reads or advances an interaction is
	// refused [ErrStaleOption] unless it names that interaction's revision
	// now — or, for the settle of a reserved line, the revision it was
	// reserved at. Zero is never current, so it is always refused for such a
	// line. Not read for a line that touches no interaction.
	Revision uint64

	// Skill is the approach the check was made with — one of the option's
	// [CheckApproach.Ability] words ([ErrNoOption] otherwise). It is what the
	// deed is decided by ([deedForSkill], R3) and what the beat reports.
	// EMPTY FOR A PLAIN LINE (rpg-project#785 R5), as DC, Total and
	// Calculation are: nothing was rolled, and a roll reported for a line
	// that rolls none is refused ([ErrNoOption]).
	Skill string

	// Beaten is whether the check beat the DC. CARRIED, NEVER COMPARED: this
	// module holds no 5e, so who decides a total beats a difficulty lives on
	// the other side of this seam. FALSE LANDS NO DEED and is not an error;
	// it answers from the option's failure rows. Not asked of a plain line,
	// which always lands: [SpeakOutput.Beaten] reports true for one.
	Beaten bool

	// DC and Total are the numbers the table sees, carried onto the beat so
	// the roll is visible whether it landed or not. Nothing here reads them.
	DC    int
	Total int

	// Calculation is the full sourced arithmetic behind Total — the d20 pool
	// with every face it threw and the keep record naming the rule that
	// decided which one counted (rpg-project#462). Optional; when present it
	// must describe Total and open with a d20 pool, or the line is refused
	// rather than written down wrong.
	Calculation *RollCalculation

	// Roller is THE WORLD'S DIE, the one the answer rows are picked with.
	// REQUIRED — supplied, never defaulted ([ErrNoRoller]): a silent default
	// puts untestable randomness into a result that looks fine, and the roll
	// is shown to the table. Per call, because the caller that rolled the
	// check is the caller that holds the die.
	Roller dice.Roller
}

// SpeakOutput reports what the line reached.
type SpeakOutput struct {
	// Beaten echoes what the caller said, so a caller reads the result off
	// the answer rather than off the fact that it called.
	Beaten bool

	// Witnesses is every member who saw it said. Sorted, the audience's own
	// order. Seeing it said teaches nothing of how it went (rpg-project#933).
	Witnesses []MemberID

	// Hearers is every member who caught the words — who was written the
	// line and its roll (rpg-project#933). In member order. Of them, those
	// who also saw the speaker hold the deed on a success.
	Hearers []MemberID

	// Deed is the deed verb that landed on the hearers who saw the speaker — [DeedIntimidate]
	// or [DeedPersuade] — or empty when none did: a miss, a skill that lands
	// none, or a prop.
	Deed string

	// Seq is the sequence number of the `spoke` beat. A missed line gets one
	// too: somebody tried, and the story is what happened.
	Seq uint64

	// Answered is the row the world's die picked from the verdict's rows,
	// with the roll — nil when the option has no rows for that verdict.
	Answered *Pick
}

// Speak reports a line said to a creature or a prop, lands the deed its skill
// lands on a beaten check, and rolls what the author wrote is answered.
//
// Validation order (R5 atomicity — every refusal before the first write): nil
// input → no actor, or not exactly one of target and prop → no option id → no
// roller → closed → THE ONE DOOR ([Encounter.replyDoor], the door
// [Encounter.ChooseOption] and [Encounter.Reserve] ask too): actor is a
// member → target is a member, not the actor and not down; a prop is shown
// to the actor → the actor is placed → the option is on the target and its
// requirement holds for the actor → its revision is current
// ([ErrStaleOption], rpg-project#924) → it is not pending on a reserved roll
// ([ErrReserved]) → a creature target can see the actor → reach; then the
// roll fits the line (a checked line's skill is one of its approaches; a
// plain line reports no roll) → the beat → the deed → the answer → the
// interaction the line advances (rpg-project#922) → the world's price for an
// action, paid only for a checked line (rpg-project#785 R5).
//
// THE SETTLE OF AN ABANDONED RESERVATION (rpg-project#938) asks neither the
// witness door nor reach: the words were spoken when the roll was made. Its
// beat, deed, answer, grants and advance are told and judged in the scene
// kept with the reservation ([ReservedScene]).
//
// THE BEAT IS APPENDED BEFORE ITS CONSEQUENCES, the law
// [Encounter.refreshSight] states: the line is the cause, and a stance beat
// ahead of the line that explains it would be a story told backwards. The
// ANSWERED beat comes after both, because it is the result (answer.go).
//
// Errors: ErrNilInput, ErrNoMember, ErrNoOption, ErrNoRoller, ErrClosed,
// ErrNotMember, ErrMemberDown, ErrBadPlacement, ErrNoProp, ErrStaleOption,
// ErrReserved, ErrUnwitnessed, ErrOutOfRange, and a refused Calculation.
//
// THE SETTLE (interaction_hold.go): a Speak of a reserved line, by its holder, to the
// same creature or prop, at the reserved revision, is the verdict that line
// waited for. It advances the interaction exactly once, then the reservation
// clears and the revision moves.
func (e *Encounter) Speak(ctx context.Context, in *SpeakInput) (*SpeakOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("speak: %w", ErrNilInput)
	}
	if in.Actor == "" {
		return nil, fmt.Errorf("speak: no actor: %w", ErrNoMember)
	}
	if (in.Target == "") == (in.Prop == "") {
		return nil, fmt.Errorf("speak: a line is said to exactly one creature or prop: %w", ErrNoMember)
	}
	if in.OptionID == "" {
		return nil, fmt.Errorf("speak: no line named: %w", ErrNoOption)
	}
	if in.Roller == nil {
		return nil, fmt.Errorf("speak: the world rolls the answer: %w", ErrNoRoller)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("speak: %w", ErrClosed)
	}
	// SPEAKING SHOWS YOU (sneaking R7, lead ruling on rpg-toolkit#2078): the
	// mode ends BEFORE the audience is asked, so a line said to a watcher the
	// speaker beat is heard and witnessed rather than refused unwitnessed. A
	// refusal below puts the mode back: a refused verb changes nothing (R5).
	wasSneaking, sneakVerdicts := e.Sneaking(in.Actor), copySneakVerdicts(e.sneakVerdicts)
	e.endSneaking(in.Actor)
	refuse := func(err error) (*SpeakOutput, error) {
		if wasSneaking {
			e.members[in.Actor].Sneaking = true
			e.sneakVerdicts = sneakVerdicts
		}
		return nil, err
	}

	// THE ONE DOOR ([Encounter.replyDoor]), the same one ChooseOption and
	// Reserve ask, so the door the host asked before charging and this one
	// cannot disagree.
	admitted, err := e.replyDoor(verbSpeak, lineReply{
		actor: in.Actor, target: in.Target, prop: in.Prop,
		optionID: in.OptionID, revision: in.Revision, reach: in.Range,
	}, true)
	if err != nil {
		return refuse(err)
	}
	option, settle, where, witnesses := admitted.option, admitted.settle, admitted.where, admitted.witnesses
	// AN ABANDONED SETTLE IS JUDGED AS SPOKEN (rpg-project#938): its grants
	// read whom the creature spoken to saw then.
	spoken := settle != nil && settle.Abandoned && settle.Scene != nil
	if err := refuseRollMismatch(in, option); err != nil {
		return refuse(err)
	}

	// A PLAIN LINE LANDS: it rolls nothing, so there is nothing to miss.
	beaten := in.Beaten || !option.Checked()

	at := uint64(e.clock.ToData().HighWater)
	seq, err := e.appendSpokeBeat(in, option, beaten, at)
	if err != nil {
		return nil, err
	}

	// THE LINE IS PERCEIVED (rpg-project#933, R1–R4, R8): each member is
	// written the parts their senses delivered — who said it to those who
	// see the speaker, the words and the roll to those they reach — and the
	// voice lands on whoever heard it. A kept roll's line is perceived as it
	// was said ([ReservedScene]).
	line := utterance{
		speaker: in.Actor, to: in.Target, prop: in.Prop, words: option.Say, option: option.ID,
		loudness: loudnessWord(option.Loudness), at: at,
	}
	if option.Checked() {
		line.roll = &spokenRoll{skill: in.Skill, dc: in.DC, total: in.Total, beaten: beaten, calculation: in.Calculation}
	}
	if spoken {
		line.scene = settle.Scene
	}
	heardLine, err := e.utter(line)
	if err != nil {
		return nil, fmt.Errorf("speak: %w", err)
	}

	// THE DEED IS THE SKILL'S (R3), and only a creature can hold one about
	// being spoken to: a prop is persuaded of nothing. IT IS KNOWN BY WHOEVER
	// CAUGHT THE WORDS AND SAW WHO SAID THEM (rpg-project#933): an
	// intimidation that worked says it worked, so one who only saw the line
	// said holds no deed of it; and a deed names its doer, so one who caught
	// the words unseeing — who has the verdict already, in the check their
	// version carries — holds none either, the speaker staying unnamed.
	landed := ""
	if verb, lands := deedForSkill(in.Skill); lands && beaten && in.Target != "" {
		if err := e.landSpokenDeed(verb, in.Actor, in.Target, where, heardLine.namedCatchers(), at); err != nil {
			return nil, fmt.Errorf("speak: %w", err)
		}
		landed = verb
	}

	key, rows := AnswerFailure, option.Failure
	if beaten {
		key, rows = AnswerSuccess, option.Success
	}
	answered, err := e.answer(ctx, answerInput{
		creature: in.Target, prop: in.Prop, actor: in.Actor,
		option: option.ID, key: key, rows: rows,
		verb: verbSpeak, beaten: beaten,
		witnesses: witnesses, lineCatchers: caughtOrNone(heardLine.catchers), at: at, roller: in.Roller,
	})
	if err != nil {
		return nil, fmt.Errorf("speak: %w", err)
	}

	// THE ROW'S GRANTS (rpg-project#929, #932): the row answered may grant a
	// record, as an outcome does — a success row when the line lands, a
	// failure row when it is missed — after the answer that says it, before
	// the interaction moves, every time the row lands.
	if answered.pick != nil && len(option.Grants) > 0 {
		iid, _ := optionInteraction(option)
		if err := e.applyGrants(option, beaten, answered.pick.Entry, recognitionLine{
			interaction: iid, option: option.ID, speaker: in.Actor, spokenTo: in.Target, witnesses: witnesses, at: at,
			spoken: spoken, seen: admitted.seen, perceived: admitted.perceived, partyAt: admitted.partyAt, hearers: heardLine.catchers,
			sawTarget: sawTargetOf(line),
		}); err != nil {
			return nil, fmt.Errorf("speak: %w", err)
		}
	}

	// THE VERDICT ADVANCES THE ENTRANCE (rpg-project#922, #916 R3): after the
	// answer has landed and before anything else, stepped synchronously so
	// the very next offer reads the new state — a landed beat would wait for
	// the next raise and leave that offer stale.
	if err := e.advanceInteraction(option, beaten, recognitionLine{
		speaker: in.Actor, spokenTo: in.Target, witnesses: witnesses, at: at, reach: in.Range,
		spoken: spoken, seen: admitted.seen, perceived: admitted.perceived, partyAt: admitted.partyAt, hearers: heardLine.catchers,
		sawTarget: sawTargetOf(line),
	}); err != nil {
		return nil, fmt.Errorf("speak: %w", err)
	}
	// THE SETTLE CLEARS AFTER THE ADVANCE, so the verdict it waited for
	// moved the progress exactly once (interaction_hold.go).
	e.settle(settle, option)

	// A BEATEN LINE SPOKEN TO A PROP IS THAT PROP'S DEED (rpg-project#766 S4):
	// `clock: bell:<prop>` for whoever listens for it — the bell rung.
	if in.Prop != "" && beaten {
		e.landClock(bellPrefix+string(in.Prop), "", at)
	}

	// THE WORLD'S PRICE FOR AN ACTION — AND ONLY A CHECKED LINE IS ONE
	// (rpg-project#785 R5: "the check is the action… speaking with no check
	// would be a free action"). Paid after the answer has landed — the fact
	// it taught, the deed it lodged, the running it set up — and before
	// anything refreshes sight (design §5, worldtime.go). It is what makes a
	// cowed goblin keep running while the party talks: the threat that scared
	// it pays the round its own `time` table then spends. A plain line pays
	// nothing: the party can greet the captain without the world moving.
	//
	// Nothing at all for a member inside a fight, either way: there the turn
	// clock prices time, and the session owns the action slot.
	if option.Checked() {
		if err := e.spendWorldAction(in.Actor); err != nil {
			return nil, fmt.Errorf("speak: %w", err)
		}
	}

	return &SpeakOutput{
		Beaten: beaten, Witnesses: witnesses, Hearers: heardLine.catchers, Deed: landed, Seq: seq, Answered: answered.pick,
	}, nil
}

// advanceInteraction steps the interaction a line's [Option.Advance] names
// on the beat its verdict lands: Success for a beaten line, Failure for a
// missed one. A fired transition that names a state appends a
// [BeatInteraction] beat — the interaction, the state it left and entered,
// the transition's reason, the option and the verdict — to the line's
// witnesses. A beat the current state does not listen for, or a guard that
// does not apply, steps nothing and writes nothing: the interaction stays
// where it was.
//
// After every refusal of [Encounter.Speak]'s door, so a refused line never
// steps it. Its own errors are construction defects the door already
// refuses (an interaction the run lacks, a state its recipe lacks), surfaced
// rather than swallowed.
func (e *Encounter) advanceInteraction(option Option, beaten bool, line recognitionLine) error {
	witnesses, at := line.witnesses, line.at
	a := option.Advance
	if a == nil {
		return nil
	}
	beat := a.Failure
	if beaten {
		beat = a.Success
	}
	if beat == "" {
		return nil
	}
	inst, ok := e.interactions[a.Interaction]
	if !ok {
		return fmt.Errorf("option %q advances interaction %q, and the run has no such interaction", option.ID, a.Interaction)
	}
	blob := inst.blob
	blob.Bag = inst.blob.Bag.Clone()
	stepped, err := inst.program.Step(&blob, beat, at, nil, at)
	if err != nil {
		return fmt.Errorf("interaction %q: %w", a.Interaction, err)
	}
	inst.blob = blob
	if stepped.Fired {
		inst.revision++ // THE PROGRESS MOVED, so the conversation did (interaction_hold.go)
	}
	if !stepped.Fired || stepped.To == "" {
		return nil
	}
	payload, err := json.Marshal(map[string]interface{}{
		"beat":        BeatInteraction,
		"interaction": string(a.Interaction),
		"from":        stepped.From,
		"to":          stepped.To,
		"reason":      stepped.Reason,
		"option":      option.ID,
		"beaten":      beaten,
	})
	if err != nil {
		return fmt.Errorf("interaction %q: marshal beat: %w", a.Interaction, err)
	}
	// THE CONVERSATION MOVED BY A LINE IS TOLD TO WHOEVER CAUGHT IT
	// (rpg-project#933, R9): the beat names the line and its verdict, so a
	// member who only saw it said learns neither here; they see where the
	// conversation stands in the next menu they are offered.
	audience := witnesses
	if line.hearers != nil {
		audience = line.hearers
	}
	if _, err := e.appendBeat(&record.AppendInput{
		At: at, Audience: audience, Tags: map[string]string{"tag": BeatInteraction}, Payload: payload,
	}); err != nil {
		return fmt.Errorf("interaction %q: %w", a.Interaction, err)
	}

	// WHAT FOLLOWS ENTERING THE STATE (recognition.go, rpg-project#928):
	// after the interaction beat, so the story tells the move before its
	// consequence; only when the state is newly entered, so a terminal
	// state's consequence happens once.
	if stepped.To != stepped.From {
		line.interaction, line.state = a.Interaction, stepped.To
		if err := e.applyOutcome(inst, line); err != nil {
			return err
		}
	}

	return nil
}

// refuseRollMismatch is Speak's roll door. A checked line must name one of
// its approaches as the skill ([ErrNoOption] otherwise). A PLAIN LINE ROLLS
// NOTHING (R5), so a skill, a DC, a total or a calculation reported for one
// is a caller that rolled a check the line does not have — refused with
// [ErrNoOption] rather than written onto a beat as if it had happened.
// Beaten is not asked of a plain line: it lands either way.
func refuseRollMismatch(in *SpeakInput, option Option) error {
	if option.Checked() {
		if !slices.ContainsFunc(option.Check, func(a CheckApproach) bool { return a.Ability == in.Skill }) {
			return fmt.Errorf("speak: line %q offers no %q approach: %w", in.OptionID, in.Skill, ErrNoOption)
		}
		return nil
	}
	if in.Skill != "" || in.DC != 0 || in.Total != 0 || in.Calculation != nil {
		return fmt.Errorf("speak: line %q rolls no check, and a roll was reported for it: %w", in.OptionID, ErrNoOption)
	}

	return nil
}

// refusePropOutOfReach is THE PROP REACH MEASURE, one for Speak's door and
// for [Encounter.PropWithinReach]: [Encounter.refuseOutOfReachCell] against
// every cell the prop stands on, any one within reach admitting it, as
// [Encounter.Hold] measures a footprint. Nil cells — a prop the member
// carries — is always within reach.
func (e *Encounter) refusePropOutOfReach(
	verb string, from spatial.Position, prop PropID, cells []spatial.Position, reach int,
) error {
	if cells == nil {
		return nil
	}
	for _, cell := range cells {
		if e.refuseOutOfReachCell(verb, from, cell, reach, string(prop)) == nil {
			return nil
		}
	}

	return fmt.Errorf("%s: %s: %w", verb, prop, ErrOutOfRange)
}

// appendSpokeBeat writes what the table saw: who said what to whom, the skill,
// the DC, the total, and whether it landed. A plain line (R5) writes an empty
// skill, a zero DC and total, and `beaten: true`: it rolled nothing and landed.
//
// EVERY WITNESS IS THE AUDIENCE, not just the two parties — everyone in the
// set learns what happened. The numbers are written unconditionally, false
// beside a miss included, and `target` and `prop` are both written with the
// one not spoken to empty, for the reason a struck beat writes `critical:
// false`: absent must not become a third state for a reader downstream.
func (e *Encounter) appendSpokeBeat(
	in *SpeakInput, option Option, beaten bool, at uint64,
) (uint64, error) {
	body := map[string]interface{}{
		"beat":   BeatSpoke,
		"actor":  string(in.Actor),
		"target": string(in.Target),
		"prop":   string(in.Prop),
		"option": option.ID,
		"line":   option.Say,
		"skill":  in.Skill,
		"dc":     in.DC,
		"total":  in.Total,
		"beaten": beaten,
	}
	if in.Calculation != nil {
		// Refused here rather than written: a beat is what the table saw, and
		// arithmetic that cannot have happened as told is not something
		// anybody saw. Absent stays absent — the key is omitted, never a
		// zero-valued calculation, which would be a third state downstream.
		if err := validateRecordedTotal(in.Calculation, in.Total); err != nil {
			return 0, fmt.Errorf("speak: calculation: %w", err)
		}
		body["calculation"] = in.Calculation
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return 0, fmt.Errorf("speak: marshal beat: %w", err)
	}

	// THE VERB'S OWN RECORD, TO ITS SPEAKER ALONE (rpg-project#933, R9: the
	// lead's call that `spoke` keeps the check and the deed). Everyone else
	// is told the line, and its roll, only by their own [BeatSpeech]
	// version, so a member who sees the speaker but did not catch the words
	// learns neither from this beat.
	out, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: []MemberID{in.Actor},
		Tags:     map[string]string{"tag": verbSpeak},
		Payload:  payload,
	})
	if err != nil {
		return 0, fmt.Errorf("speak: %w", err)
	}

	return out.Seq, nil
}

// validateOptions refuses a list of options this composition could not run:
// a line with no id or two lines under one, a line that says nothing, a plain
// line with failure rows or an approach with nothing to beat, a requirement that
// is not a fact or a holding, and a row its verdict could not roll.
//
// AT THE DOOR, NOT AT THE ROLL — [validateTable]'s reason. A line that can
// never be spoken is a misconfiguration, and the moment to find out is when
// its holder enters, not when a player finally picks it. The authoring
// dialect refuses the same things in the author's own words; this is the
// composition refusing them for every other caller, a persisted blob somebody
// edited included.
//
// WHETHER A HELD PROP EXISTS is not asked here: it is a question about the
// field, asked by [field.validateOptionHolds] once the field is compiled.
func validateOptions(options []Option) error {
	seen := make(map[string]bool, len(options))
	for i, o := range options {
		// A blank id names no line, and so does one of only spaces: the
		// option pool (selection.Evaluate) refuses both, so both stop here.
		if strings.TrimSpace(o.ID) == "" {
			return fmt.Errorf("options[%d] has no id", i)
		}
		if seen[o.ID] {
			return fmt.Errorf("option %q is declared twice", o.ID)
		}
		seen[o.ID] = true
		if o.Say == "" {
			return fmt.Errorf("option %q does not say what the character says", o.ID)
		}
		if err := validateLoudness(fmt.Sprintf("option %q", o.ID), o.Loudness); err != nil {
			return err
		}
		// NO APPROACH IS A PLAIN LINE (rpg-project#785 R5), which cannot be
		// missed. The authoring dialect refuses a `check: []` written empty;
		// here an empty list and a nil one are the same line, as the clone
		// across this door already makes them.
		if !o.Checked() && len(o.Failure) != 0 {
			return fmt.Errorf("option %q rolls no check and cannot be missed: it answers no failure rows", o.ID)
		}
		for _, a := range o.Check {
			if a.Ability == "" {
				return fmt.Errorf("option %q has an approach that rolls nothing", o.ID)
			}
			if a.DC < 1 {
				return fmt.Errorf("option %q approach %q at DC %d has nothing to beat", o.ID, a.Ability, a.DC)
			}
		}
		for n, requires := range o.Requires {
			if err := validateOptionRequirement(fmt.Sprintf("option %q requires[%d]", o.ID, n), requires); err != nil {
				return err
			}
		}
		if err := validateGrants(o); err != nil {
			return err
		}
		// ONE CONVERSATION PER LINE (rpg-project#924, interaction_hold.go): a line is
		// offered with one interaction's revision, so it may touch one.
		if touched := optionInteractions(o); len(touched) > 1 {
			return fmt.Errorf("option %q reads or advances %d interactions %q; a line touches at most one", o.ID, len(touched), touched)
		}
		if a := o.Advance; a != nil {
			if a.Interaction == "" {
				return fmt.Errorf("option %q advances no interaction", o.ID)
			}
			if a.Success == "" && a.Failure == "" {
				return fmt.Errorf("option %q advances interaction %q on neither verdict", o.ID, a.Interaction)
			}
			if !o.Checked() && a.Failure != "" {
				return fmt.Errorf("option %q rolls no check and cannot be missed: it advances nothing on a failure", o.ID)
			}
		}
		for _, verdict := range []struct {
			key  AnswerKey
			rows []Answer
		}{{AnswerSuccess, o.Success}, {AnswerFailure, o.Failure}} {
			for j, row := range verdict.rows {
				if err := validateEntry(verdict.key, j, row); err != nil {
					return fmt.Errorf("option %q: %w", o.ID, err)
				}
			}
		}
	}

	return nil
}

// validateOptionRequirement refuses one item of an option's `requires` this
// composition cannot read: nothing at all, a fact or a holding that names
// nothing, a `not` that cannot be one, and any trigger that is not a
// requirement. what names the item, `option "x" requires[n]`.
func validateOptionRequirement(what string, requires Trigger) error {
	if n, ok := requires.(TriggerNot); ok {
		inner, err := validateReadNot(what, n)
		if err != nil {
			return err
		}
		requires = inner
	}
	switch t := requires.(type) {
	case nil:
		return fmt.Errorf("%s names nothing: a requirement is a fact or a holding, or not one", what)
	case TriggerFact:
		if t.Fact == "" {
			return fmt.Errorf("%s is a fact and names none", what)
		}
	case TriggerHolds:
		if t.Prop == "" {
			return fmt.Errorf("%s is holding a prop and names none", what)
		}
	case TriggerInteraction:
		if t.Interaction == "" {
			return fmt.Errorf("%s is an interaction's state and names no interaction", what)
		}
		if len(t.In) == 0 {
			return fmt.Errorf("%s is interaction %q in no state, which nothing can meet", what, t.Interaction)
		}
	case TriggerRecognized:
		if t.Topic == "" {
			return fmt.Errorf("%s is recognition and names no topic", what)
		}
	default:
		return fmt.Errorf("%s is a %T: an option requires a fact, a holding, an interaction's state or recognition, "+
			"or not one", what, requires)
	}

	return nil
}

// validatePropOptions is [validateOptions] for a prop's lines, plus the two
// things only a prop can get wrong: it must be nameable, since a line is
// spoken to it by id, and none of its rows may `flee` — a prop cannot run.
// Refused with [ErrNoField], the door a prop comes in through.
func validatePropOptions(id PropID, ref string, options []Option) error {
	if len(options) == 0 {
		return nil
	}
	if id == "" {
		return fmt.Errorf("prop %q carries options and has no id: %w", ref, ErrNoField)
	}
	if err := validateOptions(options); err != nil {
		return fmt.Errorf("prop %q: %w: %w", id, err, ErrNoField)
	}
	for _, o := range options {
		// A PROP RECOGNISES NOBODY (rpg-project#929): the holder of a grant is
		// the one spoken to, and a prop holds nothing.
		if len(o.Grants) > 0 {
			return fmt.Errorf("prop %q option %q grants a record, and a prop holds nothing about anybody: %w",
				id, o.ID, ErrNoField)
		}
		for _, requires := range o.Requires {
			if inner, _ := unwrapNot(requires); inner != nil {
				if _, ok := inner.(TriggerRecognized); ok {
					return fmt.Errorf("prop %q option %q: %w: %w", id, o.ID, errRecognizedOnProp, ErrNoField)
				}
			}
		}
		for _, row := range append(append([]Answer(nil), o.Success...), o.Failure...) {
			if row.Flee {
				return fmt.Errorf("prop %q option %q: a prop cannot run: %w", id, o.ID, ErrNoField)
			}
		}
	}

	return nil
}

// validateOptionHolds refuses an option requiring a holding of a prop this
// field does not declare holdable — a requirement nobody can ever meet, which
// is an option nobody can ever be offered. Asked once the field is compiled,
// because only then is the holdable index whole.
func (f *field) validateOptionHolds(options []Option) error {
	for _, o := range options {
		for _, requires := range o.Requires {
			inner, _ := unwrapNot(requires)
			t, ok := inner.(TriggerHolds)
			if !ok {
				continue
			}
			if f.propIndexOf(t.Prop) < 0 && f.placedIndexOf(t.Prop) < 0 {
				return fmt.Errorf("option %q requires holding %q, and no prop has that id", o.ID, t.Prop)
			}
			if !f.holdable[t.Prop] {
				return fmt.Errorf("option %q requires holding %q, which nobody can pick up", o.ID, t.Prop)
			}
		}
	}

	return nil
}

// cloneOptions deep-copies a list of options so a caller's slice and the
// composition's cannot be the same one — [MemberInput]'s standing rule that
// nothing crossing this door stays aliased. Nil stays nil.
func cloneOptions(options []Option) []Option {
	if options == nil {
		return nil
	}
	out := make([]Option, 0, len(options))
	for _, o := range options {
		out = append(out, cloneOption(o))
	}

	return out
}

// cloneOption deep-copies one option: its approaches, its requirements and
// both verdicts' rows.
func cloneOption(o Option) Option {
	out := o
	out.Check = copyApproaches(o.Check)
	out.Requires = cloneRequirements(o.Requires)
	if o.Advance != nil {
		advance := *o.Advance
		out.Advance = &advance
	}
	out.Success = cloneRows(o.Success)
	out.Failure = cloneRows(o.Failure)
	out.Grants = cloneGrants(o.Grants)

	return out
}

// cloneRequirements deep-copies an option's requirements. Nil stays nil.
func cloneRequirements(requires []Trigger) []Trigger {
	if requires == nil {
		return nil
	}
	out := make([]Trigger, 0, len(requires))
	for _, t := range requires {
		out = append(out, cloneRequirement(t))
	}

	return out
}

// cloneRequirement deep-copies one requirement; every kind but a `not` and
// an interaction's states is a value already.
func cloneRequirement(t Trigger) Trigger {
	switch r := t.(type) {
	case TriggerNot:
		return TriggerNot{Inner: cloneRequirement(r.Inner)}
	case TriggerInteraction:
		return TriggerInteraction{Interaction: r.Interaction, In: slices.Clone(r.In)}
	}

	return t
}

// cloneRows deep-copies one verdict's rows. Nil stays nil.
func cloneRows(rows []Answer) []Answer {
	if rows == nil {
		return nil
	}
	out := make([]Answer, 0, len(rows))
	for _, row := range rows {
		out = append(out, cloneAnswer(row))
	}

	return out
}

// optionFacts is every fact id a list of options can teach — what joins
// [mintedFactIDs] so a world blob may name a fact only a line plants.
func optionFacts(options []Option) []FactID {
	var out []FactID
	for _, o := range options {
		for _, row := range wordRows(append(append([]Answer(nil), o.Success...), o.Failure...)) {
			if row.Fact != "" {
				out = append(out, row.Fact)
			}
		}
	}

	return out
}

// caughtOrNone is a line's catchers as [answerInput.lineCatchers] takes
// them: never nil, so an answer to a line always splits its pick by who
// caught the line, even when only the speaker did.
func caughtOrNone(catchers []MemberID) []MemberID {
	if catchers == nil {
		return []MemberID{}
	}
	return catchers
}
