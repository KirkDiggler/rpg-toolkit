module github.com/KirkDiggler/rpg-toolkit/mechanics/conditions

go 1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.0.0-unpublished
	github.com/KirkDiggler/rpg-toolkit/dice v0.0.0-00010101000000-000000000000
	github.com/KirkDiggler/rpg-toolkit/events v0.0.0-00010101000000-000000000000
)

require (
	go.uber.org/mock v0.5.2 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
)

replace (
	github.com/KirkDiggler/rpg-toolkit/core => ../../core
	github.com/KirkDiggler/rpg-toolkit/dice => ../../dice
	github.com/KirkDiggler/rpg-toolkit/events => ../../events
)
