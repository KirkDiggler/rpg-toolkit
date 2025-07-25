module github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e

go 1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/game v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.1.0 // indirect
	github.com/KirkDiggler/rpg-toolkit/events v0.1.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/KirkDiggler/rpg-toolkit/game => ../../game
