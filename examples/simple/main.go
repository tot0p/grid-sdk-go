package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	gridbot "github.com/tot0p/grid-sdk-go"
)

// SpaceBot picks the direction with the most reachable open space.
type SpaceBot struct{}

func (s *SpaceBot) Move(state *gridbot.GameState) gridbot.Direction {
	moves := gridbot.SafeMoves(state)
	if len(moves) == 0 {
		return gridbot.Direction(state.You.Direction) // no safe move, keep going
	}

	var bestDir gridbot.Direction
	bestSpace := -1

	for _, dir := range moves {
		nx, ny := dir.Apply(state.You.X, state.You.Y)

		// Avoid head-on collision risk
		if gridbot.HeadOnRisk(nx, ny, state) {
			continue
		}

		space := gridbot.FloodFill(nx, ny, state)
		if space > bestSpace {
			bestSpace = space
			bestDir = dir
		}
	}

	// If all moves have head-on risk, pick the one with the most space anyway
	if bestSpace < 0 {
		for _, dir := range moves {
			nx, ny := dir.Apply(state.You.X, state.You.Y)
			space := gridbot.FloodFill(nx, ny, state)
			if space > bestSpace {
				bestSpace = space
				bestDir = dir
			}
		}
	}

	return bestDir
}

func main() {
	token := flag.String("token", "", "Bot authentication token")
	server := flag.String("server", "ws://localhost:8083", "Game server URL")
	flag.Parse()

	if *token == "" {
		fmt.Fprintln(os.Stderr, "Usage: simple -token YOUR_TOKEN [-server ws://host:port]")
		os.Exit(1)
	}

	client := gridbot.NewClient(gridbot.Config{
		ServerURL: *server,
		Token:     *token,
		Strategy:  &SpaceBot{},
	})

	// Graceful shutdown on Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		log.Println("Shutting down...")
		client.Stop()
	}()

	log.Println("Starting bot...")
	if err := client.Run(); err != nil {
		log.Fatal(err)
	}
}
