// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// conversations_test.go is A CONVERSATION'S LINES WRITTEN ONCE
// (rpg-project#1025): `conversations:` at the root, `- use: <id>` in any
// `options:` list, expanded at load into exactly what the copied lines
// compile to, and every refusal in a sentence at the author's path.

type ConversationsSuite struct{ suite.Suite }

func TestConversationsSuite(t *testing.T) { suite.Run(t, new(ConversationsSuite)) }

// mapValue is the value under key in a mapping node, or nil.
func mapValue(n *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// scalar is a plain string node.
func scalar(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v} }

// useEntry is `{ use: <id> }`.
func useEntry(id string) *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{scalar("use"), scalar(id)}}
}

// lineIDs is the ids of a list of option nodes.
func optionNodeIDs(items []*yaml.Node) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, mapValue(it, "id").Value)
	}
	return out
}

// marshal writes a document node back to YAML.
func (s *ConversationsSuite) marshal(doc *yaml.Node) []byte {
	out, err := yaml.Marshal(doc)
	s.Require().NoError(err)
	return out
}

// castleBothWays is the castle fixture twice, through the same printer: as
// written (each gate speaker carries its own copy of the gate's lines) and
// with those lines written once under `conversations:`, gate-1 and gate-2
// saying `gate-after` and `gate-claims`, the captain saying `gate-claims`
// beside his own `guest` and the rest of his own lines.
func (s *ConversationsSuite) castleBothWays() (copied, shared []byte) {
	raw, err := os.ReadFile("testdata/world-builder-v4-castle.yaml")
	s.Require().NoError(err)
	var doc yaml.Node
	s.Require().NoError(yaml.Unmarshal(raw, &doc))
	copied = s.marshal(&doc)

	root := doc.Content[0]
	bindings := mapValue(mapValue(mapValue(root, "room"), "room"), "monsterBindings")
	gate1 := mapValue(mapValue(bindings, "gate-1"), "options")
	gate2 := mapValue(mapValue(bindings, "gate-2"), "options")
	captain := mapValue(mapValue(bindings, "captain"), "options")
	s.Require().Equal([]string{"vouch", "straggler", "friends-inside", "troupe", "come-quietly", "never-alive"},
		optionNodeIDs(gate1.Content), "the fixture's gate-1 still carries the gate's six lines")
	s.Require().Equal(optionNodeIDs(gate1.Content), optionNodeIDs(gate2.Content), "gate-2 carries the same")
	s.Require().Equal([]string{"guest", "troupe", "come-quietly", "never-alive"}, optionNodeIDs(captain.Content[:4]),
		"the captain carries his own guest, then the gate's three claims")

	lines := func(items []*yaml.Node) *yaml.Node {
		return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
			scalar("lines"), {Kind: yaml.SequenceNode, Tag: "!!seq", Content: append([]*yaml.Node(nil), items...)},
		}}
	}
	conversations := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map", Content: []*yaml.Node{
		scalar("gate-after"), lines(gate1.Content[:3]),
		scalar("gate-claims"), lines(gate1.Content[3:]),
	}}
	gate1.Content = []*yaml.Node{useEntry("gate-after"), useEntry("gate-claims")}
	gate2.Content = []*yaml.Node{useEntry("gate-after"), useEntry("gate-claims")}
	captain.Content = append([]*yaml.Node{captain.Content[0], useEntry("gate-claims")}, captain.Content[4:]...)
	root.Content = append(root.Content, scalar("conversations"), conversations)

	return copied, s.marshal(&doc)
}

// TestTheGateWrittenOnceCompilesAsTheCopies is the done-means proof: the
// castle with the gate's lines written once compiles to exactly the castle
// with them copied onto gate-1, gate-2 and the captain, content hash and all.
func (s *ConversationsSuite) TestTheGateWrittenOnceCompilesAsTheCopies() {
	copied, shared := s.castleBothWays()
	s.Require().Contains(string(shared), "- use: gate-claims")
	s.Require().NotContains(string(shared), "id: never-alive\n          say", "no gate speaker writes the line itself")

	want, err := dungeonspec.Load(copied)
	s.Require().NoError(err)
	got, err := dungeonspec.Load(shared)
	s.Require().NoError(err)
	s.Equal(want, got)
	wantHash, err := want.ContentHash()
	s.Require().NoError(err)
	gotHash, err := got.ContentHash()
	s.Require().NoError(err)
	s.Equal(wantHash, gotHash)

	// AND THE COMPARISON CAN FAIL: one line renamed in the shared form
	// compiles to something else.
	renamed := strings.Replace(string(shared), "id: never-alive", "id: never-taken", 1)
	s.Require().NotEqual(string(shared), renamed)
	other, err := dungeonspec.Load([]byte(renamed))
	s.Require().NoError(err)
	s.NotEqual(want, other)
}

// TestEachLineSaysWhereItWasWritten: LineSources names each carrier line's
// authored path, in the order the carrier says them.
func (s *ConversationsSuite) TestEachLineSaysWhereItWasWritten() {
	_, shared := s.castleBothWays()
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: shared})
	s.Require().NoError(err)
	fresh, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: shared})
	s.Require().NoError(err)
	at := "room.room.monsterBindings.captain"
	sources := dungeonspec.LineSources(fresh.Spec, at, fresh.Spec.Room.Gameplay.MonsterBindings["captain"].Options)
	s.Require().GreaterOrEqual(len(sources), 4)
	s.Equal(dungeonspec.LineSource{Path: at + ".options[0]"}, sources[0])
	s.Equal(dungeonspec.LineSource{Path: "conversations.gate-claims.lines[0]", Conversation: "gate-claims", Use: at + ".options[1]"}, sources[1])
	s.Equal(dungeonspec.LineSource{Path: "conversations.gate-claims.lines[2]", Conversation: "gate-claims", Use: at + ".options[1]"}, sources[3])
	s.Equal(dungeonspec.LineSource{Path: at + ".options[2]"}, sources[4], "an own line after the use keeps its authored index")
	// The decoded spec keeps what was authored: the use entry, not its lines.
	authored := decoded.Spec.Room.Gameplay.MonsterBindings["captain"].Options
	s.Equal("gate-claims", authored[1].Use)
	s.Empty(authored[1].ID)
}

// hailWith is the hail site with the porter's options replaced by options
// (each line at the list's indent) and the given `conversations:` block at
// the root.
func (s *ConversationsSuite) hailWith(conversations, options string) string {
	raw, err := os.ReadFile("testdata/hail-site.yaml")
	s.Require().NoError(err)
	src := string(raw)
	start := strings.Index(src, "        options:\n          - id: guest")
	end := strings.Index(src, "        behaviors:\n          - id: policy")
	s.Require().True(start > 0 && end > start, "the fixture still gives the porter his guest line")
	src = src[:start] + "        options:\n" + options + src[end:]
	if conversations != "" {
		src = strings.Replace(src, "room:\n  version: 3", "conversations:\n"+conversations+"room:\n  version: 3", 1)
	}
	return src
}

// welcome is the porter's guest line, status and all, as a conversation.
const welcome = `  welcome:
    lines:
      - id: guest
        say: We're expected.
        requires:
          - interaction:
              id: gate
              in: [fresh]
        advance:
          interaction: gate
          success: admitted
        success:
          - say: In you go.
            apply_status: welcomed
`

// TestALineGivingAStatusCompilesAsItsCopy: a used line carries every row
// kind an option carries, apply_status included.
func (s *ConversationsSuite) TestALineGivingAStatusCompilesAsItsCopy() {
	raw, err := os.ReadFile("testdata/hail-site.yaml")
	s.Require().NoError(err)
	want, err := dungeonspec.Load(raw)
	s.Require().NoError(err)
	got, err := dungeonspec.Load([]byte(s.hailWith(welcome, "          - use: welcome\n")))
	s.Require().NoError(err)
	s.Equal(want, got)
	s.Require().NotEmpty(got.Monsters)
	var given bool
	for _, o := range got.Monsters[0].Options {
		for _, g := range o.Grants {
			given = given || g.Consequence.ApplyStatus != nil
		}
	}
	s.True(given, "the used line still gives its status")
}

// refusals is the defects of a hail-site variant.
func (s *ConversationsSuite) refusals(conversations, options string) []string {
	return roomDefects(s.T(), s.hailWith(conversations, options))
}

func (s *ConversationsSuite) TestAnUnknownConversationIsRefused() {
	s.Contains(s.refusals(welcome, "          - use: welcom\n"),
		"room.room.monsterBindings.porter.options[0].use: `use: welcom` names no conversation in this file: it declares welcome")
}

func (s *ConversationsSuite) TestAnEmptyUseIsRefused() {
	s.Contains(s.refusals(welcome, "          - use: welcome\n          - use: \"\"\n"),
		"room.room.monsterBindings.porter.options[1].use: `use` names no conversation")
}

func (s *ConversationsSuite) TestAUseThatSaysMoreIsRefused() {
	s.Contains(s.refusals(welcome, "          - use: welcome\n            id: extra\n            say: Hello.\n"),
		"room.room.monsterBindings.porter.options[0]: a `use:` entry stands for a conversation's lines and says nothing else: "+
			"drop `id`, `say` here, or write it as a line of its own beside the use")
}

func (s *ConversationsSuite) TestALineSaidTwiceOnACarrierNamesBoth() {
	own := "          - id: guest\n            say: Hello.\n          - use: welcome\n"
	s.Contains(s.refusals(welcome, own),
		"room.room.monsterBindings.porter.options[1].use: line \"guest\" is said twice on this carrier: at "+
			"room.room.monsterBindings.porter.options[0] and at conversations.welcome.lines[0]; a line's id is unique among "+
			"what one carrier says, so rename one or drop it")
}

func (s *ConversationsSuite) TestAConversationUsedTwiceOnACarrierIsRefused() {
	s.Contains(s.refusals(welcome, "          - use: welcome\n          - use: welcome\n"),
		"room.room.monsterBindings.porter.options[1].use: conversation \"welcome\" is already used at options[0]; "+
			"a carrier says a conversation once")
}

func (s *ConversationsSuite) TestAConversationNobodyUsesIsRefused() {
	spare := welcome + "  spare:\n    lines:\n      - id: hello\n        say: Hello.\n"
	s.Contains(s.refusals(spare, "          - use: welcome\n"),
		"conversations.spare: conversation \"spare\" is used by nobody: write `- use: spare` in a creature's, a prop's "+
			"or a faction's options, or remove it")
}

func (s *ConversationsSuite) TestAConversationWithNoLinesIsRefused() {
	for _, empty := range []string{"  spare:\n    lines: []\n", "  spare: {}\n"} {
		got := s.refusals(welcome+empty, "          - use: welcome\n          - use: spare\n")
		s.Contains(got, "conversations.spare.lines: a conversation holds no lines: write the lines it offers, or remove it", empty)
		s.NotContains(got, "conversations.spare: conversation \"spare\" is used by nobody: write `- use: spare` in a "+
			"creature's, a prop's or a faction's options, or remove it", "one refusal for one fault")
	}
}

func (s *ConversationsSuite) TestAConversationDoesNotNest() {
	nested := welcome + "      - use: welcome\n"
	s.Contains(s.refusals(nested, "          - use: welcome\n"),
		"conversations.welcome.lines[1].use: a conversation's line cannot use another conversation; conversations do not "+
			"nest. Write the line here, or list both conversations on each carrier")
}

// TestAFaultInALineIsSaidOnceAtTheConversation: two carriers (the porter
// and his faction) say a conversation whose line is wrong; the fault is one
// refusal at the line's own path, and none at either carrier.
func (s *ConversationsSuite) TestAFaultInALineIsSaidOnceAtTheConversation() {
	loud := strings.Replace(welcome, "        say: We're expected.\n", "        say: We're expected.\n        loudness: bellow\n", 1)
	src := s.hailWith(loud, "          - use: welcome\n")
	src = strings.Replace(src, "factions:\n  - id: watch\n", "factions:\n  - id: watch\n    options:\n      - use: welcome\n", 1)
	got := roomDefects(s.T(), src)
	s.Require().NotEmpty(got)
	var at []string
	for _, d := range got {
		if strings.Contains(d, "loudness") {
			at = append(at, d)
		}
	}
	s.Require().Len(at, 1, "one fault, one refusal: %v", got)
	s.True(strings.HasPrefix(at[0], "conversations.welcome.lines[0].loudness: "), at[0])
}

// TestAnOwnLineKeepsItsAuthoredPath: a fault in a carrier's own line after
// a use is said at the index the author wrote, not the expanded one.
func (s *ConversationsSuite) TestAnOwnLineKeepsItsAuthoredPath() {
	own := "          - use: welcome\n          - id: hello\n            say: Hello.\n            loudness: bellow\n"
	s.Contains(strings.Join(s.refusals(welcome, own), "\n"), "room.room.monsterBindings.porter.options[1].loudness: ")
}
