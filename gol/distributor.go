package gol

import (
	"net"
	"net/rpc"
	"time"

	// "time"

	// "os"
	"strconv"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioInput    <-chan uint8
	ioOutput   chan<- uint8
	keypresses <-chan rune
}

type client struct {
	params   Params
	channels distributorChannels
	state    stubs.State
	closed   bool
}

var (
	previous [][]bool
	stopChan chan bool
)

func init() {
	stopChan = make(chan bool)
}

// Todo: improve the comments here

// The server calls this to report a change in game state
func (c *client) GameStateChange(req stubs.StateChangeReport, res *stubs.Empty) (err error) {
	println("Received state change report")
	c.state = req.New
	return
}

// The server calls this after the final turn is completed
func (c *client) FinalTurnComplete(req stubs.SaveBoardRequest, res *stubs.Empty) (err error) {
	if c.closed {
		println("CLOSED")
		println("CLOSED")
		println("CLOSED")
		println("CLOSED")
		println("CLOSED")
		println("CLOSED")
		println("CLOSED")
		println("CLOSED")
		println("CLOSED")
		println("CLOSED")
	}
	println("Final turn complete")

	c.channels.events <- FinalTurnComplete{
		CompletedTurns: req.CompletedTurns,
		Alive:          util.GetAliveCells(req.Board),
	}

	// go saveGrid(req.Board, req.CompletedTurns, c.p, c.c)
	// defer func() { c.stopChan <- true }()
	stopChan <- true
	return
}

func (c *client) TurnComplete(req stubs.SaveBoardRequest, res *stubs.Empty) (err error) {
	for row := 0; row < req.Height; row++ {
		for col := 0; col < req.Width; col++ {
			// If true send 1, else send 0
			if req.Board[row][col] != previous[row][col] {
				c.channels.events <- CellFlipped{
					CompletedTurns: req.CompletedTurns,
					Cell:           util.Cell{X: col, Y: row},
				}
			}
		}
	}
	c.channels.events <- TurnComplete{req.CompletedTurns}
	previous = req.Board
	return
}

// The server calls this when it receives a signal to save the board
func (c *client) SaveBoard(req stubs.SaveBoardRequest, res *stubs.Empty) (err error) {
	println("Received save board request")
	go saveGrid(req.Board, req.CompletedTurns, c.params, c.channels)
	return
}

// The server calls this every 2 seconds to report how many cells are alive
func (c *client) ReportAliveCells(req stubs.AliveCellsReport, res *stubs.Empty) (err error) {
	println("Received alive cells report")
	c.channels.events <- AliveCellsCount{CompletedTurns: req.CompletedTurns, CellsCount: req.NumAlive}
	return
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// defer func() {

	// 	// os.Exit(0)
	// }()
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
	previous = grid

	client := client{params: p, channels: c, state: stubs.Executing}
	// Register our RPC client

	clientSrv := rpc.NewServer()
	clientSrv.Register(&client)
	listener, err := net.Listen("tcp", "localhost:8031")
	// defer listener.Close()
	if err != nil {
		println("Error starting listener:", err.Error())
		return
	}

	go func() {
		// Connect to the RPC server
		server, err := rpc.Dial("tcp", "localhost:8020")
		if err != nil {
			println("Connection error:", err.Error())
			return
		}
		response := new(stubs.ServerResponse)

		// Todo: better loop logic here
		try := 0
		for try < 4 {
			// Ask the server to start a game
			err = server.Call(stubs.ServerStartGame, stubs.StartGameRequest{
				ClientAddress: "localhost:8031",
				Height:        p.ImageHeight,
				Width:         p.ImageWidth,
				MaxTurns:      p.Turns,
				Board:         grid,
			}, response)

			if err != nil {
				println("Connection error:", err.Error())
				return
			} else if response.Success == false {
				println("Server error:", response.Message)
				if try == 3 {
					return
				}
				time.Sleep(200 * time.Millisecond)
				try++
				continue
			}
			break
		}

		// Forward keypresses to the server
		for {
			select {
			case key := <-c.keypresses:
				err = server.Call(stubs.ServerRegisterKeypress, stubs.KeypressRequest{Key: key}, response)
				if err != nil {
					println("Error sending keypress to server:", err.Error())
				}
			case <-stopChan:
				println("Closing connections")
				client.closed = true
				server.Close()
				listener.Close()
				return
			}
		}
	}()
	clientSrv.Accept(listener)
	println()
	time.Sleep(100 * time.Millisecond)
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{p.Turns, stubs.Quitting}
	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	defer close(c.events)
}

func saveGrid(grid [][]bool, completedTurns int, p Params, c distributorChannels) {
	c.ioCommand <- ioOutput
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(completedTurns)
	println("Saving to file", filename)
	c.ioFilename <- filename
	gridToFileOutput(grid, p.ImageHeight, p.ImageWidth, c.ioOutput)
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
				events <- CellFlipped{
					CompletedTurns: 0,
					Cell:           util.Cell{X: col, Y: row},
				}
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
