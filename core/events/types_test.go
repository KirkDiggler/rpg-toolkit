// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core/events"
	"github.com/stretchr/testify/assert"
)

// Example event data keys that a rulebook might define
const (
	DataKeyLevel     events.EventDataKey = "level"
	DataKeyDuration  events.EventDataKey = "duration"
	DataKeyTarget    events.EventDataKey = "target"
	DataKeyAmount    events.EventDataKey = "amount"
)

func TestEventDataKey_TypeSafety(t *testing.T) {
	// Create a typed event data map
	data := make(map[events.EventDataKey]any)
	
	// Add data with typed keys
	data[DataKeyLevel] = 5
	data[DataKeyDuration] = 10
	data[DataKeyTarget] = "player-123"
	data[DataKeyAmount] = 25.5
	
	// Access data with typed keys
	level, ok := data[DataKeyLevel].(int)
	assert.True(t, ok)
	assert.Equal(t, 5, level)
	
	duration, ok := data[DataKeyDuration].(int)
	assert.True(t, ok)
	assert.Equal(t, 10, duration)
	
	target, ok := data[DataKeyTarget].(string)
	assert.True(t, ok)
	assert.Equal(t, "player-123", target)
	
	amount, ok := data[DataKeyAmount].(float64)
	assert.True(t, ok)
	assert.Equal(t, 25.5, amount)
}

func TestEventDataKey_StringConversion(t *testing.T) {
	// EventDataKey can be converted to string when needed
	key := DataKeyLevel
	assert.Equal(t, "level", string(key))
	
	// Can be used in string contexts if necessary
	stringMap := make(map[string]any)
	stringMap[string(DataKeyLevel)] = 5
	
	value := stringMap["level"]
	assert.Equal(t, 5, value)
}