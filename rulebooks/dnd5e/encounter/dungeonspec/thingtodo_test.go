// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// thingtodo_test.go is the authored half of SOMETHING DONE TO A THING
// (rpg-project#959): an option's `do:` and `seen:`, compiled from the
// castle's grate and altar, and refused on the small tomb vault at the
// author's own path.

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// vaultRead is the scroll's spoken line as the vault fixture writes it.
const vaultRead = "            say: (read it aloud)\n"

// vaultAsk is the captain's spoken line as the vault fixture writes it.
const vaultAsk = "            say: Where?\n"

func TestDungeonspec_AThingToDoCompiles(t *testing.T) {
	c := loadSource(t, castleSource(t))
	found := 0
	for _, p := range c.Field.Placed {
		switch p.ID {
		case "grate":
			found++
			require.Len(t, p.Options, 2)
			for _, o := range p.Options {
				require.Empty(t, o.Say, "%s is done, not said", o.ID)
				require.NotEmpty(t, o.Do)
				require.Equal(t, "lifts the grate by the hearth and climbs down into the dark", o.Seen, "the seen words name no destination")
			}
		case "altar":
			found++
			require.Len(t, p.Options, 1)
			require.Equal(t, "You examine the altar.", p.Options[0].Do)
			require.Equal(t, "looks the altar over", p.Options[0].Seen)
		}
	}
	require.Equal(t, 2, found, "the grate and the altar are on the floor")
	// And the small vault takes a thing to do on its scroll.
	_ = loadSource(t, vaultWithAPassage(t, vaultRead, "            do: You read it.\n            seen: reads the scroll\n"))
}

func TestDungeonspec_AThingToDoIsRefusedWhereItIsWrong(t *testing.T) {
	const scroll = "room.room.propBindings.hall-scroll.options[0]"
	const captain = "room.room.monsterBindings.captain.options[0]"
	for name, tc := range map[string]struct {
		old, replacement, path, message string
	}{
		"said and done": {vaultRead, vaultRead + "            do: You read it.\n",
			scroll + ".do", "an option is said or done, not both: write `say` or `do`"},
		"neither": {vaultRead, "",
			scroll + ".say", "the option does not say what the character says, or what they do"},
		"done to a creature": {vaultAsk, "            do: You shake him.\n",
			captain + ".do", "something done is done to a thing: a creature is spoken to, so its option says"},
		"done loudly": {vaultRead, "            do: You read it.\n            loudness: shout\n",
			scroll + ".loudness", "something done is seen, not heard: it has no loudness"},
		"seen beside a line": {vaultRead, vaultRead + "            seen: reads the scroll\n",
			scroll + ".seen", "`seen` is what a witness sees done: it goes beside `do`, not `say`"},
	} {
		t.Run(name, func(t *testing.T) {
			requireExactDefect(t, refusals(t, vaultWithAPassage(t, tc.old, tc.replacement)), tc.path, tc.message)
		})
	}
}
