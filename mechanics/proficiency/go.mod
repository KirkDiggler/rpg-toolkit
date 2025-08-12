module github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency

go 1.24

toolchain go1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.2.0
	github.com/KirkDiggler/rpg-toolkit/events v0.1.0
	github.com/KirkDiggler/rpg-toolkit/mechanics/effects v0.0.0
	go.uber.org/mock v0.5.2
)

require github.com/KirkDiggler/rpg-toolkit/dice v0.1.0 // indirect

replace github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../effects
