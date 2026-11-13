// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

const castlePath = "../testdata/world-builder-v4-castle.yaml"

// CastleSuite reads the castle back once and holds it to the file: every
// authored thing said or listed, every path real, and the gate said as ruled.
type CastleSuite struct {
	suite.Suite
	raw []byte
	doc *yaml.Node
	rb  *readback.ReadBack
}

func TestCastleSuite(t *testing.T) { suite.Run(t, new(CastleSuite)) }

func (s *CastleSuite) SetupSuite() {
	raw, err := os.ReadFile(castlePath)
	s.Require().NoError(err)
	s.raw = raw
	var n yaml.Node
	s.Require().NoError(yaml.Unmarshal(raw, &n))
	s.doc = n.Content[0]
	s.rb, err = readback.Of(raw)
	s.Require().NoError(err)
}

// child is a mapping's value at key, or nil.
func child(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// entries walks a mapping (by key) or a sequence (by index) under path.
func entries(n *yaml.Node, path string, fn func(*yaml.Node, string)) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			fn(n.Content[i+1], path+"."+n.Content[i].Value)
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			fn(c, fmt.Sprintf("%s[%d]", path, i))
		}
	}
}

// authored is every path the read-back must account for, walked straight off
// the YAML with no help from the package under test: interactions, their
// approach, states, transitions and outcomes; every line, its check, each
// requirement, its advance, each answer row and each grant; every post and
// pass; every routine, state, wake and table row; every creature and what it
// holds, runs and waits on; every door, prop, exit, ending, record, side,
// stat block and bundle.
func (s *CastleSuite) authored() []string {
	var out []string
	add := func(p string) { out = append(out, p) }
	lines := func(n *yaml.Node, path string) {
		entries(n, path, func(o *yaml.Node, op string) {
			add(op)
			if child(o, "check") != nil {
				add(op + ".check")
			}
			if child(o, "advance") != nil {
				add(op + ".advance")
			}
			if r := child(o, "requires"); r != nil {
				if r.Kind == yaml.SequenceNode {
					entries(r, op+".requires", func(_ *yaml.Node, p string) { add(p) })
				} else {
					add(op + ".requires")
				}
			}
			for _, verdict := range []string{"success", "failure"} {
				entries(child(o, verdict), op+"."+verdict, func(row *yaml.Node, rp string) {
					add(rp)
					if child(row, "recognize") != nil {
						add(rp + ".recognize")
					}
				})
			}
		})
	}
	entries(child(s.doc, "interactions"), "interactions", func(in *yaml.Node, p string) {
		add(p)
		if child(in, "approach") != nil {
			add(p + ".approach")
		}
		entries(child(in, "outcomes"), p+".outcomes", func(list *yaml.Node, op string) {
			entries(list, op, func(c *yaml.Node, ip string) {
				if child(c, "recognize") != nil {
					ip += ".recognize"
				}
				if child(c, "arrest") != nil {
					ip += ".arrest"
				}
				add(ip)
			})
		})
		entries(child(child(in, "recipe"), "states"), p+".recipe.states", func(st *yaml.Node, sp string) {
			add(sp)
			entries(child(st, "transitions"), sp+".transitions", func(_ *yaml.Node, tp string) { add(tp) })
		})
	})
	entries(child(s.doc, "behaviorDefinitions"), "behaviorDefinitions", func(def *yaml.Node, p string) {
		add(p)
		entries(child(child(def, "recipe"), "states"), p+".recipe.states", func(st *yaml.Node, sp string) {
			add(sp)
			entries(child(st, "transitions"), sp+".transitions", func(_ *yaml.Node, tp string) { add(tp) })
		})
		entries(child(def, "table"), p+".table", func(rows *yaml.Node, kp string) {
			entries(rows, kp, func(_ *yaml.Node, rp string) { add(rp) })
		})
	})
	gp := child(child(s.doc, "room"), "room")
	entries(child(gp, "monsterDeclarations"), "room.room.monsterDeclarations", func(_ *yaml.Node, p string) { add(p) })
	entries(child(gp, "monsterBindings"), "room.room.monsterBindings", func(b *yaml.Node, p string) {
		for _, k := range []string{"holds", "arrives", "blockingStates"} {
			if child(b, k) != nil {
				add(p + "." + k)
			}
		}
		entries(child(b, "behaviors"), p+".behaviors", func(_ *yaml.Node, bp string) { add(bp) })
		entries(child(b, "posts"), p+".posts", func(post *yaml.Node, pp string) {
			add(pp)
			if child(post, "pass") != nil {
				add(pp + ".pass")
			}
		})
		lines(child(b, "options"), p+".options")
	})
	entries(child(gp, "doorBindings"), "room.room.doorBindings", func(_ *yaml.Node, p string) { add(p) })
	entries(child(gp, "propBindings"), "room.room.propBindings", func(b *yaml.Node, p string) {
		add(p)
		lines(child(b, "options"), p+".options")
	})
	entries(child(s.doc, "factions"), "factions", func(f *yaml.Node, p string) {
		add(p)
		lines(child(f, "options"), p+".options")
	})
	for _, list := range []string{"exits", "endings", "intel", "dispositions", "templates", "bundles"} {
		entries(child(s.doc, list), list, func(_ *yaml.Node, p string) { add(p) })
	}
	return out
}

// TestNothingSkippedSilently holds every authored thing to the read-back: it
// is the path of a statement or it is in the not-yet-said list.
func (s *CastleSuite) TestNothingSkippedSilently() {
	said := map[string]bool{}
	for _, st := range s.rb.Statements() {
		said[st.Path] = true
	}
	for _, u := range s.rb.NotYetSaid {
		said[u.Path] = true
	}
	authored := s.authored()
	s.Require().Greater(len(authored), 400, "the walk found the castle's authored things")
	for _, p := range authored {
		s.True(said[p], "authored %s is neither said nor listed as not yet said", p)
	}
}

// TestEveryPathIsWhereItIsSet holds every statement's path to the file: it
// names a node the author wrote.
func (s *CastleSuite) TestEveryPathIsWhereItIsSet() {
	for _, st := range s.rb.Statements() {
		if st.Path == readback.EngineRule {
			continue
		}
		s.NotNil(resolve(s.doc, st.Path), "statement %q names %s, which the file does not have", st.Sentence, st.Path)
	}
}

// resolve walks a refusal-form path through the authored tree.
func resolve(n *yaml.Node, path string) *yaml.Node {
	for _, part := range strings.Split(path, ".") {
		key, rest, _ := strings.Cut(part, "[")
		if key != "" {
			n = child(n, key)
		}
		for rest != "" {
			var idx int
			if _, err := fmt.Sscanf(rest, "%d]", &idx); err != nil || n == nil || n.Kind != yaml.SequenceNode || idx >= len(n.Content) {
				return nil
			}
			n = n.Content[idx]
			_, rest, _ = strings.Cut(rest, "[")
		}
		if n == nil {
			return nil
		}
	}
	return n
}

// statement finds the one statement at path of kind, failing when absent.
func (s *CastleSuite) statement(kind readback.Kind, path string) readback.Statement {
	for _, st := range s.rb.Statements() {
		if st.Kind == kind && st.Path == path {
			return st
		}
	}
	s.Require().Failf("no statement", "no %s statement at %s", kind, path)
	return readback.Statement{}
}

// TestThreeGuardsHoldTheGate: every gate guard holds the gate door and lets
// through whom it recognises as admitted.
func (s *CastleSuite) TestThreeGuardsHoldTheGate() {
	for _, post := range []struct{ who, state, name string }{
		{"gate-1", "posted", "Guard (gate-1)"},
		{"gate-2", "ready", "Guard (gate-2)"},
		{"captain", "ready", "Guard Captain (captain)"},
	} {
		at := fmt.Sprintf("room.room.monsterBindings.%s.posts.%s", post.who, post.state)
		hold := s.statement(readback.KindPost, at)
		s.Contains(hold.Sentence, post.name+" holds the door `gate-wall-door`")
		s.Contains(hold.Sentence, "every creature not allied with "+post.name+" is barred")
		pass := s.statement(readback.KindPass, at+".pass")
		s.Contains(pass.Sentence, post.name+" lets a walker through that post, as an ally walks, when "+
			post.name+" recognises the walker as `admitted` ("+post.name+" holds `admitted` about the walker)")
	}
}

// TestTheCaptainsGuestClaim: the claim, its roll, and what winning grants to
// whom about whom — and who else hears it.
func (s *CastleSuite) TestTheCaptainsGuestClaim() {
	at := "room.room.monsterBindings.captain.options[0]"
	s.Contains(s.statement(readback.KindLine, at).Sentence,
		"The speaker can say the line “We’re invited guests of His Majesty, here for the feast.” (`guest`) to Guard Captain (captain).")
	s.Contains(s.statement(readback.KindLineCheck, at+".check").Sentence, "the speaker rolls Persuasion against DC 11")
	adv := s.statement(readback.KindLineAdvance, at+".advance").Sentence
	s.Contains(adv, "winning it steps the conversation `front-gate` with the beat `admitted`, which moves it from `fresh` to `admitted`")
	s.Contains(adv, "missing it steps the conversation `front-gate` with the beat `guest-failed`, which moves it from `fresh` to `warned-guest` or from `warned-troupe` to `arrested`")
	// Winning grants nothing on the line itself: the conversation's outcome
	// is what admits, and it names the captain among its holders.
	out := s.statement(readback.KindOutcome, "interactions.front-gate.outcomes.admitted[0].recognize").Sentence
	s.Contains(out, "When the conversation `front-gate` enters `admitted`, the creature the deciding line was said to "+
		"(whichever of Guard (gate-1), Guard (gate-2) and Guard Captain (captain) that was) comes to hold `admitted` about "+
		"the speaker and each member of the speaker's party that the creature sees at that moment")
	s.Contains(out, "Every other creature that carries the conversation `front-gate`")
	s.Contains(out, "was among those the line's words reached when it was said, and is up, comes to hold `admitted` "+
		"about the same members too, having heard it, whether or not it sees them itself; one the words did not reach "+
		"holds nothing, even if it saw the line said",
		"the heard channel follows the line's words, not sight (rpg-project#933, R9)")
}

// TestTheAlarmIsShoutedAndTeachesWhomItReaches: the castle's alarm rows are
// authored `shout` (rpg-project#933), and the read-back says so, and says the
// fact goes to whoever the words reach, seen or not.
func (s *CastleSuite) TestTheAlarmIsShoutedAndTeachesWhomItReaches() {
	found := false
	for _, st := range s.rb.Statements() {
		if st.Kind != readback.KindRoutineRow || !strings.Contains(st.Sentence, "learns the fact `alarm-raised`") {
			continue
		}
		found = true
		s.Contains(st.Sentence, "shouts “", "an alarm row is shouted: %s", st.Sentence)
		s.Contains(st.Sentence, "'s room and the next room over")
		s.Contains(st.Sentence, "whether or not they see")
		s.NotContains(st.Sentence, "or hears the voice", "the old sight-or-voice audience is gone")
	}
	s.True(found, "the castle's alarm rows are said")
}

// TestALineSaysItsLoudnessAndWhoPerceivesWhat: every line says how loudly it
// is said and the rule every member's version follows (rpg-project#933).
func (s *CastleSuite) TestALineSaysItsLoudnessAndWhoPerceivesWhat() {
	st := s.statement(readback.KindLine, "room.room.monsterBindings.captain.options[0]")
	s.Contains(st.Sentence, "It is said at `speak` (the default): its words reach everyone in the speaker's room")
	s.Contains(st.Sentence, fmt.Sprintf("and anyone in the next room within %d feet of the speaker by the way through an open doorway",
		encounter.SpeechCarryFeet), "the spoken reach through a doorway, from the engine's own constant (rpg-project#953)")
	s.Contains(st.Sentence, "and whoever it is said to, wherever they stand")
	s.Contains(st.Sentence, "whoever the words reach and does not see the speaker hears the words — and which line it was, "+
		"the roll, whether it worked and what it taught — from someone unnamed")
	s.Contains(st.Sentence, "whoever only sees it said learns neither the words, which line it was, whether it worked, nor what it taught")
}

// TestTheCaptainGoesIn: the captain's routine wakes on the topic and walks
// to the hall.
func (s *CastleSuite) TestTheCaptainGoesIn() {
	wake := s.statement(readback.KindRoutineWake, "behaviorDefinitions.captain-policy.recipe.states.ready.transitions[2]")
	s.Contains(wake.Sentence, "In the state `ready`, when Guard Captain (captain) comes to hold a topic about someone")
	s.Contains(wake.Sentence, "if Guard Captain (captain) holds the topic `admitted` about anyone")
	s.Contains(wake.Sentence, "Guard Captain (captain) moves to the state `in-hall`")
	walk := s.statement(readback.KindRoutineRow, "behaviorDefinitions.captain-policy.table.in-hall[0]")
	s.Contains(walk.Sentence, "Guard Captain (captain) walks toward the cell q 3, r -9")
	tick := s.statement(readback.KindRoutineWake, "behaviorDefinitions.captain-policy.recipe.states.in-hall.transitions[1]")
	s.Contains(tick.Sentence, "acts from its own `in-hall` rows")
}

// TestTheGuardsVouchAndStraggler: both guards' vouch and straggler lines,
// their requirements in words, and the failure row's grant.
func (s *CastleSuite) TestTheGuardsVouchAndStraggler() {
	for _, g := range []struct{ id, name string }{{"gate-1", "Guard (gate-1)"}, {"gate-2", "Guard (gate-2)"}} {
		base := "room.room.monsterBindings." + g.id + ".options"
		s.Contains(s.statement(readback.KindLineRequire, base+"[0].requires[1]").Sentence,
			"only while "+g.name+" recognises the speaker as `admitted`")
		vouch := s.statement(readback.KindLineGrant, base+"[0].success[0].recognize").Sentence
		s.Contains(vouch, g.name+" — the creature the line is said to — comes to hold `admitted` about the speaker and each member of the speaker's party that "+g.name+" sees")
		s.Contains(s.statement(readback.KindLineRequire, base+"[1].requires[1]").Sentence,
			"only while "+g.name+" does NOT recognise the speaker as `admitted`")
		s.Contains(s.statement(readback.KindLineRequire, base+"[1].requires[2]").Sentence,
			"only while "+g.name+" does NOT recognise the speaker as `turned-away`")
		s.Contains(s.statement(readback.KindLineCheck, base+"[1].check").Sentence,
			"Persuasion against DC 13, or Deception against DC 15")
		away := s.statement(readback.KindLineGrant, base+"[1].failure[0].recognize").Sentence
		s.Contains(away, "On a miss of the line")
		s.Contains(away, g.name+" — the creature the line is said to — comes to hold `turned-away` about the speaker alone")
		s.Contains(away, "Every other creature that carries the conversation `front-gate`")
	}
}

// topicSites walks the authored tree for every place a topic is granted (a
// `recognize`) and every place it is asked for or woken on (a `recognized`
// requirement or pass, a `topic:` guard), at the path the read-back should
// name.
func topicSites(n *yaml.Node, path string, grants, asks map[string]map[string]bool) {
	note := func(m map[string]map[string]bool, topic, at string) {
		if m[topic] == nil {
			m[topic] = map[string]bool{}
		}
		m[topic][at] = true
	}
	switch n.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, v := n.Content[i].Value, n.Content[i+1]
			at := key
			if path != "" {
				at = path + "." + key
			}
			switch {
			case key == "recognize":
				note(grants, child(v, "topic").Value, at)
			case key == "recognized" && v.Kind == yaml.ScalarNode:
				note(asks, v.Value, askingPlace.FindString(at))
			case key == "name" && strings.HasPrefix(v.Value, "topic:"):
				note(asks, strings.TrimPrefix(v.Value, "topic:"), wakingPlace.FindString(at))
			}
			topicSites(v, at, grants, asks)
		}
	case yaml.SequenceNode:
		for i, c := range n.Content {
			topicSites(c, fmt.Sprintf("%s[%d]", path, i), grants, asks)
		}
	}
}

var (
	askingPlace = regexp.MustCompile(`^.*\.(requires(\[\d+\])?|pass)`)
	wakingPlace = regexp.MustCompile(`^.*\.transitions\[\d+\]`)
)

// TestTopicHubs: each gate topic's hub lists exactly every place the file
// grants it and every place the file asks for it or wakes on it.
func (s *CastleSuite) TestTopicHubs() {
	grants, asks := map[string]map[string]bool{}, map[string]map[string]bool{}
	topicSites(s.doc, "", grants, asks)
	for _, topic := range []string{"admitted", "turned-away"} {
		g, ok := s.rb.Group("Topics", readback.Subject{Kind: "topic", ID: topic})
		s.Require().True(ok, "topic %s has a hub", topic)
		gotGrants, gotAsks := map[string]bool{}, map[string]bool{}
		for _, st := range g.Statements {
			switch st.Kind {
			case readback.KindTopicGrant:
				gotGrants[st.Path] = true
			case readback.KindTopicAsk:
				gotAsks[st.Path] = true
			}
		}
		s.Require().NotEmpty(grants[topic], "the walk found %s's grants", topic)
		s.Require().NotEmpty(asks[topic], "the walk found %s's asks", topic)
		s.Equal(grants[topic], gotGrants, "%s: every grant site and nothing else", topic)
		s.Equal(asks[topic], gotAsks, "%s: every asking site and nothing else", topic)
	}
}

// TestTheFoldedFormDropsNothing: every statement is stood for by a folded
// entry that carries its path.
func (s *CastleSuite) TestTheFoldedFormDropsNothing() {
	carried := map[string]bool{}
	var walk func(it readback.Item)
	walk = func(it readback.Item) {
		for _, p := range it.Paths {
			carried[p] = true
		}
		for _, c := range it.Items {
			walk(c)
		}
	}
	for _, sec := range s.rb.Folded() {
		for _, it := range sec.Items {
			walk(it)
		}
	}
	for _, st := range s.rb.Statements() {
		s.True(carried[st.Path], "statement at %s (%s) has no folded entry", st.Path, st.Kind)
	}
}

// TestTheFoldedFormIsNarrow: no folded line is wider than 160 columns.
func (s *CastleSuite) TestTheFoldedFormIsNarrow() {
	for _, line := range strings.Split(s.rb.FoldedText(), "\n") {
		s.LessOrEqual(len([]rune(line)), 160, line)
	}
}

// TestSameLinesFold: the second gate guard's lines are the first's and are
// named, not said again.
func (s *CastleSuite) TestSameLinesFold() {
	text := s.rb.FoldedText()
	s.Contains(text, "Guard (gate-2) has the same 4 lines as Guard (gate-1): `vouch`, `straggler`, `friends-inside` and\n"+
		"        `troupe`.")
	s.Equal(1, strings.Count(text, "“They’re with us. The captain let us in.” (`vouch`)"))
}

// TestOneWordPreventsTheFold: the second guard's vouch answer differing by
// one word is said in full, and only it; her other three lines still fold.
func (s *CastleSuite) TestOneWordPreventsTheFold() {
	gate2 := strings.Index(string(s.raw), "      gate-2:\n        faction: garrison")
	s.Require().Positive(gate2)
	edited := string(s.raw[:gate2]) + strings.Replace(string(s.raw[gate2:]),
		"Right you are. Go on through, the lot of you.", "Right you are. Go on through, the pair of you.", 1)
	rb, err := readback.Of([]byte(edited))
	s.Require().NoError(err)
	text := rb.FoldedText()
	s.Contains(text, "Guard (gate-2) has the same 3 lines as Guard (gate-1): `straggler`, `friends-inside` and `troupe`.")
	s.Equal(2, strings.Count(text, "“They’re with us. The captain let us in.” (`vouch`)"))
	s.Contains(text, "the guard answers “Right you are. Go on through, the pair of you.”")
}

// TestTheHailIsTheEngines: the hail says the engine's rule (the mover's own
// step, one hailer, the nearest offering a line that steps the conversation,
// none while it is addressing someone), and who can hail in each state.
func (s *CastleSuite) TestTheHailIsTheEngines() {
	hail := s.statement(readback.KindApproach, "interactions.front-gate.approach").Sentence
	s.Contains(hail, "When a character's own step ends within 2 cells of a carrier who sees them")
	s.Contains(hail, "that carrier offers them a line that steps `front-gate`, the nearest such carrier")
	s.Contains(hail, "unless `front-gate` is already addressing someone")
	s.Contains(hail, "It is opened within 2 cells, and stands while the other conditions hold and the character is "+
		"within Speak reach of that carrier (never less than 2 cells); it ends when any fails.")
	who := s.statement(readback.KindHailers, "interactions.front-gate.approach").Sentence
	s.Contains(who, "in `warned-troupe`, Guard Captain (captain) (by `guest`)")
	s.Contains(who, "in `admitted` and `arrested`, nobody")
	wake := s.statement(readback.KindRoutineWake, "behaviorDefinitions.captain-policy.recipe.states.ready.transitions[3]").Sentence
	s.Contains(wake, "when Guard Captain (captain) hails a character, which happens only under the conversation's hail rule")
}

// TestTheDoorSaysAllItsHolders: a door with several posts says that one
// holder's pass is not enough while another posted holder bars the walker.
func (s *CastleSuite) TestTheDoorSaysAllItsHolders() {
	door := s.statement(readback.KindDoorHold, "room.room.doorBindings.gate-wall-door").Sentence
	s.Contains(door, "held by the posts of Guard (gate-1), Guard (gate-2) and Guard Captain (captain)")
	s.Contains(door, "stays barred to a walker while any one posted holder not allied with the walker does not let that walker through")
	s.Contains(s.rb.FoldedText(), "but the door has other holders, and stays barred")
}

// TestALineSendsOnlyWhereItIsOffered: a transition names only the lines
// offered in the state it leaves, and a line's advance only the states it
// is offered in.
func (s *CastleSuite) TestALineSendsOnlyWhereItIsOffered() {
	fromTroupe := s.statement(readback.KindConvStep, "interactions.front-gate.recipe.states.warned-troupe.transitions[1]").Sentence
	s.Contains(fromTroupe, "The beat is sent by winning `guest` said to Guard Captain (captain).")
	s.NotContains(fromTroupe, "`troupe`")
	fromGuest := s.statement(readback.KindConvStep, "interactions.front-gate.recipe.states.warned-guest.transitions[1]").Sentence
	s.NotContains(fromGuest, "`guest`")
	guest := s.statement(readback.KindLineAdvance, "room.room.monsterBindings.captain.options[0].advance").Sentence
	s.Contains(guest, "which moves it from `fresh` to `admitted` or from `warned-troupe` to `admitted`;")
	s.NotContains(guest, "from `warned-guest` to `admitted`")
}

// TestTheEngineRulesSayWhenTheWaitEnds: the pending wait ends on a settle or
// a release, and a menu is stale after any change of the conversation's
// revision, a roll's wait included.
func (s *CastleSuite) TestTheEngineRulesSayWhenTheWaitEnds() {
	var rules []string
	for _, st := range s.rb.Statements() {
		if st.Kind == readback.KindEngineRule && st.About.ID == "front-gate" {
			s.Equal(readback.EngineRule, st.Path)
			rules = append(rules, st.Sentence)
		}
	}
	all := strings.Join(rules, "\n")
	s.Contains(all, "is pending for every speaker until that roll is settled or released.")
	s.Contains(all, "before `front-gate` last changed (a state change, or a roll beginning or ending its wait on it) "+
		"is refused with nothing spent")
}

// TestTheGuardsAnswerShoutsNotTalk: the read-back says the raised voice a
// post and a knight answer (rpg-project#933), as the engine judges it.
func (s *CastleSuite) TestTheGuardsAnswerShoutsNotTalk() {
	wake := s.statement(readback.KindRoutineWake, "behaviorDefinitions.gate-1-policy.recipe.states.posted.transitions[5]")
	s.Contains(wake.Sentence, "when Guard (gate-1) hears a sound, if it happened within 6 cells of Guard (gate-1) and "+
		"(NOT (what Guard (gate-1) heard was a `voice`), or what Guard (gate-1) heard was a voice at `shout` or louder "+
		"(a clang, a crash or a boom is at no loudness)), Guard (gate-1) moves to the state `alert`")
	row := s.statement(readback.KindRoutineRow, "behaviorDefinitions.keep-1-policy.table.time[5]")
	s.Contains(row.Sentence, "heard a `voice` sound at `shout` or louder within the last 3 rounds")
}

// TestTheShoutAtTheGateSaysItsReach: the stranger's shouted line
// (rpg-project#933) says it is shouted and reaches the next room — the
// gatehouse, whose guards catch it unseen.
func (s *CastleSuite) TestTheShoutAtTheGateSaysItsReach() {
	st := s.statement(readback.KindLine, "room.room.monsterBindings.pedlar.options[1]")
	s.Contains(st.Sentence, "“Oi! Pedlar! What have you got to sell?” (`hail-the-pedlar`)")
	s.Contains(st.Sentence, "It is said at `shout` (authored): its words reach everyone in the speaker's room and the next room over")
	s.Contains(st.Sentence, "It is a plain line")
}

// TestTheFoldedShoutSaysItIsShouted: the folded form the operator reads says
// a line authored louder than speech is (rpg-project#933), and a spoken line
// says nothing of its loudness.
func (s *CastleSuite) TestTheFoldedShoutSaysItIsShouted() {
	var shouted, spoken string
	for _, line := range strings.Split(strings.Join(strings.Fields(s.rb.FoldedText()), " "), " - ") {
		switch {
		case strings.Contains(line, "(`hail-the-pedlar`), said to"):
			shouted = line
		case strings.Contains(line, "(`tonight`), said to"):
			spoken = line
		}
	}
	s.Contains(shouted, "said to the commoner; shouted: its words reach the next room over; plain: rolls nothing")
	s.NotEmpty(spoken, "precondition: the stranger's spoken line is folded too")
	s.NotContains(spoken, "shouted", "a spoken line says nothing of its loudness")
	s.NotContains(spoken, "its words reach")
}

// TestAWatcherOfTheCaptainsAnswerIsToldOnlyThatHeAnswered: the read-back
// says what the engine does (rpg-project#948, castletest's
// TestMadKingTheStandAsidesPickTellsAWatcherNothing): one who sees the
// captain answer the stand-aside and caught none of it is told only that he
// answered.
func (s *CastleSuite) TestAWatcherOfTheCaptainsAnswerIsToldOnlyThatHeAnswered() {
	found := false
	for _, st := range s.rb.Statements() {
		if st.Kind == readback.KindLineRow && strings.Contains(st.Sentence, "Stand aside or bleed.") {
			found = true
			s.Contains(st.Sentence, "whoever sees Guard Captain (captain) answer and caught none of the line is told only that "+
				"Guard Captain (captain) answered and what Guard Captain (captain) is seen doing, and, if they see the speaker, sees the line said without its words")
		}
	}
	s.True(found, "the stand-aside's answer rows are said")
}

// TestAStateChangeReachesTheWholeRun: a routine's state change says that the
// change and its reason reach every member of the run, wherever they are,
// whether or not they saw or heard what caused it (rpg-project#948) — true
// against the engine (castletest's TestMadKingAStateChangeReachesTheWholeRun:
// a watcher who caught none of the stand-aside, and a member in the great
// hall who saw nothing of it, are both told the captain's `ready -> cowed`,
// "stood aside").
func (s *CastleSuite) TestAStateChangeReachesTheWholeRun() {
	found := false
	for _, st := range s.rb.Statements() {
		if st.Kind == readback.KindRoutineWake && strings.Contains(st.Sentence, "Guard Captain (captain) moves to the state `cowed`") &&
			strings.Contains(st.Sentence, "“stood aside”") {
			found = true
			s.Contains(st.Sentence, "the change and its reason reach every member of the run, wherever they are, "+
				"whether or not they saw or heard what caused it")
		}
	}
	s.True(found, "the captain's stand-aside transition is said")
}
