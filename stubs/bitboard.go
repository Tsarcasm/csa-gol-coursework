package stubs

// BitBoard stores a whole board using individual bits instead of bytes
// This divides space required by 8
// EXTENSION: bits are stored in a Run Length Encoded bit array
type BitBoard struct {
	RowLength int
	NumRows   int
	Bytes     []byte
}

// GetBitArrayCell returns a cell in a bit array as if the array was a 2d slice
func GetBitArrayCell(bytes []byte, height, width int, row, col int) bool {
	bit := uint(row*width + col)
	// Perform bitwise operations to get the byte and bit indices
	byteIdx := uint(bit >> 3)
	bitIdx := bit & 7
	// Return a boolean based on the bit value
	if (bytes[byteIdx] & (1 << (bitIdx))) > 0 {
		return true
	}
	return false
}


// BitBoardFromSlice will construct a BitBoard from a 2d board slice
func BitBoardFromSlice(board [][]bool, height, width int) *BitBoard {
	// Allocate a new bitboard
	bitBoard := new(BitBoard)
	bitBoard.RowLength = width
	bitBoard.NumRows = height
	bitBoard.Bytes = make([]byte, (width*height) / 8)

	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			bit := uint(row*width + col)
			byteIdx := uint(bit / 8)
			if board[row][col] == true {
				bitBoard.Bytes[byteIdx] = bitBoard.Bytes[byteIdx] | (1 << (bit % 8))
			} else {
				bitBoard.Bytes[byteIdx] = bitBoard.Bytes[byteIdx] & (^(1 << (bit % 8)))
			}
		}
	}

	return bitBoard
}

func (b *BitBoard) ToSlice() [][]bool {
	newBoard := make([][]bool, b.NumRows)
	for row := 0; row < b.NumRows; row++ {
		newBoard[row] = make([]bool, b.RowLength)
		for col := 0; col < b.RowLength; col++ {
			bit := uint(row*b.RowLength + col)
			byteIdx := uint(bit / 8)

			if (b.Bytes[byteIdx] & (1 << (bit % 8))) > 0 {
				newBoard[row][col] = true
			} else {
				newBoard[row][col] = false
			}
		}
	}
	return newBoard
}
