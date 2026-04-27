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
	// ServerURL is the WebSocket server URL (e.g., "ws://localhost:8083").
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
	config        Config
	conn          *websocket.Conn
	lastDirection Direction
	inMatch       bool
	lastTurn      int
	stopCh        chan struct{}
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
// It automatically reconnects with exponential backoff on disconnection.
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

func (c *Client) handleMessage(message []byte) {
	var state GameState
	if err := json.Unmarshal(message, &state); err != nil {
		return
	}

	if state.Type != "game_state" {
		return
	}

	if state.You == nil {
		return
	}

	// Detect new match
	if state.Turn <= 1 && (state.Turn < c.lastTurn || !c.inMatch) {
		c.logf("New match started!")
		c.lastDirection = Direction(state.You.Direction)
		c.inMatch = true
		if ma, ok := c.config.Strategy.(MatchAware); ok {
			ma.OnMatchStart(&state)
		}
	}
	c.lastTurn = state.Turn

	if !state.You.Alive {
		if ma, ok := c.config.Strategy.(MatchAware); ok {
			ma.OnDeath(&state)
		}
		c.inMatch = false
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
