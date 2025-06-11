package rules

/*
ApplyConwayRules applies Conway's Game of Life rules to determine the next state of a cell.

Conway's Game of Life rules: (alive && neighbors == 2) || neighbors == 3
*/
func ApplyConwayRules(neighbors int, alive bool) bool {
	return (alive && neighbors == 2) || neighbors == 3
}
