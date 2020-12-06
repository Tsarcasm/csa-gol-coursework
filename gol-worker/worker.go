package main

import (
	"flag"
	// "fmt"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
)

// Global variables
var (
	server        *rpc.Client
	serverAddress string
	ourAddress    string
)

// Worker is the struct for our RPC server
type Worker struct{}

// DoTurn is called by the server when it wants to calculate a new turn
// It will pass the board and fragment pointers
func (w *Worker) DoTurn(req stubs.DoTurnRequest, res *stubs.DoTurnResponse) (err error) {
	// fmt.Print(".")
	// Get the turn result
	frag := doTurn(req.Halo, req.Threads)
	res.Frag = frag

	return
}

// Shutdown is called by the server to disconnect and close the worker
func (w *Worker) Shutdown(req stubs.Empty, res *stubs.Empty) (err error) {
	println("Received shutdown request")
	server.Close()
	os.Exit(0)
	return
}

func main() {
	// ip := util.GetPublicIP()
	defer println("Closing worker")
	// Read in the network port we should listen on, from the commandline argument.
	portPtr := flag.String("p", "8010", "port to listen on")
	// Read in the network address of the server, from the commandline
	serverAddressPtr := flag.String("s", "localhost:8020", "server address")

	// Store addresses
	flag.Parse()
	// ourAddress = ip + ":" + *portPtr // "localhost:" + *portPtr
	ourAddress = "localhost:" + *portPtr
	serverAddress = *serverAddressPtr
	println("Starting worker (" + ourAddress + ")")

	// Register our RPC client
	rpc.Register(&Worker{})

	// Create a listener to handle rpc requests
	listener, _ := net.Listen("tcp", ourAddress)
	defer listener.Close()
	// Asynchronously handle RPC requests
	go rpc.Accept(listener)

	// Try and connect to the server for the first time
	connectToServer()

	// Ticker to ping the server every 10 seconds
	pingTicker := time.NewTicker(2 * time.Second)
	for {
		select {
		// Ping the server at an interval
		case <-pingTicker.C:
			// If we are connected, ping them
			if server != nil {
				// Ping the server
				err := server.Call(stubs.ServerPing, stubs.Empty{}, &stubs.Empty{})

				//If there is an error in pinging them, we have lost connection
				if err != nil {
					println("Error pinging server:", err.Error())

					// Close the connection anyway
					server.Close()
					server = nil
					println("Disconnected")
				}
			} else {
				// Otherwise, attempt to connect to the server
				connectToServer()
			}

		}
	}
}

// Attempt to connect to the server
// Returns true if we successfully connected
// This will also set the server global variable
func connectToServer() bool {
	println("Attempting to connect to server ", serverAddress)
	// Try and establish a connection to the server
	newServer, err := rpc.Dial("tcp", serverAddress)

	if err != nil {
		println("Cannot find server:", err.Error())
		return false
	}
	server = newServer
	response := new(stubs.ServerResponse)

	// If we have a connection, try and register ourselves as a worker
	err = server.Call(stubs.ServerConnectWorker,
		stubs.WorkerConnectRequest{WorkerAddress: ourAddress}, response)
	if err != nil {
		println("Connection error", err.Error())
		return false
	} else if response.Success == false {
		println("Server error", response.Message)
		return false
	}

	// No errors, connection successful!
	println("Connected!")
	return true
}

// GAME LOGIC BELOW

// Calculate the next turn, given pointers to the start and end to operate over
// Return a fragment of the board with the next turn's cells
func doTurn(halo stubs.Halo, threads int) (boardFragment stubs.Fragment) {
	width := halo.BitBoard.RowLength
	board := halo.BitBoard.Bytes.Decode()
	height := halo.EndPtr - halo.StartPtr
	newBoard := make([][]bool, height)

	// Don't allow there to be more threads than rows
	if threads > height {
		threads = height
	}

	var wg sync.WaitGroup
	// Split the board into threads
	fragHeight := height / threads
	for i := 0; i < threads; i++ {
		// Calculate the bounds for this thread
		start := i * fragHeight
		end := (i + 1) * fragHeight
		if i == threads-1 {
			end = height
		}
		// Add this thread to the waitgroup
		wg.Add(1)
		// Iterate over each cell
		go updateRegion(start, end, halo, newBoard, width, board, &wg)
	}

	// Wait for all threads to finish
	wg.Wait()

	// Create a fragment from the results of the threads
	boardFragment = stubs.Fragment{
		StartRow: halo.StartPtr,
		EndRow:   halo.EndPtr,
		BitBoard: stubs.BitBoardFromSlice(newBoard, halo.EndPtr-halo.StartPtr, width), // Create a new bitboard
	}

	return boardFragment

}

// Calculate the next cell state for all cells within bounds
func updateRegion(start, end int, halo stubs.Halo, newBoard [][]bool, width int, board []byte, wg *sync.WaitGroup) {
	// Iterate through the region
	for row := start; row < end; row++ {
		newBoard[row] = make([]bool, width)
		for col := 0; col < width; col++ {
			// Apply game of life rules to this cell
			newCell := nextCellState(col, row+halo.Offset, board, halo.BitBoard.NumRows, halo.BitBoard.RowLength)
			// Save the result in the new board
			newBoard[row][col] = newCell
		}
	}
	wg.Done()
}

// Calculate the next cell state according to Game Of Life rules
// Returns a bool with the next state of the cell
func nextCellState(x int, y int, board []byte, bHeight, bWidth int) bool {
	// Count the number of adjacent alive cells
	adj := countAliveNeighbours(x, y, board, bHeight, bWidth)

	// Default to dead
	newState := false

	// Find what will make a cell come to life
	if stubs.GetBitArrayCell(board, bHeight, bWidth, y, x) == true {
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
func countAliveNeighbours(x int, y int, board []byte, height, width int) int {
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
			v := stubs.GetBitArrayCell(board, height, width, wrapY, wrapX)
			if v == true {
				numNeighbours++
			}
		}
	}

	return numNeighbours
}
