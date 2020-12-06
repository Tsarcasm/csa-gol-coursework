package main

import (
	"testing"

	"uk.ac.bris.cs/gameoflife/gol"
)

func BenchmarkGol(b *testing.B) {
	// os.Stdout = nil // Disable all program output apart from benchmark results
	println("Test ", b.N)
	benchmarkGol(b)
}

func benchmarkGol(b *testing.B) {
	params := gol.Params{
		ImageWidth:  512,
		ImageHeight: 512,
		Turns:       1000,
		Threads:     4}
	events := make(chan gol.Event)

	gol.Run(params, events, nil)
	for event := range events {
		switch event.(type) {
		case gol.FinalTurnComplete:
			// print(e.Alive)
		}
	}
}
