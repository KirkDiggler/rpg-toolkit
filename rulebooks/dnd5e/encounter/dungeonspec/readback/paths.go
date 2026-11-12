// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// paths.go resolves a path in the compiler's refusal form —
// `room.room.monsterBindings.gate-1.options[0].requires[1]` — against the
// authored document, so a statement can name the place exactly as written
// (a `requires:` written as one mapping has no index).

// document is the authored file as a YAML tree.
type document struct{ root *yaml.Node }

// newDocument parses raw; a document that will not parse resolves nothing.
func newDocument(raw []byte) document {
	var n yaml.Node
	if err := yaml.Unmarshal(raw, &n); err != nil || len(n.Content) == 0 {
		return document{}
	}
	return document{root: n.Content[0]}
}

// splitPath breaks a path into keys and indexes: "a.b[2].c" → a, b, [2], c.
func splitPath(path string) []string {
	var out []string
	for _, part := range strings.Split(path, ".") {
		for part != "" {
			i := strings.IndexByte(part, '[')
			if i < 0 {
				out = append(out, part)
				break
			}
			if i > 0 {
				out = append(out, part[:i])
			}
			j := strings.IndexByte(part, ']')
			if j < i {
				out = append(out, part)
				break
			}
			out = append(out, part[i:j+1])
			part = part[j+1:]
		}
	}
	return out
}

// node is the authored node at path, or nil.
func (d document) node(path string) *yaml.Node {
	n := d.root
	for _, step := range splitPath(path) {
		if n == nil {
			return nil
		}
		if strings.HasPrefix(step, "[") {
			i, err := strconv.Atoi(strings.Trim(step, "[]"))
			if err != nil || n.Kind != yaml.SequenceNode || i < 0 || i >= len(n.Content) {
				return nil
			}
			n = n.Content[i]
			continue
		}
		if n.Kind != yaml.MappingNode {
			return nil
		}
		var next *yaml.Node
		for k := 0; k+1 < len(n.Content); k += 2 {
			if n.Content[k].Value == step {
				next = n.Content[k+1]
				break
			}
		}
		n = next
	}
	return n
}

// item is the path of the i-th entry of a list written at path, or path
// itself when the author wrote the one entry without a list.
func (d document) item(path string, i int) string {
	if n := d.node(path); n != nil && n.Kind != yaml.SequenceNode {
		return path
	}
	return path + "[" + strconv.Itoa(i) + "]"
}
