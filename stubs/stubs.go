package stubs

// Fragment stores a section of cells in the board
// StartRow points to the row in the main board where this section starts
// EndRow points to the next row in the main board after this section ends (like an exclusive upper bound)
type Fragment struct {
	StartRow int
	EndRow   int
	BitBoard *BitBoard
}

// State represents a change in the state of execution.
type State int

const (
	Paused State = iota
	Executing
	Quitting
)

// String methods allow the different types of Events and States to be printed.
func (state State) String() string {
	switch state {
	case Paused:
		return "Paused"
	case Executing:
		return "Executing"
	case Quitting:
		return "Quitting"
	default:
		return "Incorrect State"
	}
}

//    RPC STRINGS

// Server RPC strings
var ServerStartGame = "Server.StartGame"
var ServerRegisterKeypress = "Server.RegisterKeypress"
var ServerConnectWorker = "Server.ConnectWorker"
var ServerPing = "Server.Ping"

// Controller RPC strings
var ControllerGameStateChange = "Controller.GameStateChange"
var ControllerTurnComplete = "Controller.TurnComplete"
var ControllerFinalTurnComplete = "Controller.FinalTurnComplete"
var ControllerSaveBoard = "Controller.SaveBoard"
var ControllerReportAliveCells = "Controller.ReportAliveCells"

// Worker RPC strings
var WorkerDoTurn = "Worker.DoTurn"
var WorkerShutdown = "Worker.Shutdown"

// ServerResponse contains a result from a standard server RPC call
// Success indicates if the call executed its desired function
// Message contains any additional information
type ServerResponse struct {
	Success bool
	Message string
}

// StartGameRequest contains all data required for a controller to connect to a server
// and start a game
// This will send the address of the controller, along with information about the board
// and the starting board state
type StartGameRequest struct {
	ControllerAddress string

	Height   int
	Width    int
	MaxTurns int

	Board [][]bool
}

// KeypressRequest is used to send a keypress from a controller to be handled at the server
type KeypressRequest struct {
	Key rune
}

// WorkerConnectRequest is passed by a worker which wishes to connect to the server
// This contains the address of the worker so the server can establish a connection
type WorkerConnectRequest struct {
	WorkerAddress string
}

// StateChangeReport is passed to the controller to inform them of changes to game state
type StateChangeReport struct {
	Previous       State
	New            State
	CompletedTurns int
}

// TurnCompleteReport is passed to the controller every time a turn is completed
type TurnCompleteReport struct {
	CompletedTurns int
}

// SaveBoardRequest is passed to the controller to ask them to save the board
type SaveBoardRequest struct {
	CompletedTurns int

	Height int
	Width  int
	Board  [][]bool
}

// AliveCellsReport is passed to the controller every 2 seconds to tell them how many
// cells are alive
type AliveCellsReport struct {
	CompletedTurns int
	NumAlive       int
}

type Halo struct {
	BitBoard *BitBoard
	Offset   int
	StartPtr int
	EndPtr   int
}

// DoTurnRequest is passed to workers to ask them to calculate the next turn
// It sends the whole board along with fragment pointers for their portion to calculate
type DoTurnRequest struct {
	Halo Halo
}

// DoTurnResponse is returned by workers to the server containing a fragment of the new board
type DoTurnResponse struct {
	Frag Fragment
}

// Empty is used when there is no information for an RPC function to return
type Empty struct{}

type BitBoard struct {
	RowLength int
	NumRows   int
	Bytes     RLEBitArray
}

type RLEBitArray struct {
	TotalBits uint
	Runs      []byte
	// This is for use when constructing the array
	lastBit bool
}

func GetByteArrayCell(bytes []byte, height, width int, row, col int) bool {
	bit := uint(row*width + col)
	byteIdx := uint(bit / 8)

	if (bytes[byteIdx] & (1 << (bit % 8))) > 0 {
		return true
	} else {
		return false
	}
}

func (b *RLEBitArray) ToByteArray() []byte {
	bytes := make([]byte, b.TotalBits/8)
	val := false
	bit := uint(0)
	// Loop through each run
	for _, run := range b.Runs {
		// Set identical bits for the length of the run
		for r := byte(0); r < run; r++ {
			// Set the required bit in the byte array
			byteIdx := uint(bit / 8)
			if val {
				bytes[byteIdx] = bytes[byteIdx] | (1 << (bit % 8))
			} else {
				bytes[byteIdx] = bytes[byteIdx] & (^(1 << (bit % 8)))
			}
			// Increment pointer
			bit++
		}
		// Since the run is over, flip the set value
		val = !val
	}
	return bytes
}

func (b *RLEBitArray) addBit(val bool) {
	// If this is the first bit
	if len(b.Runs) == 0 {
		// default to starting with a 0 bit
		if val {
			// If the first value is true then add a 0-length run at the start
			b.Runs = append(b.Runs, 0, 1)
		} else {
			// Else append a 1 length run at the start
			b.Runs = append(b.Runs, 1)
		}
	} else {
		// if we're adding another of the same bits, increase the run length
		if val == b.lastBit {
			// If we've maxed out the run length, start a new run (skipping one)
			if b.Runs[len(b.Runs)-1] == 255 {
				b.Runs = append(b.Runs, 0, 1)
			} else {
				b.Runs[len(b.Runs)-1]++
			}
		} else {
			// Otherwise, make a new run
			b.Runs = append(b.Runs, 1)
		}
	}
	b.lastBit = val
}

func BitBoardFromSlice(board [][]bool, height, width int) *BitBoard {
	// Allocate a new bitboard
	bitBoard := new(BitBoard)
	bitBoard.RowLength = width
	bitBoard.NumRows = height
	bitBoard.Bytes = RLEBitArray{lastBit: false, TotalBits: uint(height * width)}

	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			bitBoard.Bytes.addBit(board[row][col])
		}
	}

	return bitBoard
}

func (b *BitBoard) ToSlice() [][]bool {
	newBoard := make([][]bool, b.NumRows)
	bytes := b.Bytes.ToByteArray()
	for row := 0; row < b.NumRows; row++ {
		newBoard[row] = make([]bool, b.RowLength)
		for col := 0; col < b.RowLength; col++ {
			bit := uint(row*b.RowLength + col)
			byteIdx := uint(bit / 8)

			if (bytes[byteIdx] & (1 << (bit % 8))) > 0 {
				newBoard[row][col] = true
			} else {
				newBoard[row][col] = false
			}
		}
	}
	return newBoard
}
