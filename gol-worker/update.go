package main

import (
	"sync"
	"uk.ac.bris.cs/gameoflife/stubs"
)

// Calculate the next cell state for all cells within bounds
func updateRegion(start, end int, halo stubs.Halo, newBoard [][]bool, width int, board []byte, wg *sync.WaitGroup) {
	// Iterate through the region
	for row := start; row < end; row++ {
		newBoard[row] = make([]bool, width)
		for col := 0; col < width; col++ {
			// Apply game of life rules to this cell
			newCell := nextCellState(col, row+halo.Offset, board, halo.BitBoard.NumRows, halo.BitBoard.RowLength)
			// Save the result in the new board
			newBoard[row][col] = newCell
		}
	}
	wg.Done()
}

// Calculate the next cell state according to Game Of Life rules
// Returns a bool with the next state of the cell
func nextCellState(x int, y int, board []byte, bHeight, bWidth int) bool {
	// Count the number of adjacent alive cells
	adj := countAliveNeighbours(x, y, board, bHeight, bWidth)

	// Default to dead
	newState := false

	// Find what will make a cell come to life
	if stubs.GetBitArrayCell(board, bHeight, bWidth, y, x) == true {
		if adj == 2 || adj == 3 {
			// If only 2 or 3 neighbours then stay alive
			newState = true
		}
	} else {
		if adj == 3 {
			// If there are 3 neighbours then come alive
			newState = true
		}
	}
	return newState
}

// Count how many alive neighbours a cell has
// This will correctly wrap around edges
func countAliveNeighbours(x int, y int, board []byte, height, width int) int {
	numNeighbours := 0

	// Count all alive cells in the board in a
	// 1 cell radius of the centre
	for _x := -1; _x < 2; _x++ {
		for _y := -1; _y < 2; _y++ {
			// Ignore the centre cell
			if _x == 0 && _y == 0 {
				continue
			}

			wrapX := (x + _x) % width
			//wrap left->right
			if wrapX == -1 {
				wrapX = width - 1
			}

			//wrap top->bottom
			wrapY := (y + _y) % height
			if wrapY == -1 {
				wrapY = height - 1
			}

			// test if this cell is alive
			v := stubs.GetBitArrayCell(board, height, width, wrapY, wrapX)
			if v == true {
				numNeighbours++
			}
		}
	}

	return numNeighbours
}
