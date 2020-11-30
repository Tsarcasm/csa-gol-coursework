package main

import (
	// "bufio"
	"encoding/gob"
	"fmt"
	"log"
	"net"

	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/stubs"
)

func init() {
	gob.RegisterName("AliveCellsCount", &gol.AliveCellsCount{})
	gob.RegisterName("ImageOutputComplete", &gol.ImageOutputComplete{})
	gob.RegisterName("StateChange", &gol.StateChange{})
	gob.RegisterName("CellFlipped", &gol.CellFlipped{})
	gob.RegisterName("TurnComplete", &gol.TurnComplete{})
	gob.RegisterName("FinalTurnComplete", &gol.FinalTurnComplete{})
	gob.RegisterName("BoardSave", &gol.BoardSave{})
}

/*

 Connection sequence:
 - connect
 - send BoardMsg


*/

func handleError(err error) {
	// TODO: all
	// Deal with an error event.
}

func acceptConns(ln net.Listener, conns chan net.Conn) {
	// Continuously accept a network connection from the Listener
	// and add it to the channel for handling connections.
	for {
		newConn, err := ln.Accept()
		if err != nil {
			fmt.Print("Error: ", err)
		} else {
			fmt.Print("Accepted communication")
			conns <- newConn
		}
	}
}

func signalReceiver(decoder *gob.Decoder, sigchan chan<- stubs.Signals) {
	// While the connection is active, handle signals
	for {
		var msg stubs.Signals
		// First, decode the BoardMsg
		err := decoder.Decode(&msg)
		if err != nil {
			log.Fatal("decode error:", err)
		}
		print("Received a signal from client:", msg)
		sigchan <- msg
	}
}

func clientUpdater(encoder *gob.Encoder, eventChan <-chan gol.Event) {
	for {
		select {
		case event := <-eventChan:
			err := encoder.Encode(&event)
			if err != nil {
				panic(err)
			}
			// // print("Turn", event.GetCompletedTurns(), ", ")
			// switch e := event.(type) {
			// case gol.AliveCellsCount:
			// 	println("Sending AliveCellsCount")
			// 	encoder.Encode(stubs.UpdateMessage{
			// 		CompletedTurns:  e.CompletedTurns,
			// 		AliveCellsCount: e.CellsCount,
			// 		AliveCells:      nil,
			// 		State:           -1,
			// 		Board:           nil,
			// 	})
			// case gol.FinalTurnComplete:
			// 	println("Sending FinalTurnComplete")
			// 	encoder.Encode(stubs.UpdateMessage{
			// 		CompletedTurns:  e.CompletedTurns,
			// 		AliveCellsCount: -1,
			// 		AliveCells:      e.Alive,
			// 		State:           -1,
			// 		Board:           nil,
			// 	})
			// case gol.StateChange:
			// 	println("Sending StateChange")
			// 	encoder.Encode(stubs.UpdateMessage{
			// 		CompletedTurns:  e.CompletedTurns,
			// 		AliveCellsCount: -1,
			// 		AliveCells:      nil,
			// 		State:           int(e.NewState),
			// 		Board:           nil,
			// 	})
			// case gol.TurnComplete:
			// case gol.CellFlipped:
			// 	// Don't do anything here

		}
	}
}

func handleClient(client net.Conn) {
	// Create a reader for this connection
	decoder := gob.NewDecoder(client)
	encoder := gob.NewEncoder(client)

	var msg stubs.BoardMsg
	// First, decode the BoardMsg
	err := decoder.Decode(&msg)
	if err != nil {
		log.Fatal("decode error:", err)
	}
	// We now have a board
	println("Client uploaded a board")

	p := engineParams{
		boardHeight: msg.Height,
		boardWidth:  msg.Width,
		maxTurns:    msg.MaxTurns,
		numThreads:  1,
	}

	eventChannel := make(chan gol.Event, 1000)
	// saveChannel := make(chan boardState)
	signals := make(chan stubs.Signals, 1000)

	c := engineChannels{
		events: eventChannel,
		// saveChan: saveChannel,
		signals: signals,
	}

	go signalReceiver(decoder, signals)
	go clientUpdater(encoder, eventChannel)
	// Run the engine loop synchronously
	// When the engine loop finishes we need to close the connection
	engineLoop(msg.Board, p, c)

}

func main() {

	// Read in the network port we should listen on, from the commandline argument.
	// Default to port 8030
	// portPtr := flag.String("port", ":8030", "port to listen on")
	// flag.Parse()
	ln, _ := net.Listen("tcp", ":8030")

	for {
		// Accept a connection
		conn, _ := ln.Accept()

		// Synchronously handle the client
		// This will return when the client disconnects
		handleClient(conn)

	}

}
