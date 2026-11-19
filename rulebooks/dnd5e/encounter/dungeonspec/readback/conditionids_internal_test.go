// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

// conditionids_internal_test.go holds every condition connection the
// read-back emits to the machine words table (rpg-project#997): its id
// resolves through [encounter.SplitConditionWord] to a table word, and that
// word's Args accept its open part, its value and its comparison. A reader
// of the wire finds every condition's label, args and kind.

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// requireAccepted holds c to the table word its id resolves to: the word
// takes its open part, its value and its comparison.
func requireAccepted(t *testing.T, where string, c Connection) {
	t.Helper()
	w, open, ok := encounter.SplitConditionWord(c.ID)
	require.True(t, ok, "%s: %q resolves to a table word", where, c.ID)
	arg := func(name string) (encounter.Arg, bool) {
		i := slices.IndexFunc(w.Args, func(a encounter.Arg) bool { return a.Name == name })
		if i < 0 {
			return encounter.Arg{}, false
		}
		return w.Args[i], true
	}
	takes := func(a encounter.Arg, v string) bool {
		if a.Fixed != "" {
			return v == a.Fixed
		}
		if len(a.Values) > 0 {
			return slices.Contains(a.Values, v)
		}
		switch a.Kind {
		case "int":
			_, err := strconv.Atoi(v)
			return err == nil
		case "bool":
			_, err := strconv.ParseBool(v)
			return err == nil
		}
		return v != ""
	}
	if w.Open == encounter.SlotNone {
		require.Equal(t, w.Word, c.ID, "%s: a whole word is its own id", where)
	} else {
		require.True(t, takes(w.Args[0], open), "%s: %q's open part %q is one its first argument takes", where, w.Word, open)
	}
	value, hasValue := arg("value")
	if !hasValue {
		value, hasValue = arg("n")
	}
	if hasValue {
		require.True(t, takes(value, c.Is), "%s: %q takes the value %q", where, w.Word, c.Is)
	} else {
		require.Empty(t, c.Is, "%s: %q takes no value", where, w.Word)
	}
	op := c.Op
	if op == "" {
		op = string(machine.OpEq)
	}
	if a, ok := arg("op"); ok {
		require.True(t, takes(a, op), "%s: %q compares with %q", where, w.Word, op)
	} else {
		require.Equal(t, string(machine.OpEq), op, "%s: %q takes no comparison, so it is equality", where, w.Word)
	}
}

// TestEveryConditionIDResolves: for one sample of every table word and for
// every content file the read-back says, each condition connection is
// accepted by its table word.
func TestEveryConditionIDResolves(t *testing.T) {
	for _, w := range encounter.MachineWords().Conditions {
		sample, ok := conditionSamples[w.Word]
		require.True(t, ok, "a sample for %s", w.Word)
		_, conns := guardWords(sample.g, "X", "heard")
		for _, c := range conditionsOf(conns) {
			requireAccepted(t, "sample "+w.Word, c)
		}
	}

	files, err := filepath.Glob(filepath.Join("..", "testdata", "*.yaml"))
	require.NoError(t, err)
	more, err := filepath.Glob(filepath.Join("testdata", "*.yaml"))
	require.NoError(t, err)
	seen := 0
	for _, f := range append(files, more...) {
		if filepath.Base(f) == "world-builder-v4-castle.yaml" {
			continue // the shared castle's own test, below the package line
		}
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		if !bytes.Contains(raw, []byte("transitions")) {
			continue // no step, so no condition
		}
		rb, err := Of(raw)
		if err != nil {
			continue // a refused file says no conditions
		}
		for _, st := range rb.Statements() {
			for _, c := range conditionsOf(st.Connects) {
				requireAccepted(t, filepath.Base(f)+" "+st.Path, c)
				seen++
			}
		}
	}
	require.Positive(t, seen)
}

// RequireConditionAccepted and ConditionsOf are this file's checks, for the
// castle test that reads the shared castle (conditions_test.go).
var (
	RequireConditionAccepted = requireAccepted
	ConditionsOf             = conditionsOf
)
