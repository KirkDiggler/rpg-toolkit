// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// The host supplies exclusion, but the SDK owns the entire operation lifetime.
// Enumerate the real public surface so a newly added verb cannot quietly bypass it.
type SessionLockCoverageSuite struct{ suite.Suite }

func TestSessionLockCoverageSuite(t *testing.T) {
	suite.Run(t, new(SessionLockCoverageSuite))
}

func (s *SessionLockCoverageSuite) TestEverySessionOperationAcquiresAndDefersRelease() {
	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	s.Require().NoError(err)
	methods := map[string]*ast.FuncDecl{}
	for _, file := range pkgs["session"].Files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil {
				continue
			}
			star, ok := fn.Recv.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			name, ok := star.X.(*ast.Ident)
			if ok && name.Name == "Manager" {
				methods[fn.Name.Name] = fn
			}
		}
	}
	typ := reflect.TypeOf((*session.Manager)(nil))
	checked := 0
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if method.Type.NumIn() != 3 || method.Type.In(2).Kind() != reflect.Pointer {
			continue
		}
		input := method.Type.In(2).Elem()
		if input.Kind() != reflect.Struct {
			continue
		}
		if field, ok := input.FieldByName("Session"); !ok || field.Type.Kind() != reflect.String {
			continue
		}
		checked++
		s.Run(method.Name, func() {
			fn := methods[method.Name]
			s.Require().NotNil(fn)
			acquires, releases := 0, 0
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok {
					if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "acquireSession" {
						acquires++
					}
				}
				if deferred, ok := node.(*ast.DeferStmt); ok {
					if name, ok := deferred.Call.Fun.(*ast.Ident); ok && name.Name == "release" {
						releases++
					}
				}
				return true
			})
			s.Equal(1, acquires, "acquire once at the public operation boundary")
			s.Equal(1, releases, "release on every return, not merely on commit")
		})
	}
	s.Positive(checked)
}
