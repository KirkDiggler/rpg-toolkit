// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

// bundles.go is THE NESTED TABLE'S NAMES (rpg-project#670, R12–R15): a
// behaviour authored once under `bundles:` and used as one row, `use: <id>`,
// wherever a table wants it.
//
//	bundles:
//	  pursuer:
//	    params: { distance: 6, heading: went }
//	    rows:
//	      - { when: { enemy: remembered }, toward: enemy }
//	      - { when: { enemy: lost }, search: { distance: $distance, heading: $heading } }
//	tables:
//	  gate-patrol:
//	    time:
//	      - use: pursuer
//	        with: { distance: 8 }
//
// THE NAME IS THIS DIALECT'S AND NOBODY ELSE'S. [resolveBundles] turns every
// `use:` row into a group whose rows are the bundle's, parameters
// substituted, before the grammar or the compiler reads it; mind/behavior
// holds the tree and never looks a bundle up, and neither does the engine.
//
// A PARAMETER IS A WHOLE SCALAR. `$distance` in a bundle's rows is replaced
// by the use's value for it, node for node, so a parameter can be a number,
// a word or a whole route. A `$` inside a longer string is not a parameter.

// BundleSpec is one named group of rows: the parameters it takes, each with
// its default, and the rows.
//
//	pursuer:
//	  params: { distance: 6, heading: went }
//	  rows: [ … ]
//
// A PARAMETER WITH NO DEFAULT is written with a null (`path: ~`): a use must
// then give it, and one that does not is refused naming it.
type BundleSpec struct {
	// Params is every parameter this bundle declares, to its default, or
	// to a null node when it has none.
	Params map[string]yaml.Node `yaml:"params,omitempty" json:"params,omitempty"`

	// Rows is the bundle's rows as authored, KEPT AS THE NODE they were
	// written as so each use can substitute its parameters before they are
	// read. REQUIRED and never empty: a group is a table, and a table with no
	// rows can never be picked.
	Rows yaml.Node `yaml:"rows" json:"-"`
}

// The sentences a bundle and a `use:` row are refused with, for the form.
const (
	errUseNoBundle    = "`use` names no bundle"
	errBundleNoRows   = "a bundle holds no rows; a group is a table, and a table with no rows can never be picked"
	errWithWithoutUse = "`with` gives a bundle its parameters, and this row uses no bundle"
	errAsWithoutUse   = "`as` is the word a group counts as, and this row uses no bundle"
)

// bundleResolver resolves `use:` rows against one document's bundles,
// reporting every refusal at the row that made it.
type bundleResolver struct {
	bundles map[string]BundleSpec
	add     errSink
}

// resolveBundles resolves every `use:` row in a single-room document: its
// root tables, every binding's own `on:` and options, every prop binding's
// options and every faction's orders and options. It also judges each bundle once, at its
// own path: rows written, and no `$name` it does not declare.
//
// IDEMPOTENT. Validation and compile both run it, and each run writes the
// same resolved rows over the last, so neither depends on the other having
// run first.
func resolveBundles(s *SingleRoomSpec, add errSink) {
	r := bundleResolver{bundles: s.Bundles, add: add}
	for _, id := range sortedBundleIDs(s.Bundles) {
		r.bundle(id)
	}
	for _, id := range sortedTableIDs(s.Tables) {
		r.on("tables."+id, s.Tables[id])
	}
	for i := range s.Factions {
		r.on(fmt.Sprintf("factions[%d]", i), s.Factions[i].On)
		r.options(fmt.Sprintf("factions[%d]", i), s.Factions[i].Options)
	}
	gp := &s.Room.Gameplay
	for _, id := range sortedBindingIDs(gp.MonsterBindings) {
		b := gp.MonsterBindings[id]
		at := "room.room.monsterBindings." + id
		r.on(at, b.On)
		r.options(at, b.Options)
	}
	for _, id := range sortedPropBindingIDs(gp.PropBindings) {
		r.options("room.room.propBindings."+id, gp.PropBindings[id].Options)
	}
}

// on resolves every list of one `on:`-shaped mapping, by key.
func (r bundleResolver) on(path string, on map[string][]AnswerSpec) {
	for _, key := range sortedKeys(on) {
		r.rows(path+".on."+key, on[key], nil)
	}
}

// options resolves both verdicts' rows of every option.
func (r bundleResolver) options(path string, options []OptionSpec) {
	for i := range options {
		at := fmt.Sprintf("%s.options[%d]", path, i)
		r.rows(at+".success", options[i].Success, nil)
		r.rows(at+".failure", options[i].Failure, nil)
	}
}

// rows resolves every `use:` row of one list in place. stack is the bundles
// being resolved around this list, outermost first.
func (r bundleResolver) rows(path string, rows []AnswerSpec, stack []string) {
	for j := range rows {
		if rows[j].Use == nil {
			continue
		}
		r.use(fmt.Sprintf("%s[%d]", path, j), &rows[j], stack)
	}
}

// use resolves one `use:` row: the bundle it names, the parameters it gives,
// the rows those make, and the uses inside them.
func (r bundleResolver) use(at string, entry *AnswerSpec, stack []string) {
	entry.Rows, entry.From, entry.Params = nil, "", nil
	name := *entry.Use
	bundle, declared := r.bundles[name]
	switch {
	case name == "":
		r.add(at+".use", errUseNoBundle)

		return
	case !declared:
		r.add(at+".use", fmt.Sprintf("`use: %s` names no bundle in this file: %s", name, r.declared()))

		return
	case containsString(stack, name):
		r.add(at+".use", fmt.Sprintf("bundle %q uses itself: %s → %s; a group cannot hold the table it is in",
			name, strings.Join(stack, " → "), name))

		return
	case len(stack)+1 > behavior.MaxGroupDepth:
		r.add(at+".use", fmt.Sprintf("`use: %s` is a group nested %d deep; groups nest at most %d deep",
			name, len(stack)+1, behavior.MaxGroupDepth))

		return
	}

	values, ok := r.params(at, name, bundle, entry.With)
	if !ok {
		return
	}
	if !nonEmptySequence(&bundle.Rows) {
		return // refused once, at the bundle's own path ([bundleResolver.bundle])
	}
	node := substitute(&bundle.Rows, values)
	var rows []AnswerSpec
	if err := node.Decode(&rows); err != nil {
		r.add(at+".use", fmt.Sprintf("bundle %q does not read with these parameters: %s", name, err.Error()))

		return
	}
	r.rows(at+".rows", rows, append(append([]string(nil), stack...), name))
	entry.Rows, entry.From, entry.Params = rows, name, rendered(values)
}

// params is every parameter's value for one use, defaults filled, refusing a
// name the bundle does not declare and a declared one with no default that
// the use does not give. ok is false when anything was refused.
func (r bundleResolver) params(
	at, name string, bundle BundleSpec, with map[string]yaml.Node,
) (map[string]*yaml.Node, bool) {
	ok := true
	for _, given := range sortedNodeKeys(with) {
		if _, declared := bundle.Params[given]; !declared {
			r.add(at+".with."+given, fmt.Sprintf("bundle %q takes no parameter `%s`: %s",
				name, given, paramList(bundle)))
			ok = false
		}
	}
	values := make(map[string]*yaml.Node, len(bundle.Params))
	for _, param := range sortedNodeKeys(bundle.Params) {
		if given, written := with[param]; written {
			values[param] = &given
			continue
		}
		def := bundle.Params[param]
		if def.Kind == 0 || def.Tag == "!!null" {
			r.add(at+".with", fmt.Sprintf("bundle %q needs `%s`: it has no default, so the use gives one",
				name, param))
			ok = false
			continue
		}
		values[param] = &def
	}

	return values, ok
}

// bundle judges one bundle at its own path: it has rows, and every `$name`
// in them is a parameter it declares.
func (r bundleResolver) bundle(id string) {
	b := r.bundles[id]
	at := "bundles." + id
	if !nonEmptySequence(&b.Rows) {
		r.add(at+".rows", errBundleNoRows)

		return
	}
	for _, ref := range parameterRefs(&b.Rows) {
		if _, declared := b.Params[ref.name]; !declared {
			r.add(at+".rows", fmt.Sprintf("`$%s` is not a parameter bundle %q declares (line %d): %s",
				ref.name, id, ref.line, paramList(b)))
		}
	}
}

// declared names the bundles a refusal may point at.
func (r bundleResolver) declared() string {
	ids := sortedBundleIDs(r.bundles)
	if len(ids) == 0 {
		return "it declares none"
	}

	return "it declares " + strings.Join(ids, ", ")
}

// paramList names a bundle's parameters for a refusal.
func paramList(b BundleSpec) string {
	names := sortedNodeKeys(b.Params)
	if len(names) == 0 {
		return "it takes none"
	}

	return "it takes " + strings.Join(names, ", ")
}

// paramRef is one `$name` written in a bundle's rows, and its line.
type paramRef struct {
	name string
	line int
}

// parameterRefs is every whole-scalar `$name` under a node.
func parameterRefs(n *yaml.Node) []paramRef {
	var out []paramRef
	if n.Kind == yaml.ScalarNode {
		if name, ok := parameterName(n); ok {
			out = append(out, paramRef{name: name, line: n.Line})
		}
	}
	for _, c := range n.Content {
		out = append(out, parameterRefs(c)...)
	}

	return out
}

// parameterName is the parameter a scalar names, when it is one: a plain
// `$` followed by the name, and nothing else.
func parameterName(n *yaml.Node) (string, bool) {
	if n.Kind != yaml.ScalarNode || n.Style != 0 || len(n.Value) < 2 || n.Value[0] != '$' {
		return "", false
	}

	return n.Value[1:], true
}

// substitute is a deep copy of n with every parameter scalar replaced by a
// deep copy of its value. A `$name` with no value is left as written; the
// bundle's own judgement has already refused it.
func substitute(n *yaml.Node, values map[string]*yaml.Node) *yaml.Node {
	if name, ok := parameterName(n); ok {
		if v, given := values[name]; given {
			out := deepCopy(v)
			out.Line, out.Column = n.Line, n.Column

			return out
		}
	}
	out := *n
	out.Content = nil
	for _, c := range n.Content {
		out.Content = append(out.Content, substitute(c, values))
	}

	return &out
}

// deepCopy copies a node and everything under it.
func deepCopy(n *yaml.Node) *yaml.Node {
	out := *n
	out.Content = nil
	for _, c := range n.Content {
		out.Content = append(out.Content, deepCopy(c))
	}

	return &out
}

// rendered is each parameter's value as the record shows it: a scalar as
// written, anything else in YAML's flow form, `[{ q: -5, r: 8 }, …]`.
func rendered(values map[string]*yaml.Node) map[string]string {
	out := make(map[string]string, len(values))
	for name, v := range values {
		if v.Kind == yaml.ScalarNode {
			out[name] = v.Value
			continue
		}
		flow := deepCopy(v)
		flowStyle(flow)
		raw, err := yaml.Marshal(flow)
		if err != nil {
			out[name] = ""
			continue
		}
		out[name] = strings.TrimSpace(string(raw))
	}

	return out
}

// flowStyle sets a node and everything under it to YAML's flow style.
func flowStyle(n *yaml.Node) {
	if n.Kind == yaml.MappingNode || n.Kind == yaml.SequenceNode {
		n.Style = yaml.FlowStyle
	}
	n.HeadComment, n.LineComment, n.FootComment = "", "", ""
	for _, c := range n.Content {
		flowStyle(c)
	}
}

// nonEmptySequence reports whether a bundle's rows are a list with something
// in it.
func nonEmptySequence(n *yaml.Node) bool {
	return n.Kind == yaml.SequenceNode && len(n.Content) > 0
}

// containsString reports whether s holds v.
func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}

	return false
}

// factsIn is every fact id the given rows can teach, one group down and
// further included, plus the given ids (what a record reveals): the names a
// `fact` pattern may match (rpg-project#684 L3).
func factsIn(lists [][]AnswerSpec, revealed ...string) map[string]bool {
	out := map[string]bool{}
	for _, id := range revealed {
		if id != "" {
			out[id] = true
		}
	}
	var walk func(rows []AnswerSpec)
	walk = func(rows []AnswerSpec) {
		for _, row := range rows {
			if row.Fact != nil && *row.Fact != "" {
				out[*row.Fact] = true
			}
			walk(row.Rows)
		}
	}
	for _, rows := range lists {
		walk(rows)
	}

	return out
}

// onLists is every row list of an `on:`-shaped mapping.
func onLists(on map[string][]AnswerSpec) [][]AnswerSpec {
	out := make([][]AnswerSpec, 0, len(on))
	for _, key := range sortedKeys(on) {
		out = append(out, on[key])
	}

	return out
}

// optionLists is both verdicts' rows of every option.
func optionLists(options []OptionSpec) [][]AnswerSpec {
	out := make([][]AnswerSpec, 0, 2*len(options))
	for _, o := range options {
		out = append(out, o.Success, o.Failure)
	}

	return out
}

// roomFacts is every fact a single-room document can set, read after its
// bundles are resolved: its tables', factions' (orders and lines), bindings'
// and options' rows, and its records' reveals.
func roomFacts(s *SingleRoomSpec) map[string]bool {
	var lists [][]AnswerSpec
	for _, id := range sortedTableIDs(s.Tables) {
		lists = append(lists, onLists(s.Tables[id])...)
	}
	for _, f := range s.Factions {
		lists = append(lists, onLists(f.On)...)
		lists = append(lists, optionLists(f.Options)...)
	}
	gp := &s.Room.Gameplay
	for _, id := range sortedBindingIDs(gp.MonsterBindings) {
		lists = append(lists, onLists(gp.MonsterBindings[id].On)...)
		lists = append(lists, optionLists(gp.MonsterBindings[id].Options)...)
	}
	for _, id := range sortedPropBindingIDs(gp.PropBindings) {
		lists = append(lists, optionLists(gp.PropBindings[id].Options)...)
	}
	revealed := make([]string, 0, len(s.Intel))
	for _, rec := range s.Intel {
		revealed = append(revealed, rec.Reveals.Fact)
	}

	return factsIn(lists, revealed...)
}

// specFacts is [roomFacts] for a version 2 document: its placements' and
// factions' rows and lines, its placements' options, and its records'
// reveals.
func specFacts(spec *Spec) map[string]bool {
	var lists [][]AnswerSpec
	for _, p := range spec.Place {
		lists = append(lists, onLists(p.On)...)
		lists = append(lists, optionLists(p.Options)...)
	}
	for _, f := range spec.Factions {
		lists = append(lists, onLists(f.On)...)
		lists = append(lists, optionLists(f.Options)...)
	}
	revealed := make([]string, 0, len(spec.Intel))
	for _, rec := range spec.Intel {
		revealed = append(revealed, rec.Reveals.Fact)
	}

	return factsIn(lists, revealed...)
}

// sortedBundleIDs orders the bundles' ids, for [sortedTableIDs]' reason.
func sortedBundleIDs(bundles map[string]BundleSpec) []string {
	out := make([]string, 0, len(bundles))
	for id := range bundles {
		out = append(out, id)
	}
	sort.Strings(out)

	return out
}

// sortedNodeKeys orders a parameter map's names.
func sortedNodeKeys(m map[string]yaml.Node) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)

	return out
}
