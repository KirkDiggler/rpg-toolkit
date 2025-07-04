module github.com/KirkDiggler/rpg-toolkit/examples/simple_combat

go 1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/dice v0.0.0
	github.com/KirkDiggler/rpg-toolkit/events v0.0.0
	go.uber.org/mock v0.5.2
)

require github.com/KirkDiggler/rpg-toolkit/core v0.1.0 // indirect

replace github.com/KirkDiggler/rpg-toolkit/core => ../../core

replace github.com/KirkDiggler/rpg-toolkit/dice => ../../dice

replace github.com/KirkDiggler/rpg-toolkit/events => ../../events
