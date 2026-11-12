// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// recognition.go is ADMISSION IS RECOGNITION (rpg-project#928, the gate's
// first consequence; design #916 R6 as ruled on #925 on 2026-11-11).
//
// # A relationship record, holder → subject on a topic
//
// Admission is neither a fact in the walker's head nor a door opened for
// everyone. It is a guard RECOGNISING people: a holding in the run's intel
// store whose observer is the guard, whose subject is the person, and whose
// TOPIC ([TopicID]) is authored content naming what he holds about them
// (`admitted`). Sight lives on the default topic and is never touched by a
// named one: perception reads the default topic only, so no sight reader can
// mistake a recognition for a sighting (mind/perception rule 12).
//
// # One write, authored once on the interaction
//
// What follows an interaction ENTERING a state is authored on the
// interaction, keyed by that state ([InteractionInput.Outcomes]), never
// copied onto each guard's lines. So every line that leads to `admitted`, on
// any guard, writes the same thing. It hangs BESIDE the recipe, keyed by the
// entered state, rather than as a machine act: `mind/machine`'s acts are its
// own closed vocabulary (pick, wake_in, …), and teaching it what a guard
// recognises would put this module's nouns into a leaf that knows none of
// them. A table keyed by entered state also fires the same consequence from
// every transition into the state, whichever beat drove it — "one write,
// usable from any outcome". A terminal state is entered once, so its
// consequence happens once.
//
// The one consequence kind is [Recognition]: holders come to hold a topic
// about subjects, through a channel. Its words are fixed and few — exactly
// what the gate uses — and every other word is refused at the door:
//
//   - the holder is the member the line was SAID TO ([RecognitionSpokenTo]),
//     who judged it ([ChannelJudgement]);
//   - the subjects are the speaker's party HE SEES at that moment
//     ([RecognitionPartyInSight]): the speaker's faction, read from his own
//     sight testimony, the reading his testimony and every hold refusal use.
//     A member out of his sight, or one sneaking past his notice, is not
//     recognised; a member who was not there is not admitted;
//   - optionally, the interaction's OTHER carriers who witnessed the answer
//     hold the same topic about the same subjects, heard
//     ([RecognitionCarriersWhoWitnessed], [ChannelHeard]). A carrier who did
//     not witness it holds nothing: sharing beyond that is a means not yet
//     built (rpg-project#928, "needs seen").
//
// # One write, a second place to hang it (rpg-project#929)
//
// The same [Consequence] may hang on a reply line's SUCCESS row
// ([Option.Grants], [RowGrant]): a recognised member coming back to vouch,
// a straggler making their own case. It is written by the same path, by the
// same live sight, inside the same Speak, and EVERY TIME the row lands —
// a record already held is confirmed, never written new, so when the holder
// first recognised someone survives. A checked line's FAILURE row may carry
// one too (rpg-project#932), landing only when the line is missed: a guard
// who turns a straggler away holds that he did. A second
// subject word, [RecognitionSpeaker], recognises the speaker alone. On a
// line, `heard_by` means the other carriers of the interaction the line
// reads or advances; a line that names none has nobody to hear it, and
// `heard_by` is refused there.
//
// # A routine wakes on a topic (rpg-project#929)
//
// A holder who comes to hold a topic for the first time hears the `topic`
// beat ([beatTopic]), landed as a learned fact's `fact` beat is and answered
// at the next raise; a guard reads which topic as `topic:<id>`
// ([MachineTopic]), true while the member holds it about anybody, from his
// OWN holdings. A state not listening does not hear it; a later state that
// listens reads the guard on the next topic beat, as `fact` works.
//
// # One question, asked the same way everywhere
//
// [TriggerRecognized] asks "does the OTHER PARTY hold this topic about me".
// On a reply line's `requires`, the other party is the creature spoken to and
// "me" the speaker; on a post's pass ([Post.Pass]), the other party is the
// post's holder and "me" the walker. One evaluator ([Encounter.recognizes])
// answers both.

// TopicID names what a holder holds about a subject beyond where they are —
// the topic of an intel record (holder, subject, topic). Authored content:
// the module carries it and compares it, and never reads meaning into it.
type TopicID string

// TriggerRecognized is the requirement "the other party holds this topic
// about me" (`{ recognized: <topic> }`, rpg-project#928).
//
// On an option's `requires` the other party is the creature the line is
// said to and "me" is the speaker; a line said to a prop has no other party
// who could hold anything, and the door refuses the requirement there. On a
// post's pass the other party is the post's holder and "me" is the walker.
//
// READ, NEVER EDGE-FIRED, as [TriggerHolds] is: an ending, an arrival and an
// until refuse it, because nothing notices a holding change in order to fire
// one.
type TriggerRecognized struct {
	// Topic is the topic the other party must hold about "me". REQUIRED
	// non-empty, and one some interaction's outcome grants.
	Topic TopicID
}

// isTrigger marks TriggerRecognized as a Trigger.
func (t TriggerRecognized) isTrigger() {}

// Consequence is one thing that follows an interaction entering a state
// ([InteractionInput.Outcomes]). Exactly one kind is set; [Recognition] is
// the first and only kind.
type Consequence struct {
	// Recognize is: holders come to hold a topic about subjects. REQUIRED
	// (the only kind).
	Recognize *Recognition `json:"recognize,omitzero"`
}

// RecognitionHolder names who comes to hold the topic.
type RecognitionHolder string

// RecognitionSubjects names whom the topic is held about.
type RecognitionSubjects string

// RecognitionHearers names who else holds the topic, having heard it.
type RecognitionHearers string

const (
	// RecognitionSpokenTo is the member the line that moved the interaction
	// was said to. A line said to a prop has none, and grants nothing.
	RecognitionSpokenTo RecognitionHolder = "spoken_to"

	// RecognitionPartyInSight is the speaker's party — the members of the
	// speaker's faction, the speaker included — whom the holder sees on
	// sight now, from his own testimony.
	RecognitionPartyInSight RecognitionSubjects = "party_in_sight"

	// RecognitionSpeaker is the speaker alone, when the holder sees them
	// (rpg-project#929): a straggler making their own case is recognised
	// themselves, never whoever stands beside them.
	RecognitionSpeaker RecognitionSubjects = "speaker"

	// RecognitionCarriersWhoWitnessed is every OTHER carrier of the
	// interaction (a member whose lines name it) who witnessed the line and
	// is up: each holds the same topic about the same subjects, heard.
	RecognitionCarriersWhoWitnessed RecognitionHearers = "carriers_who_witnessed"
)

// Recognition is the consequence "holders come to hold a topic about
// subjects, through a channel" (rpg-project#928). Every word is one of the
// fixed constants; any other is refused when the interaction is checked.
type Recognition struct {
	// Topic is the topic held. REQUIRED non-empty; authored content.
	Topic TopicID `json:"topic"`

	// Holder is who holds it, through [ChannelJudgement]. REQUIRED:
	// [RecognitionSpokenTo].
	Holder RecognitionHolder `json:"holder"`

	// About is whom it is held about. REQUIRED: [RecognitionPartyInSight].
	About RecognitionSubjects `json:"about"`

	// HeardBy is who else holds it, through [ChannelHeard]. Empty: nobody.
	// Otherwise [RecognitionCarriersWhoWitnessed].
	HeardBy RecognitionHearers `json:"heard_by,omitzero"`
}

// ChannelJudgement is the intel channel a holder comes to hold a topic
// through when it is his own call: the guard who heard the claim and let
// the party in recognised them himself.
const ChannelJudgement = "judgement"

// ChannelHeard is the intel channel a carrier comes to hold a topic through
// when he witnessed another's judgement: he heard the guard let them in.
const ChannelHeard = "heard"

// BeatRecognized is the "beat" value of the story beat appended when a
// holder comes to hold a topic about subjects: the holder, the subjects, the
// topic, the channel and the interaction whose outcome it was. Audienced to
// the line's witnesses, as [BeatInteraction] is. EXPORTED BECAUSE A DECODER
// READS IT.
const BeatRecognized = "recognized"

// validateOutcomes refuses an interaction's outcomes nobody could author on
// purpose: a state its recipe does not declare, a state with no consequence,
// a consequence of no kind, and any word of a [Recognition] that is not one
// of its constants.
func validateOutcomes(id InteractionID, states map[string]bool, outcomes map[string][]Consequence) error {
	for _, state := range slices.Sorted(maps.Keys(outcomes)) {
		if !states[state] {
			return fmt.Errorf("interaction %q outcome %q is not a state of its recipe", id, state)
		}
		if len(outcomes[state]) == 0 {
			return fmt.Errorf("interaction %q outcome %q names no consequence", id, state)
		}
		for i, c := range outcomes[state] {
			what := fmt.Sprintf("interaction %q outcome %q consequence %d", id, state, i)
			if err := validateConsequence(what, c, true); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateConsequence refuses one consequence nobody could author on
// purpose: no kind, no topic, and any word that is not one of its constants.
// carriers says whether the place it hangs names an interaction whose other
// carriers could hear it; where none is named, `heard_by` is refused, since
// there would be nobody to hear.
func validateConsequence(what string, c Consequence, carriers bool) error {
	r := c.Recognize
	switch {
	case r == nil:
		return fmt.Errorf("%s is of no kind: the one kind is recognize", what)
	case r.Topic == "":
		return fmt.Errorf("%s recognizes on no topic", what)
	case r.Holder != RecognitionSpokenTo:
		return fmt.Errorf("%s holder %q: the holder is %q", what, r.Holder, RecognitionSpokenTo)
	case r.About != RecognitionPartyInSight && r.About != RecognitionSpeaker:
		return fmt.Errorf("%s about %q: the subjects are %q or %q", what, r.About, RecognitionPartyInSight, RecognitionSpeaker)
	case r.HeardBy != "" && r.HeardBy != RecognitionCarriersWhoWitnessed:
		return fmt.Errorf("%s heard_by %q: the hearers are %q, or none", what, r.HeardBy, RecognitionCarriersWhoWitnessed)
	case r.HeardBy != "" && !carriers:
		return fmt.Errorf("%s heard_by %q: the line names no interaction, so it has no other carriers to hear it",
			what, r.HeardBy)
	}
	return nil
}

// cloneOutcomes copies an interaction's outcomes so no caller's value is
// aliased. Nil stays nil.
func cloneOutcomes(outcomes map[string][]Consequence) map[string][]Consequence {
	if outcomes == nil {
		return nil
	}
	out := make(map[string][]Consequence, len(outcomes))
	for state, cs := range outcomes {
		copied := make([]Consequence, 0, len(cs))
		for _, c := range cs {
			if c.Recognize != nil {
				r := *c.Recognize
				c.Recognize = &r
			}
			copied = append(copied, c)
		}
		out[state] = copied
	}
	return out
}

// grantedTopics is every topic some interaction's outcome grants, or some
// line's answer among the lists given grants (rpg-project#929): the topics a
// [TriggerRecognized] may name.
func grantedTopics(interactions map[InteractionID]*interactionRecord, lines ...[]Option) map[TopicID]bool {
	out := map[TopicID]bool{}
	for _, r := range interactions {
		for _, cs := range r.input.Outcomes {
			for _, c := range cs {
				if c.Recognize != nil {
					out[c.Recognize.Topic] = true
				}
			}
		}
	}
	for _, options := range lines {
		for _, o := range options {
			for _, g := range o.Grants {
				if g.Consequence.Recognize != nil {
					out[g.Consequence.Recognize.Topic] = true
				}
			}
		}
	}
	return out
}

// RowGrant is one consequence a line's row lands when that row is the one
// answered (rpg-project#929): the same [Consequence] an interaction's
// outcome uses, hung on a reply instead of a state. Every time the row lands
// it grants again, for whoever is in sight then; a record that is already
// held is confirmed, never written new.
//
// A FAILURE ROW MAY GRANT TOO (rpg-project#932): a guard who turns a
// straggler away comes to hold that he did, so the claim is not offered to
// the same speaker twice. It lands only when a checked line is missed — a
// plain line has no failure rows to hang one on.
type RowGrant struct {
	// Row is the index of the row whose landing grants: in [Option.Success],
	// or in [Option.Failure] when Failure is set.
	Row int `json:"row"`

	// Failure is true when Row indexes the failure rows (rpg-project#932):
	// the grant lands when the line is missed. False, the zero value, is a
	// success row's, as every grant before this field was.
	Failure bool `json:"failure,omitempty"`

	// Consequence is what it grants.
	Consequence Consequence `json:"consequence"`
}

// validateGrants refuses a line's grants nobody could author on purpose: a
// row the line does not have, and a consequence [validateConsequence]
// refuses. `heard_by` needs the line to name an interaction ([optionInteraction]).
func validateGrants(o Option) error {
	_, named := optionInteraction(o)
	for i, g := range o.Grants {
		what := fmt.Sprintf("option %q grant %d", o.ID, i)
		verdict, rows := AnswerSuccess, o.Success
		if g.Failure {
			verdict, rows = AnswerFailure, o.Failure
		}
		if g.Row < 0 || g.Row >= len(rows) {
			return fmt.Errorf("%s names %s row %d, and the line has %d", what, verdict, g.Row, len(rows))
		}
		if err := validateConsequence(what, g.Consequence, named); err != nil {
			return err
		}
	}
	return nil
}

// cloneGrants copies a line's grants so no caller's value is aliased.
func cloneGrants(grants []RowGrant) []RowGrant {
	if grants == nil {
		return nil
	}
	out := make([]RowGrant, 0, len(grants))
	for _, g := range grants {
		if g.Consequence.Recognize != nil {
			r := *g.Consequence.Recognize
			g.Consequence.Recognize = &r
		}
		out = append(out, g)
	}
	return out
}

// applyGrants runs every grant of the row that answered the line — a
// success row when it landed, a failure row when it was missed
// (rpg-project#929, #932): the same write an outcome makes, by the same live
// sight, inside the same Speak.
func (e *Encounter) applyGrants(o Option, beaten bool, row int, line recognitionLine) error {
	for _, g := range o.Grants {
		if g.Failure == beaten || g.Row != row || g.Consequence.Recognize == nil {
			continue
		}
		if err := e.grantRecognition(*g.Consequence.Recognize, line); err != nil {
			return fmt.Errorf("option %q grant: %w", o.ID, err)
		}
	}
	return nil
}

// recognizes reports whether holder holds topic about subject: the one
// read [TriggerRecognized] makes, on a line and on a post alike. Read from
// the intel store under the holder's sight identity, so it is the record
// [Encounter.grantRecognition] wrote. Any error but "not held" fails closed:
// a door is not opened, nor a line offered, on a read that failed.
func (e *Encounter) recognizes(holder, subject MemberID, topic TopicID) bool {
	if holder == "" || subject == "" || topic == "" {
		return false
	}
	_, err := e.intelLog.AboutOn(sightMember(holder), sightMember(subject), perception.Topic(topic))
	return err == nil
}

// recognitionLine is what [Encounter.applyOutcome] needs of the line that
// moved the interaction.
type recognitionLine struct {
	interaction InteractionID
	state       string
	// option is the line whose answer granted, empty for an outcome.
	option    string
	speaker   MemberID
	spokenTo  MemberID
	witnesses []MemberID
	at        uint64
	// spoken is true for an abandoned settle (rpg-project#938): the line is
	// judged in the scene it was spoken in, so whom the creature spoken to
	// sees is seen, not the live reach.
	spoken bool
	seen   []MemberID
	// hearers is who caught the words of the line that granted, when a line
	// granted (rpg-project#933, R9): the carriers who hear a recognition are
	// the word-catchers, not those who only saw it said. Nil for an outcome
	// no line moved, whose carriers are the witnesses.
	hearers []MemberID
	// perceived is, for a line judged as spoken, which members of the
	// speaker's party each witness saw when the words were said
	// ([ReservedScene.Perceived]): what each reader of its beats may be told.
	perceived map[MemberID][]MemberID
}

// perceives is whether reader perceives subject for this line's beats: on
// the live reach, or, for a line judged as spoken, as they did when the
// words were said ([ReservedScene.Perceived]).
func (line recognitionLine) perceives(reach sightReach, reader, subject MemberID) bool {
	if line.spoken {
		return slices.Contains(line.perceived[reader], subject)
	}
	return reach.Reaches(perception.Sight, reader, subject)
}

// sees is whether holder sees subject for this line: on the live reach, or,
// for a line judged as spoken ([recognitionLine.spoken]), as the creature
// spoken to saw when the words were said.
func (line recognitionLine) sees(reach sightReach, holder, subject MemberID) bool {
	if line.spoken {
		return holder == line.spokenTo && slices.Contains(line.seen, subject)
	}
	return reach.Reaches(perception.Sight, holder, subject)
}

// applyOutcome runs every consequence authored for the state the line's
// interaction entered, in order: the write [Encounter.advanceInteraction]
// hands on when a fired transition entered a new state. Inside the same
// Speak as the step, so the next offer and the next crossing read it.
func (e *Encounter) applyOutcome(r *interactionRecord, line recognitionLine) error {
	for _, c := range r.input.Outcomes[line.state] {
		if c.Recognize == nil {
			continue
		}
		if err := e.grantRecognition(*c.Recognize, line); err != nil {
			return fmt.Errorf("interaction %q outcome %q: %w", line.interaction, line.state, err)
		}
	}
	return nil
}

// grantRecognition is one [Recognition]: the member spoken to comes to hold
// the topic about every member of the speaker's party he sees now, judged;
// and, when authored, every other carrier who witnessed the line and is up
// holds the same, heard. A line said to a prop has no holder and grants
// nothing; a holder who sees nobody of the party grants nothing.
//
// "SEES NOW" IS THE LIVE ANSWER the line itself was judged by: one
// [Encounter.placedSightReach], read after Speak ended the speaker's
// sneaking, with every sneak verdict in force — the same reach
// [Encounter.audienceOf] asked to decide who witnessed the line. Never the
// stored testimony of the last sight pass, which can still hold a speaker
// hidden who has just shown themselves, or a member seen who has since
// hidden. Only current sight qualifies; a memory never does.
//
// Each holder's record is written under his sight identity
// ([sightMember]), the observer key the store's sightings use and the load
// check demands, and is told as [BeatRecognized] beats ([Encounter.tellRecognition]).
func (e *Encounter) grantRecognition(r Recognition, line recognitionLine) error {
	holder := line.spokenTo
	if holder == "" {
		return nil
	}
	reach, err := e.placedSightReach()
	if err != nil {
		return err
	}
	subjects := e.partyInSight(reach, holder, line)
	if r.About == RecognitionSpeaker {
		subjects = nil
		if line.speaker != holder && line.sees(reach, holder, line.speaker) {
			subjects = []MemberID{line.speaker}
		}
	}
	if len(subjects) == 0 {
		return nil
	}
	if err := e.holdTopic(reach, holder, subjects, r.Topic, ChannelJudgement, line); err != nil {
		return err
	}
	if r.HeardBy != RecognitionCarriersWhoWitnessed {
		return nil
	}
	down, err := e.downNow()
	if err != nil {
		return err
	}
	// A LINE TEACHES WHOEVER CATCHES ITS WORDS (rpg-project#933, R9): the
	// carriers who hear it are those the line's words reached, not those who
	// only saw it said.
	carriers := line.witnesses
	if line.hearers != nil {
		carriers = line.hearers
	}
	for _, carrier := range carriers {
		if carrier == holder || carrier == line.speaker || down[carrier] {
			continue
		}
		m, ok := e.members[carrier]
		if !ok || !carries(m.Options, line.interaction) {
			continue
		}
		if err := e.holdTopic(reach, carrier, subjects, r.Topic, ChannelHeard, line); err != nil {
			return err
		}
	}
	return nil
}

// partyInSight is the members of the speaker's faction — the speaker
// included — whom holder sees on the live reach given, or as the line was
// spoken ([recognitionLine.sees]), in id order. A speaker in no faction is a
// party of one.
func (e *Encounter) partyInSight(reach sightReach, holder MemberID, line recognitionLine) []MemberID {
	var out []MemberID
	for _, id := range e.partyOf(line.speaker) {
		if id != holder && line.sees(reach, holder, id) {
			out = append(out, id)
		}
	}
	return out
}

// partyOf is the members of the speaker's faction, the speaker included, in
// id order: a speaker in no faction is a party of one, and a speaker not in
// the run has none.
func (e *Encounter) partyOf(speaker MemberID) []MemberID {
	s, ok := e.members[speaker]
	if !ok {
		return nil
	}
	side := factionOf(s)
	var out []MemberID
	for _, id := range slices.Sorted(maps.Keys(e.members)) {
		if id == speaker || (side != "" && factionOf(e.members[id]) == side) {
			out = append(out, id)
		}
	}
	return out
}

// recognitionTestimony is the payload a recognition holds: where it came
// from. Nothing reads it to decide anything — the holding's presence is the
// answer — but a record of why is kept with it.
type recognitionTestimony struct {
	Interaction InteractionID `json:"interaction,omitempty"`
	State       string        `json:"state,omitempty"`
	Option      string        `json:"option,omitempty"`
}

// holdTopic lands one holder's record on topic about every subject, through
// channel, and tells it ([Encounter.tellRecognition]).
func (e *Encounter) holdTopic(
	reach sightReach, holder MemberID, subjects []MemberID, topic TopicID, channel string, line recognitionLine,
) error {
	payload, err := json.Marshal(recognitionTestimony{Interaction: line.interaction, State: line.state, Option: line.option})
	if err != nil {
		return fmt.Errorf("recognition payload: %w", err)
	}
	// A RECORD ALREADY HELD IS KEPT (rpg-project#929): whatever source grants
	// it again — the admission, then a vouch — the holder keeps the record he
	// has, its first payload with it, so only a confirmation lands and when
	// he first recognised the subject survives. The beat still says which
	// line or outcome granted this time.
	reports := make([]perception.Presence, 0, len(subjects))
	for _, s := range subjects {
		report := perception.Presence{ID: sightMember(s), Payload: payload}
		if held, err := e.intelLog.AboutOn(sightMember(holder), sightMember(s), perception.Topic(topic)); err == nil {
			report.Payload = held.Payload
		}
		reports = append(reports, report)
	}
	reported, err := e.intelLog.Report(perception.ReportInput{
		Observer: sightMember(holder), Channel: perception.Channel(channel), Topic: perception.Topic(topic),
		Reports: reports, At: line.at,
	})
	if err != nil {
		return fmt.Errorf("%s holds %q: %w", holder, topic, err)
	}
	// A HOLDER WHO COMES TO HOLD A TOPIC MAY WAKE ON IT (rpg-project#929):
	// only a first holding is news; a record confirmed again is not.
	if len(reported.FirstContact) > 0 {
		e.landTopicBeat(holder, line.at)
	}
	return e.tellRecognition(reach, holder, subjects, topic, channel, line)
}

// tellRecognition appends the [BeatRecognized] beats for one holder's
// record. The line's witnesses hear of it, each told ONLY the subjects they
// perceive themselves on the same live reach — or, for a line judged as
// spoken (rpg-project#938), as they perceived them when the words were said
// ([recognitionLine.perceives]) — a member is always aware of themselves —
// so a beat never names a member its reader cannot see; the
// holder is told every subject he recognised. A witness who perceives none of
// them is told nothing. Readers told the same subjects share one beat, in
// the order they were first met; each beat goes to exactly its readers, the
// per-audience shape the sighted beat already has ([Encounter.appendSightedBeats]).
func (e *Encounter) tellRecognition(
	reach sightReach, holder MemberID, subjects []MemberID, topic TopicID, channel string, line recognitionLine,
) error {
	type telling struct {
		subjects []string
		audience []MemberID
	}
	var tellings []*telling
	byKey := map[string]*telling{}
	readers := append([]MemberID{holder}, line.witnesses...)
	for _, reader := range readers {
		if _, dup := byKey["reader:"+string(reader)]; dup {
			continue
		}
		byKey["reader:"+string(reader)] = nil
		var told []string
		for _, s := range subjects {
			if reader == holder || reader == s || line.perceives(reach, reader, s) {
				told = append(told, string(s))
			}
		}
		if len(told) == 0 {
			continue
		}
		key := "subjects:" + strings.Join(told, "\x00")
		t, ok := byKey[key]
		if !ok {
			t = &telling{subjects: told}
			byKey[key] = t
			tellings = append(tellings, t)
		}
		t.audience = append(t.audience, reader)
	}
	for _, t := range tellings {
		payload := map[string]interface{}{
			"beat":     BeatRecognized,
			"holder":   string(holder),
			"subjects": t.subjects,
			"topic":    string(topic),
			"channel":  channel,
		}
		// What granted it: an interaction's outcome names the interaction; a
		// line's answer names the line, and its interaction when it has one.
		if line.interaction != "" {
			payload["interaction"] = string(line.interaction)
		}
		if line.option != "" {
			payload["option"] = line.option
		}
		beat, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("recognized beat: %w", err)
		}
		if _, err := e.appendBeat(&record.AppendInput{
			At: line.at, Audience: t.audience, Tags: map[string]string{"tag": BeatRecognized}, Payload: beat,
		}); err != nil {
			return fmt.Errorf("recognized beat: %w", err)
		}
	}
	return nil
}

// errRecognizedOnProp is the refusal of a `recognized` requirement on a
// prop's line: a prop holds nothing about anybody.
var errRecognizedOnProp = errors.New("a prop holds nothing about anybody, so a line said to one cannot require recognition")

// validateRecognizedTopics refuses a `recognized` requirement naming a topic
// no interaction of the run grants — a requirement nobody can ever meet —
// on any line or post pass of the members given.
func validateRecognizedTopics(granted map[TopicID]bool, what string, requires []Trigger) error {
	for _, t := range requires {
		inner, _ := unwrapNot(t)
		r, ok := inner.(TriggerRecognized)
		if !ok {
			continue
		}
		if !granted[r.Topic] {
			return fmt.Errorf("%s requires recognition on topic %q, and no interaction's outcome grants it", what, r.Topic)
		}
	}
	return nil
}
