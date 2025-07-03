module github.com/KirkDiggler/rpg-toolkit/mechanics/conditions

go 1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.0.0-00010101000000-000000000000
	github.com/KirkDiggler/rpg-toolkit/dice v0.0.0-00010101000000-000000000000
	github.com/KirkDiggler/rpg-toolkit/events v0.0.0-00010101000000-000000000000
)

replace (
	github.com/KirkDiggler/rpg-toolkit/core => ../../core
	github.com/KirkDiggler/rpg-toolkit/dice => ../../dice
	github.com/KirkDiggler/rpg-toolkit/events => ../../events
)