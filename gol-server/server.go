package main

import (
	"flag"
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

	workers      []*worker
	workersMutex sync.Mutex
	keypresses   chan rune
	listener     net.Listener
)

// Setup variables on program load
func init() {
	keypresses = make(chan rune, 10)
	workers = make([]*worker, 0)
}

// Send a portion of the board to a worker to process the turn for
// Copy the result into the correct place in the new board
func doWorker(halo stubs.Halo, newBoard [][]bool, threads int, worker *worker, wg *sync.WaitGroup, failFlag *bool, failMu *sync.Mutex) {
	defer wg.Done()

	// Spawn a new worker thread
	response := stubs.DoTurnResponse{}

	// Send the halo to the client, get the result
	err := worker.Client.Call(stubs.WorkerDoTurn,
		stubs.DoTurnRequest{Halo: halo, Threads: threads}, &response)
	if err != nil {
		println("Error getting fragment:", err.Error())
		// If we encounter an error then set the fail flag to true
		// Lock the mutex here to get exclusive access
		failMu.Lock()
		*failFlag = true
		failMu.Unlock()
		// Disconnect the worker
		disconnectWorker(worker)
		return
	}
	// Copy the fragment back into the board
	respCells := response.Frag.BitBoard.ToSlice()
	for row := response.Frag.StartRow; row < response.Frag.EndRow; row++ {
		copy(newBoard[row], respCells[row-response.Frag.StartRow])
	}
}

// Create a "halo" of cells containing only the cells required to calculat the next turn
// Take the whole board and return a halo which can be passed to a worker
func makeHalo(worker int, fragHeight int, numWorkers int, height, width int, board [][]bool) stubs.Halo {
	// This will hold all the cells that will be stored in  the halo
	cells := make([][]bool, 0)

	// Find the boundaries for this worker
	start := worker * fragHeight
	end := (worker + 1) * fragHeight
	if worker == numWorkers-1 {
		end = height
	}

	// DownPtr and UpPtr point to the rows of the board below and abole the boundary this worker calculates for
	downPtr := end % height // "max row + 1"
	upPtr := (start - 1)    // "min row - 1"
	if upPtr == -1 {
		upPtr = height - 1
	}
	// WorkPtr points to the the row the worker should start calculating new cells for
	workPtr := 0

	// Add the UpPtr row if wouldn't be in the board anyway
	if upPtr != end-1 {
		cells = append(cells, board[upPtr])
		workPtr = 1
	}
	// Add rows we want to calculate the next turn of
	for row := start; row < end; row++ {
		cells = append(cells, board[row])
	}
	// Add the DownPtr row if isn't included in the board
	if downPtr != start {
		cells = append(cells, board[downPtr])
	}
	// Return a new halo for these cells
	return stubs.Halo{
		BitBoard: stubs.BitBoardFromSlice(cells, len(cells), width), // Convert the grid into a bitboard
		Offset:   workPtr,
		StartPtr: start,
		EndPtr:   end,
	}
}

// Update board is called every time we want to process a turn
// This will partition the board up and send each fragment to a worker
// Workers will copy the new turn onto the newBoard slice
// Returns true if there have been no errors (and the whole board has been set)
func updateBoard(board [][]bool, newBoard [][]bool, height, width int, threads int) bool {
	// Create a WaitGroup so we only return when all workers have finished
	var wg sync.WaitGroup
	// EXTENSION: Worker goroutines will flag if a worker fails to communicate
	// We can then disconnect the worker and retry the turn
	// Assume we start with no fails
	failFlag := false
	failMu := &sync.Mutex{}

	// Lock workers so no new workers can be added / removed until all goroutines are started
	workersMutex.Lock()

	// Calculate the number of rows each worker thread should use
	numWorkers := len(workers)
	fragHeight := height / numWorkers
	// The waitgroup will wait for all workers to finish
	wg.Add(numWorkers)

	for w := 0; w < numWorkers; w++ {
		thisWorker := workers[w]
		go func(workerIdx int, worker *worker) {
			// Get all the cells required to update this fragment
			halo := makeHalo(workerIdx, fragHeight, numWorkers, height, width, board)
			// Send the fragment to the worker
			doWorker(halo, newBoard, threads, worker, &wg, &failFlag, failMu)
		}(w, thisWorker)
	}

	// We can release workers now
	workersMutex.Unlock()
	// Wait for all workers to finish
	wg.Wait()

	// Check that there have been no fails
	if failFlag {
		// One or more of the workers have hit a problem
		return false
	}
	return true
}

// This function contains the game loop and sends messages to the controller
// It will return when the final turn is completed or there is an error
// When it returns, the controller is disconnected and the server can accept new connections
func controllerLoop(board [][]bool, height, width, maxTurns, threads int, visualUpdates bool) {
	// When loop is finished, disconnect controller
	defer func() {
		controller.Close()
		controller = nil
		println("Disconnected Controller")
	}()

	// This ticker signals us to send turns complete every 2 seconds
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
					stubs.SaveBoardRequest{CompletedTurns: maxTurns, Board: stubs.BitBoardFromSlice(board, height, width)}, &stubs.Empty{})
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
						Board:          stubs.BitBoardFromSlice(board, height, width),
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
			// Make the RPC call
			err := controller.Call(stubs.ControllerReportAliveCells,
				stubs.AliveCellsReport{CompletedTurns: turn, NumAlive: len(util.GetAliveCells(board))}, &stubs.Empty{})
			// If there was an error then the client has disconnected, stop the game
			if err != nil {
				fmt.Println("Error sending num alive ", err)
				return
			}
		// If there are no other interruptions, handle the game turn
		default:
			// Get the next board state (this will send calls to workers)
			success := updateBoard(board, newBoard, height, width, threads)

			if success {
				// Copy the board buffer over to the input board
				for row := 0; row < height; row++ {
					copy(board[row], newBoard[row])
				}
				if visualUpdates {
					// println("Sending turn complete")
					// Tell the controller we have completed a turn
					// Do this concurrently since we don't need to wait for the controller
					controller.Call(stubs.ControllerTurnComplete,
						stubs.SaveBoardRequest{CompletedTurns: turn, Board: stubs.BitBoardFromSlice(board, height, width)}, &stubs.Empty{})
				}
				turn++
			} else {
				// We hit a problem (e.g. a worker disconnected)
				// Retry the turn
				println("Encountered a problem handling turn", turn)
				println("Retrying this turn")
			}
		}

	}

	println("All turns done, send final turn complete")
	// Once all turns are done, tell the controller the final turn is complete
	err := controller.Call(stubs.ControllerFinalTurnComplete,
		stubs.SaveBoardRequest{
			CompletedTurns: maxTurns,
			Board:          stubs.BitBoardFromSlice(board, height, width),
		},
		&stubs.Empty{})
	if err != nil {
		fmt.Println("Error sending final turn complete ", err)
	}
	// End the game
	return
}

// EXTENSION: Randomise board function
// This will randomise a board
func randomiseBoard(board [][]bool, height, width int) {
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			// Get a random number from 0.0-1.0
			r := rand.Float32()
			// For a smaller number of alive cells, reduce the ratio
			ratio := float32(0.2)
			if r < ratio {
				board[row][col] = true
			} else {
				board[row][col] = false
			}
		}
	}
}

// Cleanly disconnect a worker and remove it from the workers slice
func disconnectWorker(worker *worker) {
	// Lock the workers slice to get exclusive access
	workersMutex.Lock()
	defer workersMutex.Unlock()

	// Find the index of the worker
	for w := 0; w < len(workers); w++ {
		if workers[w].Address == worker.Address {
			// Try and close the RPC connection
			worker.Client.Close()
			// Rebuild the workers slice without this one in it
			workers = append(workers[:w], workers[w+1:]...)
			println("Worker", worker.Address, "disconnected")
			return
		}
	}
	// We don't contain this worker, do nothing
	println("We aren't connected to worker", worker.Address)
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

	// Run the controller loop goroutine
	go controllerLoop(req.Board.ToSlice(), req.Height, req.Width, req.MaxTurns, req.Threads, req.VisualUpdates)
	return
}

// RegisterKeypress is called by controller when a key is pressed on their SDL window
func (s *Server) RegisterKeypress(req stubs.KeypressRequest, res *stubs.ServerResponse) (err error) {
	println("Received keypress request")
	// Send the keypress down down the keypresses channel
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

	// Lock the slice to get exclusive access
	workersMutex.Lock()

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

	// Unlock the mutex
	workersMutex.Unlock()

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
	portPtr := flag.String("p", "8020", "port to listen on")
	flag.Parse()
	println("Started server")
	println("Our RPC port:", *portPtr)

	// Register our RPC server
	rpc.Register(&Server{})

	// Create a listener to handle rpc requests
	ln, _ := net.Listen("tcp", "localhost:"+*portPtr)
	listener = ln

	// This will block until the listener is closed
	rpc.Accept(listener)

	println("Server closed")
}
