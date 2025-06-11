package main

import (
	"fmt"
	"time"

	"github.com/sheikhrachel/go-gol/model"
	"github.com/sheikhrachel/go-gol/utils"
)

// initializeGame sets up the initial game state
func initializeGame(config utils.Config) (
	*model.Grid,
	*model.GridPool,
	*model.TerminalRenderer,
	*utils.Stats,
) {
	var pool *model.GridPool
	if config.UseMemoryPool {
		pool = model.NewGridPool()
	}

	grid := model.NewGrid(config.Width, config.Height)
	grid.ResetWithInterestingPatterns(config)

	renderer := &model.TerminalRenderer{}
	stats := utils.NewStats()

	return grid, pool, renderer, stats
}

// displayGameInfo shows the initial game information
func displayGameInfo(config utils.Config, grid *model.Grid) {
	fmt.Printf("Features: Memory Pool: %v, Bounded: %v (Parallel: always enabled)\n",
		config.UseMemoryPool, config.UseBoundedGrid)
	fmt.Printf("Grid: %dx%d | Initial living cells: %d\n",
		grid.GetWidth(), grid.GetHeight(), grid.CountLivingCells())
	fmt.Println("Press Ctrl+C to exit gracefully")
	fmt.Println()
	time.Sleep(2 * time.Second)
}

// updateGameState updates the game state and returns status information
func updateGameState(
	grid *model.Grid,
	generation int,
	lastFrameTime time.Time,
	stats *utils.Stats,
) (int, float64, string, bool) {
	livingCells := grid.CountLivingCells()
	density := float64(livingCells) / float64(grid.GetWidth()*grid.GetHeight()) * 100

	// Update performance stats
	frameDuration := time.Since(lastFrameTime)
	stats.Update(generation, livingCells, frameDuration)

	// Update history for stagnation detection
	grid.UpdateHistory()

	// Check for stagnation
	isStagnant := grid.IsStagnant()

	// Display status
	status := "Active"
	if isStagnant {
		status = fmt.Sprintf("Stagnant (%d)", generation) // This could be improved
	}
	if livingCells == 0 {
		status = "Extinct"
	}

	return livingCells, density, status, isStagnant
}

// displayGameStatus shows the current game status
func displayGameStatus(
	generation, livingCells int,
	density float64,
	status string,
	config utils.Config,
	grid *model.Grid,
	stats *utils.Stats,
	lastRestartGen int,
) {

	// Show bounding box info for bounded grids
	boundingInfo := ""
	if config.UseBoundedGrid {
		boundingInfo = fmt.Sprintf(" | Bounding box: %d cells", grid.GetBoundingBoxSize())
	}

	fmt.Printf("Gen: %d | Living: %d | Density: %.1f%% | Status: %s%s\n",
		generation, livingCells, density, status, boundingInfo)
	fmt.Printf("Performance: %.1f gen/sec | Avg Pop: %.1f | Runtime: %.1fs\n",
		stats.GenerationsPerSecond, stats.AveragePopulation, time.Since(stats.StartTime).Seconds())

	// Show time since last restart
	if generation > lastRestartGen {
		fmt.Printf("Generations since restart: %d\n", generation-lastRestartGen)
	}
	fmt.Println()
}

// checkRestartConditions determines if the game should restart
func checkRestartConditions(
	livingCells, stagnantCount, generation int,
	config utils.Config,
) (bool, string) {
	if livingCells == 0 {
		return true, "extinction"
	}
	if stagnantCount >= config.StagnationThreshold {
		return true, "stagnation detected"
	}
	if generation > 0 && generation%200 == 0 {
		return true, "periodic refresh"
	}
	return false, ""
}

// restartGame handles the game restart logic
func restartGame(config utils.Config) *model.Grid {
	fmt.Printf("\n🔄 Restarting...\n")
	time.Sleep(1 * time.Second)

	grid := model.NewGrid(config.Width, config.Height)
	grid.ResetWithInterestingPatterns(config)

	fmt.Printf("✨ New patterns loaded! Living cells: %d\n", grid.CountLivingCells())
	time.Sleep(2 * time.Second)

	return grid
}
