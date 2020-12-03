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

// Fragment represents a section of the game board
// startRow points to the row the section starts at  (inclusive lower bound)
// endRow points to the row after the end of the section (exclusive upper bound)
// cells is a slice containing the rows for this section
type Fragment struct {
	startRow int
	endRow   int
	cells    [][]bool
}

// distributor splits the work between threads and executes the game loop
// this function will return when the game has finished or is quit
// when it returns it will close the events channel
func distributor(p Params, c distributorChannels) {
	board := make([][]bool, p.ImageHeight)
	// Make a column array for each row
	for row := 0; row < p.ImageHeight; row++ {
		board[row] = make([]bool, p.ImageWidth)
	}

	loadBoard(c, p, board)

	// Send an event since the board has now been setup
	c.events <- TurnComplete{0}

	// Run the game loop and store how many turns it processes
	endTurn := gameLoop(board, p, c)

	// If we've finished all turns ensure turns = the last turn we did
	if endTurn > p.Turns {
		endTurn = p.Turns
	}

	// Send a final turn complete event
	c.events <- FinalTurnComplete{
		endTurn,
		getAliveCells(board),
	}

	// Finally, save the image to a new file
	saveBoard(board, endTurn, p, c)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{endTurn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

// This function runs through all of the turns in the game
// At the start of every turn, it handles input and other interrupts
// It will return the number of turns it has processed
// Contents of the board slice are set to the most recent turn result
func gameLoop(board [][]bool, p Params, c distributorChannels) int {
	var wg sync.WaitGroup

	// this channel stores the results of the worker threads
	boardFragments := make(chan Fragment, p.Threads)

	// Make a board buffer to store the next board state into
	boardBuffer := make([][]bool, p.ImageHeight)
	for row := 0; row < p.ImageHeight; row++ {
		boardBuffer[row] = make([]bool, p.ImageWidth)
	}

	// Ticker triggers us to send an aliveCellsCount event every 2 seconds
	ticker := time.NewTicker(2 * time.Second)

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

			go workerThread(board, turn, c.events, start, end, boardFragments)
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

		// Check for the ticker or keypresses
		select {
		// Every ticker tick send an AliveCellsCount event
		case <-ticker.C:
			c.events <- AliveCellsCount{
				turn,
				len(getAliveCells(board)),
			}
			// Handle any keypresses
		case key := <-c.keyPresses:
			fmt.Println("Keypress:", key)
			switch key {
			case 's':
				saveBoard(board, turn, p, c)
			case 'q':
				// To quit, just return the turn
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
			// If there are no keypresses or tickers, go to the next turn
		}

	}
	return turn
}

// WorkerThread will take a board and calculate the next turn state of a portion of it
// Pass the whole board slice along with start and end pointers for the section it needs to process
// It will send the next turn state as a fragment down the fragments channel
func workerThread(board [][]bool, turn int, events chan<- Event, startRow, endRow int, fragments chan<- Fragment) {
	width := len(board[0])
	// Setup the new fragment
	boardFragment := Fragment{
		startRow,
		endRow,
		make([][]bool, endRow-startRow),
	}

	// Iterate over the rows we need to process
	for row := startRow; row < endRow; row++ {
		// Instantiate this row's new cells
		boardFragment.cells[row-startRow] = make([]bool, width)
		for col := 0; col < width; col++ {
			// Calculate the next cell state
			newCell := nextCellState(col, row, board)

			// If the cell has flipped then send an event
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
	// Send the completed fragment down the channel
	fragments <- boardFragment
}

// Save a board slice to the file
// This will properly prepare all the channels for writing
func saveBoard(board [][]bool, turn int, p Params, c distributorChannels) {
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(turn)
	println("Saving to file", filename)

	// Set the IO channels to prepare for writing
	c.ioCommand <- ioOutput
	c.ioFilename <- filename

	boardToFileOutput(board, p.ImageHeight, p.ImageWidth, c.ioOutput)
}

// Load a board slice from a file
// This will properly prepare all the channels for reading
func loadBoard(c distributorChannels, p Params, board [][]bool) {
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	println("Reading in file", filename)

	// Set the IO channels to prepare for reading
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	boardFromFileInput(board, p.ImageHeight, p.ImageWidth, c.ioInput, c.events)
}

// Populate a board from a file input channel, sending events on cells set to alive
// Before this is run, two channels must be set:
// ioCommand <- input
// ioFilename <- "name"
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

// Save a file with the contents of a board
// Before this is run, two channels must be set:
// ioCommand <- input
// ioFilename <- "name"
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

// Calculate the next cell state according to Game Of Life rules
// Returns a bool with the next state of the cell
func nextCellState(x int, y int, board [][]bool) bool {
	// Count the number of adjacent alive cells
	adj := countAliveNeighbours(x, y, board)

	// Default to dead
	newState := false

	// Find what will make the cell alive

	if board[y][x] == true {
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
func countAliveNeighbours(x int, y int, board [][]bool) int {
	height := len(board)
	width := len(board[0])
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
			v := board[wrapY][wrapX]
			if v == true {
				numNeighbours++
			}
		}
	}

	return numNeighbours
}

// Returns a slice with all the alive cells in the board
func getAliveCells(board [][]bool) []util.Cell {
	height := len(board)
	width := len(board[0])
	aliveCells := make([]util.Cell, 0)
	// Check every cell
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			if board[row][col] == true {
				// If this cell is alive, add it to the slice
				aliveCells = append(aliveCells, util.Cell{X: col, Y: row})
			}
		}
	}
	return aliveCells
}
