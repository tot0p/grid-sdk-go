package gridbot

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Config configures the bot client.
type Config struct {
	// ServerURL is the WebSocket server URL (e.g. "wss://game.learn2code.tech").
	ServerURL string

	// Token is the bot authentication token.
	Token string

	// Strategy is the bot's decision-making implementation.
	Strategy Strategy

	// OnLog is an optional callback for log messages.
	// If nil, the standard log package is used.
	OnLog func(format string, args ...interface{})
}

// Client manages the WebSocket connection to the game server.
type Client struct {
	config              Config
	conn                *websocket.Conn
	lastDirection       Direction
	inMatch             bool
	matchStartTurn      int
	onMatchStartFired   bool
	lastState           *GameState // most recent game state received
	stopCh              chan struct{}
}

// NewClient creates a new bot client with the given configuration.
func NewClient(config Config) *Client {
	return &Client{
		config:        config,
		lastDirection: Right,
		stopCh:        make(chan struct{}),
	}
}

func (c *Client) logf(format string, args ...interface{}) {
	if c.config.OnLog != nil {
		c.config.OnLog(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func (c *Client) connect() error {
	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %v", err)
	}
	u.Path = "/ws/bot"
	u.RawQuery = "token=" + c.config.Token

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %v", err)
	}
	c.conn = conn
	return nil
}

// handshake sends the client hello and waits for the server ack.
// Returns (fatal, err) — fatal=true means the caller must not reconnect.
func (c *Client) handshake() (fatal bool, err error) {
	hello, _ := json.Marshal(map[string]string{"type": "hello", "version": SDKVersion})
	c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.conn.WriteMessage(websocket.TextMessage, hello); err != nil {
		return false, fmt.Errorf("handshake send: %w", err)
	}

	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, raw, err := c.conn.ReadMessage()
	c.conn.SetReadDeadline(time.Time{})
	if err != nil {
		return false, fmt.Errorf("handshake read: %w", err)
	}

	var resp struct {
		Type    string `json:"type"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return false, fmt.Errorf("handshake: invalid server response")
	}
	if resp.Type == "error" {
		return resp.Code == "version_mismatch", fmt.Errorf("handshake rejected: %s", resp.Message)
	}
	if resp.Type != "hello" {
		return false, fmt.Errorf("handshake: unexpected message type %q", resp.Type)
	}

	serverMajorMinor := strings.Join(strings.SplitN(resp.Version, ".", 3)[:2], ".")
	c.logf("Connected (server v%s, SDK v%s)", serverMajorMinor, SDKVersion)
	return false, nil
}

func (c *Client) close() {
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// Run connects to the server and starts the game loop.
// Automatically reconnects with exponential backoff (5s to 60s).
// Returns a non-nil error only on fatal errors (e.g. version mismatch).
// Blocks until Stop() is called.
func (c *Client) Run() error {
	backoff := 5 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-c.stopCh:
			return nil
		default:
		}

		if err := c.connect(); err != nil {
			c.logf("Connection failed: %v — retrying in %v", err, backoff)
			select {
			case <-time.After(backoff):
			case <-c.stopCh:
				return nil
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		if fatal, err := c.handshake(); err != nil {
			c.logf("Handshake failed: %v", err)
			c.close()
			if fatal {
				return fmt.Errorf("fatal: %w", err)
			}
			select {
			case <-time.After(backoff):
			case <-c.stopCh:
				return nil
			}
			backoff = min(backoff*2, maxBackoff)
			continue
		}

		backoff = 5 * time.Second
		c.readLoop()

		c.close()
		c.logf("Disconnected — reconnecting in %v", backoff)

		select {
		case <-time.After(backoff):
		case <-c.stopCh:
			return nil
		}
	}
}

// Stop gracefully stops the client and closes the connection.
func (c *Client) Stop() {
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
	c.close()
}

func (c *Client) readLoop() {
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPingHandler(func(message string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return c.conn.WriteControl(websocket.PongMessage, []byte(message), time.Now().Add(10*time.Second))
	})

	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure, websocket.CloseAbnormalClosure) {
				c.logf("Connection error: %v", err)
			}
			return
		}

		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		c.handleMessage(message)
	}
}

// envelope peeks at just the "type" field of an incoming message.
type envelope struct {
	Type string `json:"type"`
}

// serverMatchStart is the match_start message sent by the server.
type serverMatchStart struct {
	Type        string      `json:"type"`
	MatchID     uint32      `json:"match_id"`
	FieldWidth  int         `json:"field_width"`
	FieldHeight int         `json:"field_height"`
	Bots        []*BotState `json:"bots"`
}

// BotState mirrors the server's BotState struct.
type BotState struct {
	BotID     uint32 `json:"bot_id"`
	BotName   string `json:"bot_name"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Direction string `json:"direction"`
	Alive     bool   `json:"alive"`
	Score     int    `json:"score"`
	Color     string `json:"color"`
}

// serverMatchEnd is the personalised match_end message sent by the server.
type serverMatchEnd struct {
	Type     string `json:"type"`
	MatchID  uint32 `json:"match_id"`
	Won      bool   `json:"won"`
	Reason   string `json:"reason"`
	Score    int    `json:"score"`
	Position int    `json:"position"`
	Turns    int    `json:"turns"`
	WinnerID *uint32 `json:"winner_id"`
}

func (c *Client) handleMessage(message []byte) {
	var env envelope
	if err := json.Unmarshal(message, &env); err != nil {
		return
	}

	switch env.Type {

	case "match_start":
		var msg serverMatchStart
		if err := json.Unmarshal(message, &msg); err != nil {
			return
		}
		c.logf("Match %d starting (%dx%d, %d bots)", msg.MatchID, msg.FieldWidth, msg.FieldHeight, len(msg.Bots))
		c.inMatch = true
		c.matchStartTurn = 0
		c.onMatchStartFired = false
		c.lastState = nil

	case "match_end":
		var msg serverMatchEnd
		if err := json.Unmarshal(message, &msg); err != nil {
			return
		}
		result := MatchResult{
			Won:   msg.Won,
			Score: msg.Score,
			Turns: msg.Turns,
			State: c.lastState,
		}
		if msg.Won {
			c.logf("Match %d WON — score %d in %d turns", msg.MatchID, msg.Score, msg.Turns)
			if wh, ok := c.config.Strategy.(WinHandler); ok {
				wh.OnWin(c.lastState)
			}
		} else {
			c.logf("Match %d LOST — score %d in %d turns (reason: %s)", msg.MatchID, msg.Score, msg.Turns, msg.Reason)
			if dh, ok := c.config.Strategy.(DeathHandler); ok {
				dh.OnDeath(c.lastState)
			}
		}
		if me, ok := c.config.Strategy.(MatchEnder); ok {
			me.OnMatchEnd(result)
		}
		c.inMatch = false
		c.onMatchStartFired = false

	case "game_state":
		var state GameState
		if err := json.Unmarshal(message, &state); err != nil {
			return
		}
		if state.You == nil {
			return
		}
		c.lastState = &state

		// Detect match start: fires if server match_start was missed OR as
		// a safety net for older servers.
		if !c.inMatch || (state.Turn <= 1 && state.Turn < c.matchStartTurn) {
			c.logf("New match started!")
			c.lastDirection = Direction(state.You.Direction)
			c.inMatch = true
			c.matchStartTurn = state.Turn
			if !c.onMatchStartFired {
				c.onMatchStartFired = true
				if ms, ok := c.config.Strategy.(MatchStarter); ok {
					ms.OnMatchStart(&state)
				}
			}
		} else if c.inMatch && c.matchStartTurn == 0 {
			// First game_state after a match_start message
			c.matchStartTurn = state.Turn
			c.lastDirection = Direction(state.You.Direction)
			if !c.onMatchStartFired {
				c.onMatchStartFired = true
				if ms, ok := c.config.Strategy.(MatchStarter); ok {
					ms.OnMatchStart(&state)
				}
			}
		}

		// If the bot is dead, wait for the match_end message
		if !state.You.Alive {
			return
		}

		direction := c.config.Strategy.Move(&state)

		data, _ := json.Marshal(command{Action: "move", Direction: string(direction)})
		c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
			c.logf("Failed to send command: %v", err)
			return
		}
		c.lastDirection = direction
	}
}
