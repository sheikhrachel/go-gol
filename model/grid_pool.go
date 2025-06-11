package model

import "sync"

// GridToPool returns a grid to the pool for reuse
func GridToPool(grid *Grid, pool *GridPool) {
	if pool == nil {
		return
	}

	pool.Put(grid)
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

// Get retrieves a grid from the pool, resetting its dimensions
func (p *GridPool) Get(width, height int) *Grid {
	g := p.pool.Get().(*Grid)
	g.Reset(width, height)
	return g
}

// Put returns a grid to the pool, clearing its state
func (p *GridPool) Put(g *Grid) {
	// Clear the grid before returning to pool
	g.Clear()
	p.pool.Put(g)
}
