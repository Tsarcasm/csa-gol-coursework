package main
/* 

import (
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type engineChannels struct {
	events chan<- gol.Event
	// saveChan chan<- boardState
	signals <-chan stubs.Signals
}

type engineParams struct {
	boardHeight int
	boardWidth  int
	numThreads  int
	maxTurns    int
}

type boardState struct {
	grid [][]bool
	turn int
}

func engineLoop(grid [][]bool, p engineParams, c engineChannels) {
	//grid[row][column]
	ticker := time.NewTicker(2 * time.Second)

	// Send the initial cellFlipped events for the starting grid
	for row := 0; row < p.boardHeight; row++ {
		for col := 0; col < p.boardWidth; col++ {
			if grid[row][col] == true {
				c.events <- gol.CellFlipped{
					CompletedTurns: 0,
					Cell:           util.Cell{X: col, Y: row},
				}
			}
		}
	}

	//Load the image
	c.events <- gol.TurnComplete{CompletedTurns: 0}

	// Make a grid buffer to store the next grid state into
	gridFragments := make(chan stubs.Fragment, p.numThreads)

	println("frag height", p.boardHeight/p.numThreads)
	var wg sync.WaitGroup

	gridBuffer := make([][]bool, p.boardHeight)
	for row := 0; row < p.boardHeight; row++ {
		gridBuffer[row] = make([]bool, p.boardWidth)
	}

	// Now we can do the game loop
	turn := 1
GameLoop:
	for ; turn <= p.maxTurns; turn++ {
		// Make a new grid buffer
		// Calculate the number of rows each worker thread should use
		fragHeight := p.boardHeight / p.numThreads
		for thread := 0; thread < p.numThreads; thread++ {
			// Rows to process turns for
			start := thread * fragHeight
			end := (thread + 1) * fragHeight
			if thread == p.numThreads-1 {
				end = p.boardHeight
			}
			if turn == 1 {
				println(start, end)
			}
			// Spawn a new worker thread
			go turnThread(grid, turn, c.events, start, end, gridFragments)
		}

		// Get the result of each thread
		for thread := 0; thread < p.numThreads; thread++ {
			fragment := <-gridFragments
			wg.Add(1)

			// Stitch the thread results back into the grid
			go func() {
				defer wg.Done()
				for row := fragment.StartRow; row < fragment.EndRow; row++ {
					for col := 0; col < p.boardWidth; col++ {
						gridBuffer[row][col] = fragment.Cells[row-fragment.StartRow][col]
					}
				}
			}()

		}
		wg.Wait()

		// Copy the grid buffer over to the input grid
		for row := 0; row < p.boardHeight; row++ {
			copy(grid[row], gridBuffer[row])
		}

		c.events <- gol.TurnComplete{CompletedTurns: turn}
		select {
		case <-ticker.C:
			c.events <- gol.AliveCellsCount{
				CompletedTurns: turn,
				CellsCount:     len(getAliveCells(grid)),
			}
		case signal := <-c.signals:
			switch signal {
			case stubs.Save:
				c.events <- gol.BoardSave{
					CompletedTurns: turn,
					Board:          grid,
				}
			case stubs.Quit:
				break GameLoop
			case stubs.Pause:
				//Wait for another pause instruction
				println("Pausing on turn", turn)
				// Send a "pause" event
				c.events <- gol.StateChange{
					CompletedTurns: turn,
					NewState:       gol.Paused,
				}
				for <-c.signals != stubs.Pause {
				}
				// Send a "resume" event
				c.events <- gol.StateChange{
					CompletedTurns: turn,
					NewState:       gol.Executing,
				}
				println("Continuing")
			}

		default:
			// Don't wait for an instruction, keep going
		}

	}

	// If we've finished all turns ensure turns = the last turn we did
	if turn > p.maxTurns {
		turn = p.maxTurns
	}

	c.events <- gol.FinalTurnComplete{
		CompletedTurns: turn,
		Alive:          getAliveCells(grid),
	}

	// Finally, save the image to a new file
	c.events <- gol.BoardSave{
		CompletedTurns: turn,
		Board:          grid,
	}
}

func turnThread(grid [][]bool, turn int, events chan<- gol.Event, startRow, endRow int, fragments chan<- stubs.Fragment) {
	width := len(grid[0])
	gridFragment := stubs.Fragment{
		StartRow: startRow,
		EndRow:   endRow,
		Cells:    make([][]bool, endRow-startRow),
	}

	// Iterate over each cell
	for row := startRow; row < endRow; row++ {

		gridFragment.Cells[row-startRow] = make([]bool, width)
		for col := 0; col < width; col++ {

			// Calculate the next cell state
			newCell := nextCellState(col, row, grid)

			// If the cell has flipped then raise an event
			if newCell != grid[row][col] {
				events <- gol.CellFlipped{
					CompletedTurns: turn,
					Cell:           util.Cell{X: col, Y: row},
				}
			}

			// Update the value of the new cell
			gridFragment.Cells[row-startRow][col] = newCell

		}
	}
	fragments <- gridFragment
}

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

// returns the alive cells in the grid
func getAliveCells(grid [][]bool) []util.Cell {
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
*/
 