package gol

import (
	"strconv"

	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioInput    <-chan uint8
}

type Fragment struct {
	startRow int
	endRow   int
	cells    [][]bool
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	//grid[row][column]
	ticker := time.NewTicker(2000 * time.Millisecond)
	grid := make([][]bool, p.ImageHeight)

	// Make a column array for each row
	for row := 0; row < p.ImageHeight; row++ {
		grid[row] = make([]bool, p.ImageWidth)
	}

	//Load the image
	c.ioCommand <- ioInput
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	println("Reading in file", filename)
	c.ioFilename <- filename
	// Covnert image to grid
	gridFromFileInput(grid, p.ImageHeight, p.ImageWidth, c.ioInput, c.events)
	c.events <- TurnComplete{0}

	// Make a grid buffer to store the next grid state into

	gridFragments := make(chan Fragment, p.Threads)

	println("frag height", p.ImageHeight/p.Threads)
	var wg sync.WaitGroup
	// Now we can do the game loop
	for turn := 1; turn <= p.Turns; turn++ {
		gridBuffer := make([][]bool, p.ImageHeight)
		for row := 0; row < p.ImageHeight; row++ {
			gridBuffer[row] = make([]bool, p.ImageWidth)
		}
		fragHeight := p.ImageHeight / p.Threads
		for thread := 0; thread < p.Threads; thread++ {
			start := thread * fragHeight
			end := (thread + 1) * fragHeight
			if thread == p.Threads-1 {
				end = p.ImageHeight
			}
			if turn == 1 {
				println(start, end)
			}
			go turnThread(grid, turn, c.events, start, end, gridFragments)
		}

		for thread := 0; thread < p.Threads; thread++ {
			fragment := <-gridFragments
			wg.Add(1)
			go func() {
				defer wg.Done()
				for row := fragment.startRow; row < fragment.endRow; row++ {
					for col := 0; col < p.ImageWidth; col++ {
						gridBuffer[row][col] = fragment.cells[row-fragment.startRow][col]
					}
				}
			}()

		}
		wg.Wait()
		grid = gridBuffer
		c.events <- TurnComplete{turn}
		select {
		case <-ticker.C:
			c.events <- AliveCellsCount{len(makeAliveCells(grid)), turn}
		default:

		}

	}
	// println(makeAliveCells(grid))

	c.events <- FinalTurnComplete{
		p.Turns,
		makeAliveCells(grid),
	}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func turnThread(grid [][]bool, turn int, events chan<- Event, startRow, endRow int, fragments chan<- Fragment) {
	width := len(grid[0])
	gridFragment := Fragment{
		startRow,
		endRow,
		make([][]bool, endRow-startRow),
	}

	// Iterate over each cell
	for row := startRow; row < endRow; row++ {

		gridFragment.cells[row-startRow] = make([]bool, width)
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
			gridFragment.cells[row-startRow][col] = newCell

		}
	}
	fragments <- gridFragment
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
