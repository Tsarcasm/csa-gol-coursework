package main

import (
	"flag"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
)

// worker struct stores the address of a worker alongside the client object
// This helps us handle worker disconnects and reconnects more cleanly
type worker struct {
	Client  *rpc.Client
	Address string
}

// Global variables
var (
	controller      *rpc.Client
	controllerMutex sync.Mutex
	lastBoardState  [][]bool
	lastTurn        int

	workers      []*worker
	workersMutex sync.Mutex
	keypresses   chan rune
	listener     net.Listener
)

// Setup variables on program start
func init() {
	keypresses = make(chan rune, 10)
	workers = make([]*worker, 0)
}

/////////

// Game loop functions are in loop.go

/////////

// Server structure for RPC functions
type Server struct{}

// StartGame is called by the controller when it wants to connect and start a game
func (s *Server) StartGame(req stubs.StartGameRequest, res *stubs.ServerResponse) (err error) {
	// Lock the controller until we have finished
	controllerMutex.Lock()
	defer controllerMutex.Unlock()
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
		res.Message = "Failed to connect to controller"
		res.Success = false
		return err
	}

	var newBoard [][]bool
	startTurn := 0
	if req.StartNew {
		println("Starting a new game!")
		newBoard = req.Board.ToSlice()
	} else {
		println("Client resuming previous game")
		// The client wants to resume
		if lastBoardState == nil {
			// We need a previous board to resume from
			println("Error resuming board: no previous board")
			res.Message = "Error resuming: no previous board"
			res.Success = false
			return
		}

		// Continue with the previous
		// Make sure height and width match
		if req.Height != len(lastBoardState) || req.Width != len(lastBoardState[0]) {
			println("Error resuming board: controller has the wrong height and width")
			res.Message = "Error resuming: controller had the wrong height and width"
			res.Success = false
			return
		}
		// Copy the last board state
		newBoard = make([][]bool, req.Height)
		for row := 0; row < req.Height; row++ {
			newBoard[row] = make([]bool, req.Width)
			copy(newBoard[row], lastBoardState[row])
		}
		println("Resuming at turn ", lastTurn)
		startTurn = lastTurn
	}

	// If successful store the controller reference
	controller = newController
	println("Controller connected")
	res.Success = true
	res.Message = "Connected!"

	// Run the controller loop goroutine
	go controllerLoop(newBoard, startTurn, req.Height, req.Width, req.MaxTurns, req.Threads, req.VisualUpdates)
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

// Entrypoint
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
	ln, _ := net.Listen("tcp", ":"+*portPtr)
	listener = ln

	// This will block until the listener is closed
	rpc.Accept(listener)

	println("Server closed")
}
