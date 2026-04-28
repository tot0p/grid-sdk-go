# grid-sdk-go

A Go SDK for building bots that play on a Grid game server over WebSocket.

## Installation

```bash
go get github.com/tot0p/grid-sdk-go
```

## Quick Start

```go
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	gridbot "github.com/tot0p/grid-sdk-go"
)

type MyBot struct{}

func (b *MyBot) Move(state *gridbot.GameState) gridbot.Direction {
	moves := gridbot.SafeMoves(state)
	if len(moves) == 0 {
		return gridbot.Direction(state.You.Direction)
	}
	return moves[0]
}

func main() {
	token := flag.String("token", "", "Bot token")
	flag.Parse()

	client := gridbot.NewClient(gridbot.Config{
		ServerURL: "ws://localhost:8083",
		Token:     *token,
		Strategy:  &MyBot{},
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		client.Stop()
	}()

	log.Fatal(client.Run())
}
```

## Strategy Interface

Implement the `Strategy` interface to define your bot's behavior:

```go
type Strategy interface {
	Move(state *GameState) Direction
}
```

You can also use `StrategyFunc` to pass a plain function:

```go
gridbot.StrategyFunc(func(state *gridbot.GameState) gridbot.Direction {
	return gridbot.Right
})
```

Optionally implement lifecycle interfaces for match callbacks:

```go
type MatchStarter interface { OnMatchStart(state *GameState) }
type DeathHandler  interface { OnDeath(state *GameState) }
type WinHandler    interface { OnWin(state *GameState) }
type MatchEnder    interface { OnMatchEnd(result MatchResult) }

// MatchAware combines all four. Embed BaseMatchAware for no-op defaults.
type MatchAware interface {
	MatchStarter; DeathHandler; WinHandler; MatchEnder
}
```

`OnMatchStart` is guaranteed to fire exactly once per match — safe to use for per-match state initialization.

## Helper Functions

| Function | Description |
|---|---|
| `SafeMoves(state)` | Returns directions that lead to safe cells (excludes 180-degree turns) |
| `SafeMovesDetailed(state)` | Like `SafeMoves` but returns `Move` structs with position and head-on risk info |
| `FloodFill(x, y, state)` | Counts reachable empty cells from a position (BFS) |
| `VoronoiBFS(myX, myY, oppX, oppY, state)` | Territorial analysis — cells reachable first by each side |
| `HeadOnRisk(x, y, state)` | Whether an opponent could move to this cell next turn |
| `FindOpponents(state)` | All alive opponents |
| `FindClosestOpponent(state)` | Nearest alive opponent by Manhattan distance |
| `ManhattanDistance(x1, y1, x2, y2)` | Manhattan distance between two points |
| `WallCount(x, y, state)` | Number of non-safe adjacent cells |
| `IsSafe(x, y, state)` | Whether a cell is in bounds and empty |

## Direction

Directions are `Up`, `Down`, `Left`, `Right` with helper methods:

- `Opposite()` — reverse direction
- `TurnRight()` / `TurnLeft()` — 90-degree rotation
- `Apply(x, y)` — position after one step
- `IsOpposite(other)` — check if reverse
- `DeltaX()` / `DeltaY()` — axis offsets

## Examples

See the [examples/simple](examples/simple) directory for a full bot that picks the direction with the most open space.

```bash
go run ./examples/simple -token YOUR_TOKEN -server ws://localhost:8083
```

## Other SDKs

- **Python**: [`grid-sdk-py`](https://github.com/tot0p/grid-sdk-py) — `pip install grid-sdk-py==0.3.4`
