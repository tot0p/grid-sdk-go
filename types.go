package gridbot

// Bot represents a bot's state in the game.
type Bot struct {
	BotID     uint32 `json:"bot_id"`
	BotName   string `json:"bot_name"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
	Alive     bool   `json:"alive"`
	Score     int    `json:"score"`
	Color     string `json:"color"`
}

// GameState represents the state sent by the server each turn.
type GameState struct {
	Type   string  `json:"type"`
	Turn   int     `json:"turn"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	You    *Bot    `json:"you"`
	Bots   []Bot   `json:"bots"`
	Grid   [][]int `json:"grid"`
}

// Position represents a coordinate on the grid.
type Position struct {
	X int
	Y int
}

// Move represents a candidate move with pre-computed metadata.
type Move struct {
	Direction  Direction
	X, Y       int  // Target position after moving
	HeadOnRisk bool // True if an opponent could also move here next turn
}

// command is the move command sent to the server (unexported).
type command struct {
	Action    string `json:"action"`
	Direction string `json:"direction"`
}
