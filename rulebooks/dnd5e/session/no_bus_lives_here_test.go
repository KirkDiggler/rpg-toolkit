// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// busPath is the package that hands out event buses. Matched by import PATH,
// so an alias or a dot-import cannot walk one in under another name.
const busPath = "github.com/KirkDiggler/rpg-toolkit/events"

// gamectxPath is the package that installs game context, matched the same way.
const gamectxPath = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"

// TestNoBusLivesInThisModule is the guarantee the tripwire it replaces could
// only gesture at.
//
// # What the old pin could and could not do
//
// TestNoFoldLivesInThisModule matched two literal spellings across non-test
// files: an import of gamectx, and the name EffectiveAC. It caught the
// regressions it was built for and its own doc said what it could not do —
// wrap the same effect in a FoldedAC method and it was blind by construction,
// because it matched a spelling rather than an effect. It called itself a fence
// around the paths that had actually been walked, and pointed here:
//
//	"When this module holds no bus and no attach machinery at all, a fold
//	cannot run in it whatever anybody calls the method, and the guarantee
//	stops depending on a name."
//
// That is now true, so this is that test. A fold needs a bus. Every route to a
// bus — creating one, being handed one, attaching a sheet to one — begins with
// importing the package that defines it. No import, no bus; no bus, no fold,
// whatever anybody calls the method.
//
// # Test files included, and that is the strong form
//
// The pin this replaces scanned non-test files only. Including tests would have
// been impossible then and costs nothing now: NOTHING in this module imports
// either package, production or test, and that was measured before the rule was
// written rather than hoped for afterwards.
//
// It matters more than the tidiness suggests. rpg-toolkit#1251 is the case where
// every TEST installed what production never installed, and a registry looked
// wired for months because the only code that built one was the code checking
// it. A pin that exempts tests is blind to exactly that, so this one does not.
//
// # What it still cannot say
//
// It does not prove no rule lives in this package — a rule needs no bus to be
// misplaced. It proves this package cannot fold a chain, which is the specific
// claim the game-context slice makes about it, and the reason it is named for
// the bus rather than for folds.
func TestNoBusLivesInThisModule(t *testing.T) {
	fset := token.NewFileSet()

	var buses, contexts []string
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// Walked recursively: a helper subpackage that grew its own bus is
			// exactly the innocent regression a single-directory scan waves
			// through, and the escape an earlier review built by hand.
			if path != "." && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata") {
				return fs.SkipDir
			}

			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}

		for _, spec := range file.Imports {
			switch spec.Path.Value {
			case `"` + busPath + `"`:
				buses = append(buses, fset.Position(spec.Pos()).String())
			case `"` + gamectxPath + `"`:
				contexts = append(contexts, fset.Position(spec.Pos()).String())
			}
		}

		return nil
	})
	require.NoError(t, err)

	require.Empty(t, buses,
		"a file in this module imports the events package. This seam loads no sheets and "+
			"folds no chains: it hands records to resolution and takes back answers. A bus "+
			"here is the machinery for doing otherwise, and the last three verbs that held "+
			"one were the ones this slice rerouted. Ask resolution instead")

	require.Empty(t, contexts,
		"a file in this module imports gamectx. This seam holds RECORDS, not truth: a live "+
			"room and a live cast never exist above resolution, and a seam that installs one "+
			"has already broken its own law that no runtime object crosses the boundary")
}

// TestTheStandingSeamDoesNotRunAnInteraction is the half of the reentrancy
// question this module can honestly answer.
//
// # The question
//
// The standing seam is a capability the COMPOSITION calls, and the composition
// runs inside a resolution. So the seam can be entered while a resolution is in
// flight — and since the slice that added this, the seam itself calls
// resolution. If it called an entry that ran an interaction, a consult could
// open a second interaction inside the first: two casts, two buses, one of them
// mid-flight. R7 is about exactly that, and it is the law this checks against.
//
// It does not happen, and this is why: the seam reaches Standing, which folds
// nothing and drives no machine. Asserted rather than described, by naming the
// entries that DO run one and requiring the seam to name none of them.
//
// # What this cannot say, stated so the pin does not oversell itself
//
// The other half of the invariant lives in resolution: that Resolve touches the
// encounter only through LoadEncounter, Canvas and ToData, and so never reaches
// the composition's own standing consult. That is a fact about resolution's
// source, and this module cannot honestly check it — session compiles against a
// PUBLISHED resolution from the module cache, so a scan of the sibling
// directory on disk would be pinning a different artifact than the one this
// code runs. It belongs in resolution's own suite, where the scan and the code
// are the same thing. Until it lands there, that half is held by a comment
// (resolution's ErrNoStanding doc) and by measurement, not by a test.
func TestTheStandingSeamDoesNotRunAnInteraction(t *testing.T) {
	// The entries that stand an interaction up. Reaching any of these from the
	// standing seam is the reentrancy R7 forbids.
	interactionEntries := []string{"resolution.Resolve", "resolution.ProjectCharacter"}

	source, err := os.ReadFile("standing.go")
	require.NoError(t, err)

	// Asserted as a BOOL rather than with NotContains, so a failure names the
	// entry instead of dumping the file it was found in. A pin whose output has
	// to be scrolled past is a pin somebody learns to ignore.
	for _, entry := range interactionEntries {
		require.False(t, strings.Contains(string(source), entry),
			"standing.go names %s. This seam answers a question the composition asks WHILE "+
				"a resolution may be running, so an entry that runs an interaction opens a "+
				"second one inside the first — two casts, two buses, one of them mid-flight "+
				"(R7). Standing folds nothing and drives no machine, which is why it is the "+
				"entry this seam may reach", entry)
	}
}
