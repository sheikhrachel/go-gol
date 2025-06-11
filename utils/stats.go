package utils

import "time"

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
	return &Stats{StartTime: time.Now()}
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
