package main

import (
	"math/rand"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
	// "uk.ac.bris.cs/gameoflife/stubs"
)

func BenchmarkGol(b *testing.B) {
	// os.Stdout = nil // Disable all program output apart from benchmark results
	println("Test ", b.N)
	benchmarkGol(b)
}

func BenchmarkRLE(b *testing.B) {
	// Make a random board
	// size := 512
	// board := make([][]bool, size)
	// makeBoard(size, board)
	// return
	bit := 1233
	for i := 0; i < 1000000000; i++ {
		// _ = stubs.BitBoardFromSlice(board, size, size)
		// bitboard.Bytes.Decode()
		_ = bit % 7
	}

}

func makeBoard(size int, board [][]bool) {
	for row := 0; row < size; row++ {
		board[row] = make([]bool, size)
		for col := 0; col < size; col++ {

			r := rand.Float32()

			ratio := float32(0.2)
			if r < ratio {
				board[row][col] = true
			} else {
				board[row][col] = false
			}
		}
	}
}

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

func benchmarkGol(b *testing.B) {
	params := gol.Params{
		ImageWidth:    512,
		ImageHeight:   512,
		Turns:         1000,
		Threads:       2,
		ServerAddress: "54.156.128.45:8030",
		VisualUpdates: false,
		OurIP:         "185.164.183.135",
	}
	events := make(chan gol.Event)

	gol.Run(params, events, nil)
	for event := range events {
		switch event.(type) {
		case gol.FinalTurnComplete:
			// print(e.Alive)
		}
	}
}
