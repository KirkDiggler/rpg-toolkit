module github.com/KirkDiggler/rpg-toolkit/examples/conditions_demo

go 1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.0.0
	github.com/KirkDiggler/rpg-toolkit/dice v0.0.0
	github.com/KirkDiggler/rpg-toolkit/events v0.0.0
	github.com/KirkDiggler/rpg-toolkit/mechanics/conditions v0.0.0
)

replace (
	github.com/KirkDiggler/rpg-toolkit/core => ../../core
	github.com/KirkDiggler/rpg-toolkit/dice => ../../dice
	github.com/KirkDiggler/rpg-toolkit/events => ../../events
	github.com/KirkDiggler/rpg-toolkit/mechanics/conditions => ../../mechanics/conditions
)
