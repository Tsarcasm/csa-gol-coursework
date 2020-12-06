package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
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
	fmt.Print(".")
	// Get the turn result
	frag := doTurn(req.Halo)
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
	defer println("Closing worker")
	// Read in the network port we should listen on, from the commandline argument.
	portPtr := flag.String("p", "8010", "port to listen on")
	// Read in the network address of the server, from the commandline
	serverAddressPtr := flag.String("s", "localhost:8020", "server address")

	// Store addresses
	flag.Parse()
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
func doTurn(halo stubs.Halo) (boardFragment stubs.Fragment) {
	width := halo.BitBoard.RowLength
	board := halo.BitBoard.Bytes.Decode()
	newBoard := make([][]bool, halo.EndPtr-halo.StartPtr)

	// Iterate over each cell
	for row := 0; row < halo.EndPtr-halo.StartPtr; row++ {
		newBoard[row] = make([]bool, width)
		for col := 0; col < width; col++ {

			// Calculate the next cell state
			newCell := nextCellState(col, row+halo.Offset, board, halo.BitBoard.NumRows, halo.BitBoard.RowLength)

			// Update the value of the new cell
			newBoard[row][col] = newCell

		}
	}
	boardFragment = stubs.Fragment{
		StartRow: halo.StartPtr,
		EndRow:   halo.EndPtr,
		BitBoard: stubs.BitBoardFromSlice(newBoard, halo.EndPtr-halo.StartPtr, width), // Create a new bitboard
	}

	return boardFragment

}

// Calculate the next cell state according to Game Of Life rules
// Returns a bool with the next state of the cell
func nextCellState(x int, y int, board []byte, bHeight, bWidth int) bool {
	// Count the number of adjacent alive cells
	adj := countAliveNeighbours(x, y, board, bHeight, bWidth)

	// Default to dead
	newState := false

	// Find what will make the cell alive
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
