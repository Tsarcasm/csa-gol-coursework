package main

import (
	// "bufio"
	// 	"encoding/gob"
	// 	"fmt"
	// 	"log"
	// 	"net"
	"net"
	"net/rpc"
	"os"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var (
	server *rpc.Client
)

type Worker struct{}

func (w *Worker) DoTurn(req stubs.DoTurnRequest, res *stubs.DoTurnResponse) (err error) {
	println("Received request to do a turn")
	frag := doTurn(req.Board, req.FragStart, req.FragEnd)
	res = stubs.DoTurnResponse{frag}
	return
}

func (w *Worker) Shutdown(req stubs.Empty, res *stubs.Empty) (err error) {
	println("Received shutdown request")
	server.Close()
	os.Exit(0)
	return
}

func main() {

	// Read in the network port we should listen on, from the commandline argument.
	// Default to port 8030
	// portPtr := flag.String("port", ":8030", "port to listen on")
	// flag.Parse()

	// Register our RPC client
	rpc.Register(&Worker{})

	// Create a listener to handle rpc requests
	listener, _ := net.Listen("tcp", "localhost:8010")
	defer listener.Close()
	rpc.Accept(listener)

	//Todo remove this
	println("End of main function")
}

// GAME LOGIC BELOW
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

func nextCellState(x int, y int, grid [][]bool) bool {
	adj := getNeighbours(x, y, grid)

	newState := false

	if grid[y][x] == true {
		if adj < 2 {
			newState = false
		} else if adj > 3 {
			newState = false
		} else {
			newState = true
		}
	} else {
		if adj == 3 {
			newState = true
		} else {
			newState = false
		}
	}
	return newState
}

func getNeighbours(x int, y int, grid [][]bool) int {
	height := len(grid)
	width := len(grid[0])
	numNeighbours := 0

	// Check all cells in a grid around this one
	for _x := -1; _x < 2; _x++ {
		for _y := -1; _y < 2; _y++ {
			//this cell is not a neighbour
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

			v := grid[wrapY][wrapX]
			if v == true {
				numNeighbours++
			}
		}
	}

	return numNeighbours
}
