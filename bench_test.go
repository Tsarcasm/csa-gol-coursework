package main

import (
	"math/rand"
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

func BenchmarkGol(b *testing.B) {
	// os.Stdout = nil // Disable all program output apart from benchmark results
	// fmt.Println("Test ", b.N)
	benchmarkGol(b)
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
		ImageWidth:  512,
		ImageHeight: 512,
		Turns:       1000,
		Threads:     8}
	events := make(chan gol.Event)

	gol.Run(params, events, nil)
	for event := range events {
		switch event.(type) {
		case gol.FinalTurnComplete:
			// print(e.Alive)
		}
	}
}
