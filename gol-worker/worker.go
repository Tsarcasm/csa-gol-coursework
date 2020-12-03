package main

import (
	// "bufio"
	// 	"encoding/gob"
	"fmt"
	// 	"log"
	// 	"net"

	"flag"
	"net"
	"net/rpc"
	"os"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var (
	server *rpc.Client
)

// Worker is the struct for our RPC server
type Worker struct{}


func (w *Worker) DoTurn(req stubs.DoTurnRequest, res *stubs.DoTurnResponse) (err error) {
	fmt.Print(".")
	frag := doTurn(req.Board, req.FragStart, req.FragEnd)
	res.Frag = frag
	return
}

func (w *Worker) Shutdown(req stubs.Empty, res *stubs.Empty) (err error) {
	println("Received shutdown request")
	server.Close()
	os.Exit(0)
	return
}

func main() {
	defer println("Closing worker")
	// Read in the network port we should listen on, from the commandline argument.
	// Default to port 8030
	portPtr := flag.String("p", "8010", "port to listen on")
	flag.Parse()

	println("Starting worker (localhost:" + *portPtr + ")")

	// Register our RPC client
	rpc.Register(&Worker{})

	// Create a listener to handle rpc requests
	listener, _ := net.Listen("tcp", "localhost:"+*portPtr)
	defer listener.Close()
	go rpc.Accept(listener)

	println("Connecting to server")
	newServer, err := rpc.Dial("tcp", "localhost:8020")

	if err != nil {
		println("Cannot find server:", err.Error())
		return
	}
	response := new(stubs.ServerResponse)
	server = newServer
	err = server.Call(stubs.ServerConnectWorker,
		stubs.WorkerConnectRequest{WorkerAddress: "localhost:" + *portPtr}, response)
	if err != nil {
		println("Connection error", err.Error())
		return
	} else if response.Success == false {
		println("Server error", response.Message)
		return
	}

	println("Connected!")
	// Block the main function forever
	select {}
}

// GAME LOGIC BELOW

// Calculate the next turn, given pointers to the start and end to operate over
// Return a fragment of the grid with the next turn's cells 
func doTurn(grid [][]bool, startRow, endRow int) (gridFragment stubs.Fragment) {
	width := len(grid[0])
	gridFragment = stubs.Fragment{
		StartRow: startRow,
		EndRow:   endRow,
		Cells:    make([][]bool, endRow-startRow),
	}

	// Iterate over each cell
	for row := startRow; row < endRow; row++ {
		gridFragment.Cells[row-startRow] = make([]bool, width)
		for col := 0; col < width; col++ {

			// Calculate the next cell state
			newCell := nextCellState(col, row, grid)

			// Update the value of the new cell
			gridFragment.Cells[row-startRow][col] = newCell

		}
	}
	return gridFragment
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

