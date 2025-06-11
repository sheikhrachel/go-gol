package main

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

// Configuration for the game
type Config struct {
	Width               int           `json:"width"`
	Height              int           `json:"height"`
	FrameRate           time.Duration `json:"frame_rate"`
	AutoRestart         bool          `json:"auto_restart"`
	StagnationThreshold int           `json:"stagnation_threshold"`
	UseParallel         bool          `json:"use_parallel"`
	UseMemoryPool       bool          `json:"use_memory_pool"`
	UseBoundedGrid      bool          `json:"use_bounded_grid"`
	MaxGenerations      int           `json:"max_generations"`
	RandomDensity       float64       `json:"random_density"`
	InjectionCount      int           `json:"injection_count"`
	Interactive         bool          `json:"interactive"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Width:               60,
		Height:              30,
		FrameRate:           150 * time.Millisecond,
		AutoRestart:         true,
		StagnationThreshold: 5,
		UseParallel:         true,
		UseMemoryPool:       true,
		UseBoundedGrid:      true, // Enable active region optimization
		MaxGenerations:      1000,
		RandomDensity:       0.15,
		InjectionCount:      3,
		Interactive:         false,
	}
}

// LoadConfig loads configuration from JSON file
func LoadConfig(filename string) (Config, error) {
	config := DefaultConfig()

	data, err := os.ReadFile(filename)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(data, &config)
	return config, err
}

// Conway's Game of Life rules: (alive && neighbors == 2) || neighbors == 3
func applyConwayRules(neighbors int, alive bool) bool {
	return (alive && neighbors == 2) || neighbors == 3
}

// Stats for performance monitoring
type Stats struct {
	GenerationsPerSecond float64
	MemoryUsage          uint64
	AveragePopulation    float64
	TotalGenerations     int
	StartTime            time.Time
	ActiveCells          int
	BoundingBoxSize      int
}

func NewStats() *Stats {
	return &Stats{
		StartTime: time.Now(),
	}
}

func (s *Stats) Update(generation int, population int, duration time.Duration) {
	s.TotalGenerations = generation
	if duration > 0 {
		s.GenerationsPerSecond = 1.0 / duration.Seconds()
	}

	// Simple moving average for population
	if s.AveragePopulation == 0 {
		s.AveragePopulation = float64(population)
	} else {
		s.AveragePopulation = (s.AveragePopulation * 0.9) + (float64(population) * 0.1)
	}
}

// Efficient integer min/max functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TerminalRenderer implements basic terminal rendering
type TerminalRenderer struct{}

func (r *TerminalRenderer) Display(g *Grid) {
	for y := range g.height {
		for x := range g.width {
			if g.Get(x, y) {
				fmt.Print("██")
			} else {
				fmt.Print("  ")
			}
		}
		fmt.Println()
	}
}

func (r *TerminalRenderer) Clear() {
	var cmd *exec.Cmd
	cmd = exec.Command("clear")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// GridPool for memory efficiency
type GridPool struct {
	pool sync.Pool
}

func NewGridPool() *GridPool {
	return &GridPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &Grid{}
			},
		},
	}
}

func (p *GridPool) Get(width, height int) *Grid {
	g := p.pool.Get().(*Grid)
	g.Reset(width, height)
	return g
}

func (p *GridPool) Put(g *Grid) {
	// Clear the grid before returning to pool
	g.Clear()
	p.pool.Put(g)
}

// Grid represents the game board with optional optimizations
type Grid struct {
	width   int
	height  int
	cells   [][]bool
	history []string // Store recent grid states for cycle detection

	// Optional bounded grid optimization
	activeBounds struct {
		minX, maxX, minY, maxY int
		valid                  bool
	}
}

// NewGrid creates a new grid with the specified dimensions
func NewGrid(width, height int) *Grid {
	cells := make([][]bool, height)
	for i := range cells {
		cells[i] = make([]bool, width)
	}
	return &Grid{
		width:  width,
		height: height,
		cells:  cells,
	}
}

// Reset resets the grid to new dimensions
func (g *Grid) Reset(width, height int) {
	g.width = width
	g.height = height
	g.history = nil
	g.activeBounds.valid = false

	// Resize cells if needed
	if len(g.cells) != height {
		g.cells = make([][]bool, height)
	}
	for i := range g.cells {
		if len(g.cells[i]) != width {
			g.cells[i] = make([]bool, width)
		} else {
			// Clear existing cells
			for j := range g.cells[i] {
				g.cells[i][j] = false
			}
		}
	}
}

// Clear clears all cells
func (g *Grid) Clear() {
	for y := range g.height {
		for x := range g.width {
			g.cells[y][x] = false
		}
	}
	g.history = nil
	g.activeBounds.valid = false
}

// Set sets a cell to alive (true) or dead (false)
func (g *Grid) Set(x, y int, alive bool) {
	if x >= 0 && x < g.width && y >= 0 && y < g.height {
		g.cells[y][x] = alive
	}
}

// Get returns the state of a cell
func (g *Grid) Get(x, y int) bool {
	if x < 0 || x >= g.width || y < 0 || y >= g.height {
		return false
	}
	return g.cells[y][x]
}

// CountNeighborsOptimized counts living neighbors with optimized bounds checking
func (g *Grid) CountNeighborsOptimized(x, y int) int {
	count := 0

	// Calculate bounds once using efficient integer min/max
	minX := max(0, x-1)
	maxX := min(g.width-1, x+1)
	minY := max(0, y-1)
	maxY := min(g.height-1, y+1)

	// Count neighbors in the bounded region
	for ny := minY; ny <= maxY; ny++ {
		for nx := minX; nx <= maxX; nx++ {
			if nx == x && ny == y {
				continue // Skip the cell itself
			}
			if g.cells[ny][nx] {
				count++
			}
		}
	}

	return count
}

// calculateActiveBounds calculates the bounding box of living cells
func (g *Grid) calculateActiveBounds() {
	g.activeBounds.valid = false

	for y := range g.height {
		for x := range g.width {
			if g.cells[y][x] {
				if !g.activeBounds.valid {
					g.activeBounds.minX = x
					g.activeBounds.maxX = x
					g.activeBounds.minY = y
					g.activeBounds.maxY = y
					g.activeBounds.valid = true
				} else {
					g.activeBounds.minX = min(g.activeBounds.minX, x)
					g.activeBounds.maxX = max(g.activeBounds.maxX, x)
					g.activeBounds.minY = min(g.activeBounds.minY, y)
					g.activeBounds.maxY = max(g.activeBounds.maxY, y)
				}
			}
		}
	}
}

// GetBoundingBoxSize returns the size of the active region
func (g *Grid) GetBoundingBoxSize() int {
	if !g.activeBounds.valid {
		g.calculateActiveBounds()
	}
	if !g.activeBounds.valid {
		return 0
	}
	return (g.activeBounds.maxX - g.activeBounds.minX + 1) *
		(g.activeBounds.maxY - g.activeBounds.minY + 1)
}

// NextGenerationParallel calculates the next generation using parallel processing
func (g *Grid) NextGenerationParallel(pool *GridPool) *Grid {
	var next *Grid
	if pool != nil {
		next = pool.Get(g.width, g.height)
	} else {
		next = NewGrid(g.width, g.height)
	}

	var (
		eg            errgroup.Group
		numWorkers    = runtime.NumCPU()
		rowsPerWorker = (g.height + numWorkers - 1) / numWorkers // Ceiling division
	)

	for i := range numWorkers {
		var (
			startRow = i * rowsPerWorker
			endRow   = min(startRow+rowsPerWorker, g.height)
		)
		if startRow >= g.height {
			break
		}

		eg.Go(func() error {
			for y := startRow; y < endRow; y++ {
				for x := 0; x < g.width; x++ {
					if applyConwayRules(g.CountNeighborsOptimized(x, y), g.cells[y][x]) {
						next.cells[y][x] = true
					}
				}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		fmt.Printf("Error in parallel processing: %v\n", err)
	}

	return next
}

// NextGenerationBounded calculates next generation only in active region
func (g *Grid) NextGenerationBounded(pool *GridPool) *Grid {
	if !g.activeBounds.valid {
		g.calculateActiveBounds()
	}

	var next *Grid
	if pool != nil {
		next = pool.Get(g.width, g.height)
	} else {
		next = NewGrid(g.width, g.height)
	}

	// If no active cells, return empty grid
	if !g.activeBounds.valid {
		return next
	}

	// Process only the active region + 1 margin
	minX := max(0, g.activeBounds.minX-1)
	maxX := min(g.width-1, g.activeBounds.maxX+1)
	minY := max(0, g.activeBounds.minY-1)
	maxY := min(g.height-1, g.activeBounds.maxY+1)

	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			neighbors := g.CountNeighborsOptimized(x, y)
			alive := g.cells[y][x]

			if applyConwayRules(neighbors, alive) {
				next.cells[y][x] = true
			}
		}
	}

	next.calculateActiveBounds()
	return next
}

// NextGeneration calculates the next generation based on configuration
func (g *Grid) NextGeneration(config Config, pool *GridPool) *Grid {
	if config.UseBoundedGrid {
		return g.NextGenerationBounded(pool)
	}
	return g.NextGenerationParallel(pool)
}

// CountLivingCells returns the total number of living cells
func (g *Grid) CountLivingCells() (count int) {
	for y := range g.height {
		for x := range g.width {
			if g.cells[y][x] {
				count++
			}
		}
	}
	return
}

// GetGridHash returns an efficient MD5 hash of the current grid state
func (g *Grid) GetGridHash() string {
	h := md5.New()
	for y := range g.height {
		for x := range g.width {
			if g.cells[y][x] {
				h.Write([]byte{1})
			} else {
				h.Write([]byte{0})
			}
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// UpdateHistory adds current state to history and maintains size
func (g *Grid) UpdateHistory() {
	g.history = append(g.history, g.GetGridHash())

	// Keep only last 5 states to detect cycles
	if len(g.history) > 5 {
		g.history = g.history[1:]
	}
}

// IsStagnant checks if the grid is stuck in a cycle or static state
func (g *Grid) IsStagnant() bool {
	if len(g.history) < 3 {
		return false
	}

	currentHash := g.GetGridHash()

	// Check for static state and cycles
	if len(g.history) > 0 && g.history[len(g.history)-1] == currentHash {
		return true
	}
	if len(g.history) >= 2 && g.history[len(g.history)-2] == currentHash {
		return true
	}
	if len(g.history) >= 3 && g.history[len(g.history)-3] == currentHash {
		return true
	}

	return false
}

// InjectRandomLife adds some random cells to break stagnation
func (g *Grid) InjectRandomLife(count int) {
	for i := 0; i < count; i++ {
		x := rand.Intn(g.width)
		y := rand.Intn(g.height)
		g.Set(x, y, true)
	}
	// Recalculate bounds after injection
	g.activeBounds.valid = false
}

// Randomize fills the grid with random living cells
func (g *Grid) Randomize(density float64) {
	for y := range g.height {
		for x := range g.width {
			g.Set(x, y, rand.Float64() < density)
		}
	}
}

// AddGlider adds a glider pattern at the specified position
func (g *Grid) AddGlider(startX, startY int) {
	pattern := [][]bool{
		{false, true, false},
		{false, false, true},
		{true, true, true},
	}

	for y, row := range pattern {
		for x, cell := range row {
			g.Set(startX+x, startY+y, cell)
		}
	}
}

// AddOscillator adds a blinker oscillator pattern
func (g *Grid) AddOscillator(startX, startY int) {
	g.Set(startX, startY, true)
	g.Set(startX+1, startY, true)
	g.Set(startX+2, startY, true)
}

// ResetWithInterestingPatterns clears the grid and adds various interesting patterns
func (g *Grid) ResetWithInterestingPatterns(config Config) {
	// Clear the grid
	for y := range g.height {
		for x := range g.width {
			g.cells[y][x] = false
		}
	}

	// Clear history and bounds
	g.history = nil
	g.activeBounds.valid = false

	// Add some simple patterns
	if g.width >= 10 && g.height >= 10 {
		// Add some gliders
		g.AddGlider(5, 5)
		if g.width >= 20 && g.height >= 15 {
			g.AddGlider(g.width-8, 5)
		}

		// Add oscillators
		g.AddOscillator(g.width/4, g.height/4)
		if g.width >= 30 {
			g.AddOscillator(3*g.width/4, 3*g.height/4)
		}
	}

	// Add random life using configurable density
	g.Randomize(config.RandomDensity)
}

// initializeGame sets up the initial game state
func initializeGame(config Config) (*Grid, *GridPool, *TerminalRenderer, *Stats) {
	var pool *GridPool
	if config.UseMemoryPool {
		pool = NewGridPool()
	}

	grid := NewGrid(config.Width, config.Height)
	grid.ResetWithInterestingPatterns(config)

	renderer := &TerminalRenderer{}
	stats := NewStats()

	return grid, pool, renderer, stats
}

// displayGameInfo shows the initial game information
func displayGameInfo(config Config, grid *Grid) {
	fmt.Printf("Features: Memory Pool: %v, Bounded: %v (Parallel: always enabled)\n",
		config.UseMemoryPool, config.UseBoundedGrid)
	fmt.Printf("Grid: %dx%d | Initial living cells: %d\n",
		grid.width, grid.height, grid.CountLivingCells())
	fmt.Println("Press Ctrl+C to exit gracefully")
	fmt.Println()
	time.Sleep(2 * time.Second)
}

// updateGameState updates the game state and returns status information
func updateGameState(grid *Grid, generation int, lastFrameTime time.Time, stats *Stats) (int, float64, string, bool) {
	livingCells := grid.CountLivingCells()
	density := float64(livingCells) / float64(grid.width*grid.height) * 100

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
func displayGameStatus(generation, livingCells int, density float64, status string,
	config Config, grid *Grid, stats *Stats, lastRestartGen int) {

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
func checkRestartConditions(livingCells, stagnantCount, generation int, config Config) (bool, string) {
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
func restartGame(config Config) *Grid {
	fmt.Printf("\n🔄 Restarting...\n")
	time.Sleep(1 * time.Second)

	grid := NewGrid(config.Width, config.Height)
	grid.ResetWithInterestingPatterns(config)

	fmt.Printf("✨ New patterns loaded! Living cells: %d\n", grid.CountLivingCells())
	time.Sleep(2 * time.Second)

	return grid
}

func main() {
	// Load configuration - fallback to defaults if file doesn't exist
	config, err := LoadConfig("config.json")
	if err != nil {
		fmt.Println("Using default configuration (config.json not found)")
		config = DefaultConfig()
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
			gridToPool(grid, pool)

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
		gridToPool(grid, pool)
		grid = newGrid

		generation++

		// Wait before next frame
		time.Sleep(config.FrameRate)
	}
	gridToPool(grid, pool)
}

// Helper function to safely cast GridInterface to *Grid for pool operations
func gridToPool(grid *Grid, pool *GridPool) {
	if pool == nil {
		return
	}

	pool.Put(grid)
}
