package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sheikhrachel/go-gol/model"
	"github.com/sheikhrachel/go-gol/utils"
)

func main() {
	// Load configuration - fallback to defaults if file doesn't exist
	config, err := utils.LoadConfig("config.json")
	if err != nil {
		fmt.Println("Using default configuration (config.json not found)")
		config = utils.DefaultConfig()
	}

	// Initialize game
	grid, pool, renderer, stats := initializeGame(config)
	displayGameInfo(config, grid)

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Main game loop
	var (
		generation     = 0
		stagnantCount  = 0
		lastRestartGen = 0
		lastFrameTime  = time.Now()
	)

	for {
		select {
		case <-sigChan:
			fmt.Println("\n🛑 Shutting down gracefully...")
			fmt.Printf("Final stats: %d generations in %.1f seconds\n",
				generation, time.Since(stats.StartTime).Seconds())
			fmt.Printf("Average: %.1f gen/sec, %.1f avg population\n",
				stats.GenerationsPerSecond, stats.AveragePopulation)
			return
		default:
			// Continue with game loop
		}

		frameStart := time.Now()
		renderer.Clear()

		// Update game state
		livingCells, density, status, isStagnant := updateGameState(grid, generation, lastFrameTime, stats)
		lastFrameTime = frameStart

		// Update stagnation counter
		if isStagnant {
			stagnantCount++
		} else {
			stagnantCount = 0
		}

		// Display current status
		displayGameStatus(generation, livingCells, density, status, config, grid, stats, lastRestartGen)
		renderer.Display(grid)

		// Check for max generations limit
		if config.MaxGenerations > 0 && generation >= config.MaxGenerations {
			fmt.Printf("\n🏁 Reached maximum generations limit (%d)\n", config.MaxGenerations)
			break
		}

		// Check restart conditions
		shouldRestart, restartReason := checkRestartConditions(livingCells, stagnantCount, generation, config)

		if shouldRestart && config.AutoRestart {
			fmt.Printf("🔄 Restarting due to %s...\n", restartReason)

			// Return old grid to pool if using memory pooling
			model.GridToPool(grid, pool)

			// Restart game
			grid = restartGame(config)
			lastRestartGen = generation
			stagnantCount = 0
		} else if stagnantCount >= 2 && stagnantCount < config.StagnationThreshold {
			// Inject some life to try to break the stagnation
			grid.InjectRandomLife(config.InjectionCount)
		}

		// Calculate next generation
		newGrid := grid.NextGeneration(config, pool)

		// Return old grid to pool if using memory pooling
		model.GridToPool(grid, pool)
		grid = newGrid

		generation++

		// Wait before next frame
		time.Sleep(config.FrameRate)
	}
	model.GridToPool(grid, pool)
}
