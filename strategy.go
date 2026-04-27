package gridbot

// MatchResult holds the outcome of a completed match.
type MatchResult struct {
	Won   bool       // true if you survived; false if you died
	Score int        // your final score (trail length)
	Turns int        // number of turns the match lasted
	State *GameState // final game state
}

// ---------------------------------------------------------------------------
// Optional callback interfaces
//
// Implement only the ones your strategy needs — the client checks each one
// independently. Use BaseMatchAware to embed no-op defaults for all of them.
// ---------------------------------------------------------------------------

// MatchStarter is implemented by strategies that want to be notified when a
// new match begins (turn resets to ≤ 1).
type MatchStarter interface {
	OnMatchStart(state *GameState)
}

// DeathHandler is implemented by strategies that want to be notified when the
// bot dies (You.Alive becomes false).
type DeathHandler interface {
	OnDeath(state *GameState)
}

// WinHandler is implemented by strategies that want to be notified when the
// bot wins (all opponents are dead, bot still alive).
type WinHandler interface {
	OnWin(state *GameState)
}

// MatchEnder is implemented by strategies that want a single end-of-match
// callback regardless of outcome. Fires after OnDeath or OnWin.
type MatchEnder interface {
	OnMatchEnd(result MatchResult)
}

// MatchAware combines all four lifecycle interfaces.
// Embed BaseMatchAware in your strategy to satisfy MatchAware without
// implementing every method.
type MatchAware interface {
	MatchStarter
	DeathHandler
	WinHandler
	MatchEnder
}

// ---------------------------------------------------------------------------
// BaseMatchAware — embed for zero-cost no-op defaults
// ---------------------------------------------------------------------------

// BaseMatchAware provides no-op implementations of every lifecycle callback.
// Embed it in your strategy struct to implement MatchAware without writing
// boilerplate for callbacks you don't use.
//
//	type MyBot struct {
//	    BaseMatchAware
//	}
//	func (b *MyBot) OnWin(state *GameState) { fmt.Println("won!") }
//	func (b *MyBot) Move(state *GameState) Direction { ... }
type BaseMatchAware struct{}

func (BaseMatchAware) OnMatchStart(*GameState) {}
func (BaseMatchAware) OnDeath(*GameState)      {}
func (BaseMatchAware) OnWin(*GameState)        {}
func (BaseMatchAware) OnMatchEnd(MatchResult)  {}

// ---------------------------------------------------------------------------
// Strategy interface
// ---------------------------------------------------------------------------

// Strategy is the interface that bot implementations must satisfy.
type Strategy interface {
	Move(state *GameState) Direction
}

// StrategyFunc lets an ordinary function satisfy Strategy.
type StrategyFunc func(state *GameState) Direction

func (f StrategyFunc) Move(state *GameState) Direction { return f(state) }
