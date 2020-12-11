package gol

import (
	// "fmt"
	"fmt"
	"net"
	"net/rpc"
	"time"

	// "time"

	// "os"
	"strconv"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// Channel Container structure
type controllerChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioInput    <-chan uint8
	ioOutput   chan<- uint8
	keypresses <-chan rune
}

// Controller structure for the client RPC
// Contains variables specific to the game the controller is running
type Controller struct {
	params   Params
	channels controllerChannels
	state    stubs.State
	previous [][]bool

	lastAliveTurn int
	lastAliveTime time.Time
	// A value is sent down this channel when it is time to close the controller
	stopChan chan bool
}

// GameStateChange is called by the server to report a change in game state
func (c *Controller) GameStateChange(req stubs.StateChangeReport, res *stubs.Empty) (err error) {
	println("Received state change report")
	println(req.Previous.String(), "->", req.New.String())
	// Send an event
	c.channels.events <- StateChange{
		CompletedTurns: req.CompletedTurns,
		NewState:       req.New,
	}
	c.state = req.New
	if req.New == stubs.Quitting {
		c.stopChan <- true
	}
	return
}

// FinalTurnComplete is called by the server when it has processed all turns
// It will send the final board which can then be saved
func (c *Controller) FinalTurnComplete(req stubs.BoardStateReport, res *stubs.Empty) (err error) {
	println("Final turn complete")
	// Send an event
	c.channels.events <- FinalTurnComplete{
		CompletedTurns: req.CompletedTurns,
		Alive:          util.GetAliveCells(req.Board.ToSlice()),
	}

	// Save the board
	go saveBoard(req.Board.ToSlice(), req.CompletedTurns, c.params, c.channels)
	c.stopChan <- true
	return
}

// TurnComplete is called by the server when a turn has been completed
// It contains a copy of the board on this turn so we can display it
func (c *Controller) TurnComplete(req stubs.BoardStateReport, res *stubs.Empty) (err error) {
	// If any cells have changed then send a cellflipped event
	board := req.Board.ToSlice()
	for row := 0; row < req.Board.NumRows; row++ {
		for col := 0; col < req.Board.RowLength; col++ {
			// If true send 1, else send 0
			// If we have no previous board, send the 0th turn events
			if c.previous == nil {
				if board[row][col] == true {
					c.channels.events <- CellFlipped{
						CompletedTurns: req.CompletedTurns,
						Cell:           util.Cell{X: col, Y: row},
					}
				}
			} else if board[row][col] != c.previous[row][col] {
				// Otherwise, only send flipped events if there is a cell change
				c.channels.events <- CellFlipped{
					CompletedTurns: req.CompletedTurns,
					Cell:           util.Cell{X: col, Y: row},
				}
			}
		}
	}
	// Send a turn complete event
	c.channels.events <- TurnComplete{req.CompletedTurns}
	// Update the previous board to the new one
	c.previous = board
	return
}

// SaveBoard is called by the server when it wants us to save the board (e.g. if we send an 's' key)
func (c *Controller) SaveBoard(req stubs.BoardStateReport, res *stubs.Empty) (err error) {
	println("Received save board request")
	// Save the board
	go saveBoard(req.Board.ToSlice(), req.CompletedTurns, c.params, c.channels)
	return
}

// ReportAliveCells is called by the server to report how many cells are alive
// This is usually called at regular intervals
func (c *Controller) ReportAliveCells(req stubs.AliveCellsReport, res *stubs.Empty) (err error) {
	println("Received alive cells report")
	println("Turn:", req.CompletedTurns, ",", req.NumAlive)
	now := time.Now()
	turnsDiff := req.CompletedTurns - c.lastAliveTurn
	timeDiff := now.Sub(c.lastAliveTime)
	fmt.Printf("%.2f", float64(turnsDiff)/timeDiff.Seconds())
	println(" turns/s")

	c.lastAliveTime = now
	c.lastAliveTurn = req.CompletedTurns

	// Send an event
	c.channels.events <- AliveCellsCount{CompletedTurns: req.CompletedTurns, CellsCount: req.NumAlive}
	return
}

// The controller function sets up the controller to connect to the server
// It will also start an RPC server and only returns when this is closed
// When this function ends, it will cleanly close the events channel, signaling the program to halt
func controller(p Params, c controllerChannels) {
	// Create a new board to store 0th turn
	board := make([][]bool, p.ImageHeight)
	// Make a column array for each row
	for row := 0; row < p.ImageHeight; row++ {
		board[row] = make([]bool, p.ImageWidth)
	}

	if p.ResumeGame {
		// If we want to resume a game, no need to load the board
		println("Resuming game from the server")
	} else {
		// Else if we are starting a new game, load the board from a file
		println("Starting new game")

		// Prepare IO for reading
		loadBoard(c, p, board)
	}

	// Create a RPC server for ourselves
	controller := Controller{
		params:   p,
		channels: c,
		state:    stubs.Executing,
		previous: nil,

		lastAliveTurn: 0,
		lastAliveTime: time.Now(),

		stopChan: make(chan bool),
	}
	controllerRPC := rpc.NewServer()
	controllerRPC.Register(&controller)

	// Start a listener to accept incoming RPC calls
	listener, err := net.Listen("tcp", ":"+p.Port)
	if err != nil {
		println("Error starting listener:", err.Error())
		return
	}

	// Start a goroutine to connect to the server and start a game
	go runGame(p, c, board, controller, listener)

	// Block this routiune and handle incoming RPC calls
	// This will return when the listener is closed
	controllerRPC.Accept(listener)

	// At this point the game has ended
	//Gracefully close everything

	println()
	time.Sleep(100 * time.Millisecond)
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	// c.events <- StateChange{p.Turns, stubs.Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	defer close(c.events)
}

// RunGame is responsible for connecting to the server and handling channels from the server
// It will attempt to establish a connection, if this is successful it will then call ServerStartGame
func runGame(p Params, c controllerChannels, board [][]bool, controller Controller, listener net.Listener) {
	// When this function returns, close the listener
	defer listener.Close()
	// Attempt to connect to the server
	server, err := rpc.Dial("tcp", p.ServerAddress)
	defer server.Close()

	if err != nil {
		// If we can't connect to the server then bail
		println("Connection error:", err.Error())
		return
	}

	println("Established connection with the server: ", p.ServerAddress)
	// This contains the response of the StartGame RPC call
	response := new(stubs.ServerResponse)

	// Attempt to start a game with the server
	// We allow for 4 retries incase the server is slow at closing a previous connection
	try := 0
	for ; ; try++ {
		if try == 4 {
			println("Exhausted attempts to start a game, exiting")
			return
		}

		// Ask the server to start a game
		// Pass all the information required to start (or continue) a game
		err = server.Call(stubs.ServerStartGame, stubs.StartGameRequest{
			ControllerAddress: p.OurIP + ":" + p.Port,
			Height:            p.ImageHeight,
			Width:             p.ImageWidth,
			MaxTurns:          p.Turns,
			Threads:           p.Threads,
			Board:             stubs.BitBoardFromSlice(board, p.ImageHeight, p.ImageWidth),
			VisualUpdates:     p.VisualUpdates,
			StartNew:          !p.ResumeGame,
		}, response)

		// No errors, we can start responding to channels
		if err == nil && response.Success {
			println("Game starting!")
			break
		}

		// Print any errors
		if err != nil {
			println("Connection error:", err.Error())
		} else if response.Success == false {
			println("Server error:", response.Message)
		}
		// Delay 0.5 seconds incase the server is still busy
		time.Sleep(500 * time.Millisecond)
	}

	// Handle all keypresses and channel inputs until the game stops
	for {
		select {
		case key := <-c.keypresses:
			// Send any keypresses we receive from SDL to the server
			err = server.Call(stubs.ServerRegisterKeypress, stubs.KeypressRequest{Key: key}, response)
			if err != nil {
				println("Error sending keypress to server:", err.Error())
			}
		case <-controller.stopChan:
			// If we receive a stop signal then exit the game loop
			println("Received stop signal")
			println("Closing RPC server")
			return
		}
	}

}

// Load a board slice from a file
// This will properly prepare all the channels for reading
func loadBoard(c controllerChannels, p Params, board [][]bool) {
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	println("Reading in file", filename)

	// Set the IO channels to prepare for reading
	c.ioCommand <- ioInput
	c.ioFilename <- filename

	// Read the board from the io goroutine in bytes
	boardFromFileInput(board, p.ImageHeight, p.ImageWidth, c.ioInput, c.events)
}

// Save a board slice to the file
// This will properly prepare all the channels for writing
func saveBoard(board [][]bool, completedTurns int, p Params, c controllerChannels) {
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(completedTurns)
	println("Saving to file", filename)

	// Set the IO channels to prepare for writing
	c.ioCommand <- ioOutput
	c.ioFilename <- filename

	// Send the board to the io goroutine in bytes
	boardToFileOutput(board, p.ImageHeight, p.ImageWidth, c.ioOutput)
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
