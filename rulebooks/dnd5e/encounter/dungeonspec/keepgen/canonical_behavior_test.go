// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type generatedBehaviorSuite struct{ suite.Suite }

func TestGeneratedBehaviorSuite(t *testing.T) { suite.Run(t, new(generatedBehaviorSuite)) }

func (s *generatedBehaviorSuite) TestRoutesBindOneSharedDefinition() {
	doc, _, err := Generate(DefaultPlan(), garrisonOptions(5, 2))
	s.Require().NoError(err)
	compiled, err := Prove(doc)
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(compiled.Monsters), 5)
	first, second := compiled.Monsters[0].Behaviors, compiled.Monsters[1].Behaviors
	s.Require().NotNil(first)
	s.Require().NotNil(second)
	s.Equal("duty", first.Instances[0].Instance.ID)
	s.Equal(first.Instances[0].Instance.Recipe.Ref, second.Instances[0].Instance.Recipe.Ref)
	s.NotEqual(first.Instances[0].Instance.Table, second.Instances[0].Instance.Table, "different supplied routes, not different behavior factories")
	s.Contains(string(doc), "use: patrol")
	s.NotContains(string(doc), "table: patrol-")
	s.NotEmpty(first.Instances[0].Completions)
	s.True(compiled.Monsters[2].Behaviors.Instances[0].Instance.Table["hold"][0].Hold)
}

func (s *generatedBehaviorSuite) TestListeningPostIsExplicitData() {
	options := garrisonOptions(3, 1)
	options.PostsListen = true
	doc, _, err := Generate(DefaultPlan(), options)
	s.Require().NoError(err)
	compiled, err := Prove(doc)
	s.Require().NoError(err)
	post := compiled.Monsters[1].Behaviors.Instances[0].Instance
	s.Equal("posted", post.Recipe.Initial)
	s.Equal("heard", string(post.Recipe.States["posted"].Transitions[0].On))
	s.Equal(6, post.Recipe.States["posted"].Transitions[0].When.Scope.Within)
	s.Contains(string(doc), "use: listening-post")
}
