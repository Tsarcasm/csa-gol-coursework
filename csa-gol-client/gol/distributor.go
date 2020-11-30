package gol

import (
	"encoding/gob"
	"net"
	"reflect"

	"strconv"

	"uk.ac.bris.cs/gameoflife/stubs"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioInput    <-chan uint8
	ioOutput   chan<- uint8
	signals    <-chan stubs.Signals
}

func init() {
	gob.RegisterName("AliveCellsCount", &AliveCellsCount{})
	gob.RegisterName("ImageOutputComplete", &ImageOutputComplete{})
	gob.RegisterName("StateChange", &StateChange{})
	gob.RegisterName("CellFlipped", &CellFlipped{})
	gob.RegisterName("TurnComplete", &TurnComplete{})
	gob.RegisterName("FinalTurnComplete", &FinalTurnComplete{})
	gob.RegisterName("BoardSave", &BoardSave{})
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
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	println("Reading in file", filename)
	c.ioFilename <- filename

	// Covnert image to grid
	gridFromFileInput(grid, p.ImageHeight, p.ImageWidth, c.ioInput, c.events)

	// Open a connection to the server
	conn, _ := net.Dial("tcp", "localhost:8030")

	encoder := gob.NewEncoder(conn)
	decoder := gob.NewDecoder(conn)

	msg := stubs.BoardMsg{
		Height:   p.ImageHeight,
		Width:    p.ImageWidth,
		MaxTurns: p.Turns,
		Board:    grid,
	}

	// Send the boardmsg
	encoder.Encode(msg)

	// Run the goroutine to handle sending signals
	go sendSignals(encoder, c)

	// Loop and handle events
ConnectionLoop:
	for {
		var response Event
		err := decoder.Decode(&response)
		if err != nil {
			print("Decode Error!", err.Error())
			break
		}

		// We want to handle some specific events
		// Such as a board save event
		switch e := response.(type) {
		case *BoardSave:
			saveGrid(e, p, c)
			// If this is the final turn output
			if e.CompletedTurns == p.Turns {
				break ConnectionLoop
			}
		// case *AliveCellsCount:
		// 	c.events <- e.(AliveCellsCount)
		// case *StateChange:
		// 	c.events <- e.(StateChange)
		// case *CellFlipped:
		// 	c.events <- e.(CellFlipped)
		// case *TurnComplete:
		// 	c.events <- e.(TurnComplete)
		// case *FinalTurnComplete:
		// 	c.events <- e.(FinalTurnComplete)
		default:
			println(reflect.TypeOf(e.(Event)).String())
			c.events <- e.(Event)
			// println("Unhandled Event")
		}

		// // print("Received message")
	}

	// c.events <- TurnComplete{0}

	// Make a grid buffer to store the next grid state into

	// println("frag height", p.ImageHeight/p.Threads)

	// Finally, save the image to a new file
	// saveGrid(grid, turn, p, c)

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// c.events <- StateChange{turn, Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}

func sendSignals(encoder *gob.Encoder, c distributorChannels) {
	for {
		signal := <-c.signals
		encoder.Encode(signal)
	}
}

func saveGrid(saveEvent *BoardSave, p Params, c distributorChannels) {
	c.ioCommand <- ioOutput
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(saveEvent.CompletedTurns)
	println("Saving to file", filename)
	c.ioFilename <- filename
	gridToFileOutput(saveEvent.Board, p.ImageHeight, p.ImageWidth, c.ioOutput)
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
			}
		}
	}
}

func gridToFileOutput(grid [][]bool, height, width int, fileOutput chan<- uint8) {
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			// If true send 1, else send 0
			if grid[row][col] {
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
