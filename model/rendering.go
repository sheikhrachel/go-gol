package model

import (
	"fmt"
	"os"
	"os/exec"
)

const (
	gridPosBlock = "██"
	gridPosEmpty = "  "

	macosClearCmd = "clear"
)

// TerminalRenderer implements basic terminal rendering
type TerminalRenderer struct{}

// Display renders the grid to the terminal
func (r *TerminalRenderer) Display(g *Grid) {
	for y := range g.height {
		for x := range g.width {
			if g.Get(x, y) {
				fmt.Print(gridPosBlock)
			} else {
				fmt.Print(gridPosEmpty)
			}
		}
		fmt.Println()
	}
}

// Clear clears the terminal screen
func (r *TerminalRenderer) Clear() {
	var cmd *exec.Cmd
	cmd = exec.Command(macosClearCmd)
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		fmt.Println("Error clearing terminal:", err)
	}
}
