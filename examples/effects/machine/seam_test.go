// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

const toolkit = "github.com/KirkDiggler/rpg-toolkit/"

// machineAllows is the seam of design E9 for the machine: the standard
// library, core, core/chain and events.
func machineAllows(path string) bool {
	if !strings.Contains(strings.Split(path, "/")[0], ".") {
		return true // standard library
	}
	switch path {
	case toolkit + "core", toolkit + "core/chain", toolkit + "events":
		return true
	}
	return false
}

// importsOutside parses every non-test .go file in dir (imports only) and
// returns "file: path" for each import the allow-list refuses.
func importsOutside(dir string, allows func(string) bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var refused []string
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ImportsOnly)
		if err != nil {
			return nil, err
		}
		for _, imp := range f.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				return nil, err
			}
			if !allows(path) {
				refused = append(refused, name+": "+path)
			}
		}
	}
	return refused, nil
}

type seamSuite struct {
	suite.Suite
}

func (s *seamSuite) TestPackageImportsAreAllowed() {
	refused, err := importsOutside(".", machineAllows)
	s.Require().NoError(err)
	s.Empty(refused, "machine imports outside its seam (design E9)")
}

// The check itself must be able to fail: a file importing a rulebook is
// named with its path, and a standard-library-only file passes.
func (s *seamSuite) TestCheckIsRedOnARulebookImport() {
	dir := s.T().TempDir()
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "ok.go"),
		[]byte("package x\n\nimport \"fmt\"\n\nvar _ = fmt.Sprint\n"), 0o600))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "bad.go"),
		[]byte("package x\n\nimport _ \""+toolkit+"rulebooks/dnd5e\"\n"), 0o600))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, "bad_test.go"),
		[]byte("package x\n\nimport _ \""+toolkit+"rulebooks/dnd5e\"\n"), 0o600))

	refused, err := importsOutside(dir, machineAllows)
	s.Require().NoError(err)
	s.Equal([]string{"bad.go: " + toolkit + "rulebooks/dnd5e"}, refused)
}

func (s *seamSuite) TestComposerIsOutsideTheMachineSeam() {
	s.False(machineAllows(toolkit + "examples/effects"))
}

func TestImportsStayInsideTheSeam(t *testing.T) {
	suite.Run(t, new(seamSuite))
}
