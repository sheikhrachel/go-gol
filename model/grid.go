package model

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"runtime"

	"golang.org/x/sync/errgroup"

	"github.com/sheikhrachel/go-gol/rules"
	"github.com/sheikhrachel/go-gol/utils"
)

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

// GetWidth returns the width of the grid
func (g *Grid) GetWidth() int {
	return g.width
}

// GetHeight returns the height of the grid
func (g *Grid) GetHeight() int {
	return g.height
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
					if rules.ApplyConwayRules(g.CountNeighborsOptimized(x, y), g.cells[y][x]) {
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

			if rules.ApplyConwayRules(neighbors, alive) {
				next.cells[y][x] = true
			}
		}
	}

	next.calculateActiveBounds()
	return next
}

// NextGeneration calculates the next generation based on configuration
func (g *Grid) NextGeneration(config utils.Config, pool *GridPool) *Grid {
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
		g.Set(rand.Intn(g.width), rand.Intn(g.height), true)
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
func (g *Grid) ResetWithInterestingPatterns(config utils.Config) {
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
