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
type Controller struct {
	params   Params
	channels controllerChannels
	state    stubs.State
}

var (
	previous [][]bool
	stopChan chan bool
)

func init() {
	stopChan = make(chan bool)
}

// Todo: improve the comments here

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
		stopChan <- true
	}
	return
}

// FinalTurnComplete is called by the server when it has processed all turns
// It will send the final board which can then be saved
func (c *Controller) FinalTurnComplete(req stubs.SaveBoardRequest, res *stubs.Empty) (err error) {
	println("Final turn complete")
	// Send an event
	c.channels.events <- FinalTurnComplete{
		CompletedTurns: req.CompletedTurns,
		Alive:          util.GetAliveCells(req.Board),
	}

	//todo re-enable board saving on last turn

	go saveBoard(req.Board, req.CompletedTurns, c.params, c.channels)
	// defer func() { c.stopChan <- true }()
	stopChan <- true
	return
}

// TurnComplete is called by the server when a turn has been completed
// It contains a copy of the board on this turn so we can display it
func (c *Controller) TurnComplete(req stubs.SaveBoardRequest, res *stubs.Empty) (err error) {
	// If any cells have changed then send a cellflipped event
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
	// Send a turn complete event
	c.channels.events <- TurnComplete{req.CompletedTurns}
	// Update the previous board to the new one
	previous = req.Board
	return
}

// SaveBoard is called by the server when it wants us to save the board (e.g. if we send an 's' key)
func (c *Controller) SaveBoard(req stubs.SaveBoardRequest, res *stubs.Empty) (err error) {
	println("Received save board request")
	// Save the board
	go saveBoard(req.Board, req.CompletedTurns, c.params, c.channels)
	return
}

// ReportAliveCells is called by the server to report how many cells are alive
// This is usually called at regular intervals
func (c *Controller) ReportAliveCells(req stubs.AliveCellsReport, res *stubs.Empty) (err error) {
	println("Received alive cells report")
	println("Turn:", req.CompletedTurns, ",", req.NumAlive)
	// Send an event
	c.channels.events <- AliveCellsCount{CompletedTurns: req.CompletedTurns, CellsCount: req.NumAlive}
	return
}

// distributor divides the work between workers and interacts with other goroutines.
func controller(p Params, c controllerChannels) {
	println("Starting new game")
	// Create a new board to store 0th turn
	board := make([][]bool, p.ImageHeight)
	// Make a column array for each row
	for row := 0; row < p.ImageHeight; row++ {
		board[row] = make([]bool, p.ImageWidth)
	}

	// Prepare IO for reading
	c.ioCommand <- ioInput
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight)
	c.ioFilename <- filename
	println("Reading in file", filename)

	// Load the image and store it in the board
	boardFromFileInput(board, p.ImageHeight, p.ImageWidth, c.ioInput, c.events)
	previous = board

	// Create a RPC server for ourselves
	controller := Controller{params: p, channels: c, state: stubs.Executing}
	controllerRPC := rpc.NewServer()
	controllerRPC.Register(&controller)

	// Start a listener to accept incoming RPC calls
	listener, err := net.Listen("tcp", "localhost:"+p.Port)
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

	c.events <- StateChange{p.Turns, stubs.Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	defer close(c.events)
}

func runGame(p Params, c controllerChannels, board [][]bool, controller Controller, listener net.Listener) {
	server, err := rpc.Dial("tcp", p.ServerAddress)
	if err != nil {
		println("Connection error:", err.Error())
		return
	}
	response := new(stubs.ServerResponse)

	try := 0
	for try < 4 {

		err = server.Call(stubs.ServerStartGame, stubs.StartGameRequest{
			ControllerAddress: "localhost:" + p.Port,
			Height:            p.ImageHeight,
			Width:             p.ImageWidth,
			MaxTurns:          p.Turns,
			Threads:           p.Threads,
			Board:             board,
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

	for {
		select {
		case key := <-c.keypresses:
			err = server.Call(stubs.ServerRegisterKeypress, stubs.KeypressRequest{Key: key}, response)
			if err != nil {
				println("Error sending keypress to server:", err.Error())
			}
		case <-stopChan:
			println("Closing connections")
			server.Close()
			listener.Close()
			return
		}
	}

}

func saveBoard(board [][]bool, completedTurns int, p Params, c controllerChannels) {
	c.ioCommand <- ioOutput
	filename := strconv.Itoa(p.ImageWidth) + "x" + strconv.Itoa(p.ImageHeight) + "x" + strconv.Itoa(completedTurns)
	println("Saving to file", filename)
	c.ioFilename <- filename
	boardToFileOutput(board, p.ImageHeight, p.ImageWidth, c.ioOutput)
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
