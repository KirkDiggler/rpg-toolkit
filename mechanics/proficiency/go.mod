module github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency

go 1.24

toolchain go1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.1.1
	github.com/KirkDiggler/rpg-toolkit/events v0.1.2
)

replace github.com/KirkDiggler/rpg-toolkit/core => ../../core

replace github.com/KirkDiggler/rpg-toolkit/events => ../../events
