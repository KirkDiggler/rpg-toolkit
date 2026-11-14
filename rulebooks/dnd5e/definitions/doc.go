// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package definitions supplies named, discoverable D&D content built from the
// effects vocabulary. Fields are the actual configured values projected by
// groups and complete definitions; there is no parallel preset implementation.
//
// Definitions are unbound and own their authoring data. The effects compiler
// validates and detaches them. Conditions/applications supply identity, roles,
// terminal placement and persistence; activation sources own eligibility/payment.
// This package imports none of those runtimes and creates no subscriptions.
//
// Catalogue explicitly registers projected built-in values. Configuration and
// typed field selection resolve to the same canonical effects/compositions.
package definitions
