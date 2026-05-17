package sudoku

import (
	"math/rand"
	"time"
)

type Board [9][9]int

func Generate() (puzzle Board, solution Board) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	fillBoard(&solution, r)

	puzzle = solution

	// Remove some cells to create the puzzle
	// A standard sudoku has ~20-30 cells missing for easy/medium
	// Let's remove 40 cells for a decent challenge
	cellsToRemove := 40

	for cellsToRemove > 0 {
		row := r.Intn(9)
		col := r.Intn(9)

		if puzzle[row][col] != 0 {
			puzzle[row][col] = 0
			cellsToRemove--
		}
	}

	return puzzle, solution
}

func fillBoard(board *Board, r *rand.Rand) bool {
	for row := 0; row < 9; row++ {
		for col := 0; col < 9; col++ {
			if board[row][col] == 0 {
				nums := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
				r.Shuffle(len(nums), func(i, j int) {
					nums[i], nums[j] = nums[j], nums[i]
				})

				for _, num := range nums {
					if isValid(board, row, col, num) {
						board[row][col] = num
						if fillBoard(board, r) {
							return true
						}
						board[row][col] = 0
					}
				}
				return false
			}
		}
	}
	return true
}

func isValid(board *Board, row, col, num int) bool {
	for i := 0; i < 9; i++ {
		if board[row][i] == num {
			return false
		}
	}
	for i := 0; i < 9; i++ {
		if board[i][col] == num {
			return false
		}
	}
	startRow := row - row%3
	startCol := col - col%3
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if board[i+startRow][j+startCol] == num {
				return false
			}
		}
	}
	return true
}
