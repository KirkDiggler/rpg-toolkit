// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"flag"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestMain starts the one launched 10k keep the package reads
// ([launchedKeep10k]) beside the serial tests when the run selects a test
// that reads it: the launch is a whole garrison's first look, most of a
// minute under -race, and on its own goroutine it costs the run's wall clock
// nothing the serial tests were not already spending. A run that selects no
// such test (a single test, the benchmarks) never launches it.
func TestMain(m *testing.M) {
	flag.Parse()
	if run := flag.Lookup("test.run"); run != nil && selects(run.Value.String(), launchedKeepReaders...) {
		go launchKeep10k()
	}
	os.Exit(m.Run())
}

// launchedKeepReaders are the tests that read [launchedKeep10k].
var launchedKeepReaders = []string{"TestBlobSize_Keep10k"}

// selects reports whether a -run pattern selects any of the top-level tests
// named; an empty pattern selects every test. A pattern that does not compile
// selects nothing here, and the test run reports it.
func selects(pattern string, names ...string) bool {
	if pattern == "" {
		return true
	}
	re, err := regexp.Compile(firstLevel(pattern))
	if err != nil {
		return false
	}
	for _, name := range names {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// firstLevel is a -run pattern's top-level part: -run splits on unbracketed
// slashes, one element per level of subtest.
func firstLevel(pattern string) string {
	depth := 0
	for i, r := range pattern {
		switch r {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case '/':
			if depth == 0 {
				return pattern[:i]
			}
		}
	}
	return pattern
}

var (
	launchedKeepOnce sync.Once
	launchedKeepEnc  *encounter.Encounter
	launchedKeepErr  error
)

// launchKeep10k launches the 10k keep once; a caller that arrives while the
// launch runs waits for it.
func launchKeep10k() {
	launchedKeepOnce.Do(func() {
		name, read := keepDocument(10000)
		compiled, err := compiledOnceErr(name, read)
		if err != nil {
			launchedKeepErr = err
			return
		}
		launchedKeepEnc, launchedKeepErr = launchSiteErr(compiled)
	})
}

// launchedKeep10k is the 10k keep after the session's launch — every seat and
// every monster boarded and looking — shared by the tests that only read it.
func launchedKeep10k(tb testing.TB) *encounter.Encounter {
	tb.Helper()
	launchKeep10k()
	require.NoError(tb, launchedKeepErr, "the keep launched")

	return launchedKeepEnc
}
