module github.com/KirkDiggler/rpg-toolkit/examples/dndbot_integration

go 1.24.1

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.9.0
	github.com/KirkDiggler/rpg-toolkit/events v0.1.0
	github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency v0.0.0
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/KirkDiggler/rpg-toolkit/dice v0.1.0 // indirect
	github.com/KirkDiggler/rpg-toolkit/mechanics/effects v0.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/KirkDiggler/rpg-toolkit/core => ../../core
	github.com/KirkDiggler/rpg-toolkit/dice => ../../dice
	github.com/KirkDiggler/rpg-toolkit/events => ../../events
	github.com/KirkDiggler/rpg-toolkit/mechanics/conditions => ../../mechanics/conditions
	github.com/KirkDiggler/rpg-toolkit/mechanics/effects => ../../mechanics/effects
	github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency => ../../mechanics/proficiency
	github.com/KirkDiggler/rpg-toolkit/mechanics/resources => ../../mechanics/resources
)
