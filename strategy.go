package gridbot

// Strategy is the interface that bot implementations must satisfy.
// Given the current game state, return the direction to move.
type Strategy interface {
	// Move is called each turn with the current game state.
	// Return the direction to move: Up, Down, Left, or Right.
	Move(state *GameState) Direction
}

// StrategyFunc is an adapter to allow the use of ordinary functions as strategies.
// It follows the same pattern as http.HandlerFunc.
type StrategyFunc func(state *GameState) Direction

// Move calls f(state).
func (f StrategyFunc) Move(state *GameState) Direction {
	return f(state)
}

// MatchAware is an optional interface that strategies can implement
// to receive match lifecycle callbacks.
type MatchAware interface {
	// OnMatchStart is called when a new match begins (turn <= 1).
	OnMatchStart(state *GameState)
	// OnDeath is called when the bot dies (You.Alive becomes false).
	OnDeath(state *GameState)
}
