// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"runtime/debug"
	"sync"
)

// encounterModule is this module's path, as a build lists its version.
const encounterModule = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"

// engineVersion is this module's version as the running build lists it
// (`v0.226.0`), or "" when the build cannot say (a test of this module, a
// workspace build): read once.
var engineVersion = sync.OnceValue(func() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	version := ""
	if info.Main.Path == encounterModule {
		version = info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path == encounterModule {
			version = dep.Version
			if dep.Replace != nil {
				version = ""
			}
		}
	}
	if version == "(devel)" {
		return ""
	}
	return version
})
