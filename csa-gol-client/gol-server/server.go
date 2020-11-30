package main

import (
	"net"
	"net/rpc"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var (
	client     *rpc.Client
	workers    []*rpc.Client
	keypresses chan rune
)

// Setup variables
func init() {
	keypresses := make(chan rune, 10)
	workers := make([]*rpc.Client, 0)
}

// This is called each turn and passed the current board and a newBoard buffer
// This will partition the grid up and send each fragment to a worker
func updateBoard(board [][]bool, newBoard [][]bool, height, width int) {
	// Calculate the number of rows each worker thread should use
	numWorkers := len(workers)
	fragHeight := height / numWorkers

	// Create a WaitGroup to wait for all workers to finish
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
		go doWorker(board, newBoard, start, end, workers[worker], wg)
	}
	wg.Wait()
}

// Send a portion of the board to a worker to process the turn for
// Copy the result into the correct place in the new board
func doWorker(board [][]bool, newBoard [][]bool, start, end int, worker *rpc.Client, wg sync.WaitGroup) {
	defer wg.Done()

	// Spawn a new worker thread
	response := stubs.DoTurnResponse{}
	worker.Call(stubs.WorkerDoTurn, stubs.DoTurnRequest{
		Board: board, FragStart: start, FragEnd: end}, &response)

	// Copy the fragment back into the board
	for row := response.Frag.StartRow; row < response.Frag.EndRow; row++ {
		copy(newBoard[row], response.Frag.Cells[row])
	}

}

// returns the alive cells in the grid
func getAliveCells(grid [][]bool) []util.Cell {
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

func handleClient(board [][]bool, height, width, maxTurns int) {
	// When loop is finished, set client to empty
	defer func() {
		client.Close()
		client = nil
	}()

	ticker := time.NewTicker(2 * time.Second)

	turn := 0
	// Make a new board buffer
	newBoard := make([][]bool, height)
	for row := 0; row < height; row++ {
		newBoard[row] = make([]bool, width)
	}

	// Update the board each turn
	for turn <= maxTurns {
		select {
		case key := <-keypresses:
			println("Received keypress: ", key)
			switch key {
			case 'q':
				// Tell the client we're quitting
				client.Call(stubs.ClientGameStateChange,
					stubs.StateChangeReport{stubs.Executing, stubs.Quitting, turn}, &stubs.ClientResponse{})
				println("Closing client")
				return
			case 'p':
				println("Pausing execution")
				// Send a "pause" event
				client.Call(stubs.ClientGameStateChange,
					stubs.StateChangeReport{stubs.Executing, stubs.Paused, turn}, &stubs.ClientResponse{})
				// Wait for another p key
				for <-keypresses != 'p' {
				}
				// Send a "resume" event
				client.Call(stubs.ClientGameStateChange,
					stubs.StateChangeReport{stubs.Paused, stubs.Executing, turn}, &stubs.ClientResponse{})
				println("Resuming execution")
			case 's':
				println("Telling client to save board")
				// Send the board to the client to save
				client.Call(stubs.ClientSaveBoard, stubs.SaveBoardRequest{
					CompletedTurns: maxTurns,
					Height:         height,
					Width:          width,
					Board:          board,
				}, &stubs.ClientResponse{})

			}
		case <-ticker.C:
			println("Telling client number of cells alive")
			client.Call(stubs.ClientReportCellsAlive,
				stubs.AliveCellsReport{turns, len(getAliveCells(grid))}, &stubs.ClientResponse{})
		default:
			updateBoard(board, newBoard, height, width)
			turn++
		}

	}

	// Once all turns are done, tell the client the final turn is complete
	client.Call(stubs.ClientFinalTurnComplete,
		stubs.SaveBoardRequest{
			CompletedTurns: maxTurns,
			Height:         height,
			Width:          width,
			Board:          board,
		},
		&stubs.ClientResponse{})

}

type Server struct{}

// Methods called by the client
func (s *Server) StartGame(req stubs.StartGameRequest, res *stubs.ServerResponse) (err error) {
	println("Received request to start a game")
	// If we already have a client respond false
	if client != nil {
		println("We already have a client")
		res = &stubs.ServerResponse{false, "Server Busy"}
		return
	}

	if len(workers) != nil {
		println("We have no workers available")
		res = &stubs.ServerResponse{false, "Server has no workers"}
		return
	}

	// Connect to the new client's RPC server
	newClient, err := rpc.Dial("tcp", req.ClientAddress)
	if err != nil {
		println("Error connecting to client: ", err.Error())
		res = &stubs.ServerResponse{false, "Failed to connect to you"}
		return err
	}

	// If successful store the client reference
	client = newClient
	println("Client connected")
	res = &stubs.ServerResponse{true, "Connected!"}

	// Run the client handler goroutine
	go handleClient(req.Board, req.Height, req.Width, req.MaxTurns)
	return
}

func (s *Server) RegisterKeypress(req stubs.KeypressRequest, res *stubs.ServerResponse) (err error) {
	println("Received keypress request")
	keypresses <- req.Key
	return
}

// Methods called by the workers
func (s *Server) ConnectWorker(req stubs.WorkerConnectRequest, res *stubs.ServerResponse) (err error) {
	println("Received reqest by a worker to connect")
	// Try to connect to the workers RPC
	newWorker, err := rpc.Dial("tcp", req.WorkerAddress)
	if err != nil {
		println("Error connecting to worker: ", err.Error())
		res = &stubs.ServerResponse{false, "Failed to connect to  you"}
		return err
	}

	// If successful add the worker to the workers slice
	workers = append(workers, newWorker)
	println("Worker added! We now have", len(workers), "workers.")
	res = &stubs.ServerResponse{true, "Connected!"}
	return
}

func main() {

	// Read in the network port we should listen on, from the commandline argument.
	// Default to port 8030
	// portPtr := flag.String("port", ":8030", "port to listen on")
	// flag.Parse()

	// Register our RPC client
	rpc.Register(&Server{})

	// Create a listener to handle rpc requests
	listener, _ := net.Listen("tcp", "localhost:8020")
	defer listener.Close()
	rpc.Accept(listener)

	//Todo remove this
	println("End of main function")
}
