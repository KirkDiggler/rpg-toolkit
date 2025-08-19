// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package core

// Topic represents a typed event routing key for pub/sub systems.
// Topics provide compile-time safety and avoid magic strings.
type Topic string
