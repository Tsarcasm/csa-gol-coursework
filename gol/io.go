package gol

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"uk.ac.bris.cs/gameoflife/util"
)

/*

	Reading an image:
	first, send an ioInput command down the command channel
	next, send the filename down the filename channel

	the image is sent row by row down the input channel

*/
type ioChannels struct {
	command <-chan ioCommand // read only
	idle    chan<- bool      // set only

	filename <-chan string
	output   <-chan uint8
	input    chan<- uint8
}

// ioState is the internal ioState of the io goroutine.
type ioState struct {
	params   Params
	channels ioChannels
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type ioCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//		ioOutput 	= 0
//		ioInput 	= 1
//		ioCheckIdle = 2
const (
	ioOutput ioCommand = iota
	ioInput
	ioCheckIdle
)

// writePgmImage receives an array of bytes and writes it to a pgm file.
func (io *ioState) writePgmImage(doWrite bool) {
	_ = os.Mkdir("out", os.ModePerm)

	filename := <-io.channels.filename
	file, ioError := os.Create("out/" + filename + ".pgm")
	util.Check(ioError)
	defer file.Close()
	if doWrite {
		_, _ = file.WriteString("P5\n")
		//_, _ = file.WriteString("# PGM file writer by pnmmodules (https://github.com/owainkenwayucl/pnmmodules).\n")
		_, _ = file.WriteString(strconv.Itoa(io.params.ImageWidth))
		_, _ = file.WriteString(" ")
		_, _ = file.WriteString(strconv.Itoa(io.params.ImageHeight))
		_, _ = file.WriteString("\n")
		_, _ = file.WriteString(strconv.Itoa(255))
		_, _ = file.WriteString("\n")
	}
	world := make([][]byte, io.params.ImageHeight)
	for i := range world {
		world[i] = make([]byte, io.params.ImageWidth)
	}

	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {
			val := <-io.channels.output
			//if val != 0 {
			//	fmt.Println(x, y)
			//}
			world[y][x] = val
		}
	}
	if doWrite {
		for y := 0; y < io.params.ImageHeight; y++ {
			for x := 0; x < io.params.ImageWidth; x++ {
				_, ioError = file.Write([]byte{world[y][x]})
				util.Check(ioError)
			}
		}
	}
	ioError = file.Sync()
	util.Check(ioError)

	fmt.Println("File", filename, "output done!")
}

// readPgmImage opens a pgm file and sends its data as an array of bytes.
func (io *ioState) readPgmImage() {
	filename := <-io.channels.filename
	data, ioError := ioutil.ReadFile("images/" + filename + ".pgm")
	util.Check(ioError)

	fields := strings.Fields(string(data))

	if fields[0] != "P5" {
		panic("Not a pgm file")
	}

	width, _ := strconv.Atoi(fields[1])
	if width != io.params.ImageWidth {
		panic("Incorrect width")
	}

	height, _ := strconv.Atoi(fields[2])
	if height != io.params.ImageHeight {
		panic("Incorrect height")
	}

	maxval, _ := strconv.Atoi(fields[3])
	if maxval != 255 {
		panic("Incorrect maxval/bit depth")
	}

	image := []byte(fields[4])
	for _, b := range image {
		io.channels.input <- b
	}

	fmt.Println("File", filename, "input done!")
}

// startIo should be the entrypoint of the io goroutine.
func startIo(p Params, c ioChannels) {
	io := ioState{
		params:   p,
		channels: c,
	}

	for {
		select {
		case command := <-io.channels.command:
			switch command {
			case ioInput:
				io.readPgmImage()
			case ioOutput:
				io.writePgmImage(true)
			case ioCheckIdle:
				io.channels.idle <- true
			}
		}
	}
}
