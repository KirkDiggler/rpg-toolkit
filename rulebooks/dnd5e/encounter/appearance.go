// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// validateAppearanceRef checks only reference structure. Empty means the caller
// supplied no explicit appearance; catalog membership is never this module's.
func validateAppearanceRef(ref string) error {
	if ref == "" {
		return nil
	}
	if _, err := core.ParseString(ref); err != nil {
		return fmt.Errorf("appearance ref %q: %w", ref, err)
	}
	return nil
}
