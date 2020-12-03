package main

import (
	// "bufio"
	// 	"encoding/gob"
	"fmt"
	"time"

	// 	"log"
	// 	"net"

	"flag"
	"net"
	"net/rpc"
	"os"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var (
	server     *rpc.Client
	ourAddress string
)

// Worker is the struct for our RPC server
type Worker struct{}

// DoTurn is called by the server when it wants to calculate a new turn
// It will pass the board and fragment pointers
func (w *Worker) DoTurn(req stubs.DoTurnRequest, res *stubs.DoTurnResponse) (err error) {
	fmt.Print(".")
	// Get the turn result
	frag := doTurn(req.Board, req.FragStart, req.FragEnd)
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
	// Default to port 8010
	portPtr := flag.String("p", "8010", "port to listen on")
	flag.Parse()
	ourAddress = "localhost:" + *portPtr
	println("Starting worker (" + ourAddress + ")")

	// Register our RPC client
	rpc.Register(&Worker{})

	// Create a listener to handle rpc requests
	listener, _ := net.Listen("tcp", ourAddress)
	defer listener.Close()
	go rpc.Accept(listener)

	connectToServer()

	// Ticker to ping the server every 10 seconds
	pingTicker := time.NewTicker(6 * time.Second)
	for {
		select {
		// Ping the server at an interval
		case <-pingTicker.C:
			// If we are connected, ping them
			if server != nil {
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
				// Otherwise, attempt to connect
				connectToServer()
			}

		}
	}
}

func connectToServer() bool {
	println("Attempting to connect to server")
	// Try and establish a connection to the server
	newServer, err := rpc.Dial("tcp", "localhost:8020")

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
func doTurn(board [][]bool, startRow, endRow int) (boardFragment stubs.Fragment) {
	width := len(board[0])
	boardFragment = stubs.Fragment{
		StartRow: startRow,
		EndRow:   endRow,
		Cells:    make([][]bool, endRow-startRow),
	}

	// Iterate over each cell
	for row := startRow; row < endRow; row++ {
		boardFragment.Cells[row-startRow] = make([]bool, width)
		for col := 0; col < width; col++ {

			// Calculate the next cell state
			newCell := nextCellState(col, row, board)

			// Update the value of the new cell
			boardFragment.Cells[row-startRow][col] = newCell

		}
	}
	return boardFragment
}

// Calculate the next cell state according to Game Of Life rules
// Returns a bool with the next state of the cell
func nextCellState(x int, y int, board [][]bool) bool {
	// Count the number of adjacent alive cells
	adj := countAliveNeighbours(x, y, board)

	// Default to dead
	newState := false

	// Find what will make the cell alive

	if board[y][x] == true {
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
func countAliveNeighbours(x int, y int, board [][]bool) int {
	height := len(board)
	width := len(board[0])
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
			v := board[wrapY][wrapX]
			if v == true {
				numNeighbours++
			}
		}
	}

	return numNeighbours
}
