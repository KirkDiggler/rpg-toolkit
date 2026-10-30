// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// validateDefaultActivityRef checks reference structure, not presentation meaning.
// Empty means no authored override; activity/clip catalogs belong to consumers.
func validateDefaultActivityRef(ref string) error {
	if ref == "" {
		return nil
	}
	if _, err := core.ParseString(ref); err != nil {
		return fmt.Errorf("default activity ref %q: %w", ref, err)
	}
	return nil
}
