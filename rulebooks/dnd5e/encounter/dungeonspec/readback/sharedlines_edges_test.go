// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// sharedlines_edges_test.go holds the promise that a conversation's lines
// written once state EXACTLY what the same lines copied onto each carrier
// state (rpg-project#1025): nothing a copy states is lost, and nothing is
// stated that no copy states.

const conversationEdgesPath = "testdata/conversation-edges-site.yaml"

// carrierCopy is where one carrier's copy of a conversation's lines starts:
// the carrier as a connection names it, its options path, and the index of
// its copy of the conversation's first line.
type carrierCopy struct {
	carrier readback.Connection
	base    string
	off     int
}

// line is the path of the carrier's copy of line k.
func (c carrierCopy) line(k int) string { return fmt.Sprintf("%s[%d]", c.base, c.off+k) }

// copiedOut is a shared-form site with every `- use: X` replaced by X's
// lines, in place, and the root `conversations:` removed: the site as it was
// written before a conversation could be written once. It also returns, per
// conversation, where each carrier's copy of it starts.
func copiedOut(t *testing.T, raw []byte) ([]byte, map[string][]carrierCopy) {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	root := doc.Content[0]
	convs := child(root, "conversations")
	require.NotNil(t, convs, "the site writes its lines once")
	copies := map[string][]carrierCopy{}
	expand := func(opts *yaml.Node, kind, id, at string) {
		if opts == nil {
			return
		}
		var expanded []*yaml.Node
		for _, o := range opts.Content {
			use := child(o, "use")
			if use == nil {
				expanded = append(expanded, o)
				continue
			}
			lines := child(child(convs, use.Value), "lines")
			require.NotNil(t, lines, "use: %s names a conversation", use.Value)
			copies[use.Value] = append(copies[use.Value], carrierCopy{
				carrier: readback.Connection{Kind: kind, ID: id}, base: at + ".options", off: len(expanded),
			})
			expanded = append(expanded, lines.Content...)
		}
		opts.Content = expanded
	}
	if factions := child(root, "factions"); factions != nil {
		for i, f := range factions.Content {
			expand(child(f, "options"), "faction", child(f, "id").Value, fmt.Sprintf("factions[%d]", i))
		}
	}
	gp := child(child(root, "room"), "room")
	for kind, key := range map[string]string{"creature": "monsterBindings", "prop": "propBindings"} {
		bindings := child(gp, key)
		for i := 0; bindings != nil && i+1 < len(bindings.Content); i += 2 {
			id := bindings.Content[i].Value
			expand(child(bindings.Content[i+1], "options"), kind, id, "room.room."+key+"."+id)
		}
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "conversations" {
			root.Content = append(root.Content[:i], root.Content[i+2:]...)
			break
		}
	}
	out, err := yaml.Marshal(&doc)
	require.NoError(t, err)
	return out, copies
}

// stated is one connection a statement states, with the statement's kind
// and the statement's path below the line it belongs to.
type stated struct {
	kind   readback.Kind
	suffix string
	conn   readback.Connection
}

func (s stated) String() string {
	return fmt.Sprintf("%s at <line>%s: %+v", s.kind, s.suffix, s.conn)
}

// statedAt is every connection stated by a statement at line's path or
// below it.
func statedAt(rb *readback.ReadBack, line string) map[stated]bool {
	out := map[stated]bool{}
	for _, st := range rb.Statements() {
		if st.Path != line && !strings.HasPrefix(st.Path, line+".") {
			continue
		}
		for _, c := range st.Connects {
			out[stated{kind: st.Kind, suffix: strings.TrimPrefix(st.Path, line), conn: c}] = true
		}
	}
	return out
}

// saidToCarrier reports whether c says a line is said or done to a carrier
// of the conversation: the one connection a copy states that the line
// written once states as the conversation (rpg-toolkit#2479).
func saidToCarrier(c readback.Connection, copies []carrierCopy) bool {
	if c.Role != "said-to" && c.Role != "done-to" {
		return false
	}
	for _, cc := range copies {
		if cc.carrier.Kind == c.Kind && cc.carrier.ID == c.ID {
			return true
		}
	}
	return false
}

// assertSharedIsTheUnionOfItsCopies builds raw twice — as written, and with
// every `use:` copied out — and asserts, for every conversation and every
// line, that the line written once states exactly the union of what its
// carriers' copies state, of the same kind at the same place under the
// line: none missing, none extra. The one translation is whom the line is
// said to: a copy says it to its carrier; the line written once says it to
// the conversation, whose head names that carrier among those that carry it.
// Across the whole read-back the same holds both ways, but for the
// conversation's own statements.
func assertSharedIsTheUnionOfItsCopies(t *testing.T, raw []byte) {
	t.Helper()
	shared, err := readback.Of(raw)
	require.NoError(t, err)
	copiedRaw, copies := copiedOut(t, raw)
	copied, err := readback.Of(copiedRaw)
	require.NoError(t, err)
	require.NotEmpty(t, copies, "the site uses a conversation")

	for conv, cs := range copies {
		var head *readback.Statement
		lines := 0
		for _, st := range shared.Statements() {
			if st.Kind == readback.KindSharedLines && st.Path == "conversations."+conv {
				head = &st
			}
			if st.Kind == readback.KindLine && strings.HasPrefix(st.Path, "conversations."+conv+".lines[") {
				lines++
			}
		}
		require.NotNil(t, head, "the conversation %s is said", conv)
		require.NotZero(t, lines)
		for _, c := range cs {
			require.Contains(t, head.Connects, readback.Connection{Kind: c.carrier.Kind, ID: c.carrier.ID, Role: "carries"})
		}
		for k := 0; k < lines; k++ {
			sharedAt := fmt.Sprintf("conversations.%s.lines[%d]", conv, k)
			union := map[stated]bool{}
			for _, c := range cs {
				at := c.line(k)
				want := statedAt(copied, at)
				require.NotEmpty(t, want, "the copy at %s states something", at)
				for w := range want {
					if saidToCarrier(w.conn, cs) {
						w.conn = readback.Connection{Kind: readback.ConnSharedLines, ID: conv, Role: w.conn.Role}
					}
					union[w] = true
				}
			}
			got := statedAt(shared, sharedAt)
			for w := range union {
				require.True(t, got[w], "a copy of %s states %s; the line written once does not", sharedAt, w)
			}
			for g := range got {
				require.True(t, union[g], "the line written once at %s states %s; no copy of it does", sharedAt, g)
			}
		}
	}

	everywhere := func(rb *readback.ReadBack) map[stated]bool {
		out := map[stated]bool{}
		for _, st := range rb.Statements() {
			if st.Kind == readback.KindSharedLines || st.Kind == readback.KindUsesLines {
				continue
			}
			for _, c := range st.Connects {
				if c.Role == "said-to" || c.Role == "done-to" {
					continue
				}
				out[stated{kind: st.Kind, conn: c}] = true
			}
		}
		return out
	}
	sharedAll, copiedAll := everywhere(shared), everywhere(copied)
	for w := range copiedAll {
		require.True(t, sharedAll[w], "the copies state %s; the shared form states it nowhere", w)
	}
	for g := range sharedAll {
		require.True(t, copiedAll[g], "the shared form states %s; no copy states it anywhere", g)
	}
}

// edgesSite is the conversation edges site with each edit applied to its
// text, each edit required to apply.
func edgesSite(t *testing.T, edits ...[2]string) []byte {
	t.Helper()
	raw, err := os.ReadFile(conversationEdgesPath)
	require.NoError(t, err)
	src := string(raw)
	for _, e := range edits {
		require.Contains(t, src, e[0])
		src = strings.Replace(src, e[0], e[1], 1)
	}
	return []byte(src)
}

// The carriers' `use:` entries as the fixture writes them.
var (
	gate2Uses = [2]string{"      gate-2:\n        faction: watch\n        options:\n          - use: gate-lines\n",
		"      gate-2:\n        faction: watch\n"}
	captainUses = [2]string{"            success:\n              - say: Then welcome. Through to the hall.\n          - use: gate-lines\n",
		"            success:\n              - say: Then welcome. Through to the hall.\n"}
	factionUses = [2]string{"factions:\n  - id: watch\n", "factions:\n  - id: watch\n    options:\n      - use: gate-lines\n"}
)

// TestAConversationWrittenOnceStatesExactlyItsCopies: three creatures carry
// the conversation.
func TestAConversationWrittenOnceStatesExactlyItsCopies(t *testing.T) {
	assertSharedIsTheUnionOfItsCopies(t, edgesSite(t))
}

// TestAConversationOneCreatureCarriesStatesExactlyItsCopy: gate-1 alone
// carries it, so its line is said to gate-1 and nobody else; gate-1 never
// overhears a line said to itself (rpg-toolkit#2485 review).
func TestAConversationOneCreatureCarriesStatesExactlyItsCopy(t *testing.T) {
	assertSharedIsTheUnionOfItsCopies(t, edgesSite(t, gate2Uses, captainUses))
}

// TestAConversationACreatureAndAFactionCarryStatesExactlyItsCopies: gate-1
// and the faction carry it, and gate-2, with no lines of its own, speaks the
// faction's. The faction is no creature spoken to, but the line may be said
// to a member of it, so gate-1 may hear it said.
func TestAConversationACreatureAndAFactionCarryStatesExactlyItsCopies(t *testing.T) {
	assertSharedIsTheUnionOfItsCopies(t, edgesSite(t, gate2Uses, captainUses, factionUses))
}

// TestTheConversationsSiteStatesExactlyItsCopies: the conversations site,
// whose `gate-claims` the three gate guards carry beside the captain's own
// line, read the same way.
func TestTheConversationsSiteStatesExactlyItsCopies(t *testing.T) {
	raw, err := os.ReadFile(conversationsPath)
	require.NoError(t, err)
	assertSharedIsTheUnionOfItsCopies(t, raw)
}
