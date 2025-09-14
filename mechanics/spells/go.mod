module github.com/KirkDiggler/rpg-toolkit/mechanics/spells

go 1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.9.0
	github.com/KirkDiggler/rpg-toolkit/events v0.1.0
	github.com/KirkDiggler/rpg-toolkit/mechanics/conditions v0.1.0
	github.com/KirkDiggler/rpg-toolkit/mechanics/resources v0.1.0
	github.com/stretchr/testify v1.10.0
	go.uber.org/mock v0.5.2
)

require (
	github.com/KirkDiggler/rpg-toolkit/dice v0.1.0 // indirect
	github.com/KirkDiggler/rpg-toolkit/mechanics/effects v0.1.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/KirkDiggler/rpg-toolkit/core => ../../core
	github.com/KirkDiggler/rpg-toolkit/dice => ../../dice
	github.com/KirkDiggler/rpg-toolkit/events => ../../events
	github.com/KirkDiggler/rpg-toolkit/mechanics/conditions => ../conditions
	github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../effects
	github.com/KirkDiggler/rpg-toolkit/mechanics/resources => ../resources
)
