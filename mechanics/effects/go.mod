module github.com/KirkDiggler/rpg-toolkit/mechanics/effects

go 1.27.0

// These versions contain the retired legacy effects API.
// The replacement composer is not a drop-in-compatible API.
retract [v0.1.0, v0.2.1]

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.12.0
	github.com/KirkDiggler/rpg-toolkit/events v0.6.3
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
