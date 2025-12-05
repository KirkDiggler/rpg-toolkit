//go:build generate

package core

//go:generate mockgen -destination=mock/mock_entity.go -package=mock github.com/KirkDiggler/rpg-toolkit/core Entity
