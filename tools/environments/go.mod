module github.com/KirkDiggler/rpg-toolkit/tools/environments

go 1.24

toolchain go1.24.5

require (
	github.com/KirkDiggler/rpg-toolkit/core v0.9.0
	github.com/KirkDiggler/rpg-toolkit/events v0.6.0
	github.com/KirkDiggler/rpg-toolkit/tools/selectables v0.0.0-20250719072111-13639d895a46
	github.com/KirkDiggler/rpg-toolkit/tools/spatial v0.0.0-20250719072111-13639d895a46
	github.com/stretchr/testify v1.10.0
)

require (
	github.com/KirkDiggler/rpg-toolkit/dice v0.1.0 // indirect
	github.com/KirkDiggler/rpg-toolkit/game v0.0.0-20250725235802-69ff839b4774 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/KirkDiggler/rpg-toolkit/tools/spatial => ../spatial

replace github.com/KirkDiggler/rpg-toolkit/tools/selectables => ../selectables
