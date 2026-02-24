package gridbot

// Direction represents a movement direction.
type Direction string

const (
	Up    Direction = "up"
	Down  Direction = "down"
	Left  Direction = "left"
	Right Direction = "right"
)

// AllDirections returns all four cardinal directions.
func AllDirections() []Direction {
	return []Direction{Up, Down, Left, Right}
}

// Opposite returns the opposite direction.
func (d Direction) Opposite() Direction {
	switch d {
	case Up:
		return Down
	case Down:
		return Up
	case Left:
		return Right
	case Right:
		return Left
	default:
		return d
	}
}

// DeltaX returns the x offset for a direction (-1, 0, or 1).
func (d Direction) DeltaX() int {
	switch d {
	case Left:
		return -1
	case Right:
		return 1
	default:
		return 0
	}
}

// DeltaY returns the y offset for a direction (-1, 0, or 1).
// Up is -1 (y decreases), Down is +1 (y increases).
func (d Direction) DeltaY() int {
	switch d {
	case Up:
		return -1
	case Down:
		return 1
	default:
		return 0
	}
}

// Apply returns the position after moving one step in direction d.
func (d Direction) Apply(x, y int) (int, int) {
	return x + d.DeltaX(), y + d.DeltaY()
}

// IsOpposite returns true if d is the opposite of other.
func (d Direction) IsOpposite(other Direction) bool {
	return d == other.Opposite()
}

// TurnRight returns the direction 90 degrees clockwise.
func (d Direction) TurnRight() Direction {
	switch d {
	case Up:
		return Right
	case Right:
		return Down
	case Down:
		return Left
	case Left:
		return Up
	default:
		return d
	}
}

// TurnLeft returns the direction 90 degrees counter-clockwise.
func (d Direction) TurnLeft() Direction {
	switch d {
	case Up:
		return Left
	case Left:
		return Down
	case Down:
		return Right
	case Right:
		return Up
	default:
		return d
	}
}
