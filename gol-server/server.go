package main

import (
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
	// "uk.ac.bris.cs/gameoflife/util"
)

// worker struct stores the address of a worker alongside the client object
// This helps us handle worker disconnects and reconnects more cleanly
type worker struct {
	Client  *rpc.Client
	Address string
}

// Global variables
var (
	controller *rpc.Client
	workers    []*worker
	keypresses chan rune
	listener   net.Listener
)

// Setup variables on program load
func init() {
	keypresses = make(chan rune, 10)
	workers = make([]*worker, 0)
}

// Send a portion of the board to a worker to process the turn for
// Copy the result into the correct place in the new board
func doWorker(board [][]bool, newBoard [][]bool, start, end int, worker *rpc.Client, wg *sync.WaitGroup) {
	defer wg.Done()

	// Spawn a new worker thread
	response := stubs.DoTurnResponse{}
	err := worker.Call(stubs.WorkerDoTurn, stubs.DoTurnRequest{
		Board: board, FragStart: start, FragEnd: end}, &response)
	if err != nil {
		fmt.Println(err)
	}
	// Copy the fragment back into the board
	for row := response.Frag.StartRow; row < response.Frag.EndRow; row++ {
		copy(newBoard[row], response.Frag.Cells[row-response.Frag.StartRow])
	}
}

// Update board is called every time we want to process a turn
// This will partition the board up and send each fragment to a worker
// Workers will copy the new turn onto the newBoard slice
func updateBoard(board [][]bool, newBoard [][]bool, height, width int) {
	// Calculate the number of rows each worker thread should use
	numWorkers := len(workers)
	fragHeight := height / numWorkers

	// Create a WaitGroup so we only return when all workers have finished
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for worker := 0; worker < numWorkers; worker++ {
		// Rows to process turns for
		start := worker * fragHeight
		end := (worker + 1) * fragHeight
		// Final worker gets any rows that can't be evenly split
		if worker == numWorkers-1 {
			end = height
		}
		// Update this fragment
		go doWorker(board, newBoard, start, end, workers[worker].Client, &wg)
	}
	wg.Wait()
}

// This function contains the game loop and sends messages to the controller
// It will return when the final turn is completed or there is an error
// When it returns, the controller is disconnected and the server can accept new connections
func controllerLoop(board [][]bool, height, width, maxTurns int) {
	// When loop is finished, disconnect controller
	defer func() {
		controller.Close()
		controller = nil
		println("Disconnected Controller")
		// w.Start()
	}()

	ticker := time.NewTicker(2 * time.Second)

	turn := 0
	// Make a new board buffer
	newBoard := make([][]bool, height)
	for row := 0; row < height; row++ {
		newBoard[row] = make([]bool, width)
	}
	println("Max turns: ", maxTurns)

	// Update the board each turn
	for turn < maxTurns {
		select {
		// Handle incoming keypresses
		case key := <-keypresses:
			println("Received keypress: ", key)
			switch key {
			case 'q':
				// Tell the controller we're quitting
				controller.Call(stubs.ControllerGameStateChange,
					stubs.StateChangeReport{Previous: stubs.Executing, New: stubs.Quitting, CompletedTurns: turn}, &stubs.Empty{})
				println("Closing controller")
				return
			case 'p':
				println("Pausing execution")
				// Send a "pause" event
				controller.Call(stubs.ControllerGameStateChange,
					stubs.StateChangeReport{Previous: stubs.Executing, New: stubs.Paused, CompletedTurns: turn}, &stubs.Empty{})
				// Wait for another p key
				for <-keypresses != 'p' {
				}
				// Send a "resume" event
				controller.Call(stubs.ControllerGameStateChange,
					stubs.StateChangeReport{Previous: stubs.Paused, New: stubs.Executing, CompletedTurns: turn}, &stubs.Empty{})
				println("Resuming execution")
			case 's':
				println("Telling controller to save board")
				// Send the board to the controller to save
				controller.Call(stubs.ControllerSaveBoard,
					stubs.SaveBoardRequest{CompletedTurns: maxTurns, Height: height, Width: width, Board: board}, &stubs.Empty{})
			case 'k':
				println("Controller wants to close everything")

				for w := 0; w < len(workers); w++ {
					println("Disconnecting worker", w)
					workers[w].Client.Call(stubs.WorkerShutdown, stubs.Empty{}, &stubs.Empty{})
					workers[w].Client.Close()
				}

				// Send them a final turn complete
				controller.Call(stubs.ControllerFinalTurnComplete,
					stubs.SaveBoardRequest{
						CompletedTurns: turn,
						Height:         height,
						Width:          width,
						Board:          board,
					},
					&stubs.Empty{})

				// close ourselves
				listener.Close()
				return
				// EXTENSION:
				// If the client presses R then the board will be randomised
			case 'r':
				println("Randomising Board")
				randomiseBoard(board, height, width)
			}
		// Tell the controller how many cells are alive every 2 seconds
		case <-ticker.C:
			println("Telling controller number of cells alive")
			err := controller.Call(stubs.ControllerReportAliveCells,
				stubs.AliveCellsReport{CompletedTurns: turn, NumAlive: len(util.GetAliveCells(board))}, &stubs.Empty{})
			if err != nil {
				fmt.Println("Error sending num alive ", err)
				return
			}
		// If there are no other interruptions, handle the game turn
		default:
			updateBoard(board, newBoard, height, width)
			// Copy the board buffer over to the input board
			for row := 0; row < height; row++ {
				copy(board[row], newBoard[row])
			}

			// println("Sending turn complete")
			// Tell the controller we have completed a turn
			// Do this concurrently since we don't need to wait for the controller
			// controller.Call(stubs.ControllerTurnComplete,
			// 	stubs.SaveBoardRequest{CompletedTurns: maxTurns, Height: height, Width: width, Board: board}, &stubs.Empty{})
			turn++
		}

	}

	println("All turns done, send final turn complete")
	// Once all turns are done, tell the controller the final turn is complete
	err := controller.Call(stubs.ControllerFinalTurnComplete,
		stubs.SaveBoardRequest{
			CompletedTurns: maxTurns,
			Height:         height,
			Width:          width,
			Board:          board,
		},
		&stubs.Empty{})
	if err != nil {
		fmt.Println("Error sending final turn complete ", err)
		return
	}
	return
}

// This will randomise a board
func randomiseBoard(board [][]bool, height, width int) {
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			// Get a random number from 0.0-1.0
			r := rand.Float32()
			// For a smaller number of alive cells, reduce ratio
			ratio := float32(0.2)
			if r < ratio {
				board[row][col] = true
			} else {
				board[row][col] = false
			}
		}
	}
}

// Server structure for RPC functions
type Server struct{}

// StartGame is called by the controller when it wants to connect and start a game
func (s *Server) StartGame(req stubs.StartGameRequest, res *stubs.ServerResponse) (err error) {
	println("Received request to start a game")
	// If we already have a controller respond false
	if controller != nil {
		println("We already have a controller")
		res.Message = "Server already has a controller"
		res.Success = false
		return
	}

	// Controllers can't connect if we have no workers
	if len(workers) == 0 {
		println("We have no workers available")
		res.Message = "Server has no workers"
		res.Success = false
		return
	}

	// Connect to the new controller's RPC server
	newController, err := rpc.Dial("tcp", req.ControllerAddress)
	if err != nil {
		println("Error connecting to controller: ", err.Error())
		res.Message = "Failed to connect to you"
		res.Success = false
		return err
	}

	// If successful store the controller reference
	controller = newController
	println("Controller connected")
	res.Success = true
	res.Message = "Connected!"

	// Run the controller handler goroutine
	go controllerLoop(req.Board, req.Height, req.Width, req.MaxTurns)
	return
}

// RegisterKeypress is called by controller when a key is pressed on their SDL window
func (s *Server) RegisterKeypress(req stubs.KeypressRequest, res *stubs.ServerResponse) (err error) {
	println("Received keypress request")
	keypresses <- req.Key
	return
}

// ConnectWorker is called by workers who want to connect
func (s *Server) ConnectWorker(req stubs.WorkerConnectRequest, res *stubs.ServerResponse) (err error) {
	println("Worker at", req.WorkerAddress, "wants to connect")
	// Try to connect to the worker's RPC
	workerClient, err := rpc.Dial("tcp", req.WorkerAddress)
	if err != nil {
		println("Error connecting to worker: ", err.Error())
		return err
	}

	// If successful add the worker to the workers slice
	newWorker := worker{Address: req.WorkerAddress, Client: workerClient}
	foundExisting := false
	// Make sure we don't already contain this worker
	for w := 0; w < len(workers); w++ {
		if workers[w].Address == req.WorkerAddress {
			println("Duplicate worker, disconnecting and reconnecting")
			// We are already connected to this worker
			// It's possible they disconnected, just close the previous connection
			workers[w].Client.Close()
			workers[w] = &newWorker
			foundExisting = true
			break
		}
	}
	// If they don't already exist then add them as a new worker
	if !foundExisting {
		workers = append(workers, &newWorker)
	}

	println("Worker added! We now have", len(workers), "workers.")

	res.Message = "Connected!"
	res.Success = true
	return
}

// Ping exists so workers can poll their connection to us
func (s *Server) Ping(req stubs.Empty, res *stubs.Empty) (err error) {
	// No need to do anything here
	return
}

func main() {
	// Read in the network port we should listen on, from the commandline argument.
	// Default to port 8030
	// portPtr := flag.String("port", ":8030", "port to listen on")
	// flag.Parse()
	println("Started server")

	// w.Start()
	// w.Text(" Awaiting connections")

	// Register our RPC server
	rpc.Register(&Server{})

	// Create a listener to handle rpc requests
	ln, _ := net.Listen("tcp", "localhost:8020")
	listener = ln

	// This will block until the listener is closed
	rpc.Accept(listener)

	println("Server closed")
}
