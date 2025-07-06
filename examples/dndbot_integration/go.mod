module github.com/KirkDiggler/rpg-toolkit/examples/dndbot_integration

go 1.24.1

replace (
	github.com/KirkDiggler/rpg-toolkit/core => ../../core
	github.com/KirkDiggler/rpg-toolkit/dice => ../../dice
	github.com/KirkDiggler/rpg-toolkit/events => ../../events
	github.com/KirkDiggler/rpg-toolkit/mechanics/conditions => ../../mechanics/conditions
	github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../../mechanics/effects
	github.com/KirkDiggler/rpg-toolkit/mechanics/resources => ../../mechanics/resources
)
