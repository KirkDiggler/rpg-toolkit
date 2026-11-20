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
// written once LOSE NOTHING against the same lines copied onto each carrier
// (rpg-project#1025): every connection a carrier's copy of a line states is
// reachable from that line written once.

const conversationEdgesPath = "testdata/conversation-edges-site.yaml"

// copiedOut is a shared-form site with every `- use: X` replaced by X's
// lines, in place, and the root `conversations:` removed: the site as it was
// written before a conversation could be written once. It also returns, per
// conversation, each creature carrier's index of the conversation's first
// line in its expanded options.
func copiedOut(t *testing.T, raw []byte) ([]byte, map[string]map[string]int) {
	t.Helper()
	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	root := doc.Content[0]
	convs := child(root, "conversations")
	require.NotNil(t, convs, "the site writes its lines once")
	bindings := child(child(child(root, "room"), "room"), "monsterBindings")
	require.NotNil(t, bindings)
	offsets := map[string]map[string]int{}
	for i := 0; i+1 < len(bindings.Content); i += 2 {
		carrier := bindings.Content[i].Value
		opts := child(bindings.Content[i+1], "options")
		if opts == nil {
			continue
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
			if offsets[use.Value] == nil {
				offsets[use.Value] = map[string]int{}
			}
			offsets[use.Value][carrier] = len(expanded)
			expanded = append(expanded, lines.Content...)
		}
		opts.Content = expanded
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "conversations" {
			root.Content = append(root.Content[:i], root.Content[i+2:]...)
			break
		}
	}
	out, err := yaml.Marshal(&doc)
	require.NoError(t, err)
	return out, offsets
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

// TestAConversationWrittenOnceStatesEveryEdgeOfItsCopies: built twice — the
// lines copied onto three carriers, and one conversation the three use —
// every connection a carrier's copy of a line states is stated by that line
// written once, of the same kind at the same place under the line. The one
// translation is whom the line is said to: a copy says it to its carrier;
// the line written once says it to the conversation, which names that
// carrier among those that carry it.
func TestAConversationWrittenOnceStatesEveryEdgeOfItsCopies(t *testing.T) {
	raw, err := os.ReadFile(conversationEdgesPath)
	require.NoError(t, err)
	shared, err := readback.Of(raw)
	require.NoError(t, err)
	copiedRaw, offsets := copiedOut(t, raw)
	copied, err := readback.Of(copiedRaw)
	require.NoError(t, err)

	// Across the whole read-back, no connection of a kind the copies state
	// is missing from the shared form but whom a line is said to.
	everywhere := func(rb *readback.ReadBack) map[stated]bool {
		out := map[stated]bool{}
		for _, st := range rb.Statements() {
			for _, c := range st.Connects {
				out[stated{kind: st.Kind, conn: c}] = true
			}
		}
		return out
	}
	sharedAll := everywhere(shared)
	for w := range everywhere(copied) {
		if w.conn.Role == "said-to" || w.conn.Role == "done-to" {
			continue
		}
		require.True(t, sharedAll[w], "the copies state %s; the shared form states it nowhere", w)
	}

	for conv, carriers := range offsets {
		require.Len(t, carriers, 3, "three creatures carry %s", conv)
		var head *readback.Statement
		for _, st := range shared.Statements() {
			if st.Kind == readback.KindSharedLines && st.Path == "conversations."+conv {
				head = &st
			}
		}
		require.NotNil(t, head, "the conversation %s is said", conv)
		var lines []any
		for _, st := range shared.Statements() {
			if st.Kind == readback.KindLine && strings.HasPrefix(st.Path, "conversations."+conv+".lines[") {
				lines = append(lines, st.Path)
			}
		}
		require.NotEmpty(t, lines)
		for carrier, offset := range carriers {
			require.Contains(t, head.Connects, readback.Connection{Kind: "creature", ID: carrier, Role: "carries"})
			for k := range lines {
				sharedAt := fmt.Sprintf("conversations.%s.lines[%d]", conv, k)
				copyAt := fmt.Sprintf("room.room.monsterBindings.%s.options[%d]", carrier, offset+k)
				want := statedAt(copied, copyAt)
				require.NotEmpty(t, want, "the copy at %s states something", copyAt)
				got := statedAt(shared, sharedAt)
				for w := range want {
					if (w.conn.Role == "said-to" || w.conn.Role == "done-to") && w.conn.Kind == "creature" && w.conn.ID == carrier {
						w.conn = readback.Connection{Kind: readback.ConnSharedLines, ID: conv, Role: w.conn.Role}
					}
					require.True(t, got[w], "%s's copy of %s states %s; the line written once at %s does not",
						carrier, sharedAt, w, sharedAt)
				}
			}
		}
	}
}
