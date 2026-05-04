---
name: how to add a mechanic
description: Step-by-step guide for adding a new D&D 5e mechanic (condition, feature, or combat ability)
updated: 2026-05-02
---

# How to add a mechanic

All work is in `rulebooks/dnd5e`. The pattern is the same for conditions (RagingCondition, DodgingCondition), features (Rage, SecondWind, MartialArts), and combat abilities (Attack, Dash, Disengage).

## 1. Define a Ref

In `rulebooks/dnd5e/refs/`:
```go
// refs/conditions.go
func (c conditions) MyNewCondition() *core.Ref {
    return &core.Ref{Module: "dnd5e", Type: "conditions", Value: "my-new-condition"}
}
```

The `refs` package is the single source of truth for string identifiers. Use it everywhere instead of raw strings.

## 2. Create the Data struct (for serialization)

```go
// rulebooks/dnd5e/conditions/my_new_condition.go
type MyNewConditionData struct {
    Ref         core.Ref `json:"ref"`
    CharacterID string   `json:"character_id"`
    // ... state fields that need to survive serialization
}
```

The `Ref` field is mandatory — it is how `LoadJSON` routes to this type.

## 3. Create the runtime struct

```go
type MyNewCondition struct {
    CharacterID   string
    // ... runtime-only fields (event subscriptions, etc.)
    subscriptions []string  // unsubscribe IDs, not serialized
}
```

The runtime struct should not have JSON tags. Only `Data` has JSON tags.

## 4. Implement BusEffect

```go
func (c *MyNewCondition) Apply(bus events.EventBus) error {
    id, err := someTopic.On(bus).Subscribe(ctx, c.handleSomeEvent)
    if err != nil {
        return fmt.Errorf("subscribe MyNewCondition: %w", err)
    }
    c.subscriptions = append(c.subscriptions, id)
    return nil
}

func (c *MyNewCondition) Remove(bus events.EventBus) error {
    var errs []error
    for _, id := range c.subscriptions {
        if err := bus.Unsubscribe(ctx, id); err != nil {
            errs = append(errs, err)
        }
    }
    c.subscriptions = nil
    if len(errs) > 0 {
        return errors.Join(errs...)  // collect all, don't stop at first
    }
    return nil
}

func (c *MyNewCondition) IsApplied() bool {
    return len(c.subscriptions) > 0
}
```

Note: `Remove` collects all unsubscribe errors (PR #603 pattern). Never stop at the first error.

## 5. Implement ToJSON / loadJSON

```go
func (c *MyNewCondition) ToJSON() (json.RawMessage, error) {
    data := MyNewConditionData{
        Ref:         *refs.Conditions.MyNewCondition(),
        CharacterID: c.CharacterID,
    }
    return json.Marshal(data)
}

func (c *MyNewCondition) loadJSON(raw json.RawMessage) error {
    var data MyNewConditionData
    if err := json.Unmarshal(raw, &data); err != nil {
        return fmt.Errorf("unmarshal MyNewConditionData: %w", err)
    }
    c.CharacterID = data.CharacterID
    return nil
}
```

## 6. Register in the loader

In `rulebooks/dnd5e/conditions/loader.go`:
```go
func LoadJSON(data json.RawMessage) (ConditionBehavior, error) {
    var peek struct{ Ref core.Ref `json:"ref"` }
    if err := json.Unmarshal(data, &peek); err != nil {
        return nil, err
    }
    switch peek.Ref.Value {
    case "my-new-condition":
        c := &MyNewCondition{}
        if err := c.loadJSON(data); err != nil {
            return nil, err
        }
        return c, nil
    // ... existing cases
    }
    return nil, fmt.Errorf("unknown condition ref: %s", peek.Ref.Value)
}
```

## 7. Write tests

Follow the suite pattern:
```go
type MyNewConditionTestSuite struct {
    suite.Suite
    bus  events.EventBus
    cond *MyNewCondition
}

func (s *MyNewConditionTestSuite) SetupTest() {
    s.bus = events.NewEventBus()
    s.cond = &MyNewCondition{CharacterID: "char-1"}
}

func (s *MyNewConditionTestSuite) TestApplySubscribes() {
    err := s.cond.Apply(s.bus)
    s.Require().NoError(err)
    s.True(s.cond.IsApplied())
}

func (s *MyNewConditionTestSuite) TestRoundTrip() {
    s.cond.Apply(s.bus)
    raw, err := s.cond.ToJSON()
    s.Require().NoError(err)
    
    loaded, err := LoadJSON(raw)
    s.Require().NoError(err)
    s.IsType(&MyNewCondition{}, loaded)
}

func TestMyNewCondition(t *testing.T) {
    suite.Run(t, new(MyNewConditionTestSuite))
}
```

## 8. Run tests and lint

```bash
cd /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e
go test -race ./conditions/...
go test -race ./integration/...   # make sure integration tests still pass
golangci-lint run ./conditions/...
```

## 9. Add to integration test

Add a scenario in `rulebooks/dnd5e/integration/` that exercises the new mechanic through a real character. This is the forcing function — if the condition does not survive a `LoadFromData` round-trip under load, the integration test will catch it.
