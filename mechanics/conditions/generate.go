// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

//go:generate mockgen -destination=mock/mock_condition.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/conditions Condition
//go:generate mockgen -destination=mock/mock_eventbus.go -package=mock github.com/KirkDiggler/rpg-toolkit/events EventBus

package conditions