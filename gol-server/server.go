package main

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"

	"github.com/gernest/wow"
	"github.com/gernest/wow/spin"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
	// "uk.ac.bris.cs/gameoflife/util"
)

type worker struct {
	Client  *rpc.Client
	Address string
}

var (
	client     *rpc.Client
	workers    []*worker
	keypresses chan rune
	w          *wow.Wow
)

// Setup variables
func init() {
	keypresses = make(chan rune, 10)
	workers = make([]*worker, 0)
	w = wow.New(os.Stdout, spin.Get(spin.Arc), "")
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
// This will partition the grid up and send each fragment to a worker
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

// This function contains the game loop and sends messages to the client
// It will return when the final turn is completed or there is an error
// When it returns, the client is disconnected and the server can accept new connections
func clientLoop(board [][]bool, height, width, maxTurns int) {
	// When loop is finished, disconnect client
	defer func() {
		client.Close()
		client = nil
		println("Disconnected Client")
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
				// Tell the client we're quitting
				client.Call(stubs.ClientGameStateChange,
					stubs.StateChangeReport{Previous: stubs.Executing, New: stubs.Quitting, CompletedTurns: turn}, &stubs.Empty{})
				println("Closing client")
				return
			case 'p':
				println("Pausing execution")
				// Send a "pause" event
				client.Call(stubs.ClientGameStateChange,
					stubs.StateChangeReport{Previous: stubs.Executing, New: stubs.Paused, CompletedTurns: turn}, &stubs.Empty{})
				// Wait for another p key
				for <-keypresses != 'p' {
				}
				// Send a "resume" event
				client.Call(stubs.ClientGameStateChange,
					stubs.StateChangeReport{Previous: stubs.Paused, New: stubs.Executing, CompletedTurns: turn}, &stubs.Empty{})
				println("Resuming execution")
			case 's':
				println("Telling client to save board")
				// Send the board to the client to save
				client.Call(stubs.ClientSaveBoard,
					stubs.SaveBoardRequest{CompletedTurns: maxTurns, Height: height, Width: width, Board: board}, &stubs.Empty{})

			}
		// Tell the client how many cells are alive every 2 seconds
		case <-ticker.C:
			println("Telling client number of cells alive")
			err := client.Call(stubs.ClientReportAliveCells,
				stubs.AliveCellsReport{CompletedTurns: turn, NumAlive: len(util.GetAliveCells(board))}, &stubs.Empty{})
			if err != nil {
				fmt.Println("Error sending num alive ", err)
				return
			}
		// If there are no other interruptions, handle the game turn
		default:
			updateBoard(board, newBoard, height, width)
			// Copy the grid buffer over to the input grid
			for row := 0; row < height; row++ {
				copy(board[row], newBoard[row])
			}

			println("Sending turn complete")
			// Tell the client we have completed a turn
			// Do this concurrently since we don't need to wait for the client
			client.Call(stubs.ClientTurnComplete,
				stubs.SaveBoardRequest{CompletedTurns: maxTurns, Height: height, Width: width, Board: board}, &stubs.Empty{})
			turn++
		}

	}

	println("All turns done, send final turn complete")
	// Once all turns are done, tell the client the final turn is complete
	err := client.Call(stubs.ClientFinalTurnComplete,
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

// Server structure for RPC functions
type Server struct{}

// StartGame is called by the client when it wants to connect and start a game
func (s *Server) StartGame(req stubs.StartGameRequest, res *stubs.ServerResponse) (err error) {
	println("Received request to start a game")
	// If we already have a client respond false
	if client != nil {
		println("We already have a client")
		res.Message = "Server already has a client"
		res.Success = false
		return
	}

	if len(workers) == 0 {
		println("We have no workers available")
		res.Message = "Server has no workers"
		res.Success = false
		return
	}

	// Connect to the new client's RPC server
	newClient, err := rpc.Dial("tcp", req.ClientAddress)
	if err != nil {
		println("Error connecting to client: ", err.Error())
		res.Message = "Failed to connect to you"
		res.Success = false
		return err
	}

	// If successful store the client reference
	client = newClient
	w.Stop()
	println("Client connected")
	res.Success = true
	res.Message = "Connected!"

	// Run the client handler goroutine
	go clientLoop(req.Board, req.Height, req.Width, req.MaxTurns)
	return
}

// RegisterKeypress is called by clients when a key is pressed on their SDL window
func (s *Server) RegisterKeypress(req stubs.KeypressRequest, res *stubs.ServerResponse) (err error) {
	println("Received keypress request")
	keypresses <- rune(req.Key)
	return
}

// ConnectWorker is called by workers who want to connect
func (s *Server) ConnectWorker(req stubs.WorkerConnectRequest, res *stubs.ServerResponse) (err error) {
	println("Worker at", req.WorkerAddress, "wants to connect")
	// Try to connect to the workers RPC
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
	if !foundExisting {
		workers = append(workers, &newWorker)
	}

	println("Worker added! We now have", len(workers), "workers.")

	res.Message = "Connected!"
	res.Success = true
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

	// Register our RPC client
	rpc.Register(&Server{})

	// Create a listener to handle rpc requests
	listener, _ := net.Listen("tcp", "localhost:8020")
	defer listener.Close()
	rpc.Accept(listener)
	println("Server closed")
}
