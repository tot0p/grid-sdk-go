package gridbot

// ---------------------------------------------------------------------------
// Cell inspection
// ---------------------------------------------------------------------------

// IsSafe returns true if (x, y) is within bounds and the cell is empty.
func IsSafe(x, y int, state *GameState) bool {
	return x >= 0 && x < state.Width && y >= 0 && y < state.Height && state.Grid[y][x] == 0
}

// GridValue returns the raw grid value at (x, y), or -1 if out of bounds.
// 0 = empty; any other value = occupied by a bot's trail.
func GridValue(x, y int, state *GameState) int {
	if x < 0 || x >= state.Width || y < 0 || y >= state.Height {
		return -1
	}
	return state.Grid[y][x]
}

// Neighbors returns all safe 4-connected positions adjacent to (x, y).
func Neighbors(x, y int, state *GameState) []Position {
	var out []Position
	for _, d := range AllDirections() {
		nx, ny := d.Apply(x, y)
		if IsSafe(nx, ny, state) {
			out = append(out, Position{nx, ny})
		}
	}
	return out
}

// WallCount returns the number of non-safe adjacent cells around (x, y)
// (out-of-bounds + occupied cells). Range: 0–4.
func WallCount(x, y int, state *GameState) int {
	count := 0
	for _, d := range AllDirections() {
		nx, ny := d.Apply(x, y)
		if !IsSafe(nx, ny, state) {
			count++
		}
	}
	return count
}

// FillRatio returns the fraction of the grid that is occupied (0.0–1.0).
// Useful for detecting game phase: < 0.15 = early, 0.15–0.5 = mid, > 0.5 = late.
func FillRatio(state *GameState) float64 {
	total := state.Width * state.Height
	if total == 0 {
		return 0
	}
	filled := 0
	for _, row := range state.Grid {
		for _, v := range row {
			if v != 0 {
				filled++
			}
		}
	}
	return float64(filled) / float64(total)
}

// ---------------------------------------------------------------------------
// Move generation
// ---------------------------------------------------------------------------

// SafeMoves returns all directions from the bot's current position that lead
// to a safe cell, excluding the reverse direction (180-degree turn).
func SafeMoves(state *GameState) []Direction {
	if state.You == nil {
		return nil
	}
	current := Direction(state.You.Direction)
	var out []Direction
	for _, d := range AllDirections() {
		if d.IsOpposite(current) {
			continue
		}
		nx, ny := d.Apply(state.You.X, state.You.Y)
		if IsSafe(nx, ny, state) {
			out = append(out, d)
		}
	}
	return out
}

// SafeMovesDetailed returns all safe moves from the bot's current position
// with pre-computed target positions and head-on risk. Excludes reverse.
func SafeMovesDetailed(state *GameState) []Move {
	if state.You == nil {
		return nil
	}
	current := Direction(state.You.Direction)
	var out []Move
	for _, d := range AllDirections() {
		if d.IsOpposite(current) {
			continue
		}
		nx, ny := d.Apply(state.You.X, state.You.Y)
		if IsSafe(nx, ny, state) {
			out = append(out, Move{
				Direction:  d,
				X:          nx,
				Y:          ny,
				HeadOnRisk: HeadOnRisk(nx, ny, state),
			})
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Space analysis
// ---------------------------------------------------------------------------

// FloodFill counts reachable empty cells from (x, y) via BFS.
func FloodFill(x, y int, state *GameState) int {
	if !IsSafe(x, y, state) {
		return 0
	}
	w, h := state.Width, state.Height
	visited := make([]bool, w*h)
	type cell struct{ x, y int }
	queue := []cell{{x, y}}
	visited[y*w+x] = true
	count := 1

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		for _, d := range AllDirections() {
			nx, ny := d.Apply(p.x, p.y)
			if nx >= 0 && nx < w && ny >= 0 && ny < h {
				idx := ny*w + nx
				if !visited[idx] && IsSafe(nx, ny, state) {
					visited[idx] = true
					count++
					queue = append(queue, cell{nx, ny})
				}
			}
		}
	}
	return count
}

// IsTrapped returns true if the bot's reachable space is below threshold.
// Useful as a quick escape-priority check: threshold of 10–20 is typical.
func IsTrapped(state *GameState, threshold int) bool {
	if state.You == nil {
		return true
	}
	return FloodFill(state.You.X, state.You.Y, state) < threshold
}

// VoronoiBFS runs simultaneous BFS from two positions and returns how many
// cells each side can reach first. Useful for territorial advantage analysis.
func VoronoiBFS(myX, myY, oppX, oppY int, state *GameState) (myCount, oppCount int) {
	w, h := state.Width, state.Height
	if !IsSafe(myX, myY, state) {
		return 0, 0
	}

	owner := make([]byte, w*h)
	type cell struct{ x, y int }

	myQueue := []cell{{myX, myY}}
	owner[myY*w+myX] = 1
	myCount = 1

	var oppQueue []cell
	if IsSafe(oppX, oppY, state) {
		idx := oppY*w + oppX
		if owner[idx] == 0 {
			owner[idx] = 2
			oppQueue = []cell{{oppX, oppY}}
			oppCount = 1
		}
	}

	for len(myQueue) > 0 || len(oppQueue) > 0 {
		next := make([]cell, 0, len(myQueue)*2)
		for _, p := range myQueue {
			for _, d := range AllDirections() {
				nx, ny := d.Apply(p.x, p.y)
				if nx >= 0 && nx < w && ny >= 0 && ny < h {
					idx := ny*w + nx
					if owner[idx] == 0 && IsSafe(nx, ny, state) {
						owner[idx] = 1
						myCount++
						next = append(next, cell{nx, ny})
					}
				}
			}
		}
		myQueue = next

		next = make([]cell, 0, len(oppQueue)*2)
		for _, p := range oppQueue {
			for _, d := range AllDirections() {
				nx, ny := d.Apply(p.x, p.y)
				if nx >= 0 && nx < w && ny >= 0 && ny < h {
					idx := ny*w + nx
					if owner[idx] == 0 && IsSafe(nx, ny, state) {
						owner[idx] = 2
						oppCount++
						next = append(next, cell{nx, ny})
					}
				}
			}
		}
		oppQueue = next
	}
	return
}

// ---------------------------------------------------------------------------
// Distance
// ---------------------------------------------------------------------------

// ManhattanDistance returns |x1-x2| + |y1-y2|.
func ManhattanDistance(x1, y1, x2, y2 int) int {
	dx := x1 - x2
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y2
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

// DistanceTo returns the Manhattan distance from the bot to a target bot.
// Returns -1 if state.You is nil.
func DistanceTo(state *GameState, target *Bot) int {
	if state.You == nil || target == nil {
		return -1
	}
	return ManhattanDistance(state.You.X, state.You.Y, target.X, target.Y)
}

// ---------------------------------------------------------------------------
// Opponent analysis
// ---------------------------------------------------------------------------

// HeadOnRisk returns true if an alive opponent could also move to (x, y)
// on the next turn.
func HeadOnRisk(x, y int, state *GameState) bool {
	if state.You == nil {
		return false
	}
	myID := state.You.BotID
	for _, bot := range state.Bots {
		if bot.BotID == myID || !bot.Alive {
			continue
		}
		for _, d := range AllDirections() {
			ox, oy := d.Apply(bot.X, bot.Y)
			if ox == x && oy == y {
				return true
			}
		}
	}
	return false
}

// FindOpponents returns all alive bots that are not you.
func FindOpponents(state *GameState) []Bot {
	if state.You == nil {
		return nil
	}
	myID := state.You.BotID
	var out []Bot
	for _, bot := range state.Bots {
		if bot.BotID != myID && bot.Alive {
			out = append(out, bot)
		}
	}
	return out
}

// FindClosestOpponent returns the closest alive opponent by Manhattan distance,
// or nil if no opponents exist.
func FindClosestOpponent(state *GameState) *Bot {
	if state.You == nil {
		return nil
	}
	opponents := FindOpponents(state)
	if len(opponents) == 0 {
		return nil
	}
	closest := &opponents[0]
	best := ManhattanDistance(state.You.X, state.You.Y, closest.X, closest.Y)
	for i := 1; i < len(opponents); i++ {
		d := ManhattanDistance(state.You.X, state.You.Y, opponents[i].X, opponents[i].Y)
		if d < best {
			best = d
			closest = &opponents[i]
		}
	}
	return closest
}

// BotByID returns the bot with the given ID, or nil if not found.
func BotByID(id uint32, state *GameState) *Bot {
	for i := range state.Bots {
		if state.Bots[i].BotID == id {
			return &state.Bots[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal
// ---------------------------------------------------------------------------

// Abs returns the absolute value of x.
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
