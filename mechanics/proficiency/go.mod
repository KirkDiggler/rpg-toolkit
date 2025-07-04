module github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency

go 1.24

toolchain go1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.1.0
	github.com/KirkDiggler/rpg-toolkit/events v0.1.0
	github.com/KirkDiggler/rpg-toolkit/mechanics/effects v0.0.0
)

replace github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../effects
