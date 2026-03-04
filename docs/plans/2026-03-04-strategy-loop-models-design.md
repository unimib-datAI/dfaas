# Strategy Loop Models — Design

## Goal

Decouple the *when* (loop execution model) from the *what* (strategy logic), giving strategy developers an explicit choice between three execution models: periodic, event-driven, and hybrid.

## Context

The current architecture runs every strategy in a self-managed `RunStrategy()` goroutine that loops on a time ticker. This works well for existing strategies but forces all future strategies into the same periodic mold, even when immediate reactivity to peer events would be preferable.

With the common message vocabulary now in place (`MsgOverloadAlert`, `MsgFunctionEvent`, etc.), it is natural to let strategies subscribe to these events as execution triggers.

## Architecture

### Interface Hierarchy

```go
// Strategy is the base interface embedded by every loop model.
// OnReceived is called for every incoming broadcast message and is
// used for state table updates only — it does NOT trigger a recalculation cycle.
type Strategy interface {
    OnReceived(msg *pubsub.Message) error
}

// PeriodicStrategy runs Tick on a fixed schedule.
type PeriodicStrategy interface {
    Strategy
    Period() time.Duration
    Tick(ctx context.Context) error
}

// EventDrivenStrategy runs React whenever a subscribed event arrives.
type EventDrivenStrategy interface {
    Strategy
    TriggerEvents() []string    // msgtypes.Type* constants to subscribe to
    Debounce() time.Duration    // 0 = no debounce
    React(ctx context.Context, ev StrategyEvent) error
}

// HybridStrategy combines a periodic baseline with event triggers.
// Tick and React are never called concurrently.
type HybridStrategy interface {
    PeriodicStrategy
    EventDrivenStrategy
}

// StrategyEvent carries the type discriminator and raw payload of the
// event that triggered a React call.
type StrategyEvent struct {
    Type string          // msgtypes.Type* constant
    Raw  json.RawMessage
}
```

### Runner Dispatcher

`RunStrategy()` in `loadbalancer.go` becomes a dispatcher. The type-switch order matters: `HybridStrategy` must be checked before the two constituent interfaces.

```go
func RunStrategy(ctx context.Context) error {
    switch s := _strategy.(type) {
    case HybridStrategy:
        return runHybrid(ctx, s)
    case EventDrivenStrategy:
        return runEventDriven(ctx, s)
    case PeriodicStrategy:
        return runPeriodic(ctx, s)
    default:
        return fmt.Errorf("strategy %T implements no known loop interface", s)
    }
}
```

### Event Flow

The existing `MakeCommonCallback` pre-filter already intercepts all broadcast messages. For EventDriven and Hybrid strategies the framework adds a second step: after calling `OnReceived`, if the message type is in `TriggerEvents()`, the event is forwarded to the worker goroutine via a buffered channel.

```
[PubSub receiver]
      │
      ├── OnReceived()                   (always — state table update)
      │
      └── if type ∈ TriggerEvents()
                │
                ▼
          [event channel, cap 1]
                │
                ▼
          [worker goroutine] ──► React() or Tick()   (serialised)
```

### Concurrency Guarantee

Each runner owns a single worker goroutine. `Tick` and `React` are never concurrent.

- **Periodic**: straightforward ticker loop.
- **EventDriven**: debounce implemented as a timer reset — rapid events collapse into one `React` call.
- **Hybrid**: tick and event triggers share the same worker channel. If an event arrives while a `Tick` is running it is queued (channel capacity 1); excess events are dropped. If a `Tick` fires while a `React` is running, the tick is skipped.

### Error Handling

- Errors from `Tick()` or `React()` are logged but do not stop the loop; the strategy continues at the next cycle.
- A cancelled context propagates as a fatal error to `chanErr` in `agent.go`, terminating the agent.
- Errors from `OnReceived()` are discarded (preserving current behaviour).

## File Structure

```
dfaasagent/agent/loadbalancer/
├── loadbalancer.go          // modified: RunStrategy() becomes dispatcher
├── interfaces.go            // NEW: all interfaces and StrategyEvent
├── runner_periodic.go       // NEW: runPeriodic()
├── runner_eventdriven.go    // NEW: runEventDriven() with debounce
├── runner_hybrid.go         // NEW: runHybrid()
├── recalcstrategy.go        // modified: remove RunStrategy()
├── staticstrategy.go        // modified: remove RunStrategy()
├── alllocal.go              // modified: remove RunStrategy()
└── nodemarginstrategy.go    // modified: remove RunStrategy()
```

## Migration of Existing Strategies

All four existing strategies migrate to `PeriodicStrategy`. The internal logic is unchanged; only the loop hosting is removed.

```go
// Before
func (s *RecalcStrategy) RunStrategy() error {
    ticker := time.NewTicker(s.cfg.RecalcPeriod)
    for range ticker.C {
        s.recalcStep1()
        time.Sleep(s.cfg.RecalcPeriod / 2)
        s.recalcStep2()
    }
}

// After
func (s *RecalcStrategy) Period() time.Duration { return s.cfg.RecalcPeriod }
func (s *RecalcStrategy) Tick(ctx context.Context) error {
    s.recalcStep1()
    time.Sleep(s.cfg.RecalcPeriod / 2)
    return s.recalcStep2()
}
```

| Strategy | Loop model | Change |
|---|---|---|
| `RecalcStrategy` | `PeriodicStrategy` | Remove `RunStrategy()`, add `Period()` + `Tick()` |
| `NodeMarginStrategy` | `PeriodicStrategy` | Same |
| `StaticStrategy` | `PeriodicStrategy` | Same |
| `AllLocalStrategy` | `PeriodicStrategy` | Same |

## Example: New Reactive Strategy

A future strategy reacting immediately to peer overload:

```go
func (s *ReactiveStrategy) TriggerEvents() []string {
    return []string{msgtypes.TypeOverloadAlert, msgtypes.TypeFunctionEvent}
}
func (s *ReactiveStrategy) Debounce() time.Duration { return 2 * time.Second }
func (s *ReactiveStrategy) React(ctx context.Context, ev StrategyEvent) error {
    // recalculate weights and update HAProxy
}
```

## Testing

Each runner (`runner_periodic.go`, etc.) is tested with a mock strategy implementing the corresponding interface — no dependency on libp2p or HAProxy required in runner tests.
