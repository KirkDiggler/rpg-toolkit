module github.com/KirkDiggler/rpg-toolkit/examples/conditions_demo

go 1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.9.0
	github.com/KirkDiggler/rpg-toolkit/dice v0.1.0
	github.com/KirkDiggler/rpg-toolkit/events v0.1.0
	github.com/KirkDiggler/rpg-toolkit/mechanics/conditions v0.0.0
)

require github.com/KirkDiggler/rpg-toolkit/mechanics/effects v0.0.0 // indirect

replace (
	github.com/KirkDiggler/rpg-toolkit/core => ../../core
	github.com/KirkDiggler/rpg-toolkit/dice => ../../dice
	github.com/KirkDiggler/rpg-toolkit/events => ../../events
	github.com/KirkDiggler/rpg-toolkit/mechanics/conditions => ../../mechanics/conditions
	github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../../mechanics/effects
)
