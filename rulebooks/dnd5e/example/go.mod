module github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/example

go 1.24.1

require github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e v0.0.0

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.9.0 // indirect
	github.com/KirkDiggler/rpg-toolkit/events v0.6.0 // indirect
	github.com/KirkDiggler/rpg-toolkit/game v0.1.0 // indirect
)

replace (
	github.com/KirkDiggler/rpg-toolkit/game => ../../../game
	github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e => ../
)
