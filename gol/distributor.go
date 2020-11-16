package gol

import (
	"strconv"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioInput    <-chan uint8
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	//grid[row][column]

	grid := make([][]bool, p.ImageHeight)

	// Make a column array for each row
	for row := 0; row < p.ImageHeight; row++ {
		grid[row] = make([]bool, p.ImageWidth)
	}

	//Load the image
	c.ioCommand <- ioInput
	// Filename is width x height
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	println("Reading in file", filename)
	c.ioFilename <- filename
	// Read in the data
	gridFromFileInput(grid, p.ImageHeight, p.ImageWidth, c.ioInput, c.events)

	c.events <- TurnComplete{0}
	// turn++
	// After the setup turn increment to turn = 1
	for turn := 1; turn <= p.Turns; turn++ {
		grid = doTurn(grid, turn, c.events)
		c.events <- TurnComplete{turn}
	}
	// println(makeAliveCells(grid))

	c.events <- FinalTurnComplete{
		p.Turns,
		makeAliveCells(grid),
	}

	// TODO: Execute all turns of the Game of Life.
	// TODO: Send correct Events when required, e.g. CellFlipped, TurnComplete and FinalTurnComplete.
	//		 See event.go for a list of all events.

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

//Populate a grid from a file input channel, sending events on cells set to alive
func gridFromFileInput(grid [][]bool, height, width int, fileInput <-chan uint8, events chan<- Event) {
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			cell := <-fileInput
			// Set the cell value to the corresponding image pixel
			if cell == 0 {
				grid[row][col] = false
			} else {
				grid[row][col] = true
				// Since the cell is being set to alive, call a CellFlipped event
				events <- CellFlipped{
					CompletedTurns: 0,
					Cell:           util.Cell{X: col, Y: row},
				}
			}
		}
	}
}

/*

any live cell with fewer than two live neighbours dies
any live cell with two or three live neighbours is unaffected
any live cell with more than three live neighbours dies
any dead cell with exactly three live neighbours becomes alive

*/

func nextCellState(x int, y int, grid [][]bool) bool {
	adj := getNeighbours(x, y, grid)

	newState := false

	if grid[y][x] == true {
		if adj < 2 {
			newState = false
		} else if adj > 3 {
			newState = false
		} else {
			newState = true
		}
	} else {
		if adj == 3 {
			newState = true
		} else {
			newState = false
		}
	}
	return newState
}

func getNeighbours(x int, y int, grid [][]bool) int {
	height := len(grid)
	width := len(grid[0])
	numNeighbours := 0

	// Check all cells in a grid around this one
	for _x := -1; _x < 2; _x++ {
		for _y := -1; _y < 2; _y++ {
			//this cell is not a neighbour
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

			v := grid[wrapY][wrapX]
			if v == true {
				numNeighbours++
			}
		}
	}

	return numNeighbours
}

func doTurn(grid [][]bool, turn int, events chan<- Event) [][]bool {
	height := len(grid)
	width := len(grid[0])
	newGrid := make([][]bool, height)

	// Iterate over each cell
	for row := 0; row < height; row++ {
		newGrid[row] = make([]bool, width)
		for col := 0; col < width; col++ {

			// Calculate the next cell state
			newCell := nextCellState(col, row, grid)

			// If the cell has flipped then raise an event
			if newCell != grid[row][col] {
				events <- CellFlipped{
					CompletedTurns: turn,
					Cell:           util.Cell{X: col, Y: row},
				}
			}

			// Update the value of the new cell
			newGrid[row][col] = newCell

		}
	}
	return newGrid
}

// returns the alive cells in the grid
func makeAliveCells(grid [][]bool) []util.Cell {
	height := len(grid)
	width := len(grid[0])
	aliveCells := make([]util.Cell, 0)
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if grid[row][col] == true {
				aliveCells = append(aliveCells, util.Cell{X: col, Y: row})
			}
		}
	}
	return aliveCells
}
