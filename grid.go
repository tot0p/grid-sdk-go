package gridbot

// Abs returns the absolute value of x.
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// IsSafe returns true if the cell (x, y) is within bounds and empty (value 0).
func IsSafe(x, y int, state *GameState) bool {
	return x >= 0 && x < state.Width && y >= 0 && y < state.Height && state.Grid[y][x] == 0
}

// SafeMoves returns all directions that lead to a safe cell from the bot's
// current position, excluding the reverse direction (180-degree turn).
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
// with pre-computed target positions and head-on risk information.
// Excludes the reverse direction (180-degree turn).
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

// FloodFill counts the number of reachable empty cells from position (x, y)
// using BFS. Useful for estimating available space.
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

// VoronoiBFS runs simultaneous BFS from two positions, returning how many
// cells each side can reach first. Useful for territorial analysis.
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

// ManhattanDistance returns the Manhattan distance between two positions.
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

// WallCount returns the number of non-safe adjacent cells around (x, y).
// Includes walls and trails.
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
	var opponents []Bot
	for _, bot := range state.Bots {
		if bot.BotID != myID && bot.Alive {
			opponents = append(opponents, bot)
		}
	}
	return opponents
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
	bestDist := ManhattanDistance(state.You.X, state.You.Y, closest.X, closest.Y)
	for i := 1; i < len(opponents); i++ {
		d := ManhattanDistance(state.You.X, state.You.Y, opponents[i].X, opponents[i].Y)
		if d < bestDist {
			bestDist = d
			closest = &opponents[i]
		}
	}
	return closest
}
