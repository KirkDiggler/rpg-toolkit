package encounter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	monsteractions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// NPCAct executes a single turn for the named NPC. It rehydrates the
// monster from MonsterData.DataJSON onto a per-call dnd5e bus, builds
// monster.TurnInput from encounter state, subscribes a temporary listener
// to capture the dnd5e events the action publishes, and re-publishes them
// as encounter-scoped per-viewer events.
//
// Wave 2.8 only handles single-target melee attacks (the shape goblin /
// scimitar / bite emit). For each captured dnd5e.AttackEvent, the
// encounter SDK resolves hit/damage with its own roller (since the
// dnd5e action layer does not run resolution itself today — see issue
// followups), mutates the target player's HP, and emits AttackResolved
// + DamageDealt encounter events.
//
// Position is updated from TurnResult.Movement; a MoveEvent is emitted
// per-viewer for the NPC's path.
//
// Does NOT auto-cycle the turn — orchestrator calls EndTurn(npcID) next.
func (e *Encounter) NPCAct(ctx context.Context, npcID encountercore.EntityID) error {
	if e.data.Mode != encountercore.ModeTurnBased {
		return ErrNotTurnBased
	}
	if active := e.ActiveActor(); active != npcID {
		return fmt.Errorf("%w: active=%q got=%q", ErrNotYourTurn, active, npcID)
	}
	mon, ok := e.data.Monsters[npcID]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownTarget, npcID)
	}
	if len(mon.DataJSON) == 0 {
		// No rehydratable monster — fall back to a minimal scripted attack
		// against the closest reachable player, using the monster's stored
		// AttackBonus / DamageDice / DamageType. This keeps the verb
		// non-blocking when the orchestrator only seeded the snapshot fields.
		return e.npcActScripted(ctx, mon)
	}

	bus := dnd5events.NewEventBus()
	captured, unsubAttack, err := subscribeAttacks(ctx, bus)
	if err != nil {
		return fmt.Errorf("subscribe dnd5e attack: %w", err)
	}
	defer func() { _ = unsubAttack() }()

	capturedDmg, unsubDmg, err := subscribeDamage(ctx, bus)
	if err != nil {
		return fmt.Errorf("subscribe dnd5e damage: %w", err)
	}
	defer func() { _ = unsubDmg() }()

	capturedCond, unsubCond, err := subscribeConditions(ctx, bus)
	if err != nil {
		return fmt.Errorf("subscribe dnd5e condition: %w", err)
	}
	defer func() { _ = unsubCond() }()

	var data monster.Data
	if err := json.Unmarshal(mon.DataJSON, &data); err != nil {
		return fmt.Errorf("unmarshal monster data: %w", err)
	}
	loaded, err := monster.LoadFromData(ctx, &data, bus)
	if err != nil {
		return fmt.Errorf("load monster: %w", err)
	}
	if err := monsteractions.LoadMonsterActions(loaded, data.Actions); err != nil {
		return fmt.Errorf("load monster actions: %w", err)
	}

	perception := e.buildPerception(mon)
	speed := mon.Speed
	if speed <= 0 {
		speed = 6
	}
	economy := combat.NewActionEconomy()
	economy.MovementRemaining = speed

	input := &monster.TurnInput{
		Bus:           bus,
		ActionEconomy: economy,
		Perception:    perception,
		Roller:        e.roller,
		Speed:         speed,
	}

	result, err := loaded.TakeTurn(ctx, input)
	if err != nil {
		return fmt.Errorf("monster.TakeTurn: %w", err)
	}

	if err := e.applyNPCMovement(mon, result.Movement); err != nil {
		return err
	}
	if err := e.applyCapturedAttacks(mon, *captured); err != nil {
		return err
	}
	if err := e.applyCapturedDamage(mon, *capturedDmg); err != nil {
		return err
	}
	if err := e.applyCapturedConditions(mon, *capturedCond); err != nil {
		return err
	}
	return nil
}

// applyNPCMovement walks the NPC to the final hex from the captured
// movement path and emits a MoveEvent for any viewer who saw any segment.
func (e *Encounter) applyNPCMovement(mon *MonsterData, movement []spatial.CubeCoordinate) error {
	if len(movement) == 0 {
		return nil
	}
	final := movement[len(movement)-1]
	mon.Position = encountercore.Hex{Q: final.X, R: final.Y, S: final.Z}
	path := make([]encountercore.Hex, 0, len(movement))
	for _, hop := range movement {
		path = append(path, encountercore.Hex{Q: hop.X, R: hop.Y, S: hop.Z})
	}
	movePerPlayer := make(map[encountercore.PlayerID]events.MovePlayerSlice)
	for viewerID, viewer := range e.data.Players {
		seen := make([]encountercore.Hex, 0, len(path))
		for _, h := range path {
			if e.viewerCanSee(viewer, h) {
				seen = append(seen, h)
			}
		}
		if len(seen) > 0 {
			movePerPlayer[viewerID] = events.MovePlayerSlice{SeenSegments: seen}
		}
	}
	if len(movePerPlayer) == 0 {
		return nil
	}
	if err := e.broker.Publish(events.NewMoveEvent(
		e.data.ID, e.nextSeq(), mon.ID, path, movePerPlayer,
	)); err != nil {
		return fmt.Errorf("publish npc move: %w", err)
	}
	return nil
}

// applyCapturedAttacks resolves each captured dnd5e AttackEvent (cause-only)
// using the encounter's stand-in resolver, mutates the target player's HP,
// and emits AttackResolved + DamageDealt encounter events.
func (e *Encounter) applyCapturedAttacks(mon *MonsterData, attacks []dnd5eEvents.AttackEvent) error {
	for _, atk := range attacks {
		targetID := encountercore.EntityID(atk.TargetID)
		targetPlayer := e.findPlayerByEntityID(targetID)
		if targetPlayer == nil {
			continue
		}
		dmgType := mon.DamageType
		if dmgType == "" {
			dmgType = damageTypeUntyped
		}
		res := e.resolveAttack(mon.AttackBonus, targetPlayer.AC, mon.DamageDice)
		if res.hit {
			targetPlayer.HP -= res.damage
			if targetPlayer.HP < 0 {
				targetPlayer.HP = 0
			}
		}
		if err := e.publishAttackOutcome(
			mon.ID, targetID, res,
			targetPlayer.HP, targetPlayer.MaxHP, dmgType,
			mon.Position, targetPlayer.View.Position,
		); err != nil {
			return err
		}
	}
	return nil
}

// applyCapturedDamage translates each dnd5e DamageReceivedEvent into an
// encounter DamageDealtEvent. Today no shipped action publishes one
// through this bus, but the wiring is in place so downstream changes
// flow automatically.
func (e *Encounter) applyCapturedDamage(mon *MonsterData, damages []dnd5eEvents.DamageReceivedEvent) error {
	for _, dmg := range damages {
		targetID := encountercore.EntityID(dmg.TargetID)
		sourceID := encountercore.EntityID(dmg.SourceID)
		targetPlayer := e.findPlayerByEntityID(targetID)
		if targetPlayer != nil {
			targetPlayer.HP -= dmg.Amount
			if targetPlayer.HP < 0 {
				targetPlayer.HP = 0
			}
		}
		var hpAfter, maxHP int
		var targetPos encountercore.Hex
		if targetPlayer != nil {
			hpAfter = targetPlayer.HP
			maxHP = targetPlayer.MaxHP
			targetPos = targetPlayer.View.Position
		}
		damageType := string(dmg.DamageType)
		if damageType == "" {
			damageType = damageTypeUntyped
		}
		damagePerPlayer := make(map[encountercore.PlayerID]events.DamageDealtSlice)
		for viewerID, viewer := range e.data.Players {
			visible := e.viewerCanSee(viewer, mon.Position) || e.viewerCanSee(viewer, targetPos)
			damagePerPlayer[viewerID] = events.DamageDealtSlice{Visible: visible}
		}
		if err := e.broker.Publish(events.NewDamageDealtEvent(
			e.data.ID, e.nextSeq(),
			targetID, sourceID,
			dmg.Amount, damageType,
			hpAfter, maxHP,
			damagePerPlayer,
		)); err != nil {
			return fmt.Errorf("publish damage dealt: %w", err)
		}
	}
	return nil
}

// applyCapturedConditions translates each dnd5e ConditionAppliedEvent into
// an encounter ConditionAppliedEvent. As with damage, no shipped action
// publishes one through this bus today.
func (e *Encounter) applyCapturedConditions(mon *MonsterData, conds []dnd5eEvents.ConditionAppliedEvent) error {
	for _, cond := range conds {
		targetID := encountercore.EntityID("")
		if cond.Target != nil {
			targetID = encountercore.EntityID(cond.Target.GetID())
		}
		condRef := string(cond.Type)
		condPerPlayer := make(map[encountercore.PlayerID]events.ConditionAppliedSlice)
		for viewerID := range e.data.Players {
			condPerPlayer[viewerID] = events.ConditionAppliedSlice{Visible: true}
		}
		if err := e.broker.Publish(events.NewConditionAppliedEvent(
			e.data.ID, e.nextSeq(),
			targetID, mon.ID, condRef, 0, condPerPlayer,
		)); err != nil {
			return fmt.Errorf("publish condition applied: %w", err)
		}
	}
	return nil
}

// npcActScripted runs a minimal attack against the closest player when
// the monster has no DataJSON to rehydrate. Used for tests and
// orchestrator-seeded fixtures that don't carry a serialized monster.
func (e *Encounter) npcActScripted(_ context.Context, mon *MonsterData) error {
	target := e.closestPlayer(mon.Position)
	if target == nil {
		return nil
	}
	dmgType := mon.DamageType
	if dmgType == "" {
		dmgType = damageTypeUntyped
	}
	res := e.resolveAttack(mon.AttackBonus, target.AC, mon.DamageDice)
	if res.hit {
		target.HP -= res.damage
		if target.HP < 0 {
			target.HP = 0
		}
	}
	return e.publishAttackOutcome(
		mon.ID, target.EntityID, res,
		target.HP, target.MaxHP, dmgType,
		mon.Position, target.View.Position,
	)
}

// buildPerception assembles the PerceptionData a monster needs to choose
// and target an action. Wave 2.8 treats every player as an enemy and
// computes distances directly from hex coordinates — no walls or cover.
func (e *Encounter) buildPerception(mon *MonsterData) *monster.PerceptionData {
	pos := spatial.CubeCoordinate{X: mon.Position.Q, Y: mon.Position.R, Z: mon.Position.S}
	pd := &monster.PerceptionData{
		MyPosition: pos,
	}
	for _, p := range e.data.Players {
		dist := hexDistance(mon.Position, p.View.Position)
		pd.Enemies = append(pd.Enemies, monster.PerceivedEntity{
			Entity:   &playerEntity{id: string(p.EntityID), name: string(p.ID)},
			Position: spatial.CubeCoordinate{X: p.View.Position.Q, Y: p.View.Position.R, Z: p.View.Position.S},
			Distance: dist,
			Adjacent: dist == 1,
			HP:       p.HP,
			AC:       p.AC,
		})
	}
	// Sort enemies by distance, closest first — required by the targeting
	// strategies in monster.PerceptionData.
	sortEnemiesByDistance(pd.Enemies)
	return pd
}

// playerEntity adapts a player into a core.Entity for the dnd5e
// perception layer. Only ID is read by the targeting code paths shipped
// in Wave 2.8.
type playerEntity struct {
	id   string
	name string
}

func (p *playerEntity) GetID() string            { return p.id }
func (p *playerEntity) GetType() core.EntityType { return "character" }
func (p *playerEntity) GetName() string          { return p.name }

// closestPlayer returns the player nearest the given hex, or nil if the
// encounter has no players.
func (e *Encounter) closestPlayer(from encountercore.Hex) *PlayerData {
	var best *PlayerData
	bestDist := -1
	for _, p := range e.data.Players {
		d := hexDistance(from, p.View.Position)
		if best == nil || d < bestDist {
			best = p
			bestDist = d
		}
	}
	return best
}

// findPlayerByEntityID returns the player whose EntityID matches id.
func (e *Encounter) findPlayerByEntityID(id encountercore.EntityID) *PlayerData {
	for _, p := range e.data.Players {
		if p.EntityID == id {
			return p
		}
	}
	return nil
}

// viewerCanSee delegates to the perception stub LoS but tolerates a nil view.
func (e *Encounter) viewerCanSee(p *PlayerData, h encountercore.Hex) bool {
	if p == nil || p.View == nil {
		return false
	}
	return hexDistance(p.View.Position, h) <= p.View.SightRange
}

// hexDistance is the cube-coordinate hex distance between two hexes.
func hexDistance(a, b encountercore.Hex) int {
	dq := absInt(a.Q - b.Q)
	dr := absInt(a.R - b.R)
	ds := absInt(a.S - b.S)
	if dq > dr {
		if dq > ds {
			return dq
		}
		return ds
	}
	if dr > ds {
		return dr
	}
	return ds
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// sortEnemiesByDistance sorts perceived enemies in ascending distance
// order, ties broken by entity id for determinism.
func sortEnemiesByDistance(es []monster.PerceivedEntity) {
	for i := 1; i < len(es); i++ {
		for j := i; j > 0; j-- {
			if es[j].Distance < es[j-1].Distance ||
				(es[j].Distance == es[j-1].Distance && es[j].Entity.GetID() < es[j-1].Entity.GetID()) {
				es[j], es[j-1] = es[j-1], es[j]
				continue
			}
			break
		}
	}
}

// subscribeAttacks attaches a listener for the dnd5e AttackTopic on bus
// and returns (capturedSlicePtr, unsubscribe, err). The slice grows for
// the lifetime of the bus subscription.
func subscribeAttacks(ctx context.Context, bus dnd5events.EventBus) (*[]dnd5eEvents.AttackEvent, func() error, error) {
	captured := &[]dnd5eEvents.AttackEvent{}
	topic := dnd5eEvents.AttackTopic.On(bus)
	subID, err := topic.Subscribe(ctx, func(_ context.Context, evt dnd5eEvents.AttackEvent) error {
		*captured = append(*captured, evt)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	unsub := func() error { return topic.Unsubscribe(ctx, subID) }
	return captured, unsub, nil
}

// subscribeDamage attaches a listener for the dnd5e DamageReceivedTopic.
func subscribeDamage(
	ctx context.Context, bus dnd5events.EventBus,
) (*[]dnd5eEvents.DamageReceivedEvent, func() error, error) {
	captured := &[]dnd5eEvents.DamageReceivedEvent{}
	topic := dnd5eEvents.DamageReceivedTopic.On(bus)
	subID, err := topic.Subscribe(ctx, func(_ context.Context, evt dnd5eEvents.DamageReceivedEvent) error {
		*captured = append(*captured, evt)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	unsub := func() error { return topic.Unsubscribe(ctx, subID) }
	return captured, unsub, nil
}

// subscribeConditions attaches a listener for the dnd5e ConditionAppliedTopic.
func subscribeConditions(
	ctx context.Context, bus dnd5events.EventBus,
) (*[]dnd5eEvents.ConditionAppliedEvent, func() error, error) {
	captured := &[]dnd5eEvents.ConditionAppliedEvent{}
	topic := dnd5eEvents.ConditionAppliedTopic.On(bus)
	subID, err := topic.Subscribe(ctx, func(_ context.Context, evt dnd5eEvents.ConditionAppliedEvent) error {
		*captured = append(*captured, evt)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	unsub := func() error { return topic.Unsubscribe(ctx, subID) }
	return captured, unsub, nil
}
