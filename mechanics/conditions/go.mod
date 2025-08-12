module github.com/KirkDiggler/rpg-toolkit/mechanics/conditions

go 1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.1.2
	github.com/KirkDiggler/rpg-toolkit/events v0.1.0
	github.com/KirkDiggler/rpg-toolkit/mechanics/effects v0.0.0
	github.com/stretchr/testify v1.10.0
	go.uber.org/mock v0.5.2
)

require (
	github.com/KirkDiggler/rpg-toolkit/dice v0.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../effects

replace github.com/KirkDiggler/rpg-toolkit/events => ../../events

replace github.com/KirkDiggler/rpg-toolkit/core => ../../core

replace github.com/KirkDiggler/rpg-toolkit/dice => ../../dice
