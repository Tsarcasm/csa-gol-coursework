package gol

import (
	"fmt"
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
	ioOutput   chan<- uint8
	keyPresses <-chan rune
}

type Fragment struct {
	startRow int
	endRow   int
	cells    [][]bool
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	ticker := time.NewTicker(2 * time.Second)
	board := make([][]bool, p.ImageHeight)

	// Make a column array for each row
	for row := 0; row < p.ImageHeight; row++ {
		board[row] = make([]bool, p.ImageWidth)
	}

	//Load the image
	c.ioCommand <- ioInput
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	println("Reading in file", filename)
	c.ioFilename <- filename
	// Covnert image to board
	boardFromFileInput(board, p.ImageHeight, p.ImageWidth, c.ioInput, c.events)
	c.events <- TurnComplete{0}

	// Make a board buffer to store the next board state into

	boardFragments := make(chan Fragment, p.Threads)

	println("frag height", p.ImageHeight/p.Threads)
	var wg sync.WaitGroup

	// Now we can do the game loop
	boardBuffer := make([][]bool, p.ImageHeight)
	for row := 0; row < p.ImageHeight; row++ {
		boardBuffer[row] = make([]bool, p.ImageWidth)
	}
	turn := gameLoop(p, board, c, boardFragments, &wg, boardBuffer, ticker)

	// If we've finished all turns ensure turns = the last turn we did
	if turn > p.Turns {
		turn = p.Turns
	}

	c.events <- FinalTurnComplete{
		turn,
		makeAliveCells(board),
	}

	// Finally, save the image to a new file
	saveBoard(board, turn, p, c)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func gameLoop(p Params, board [][]bool, c distributorChannels, boardFragments chan Fragment, wg *sync.WaitGroup, boardBuffer [][]bool, ticker *time.Ticker) int {
	// Starting turn is 1
	turn := 1
	fragHeight := p.ImageHeight / p.Threads
	for ; turn <= p.Turns; turn++ {
		// Divide the board into fragments
		// Give each to a worker goroutine
		for thread := 0; thread < p.Threads; thread++ {
			start := thread * fragHeight
			end := (thread + 1) * fragHeight

			// Give any remaining rows to the last worker
			if thread == p.Threads-1 {
				end = p.ImageHeight
			}

			go turnThread(board, turn, c.events, start, end, boardFragments)
		}

		// Get the result in from the threads
		// Copy them into the new board
		for thread := 0; thread < p.Threads; thread++ {
			fragment := <-boardFragments
			wg.Add(1)

			// Copy the fragment into board buffer
			go func() {
				defer wg.Done()
				for row := fragment.startRow; row < fragment.endRow; row++ {
					for row := fragment.startRow; row < fragment.endRow; row++ {
						copy(boardBuffer[row], fragment.cells[row-fragment.startRow])
					}
				}
			}()

		}
		// Wait for all the fragments to be copied in
		wg.Wait()

		// Copy the board buffer back into the board
		for row := 0; row < p.ImageHeight; row++ {
			copy(board[row], boardBuffer[row])
		}

		// Send a turn complete event
		c.events <- TurnComplete{turn}
		select {
		// Every ticker tick send an AliveCellsCount event
		case <-ticker.C:
			c.events <- AliveCellsCount{
				turn,
				len(makeAliveCells(board)),
			}
			//
		case key := <-c.keyPresses:
			fmt.Println("Keypress:", key)
			switch key {
			case 's':
				saveBoard(board, turn, p, c)
			case 'q':
				// To quit, just return the turn
				// Send a quit event
				c.events <- StateChange{turn, Quitting}
				return turn
			case 'p':
				println("Pausing on turn", turn)
				// Send a pause event
				c.events <- StateChange{turn, Paused}
				// Wait for another p keypress
				for <-c.keyPresses != 'p' {
				}
				c.events <- StateChange{turn, Executing}
				println("Continuing")
			}

		default:
		}

	}
	return turn
}

func saveBoard(board [][]bool, turn int, p Params, c distributorChannels) {
	c.ioCommand <- ioOutput
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turn)
	println("Saving to file", filename)
	c.ioFilename <- filename
	boardToFileOutput(board, p.ImageHeight, p.ImageWidth, c.ioOutput)
}

func turnThread(board [][]bool, turn int, events chan<- Event, startRow, endRow int, fragments chan<- Fragment) {
	width := len(board[0])
	boardFragment := Fragment{
		startRow,
		endRow,
		make([][]bool, endRow-startRow),
	}

	// Iterate over each cell
	for row := startRow; row < endRow; row++ {

		boardFragment.cells[row-startRow] = make([]bool, width)
		for col := 0; col < width; col++ {

			// Calculate the next cell state
			newCell := nextCellState(col, row, board)

			// If the cell has flipped then raise an event
			if newCell != board[row][col] {
				events <- CellFlipped{
					CompletedTurns: turn,
					Cell:           util.Cell{X: col, Y: row},
				}
			}

			// Update the value of the new cell
			boardFragment.cells[row-startRow][col] = newCell

		}
	}
	fragments <- boardFragment
}

//Populate a board from a file input channel, sending events on cells set to alive
func boardFromFileInput(board [][]bool, height, width int, fileInput <-chan uint8, events chan<- Event) {
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			cell := <-fileInput
			// Set the cell value to the corresponding image pixel
			if cell == 0 {
				board[row][col] = false
			} else {
				board[row][col] = true
				// Since the cell is being set to alive, call a CellFlipped event
				events <- CellFlipped{
					CompletedTurns: 0,
					Cell:           util.Cell{X: col, Y: row},
				}
			}
		}
	}
}

func boardToFileOutput(board [][]bool, height, width int, fileOutput chan<- uint8) {
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			// If true send 1, else send 0
			if board[row][col] {
				fileOutput <- 1
			} else {
				fileOutput <- 0
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

func nextCellState(x int, y int, board [][]bool) bool {
	adj := getNeighbours(x, y, board)

	newState := false

	if board[y][x] == true {
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

func getNeighbours(x int, y int, board [][]bool) int {
	height := len(board)
	width := len(board[0])
	numNeighbours := 0

	// Check all cells in a board around this one
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

			v := board[wrapY][wrapX]
			if v == true {
				numNeighbours++
			}
		}
	}

	return numNeighbours
}

// returns the alive cells in the board
func makeAliveCells(board [][]bool) []util.Cell {
	height := len(board)
	width := len(board[0])
	aliveCells := make([]util.Cell, 0)
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if board[row][col] == true {
				aliveCells = append(aliveCells, util.Cell{X: col, Y: row})
			}
		}
	}
	return aliveCells
}
