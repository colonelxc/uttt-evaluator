package main

import (
	"fmt"
	"log"
)

// An individual move
type Move struct {
	Col int
	Row int
}

func (m Move) toMoveString() (string) {
	return fmt.Sprintf("place_move %d %d", m.Col, m.Row)
}

func FromMoveString(move string) (*Move) {
	var row int
	var col int
	fmt.Sscanf(move, "place_move %d %d", &col, &row)
	newmove := Move{Col:col, Row:row}
	return &newmove
}

// Game state and functions
//
//
type State struct {
	Field           [9][9]int8
	MacroBoard      [3][3]int8
	PlayerJustMoved int8
	GameFinished    int8
}

func (s *State) LogState() {
	log.Printf("GameFinished: %d, PlayerJustMoved: %d", s.GameFinished, s.PlayerJustMoved)
	log.Println("MacroBoard")
	for i := 0; i < 3; i++ {
		log.Printf("%v", s.MacroBoard[i])
	}

	log.Println("Field")
	for i := 0; i < 9; i++ {
		if i == 3 || i == 6 {
			log.Println("----------------------")
		}
		field_line := ""
		for j := 0; j < 9; j++ {
			if j == 3 || j == 6 {
				field_line = field_line + "| "
			}
			field_line = field_line + fmt.Sprintf("%d ", s.Field[i][j])
		}
		log.Println(field_line)
	}
}

func (s *State) Clone() *State {
	clone := new(State)
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			clone.Field[i][j] = s.Field[i][j]
		}
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			clone.MacroBoard[i][j] = s.MacroBoard[i][j]
		}
	}
	clone.PlayerJustMoved = s.PlayerJustMoved
	// really shouldn't need this
	clone.GameFinished = clone.GameFinished
	return clone
}

func (s *State) MarkDeadBoards() {
	for macro_i := 0; macro_i < 3; macro_i++ {
		for macro_j := 0; macro_j < 3; macro_j++ {
			if s.MacroBoard[macro_i][macro_j] == 0 {
				live := false
				for i := 0; i < 3; i++ {
					for j := 0; j < 3; j++ {
						if s.Field[macro_i*3 + i][macro_j*3 + j] == 0 {
							live = true
							break
						}
					}
					if live {
						break
					}
				}
				if ! live {
					s.MacroBoard[macro_i][macro_j] = 9
				}
			}
		}
	}
}


func (s *State) GetMoves() []*Move {
	moves := make([]*Move, 0, 9)
	for macro_i := 0; macro_i < 3; macro_i++ {
		for macro_j := 0; macro_j < 3; macro_j++ {
			if s.MacroBoard[macro_i][macro_j] == -1 {
				sub_i := macro_i * 3
				sub_j := macro_j * 3
				for i := 0; i < 3; i++ {
					for j := 0; j < 3; j++ {
						if s.Field[sub_i+i][sub_j+j] == 0 {
							moves = append(moves, &Move{Row: sub_i + i, Col: sub_j + j})
						}
					}
				}
			}
		}
	}
	return moves
}

func (s *State) GetResult(playerJustMoved int8) float64 {
	if s.GameFinished == 0 {
		log.Println("GetResult error, called when game wasn't finished!")
		return -1.0
	}
	if s.GameFinished == 3 { //draw
		return 0.5
	}
	if s.GameFinished == playerJustMoved {
		return 1.0
	}
	return 0.0
}

func (s *State) DoMove(m *Move) {
	if s.Field[m.Row][m.Col] != 0 {
		log.Println("DoMove error, move at row %d, col %d is already %d", m.Row, m.Col, s.Field[m.Row][m.Col])
	}

	if s.PlayerJustMoved == 1 {
		s.PlayerJustMoved = 2
	} else {
		s.PlayerJustMoved = 1
	}
	s.Field[m.Row][m.Col] = s.PlayerJustMoved

	sub_row := (m.Row / 3) * 3
	sub_col := (m.Col / 3) * 3
	macro_row := m.Row / 3
	macro_col := m.Col / 3

	// Find out if someone just won a sub-board
	macro_board_taken := false
	for i := 0; i < 3; i++ {
		if s.Field[sub_row+i][sub_col+0] == s.PlayerJustMoved && s.Field[sub_row+i][sub_col+1] == s.PlayerJustMoved && s.Field[sub_row+i][sub_col+2] == s.PlayerJustMoved {
			macro_board_taken = true
		}

		if s.Field[sub_row+0][sub_col+i] == s.PlayerJustMoved && s.Field[sub_row+1][sub_col+i] == s.PlayerJustMoved && s.Field[sub_row+2][sub_col+i] == s.PlayerJustMoved {
			macro_board_taken = true
		}
	}
	// diags
	if s.Field[sub_row][sub_col] == s.PlayerJustMoved && s.Field[sub_row+1][sub_col+1] == s.PlayerJustMoved && s.Field[sub_row+2][sub_col+2] == s.PlayerJustMoved {
		macro_board_taken = true
	}

	if s.Field[sub_row+2][sub_col+0] == s.PlayerJustMoved && s.Field[sub_row+1][sub_col+1] == s.PlayerJustMoved && s.Field[sub_row+0][sub_col+2] == s.PlayerJustMoved {
		macro_board_taken = true
	}

	if macro_board_taken {
		s.MacroBoard[macro_row][macro_col] = s.PlayerJustMoved
		// Did PlayerJustMoved win?
		s.checkMacroWin()
		if s.GameFinished > 0 {
			return
		}
	}

	// Find out if this board is now dead
	dead := true
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if s.Field[sub_row+i][sub_col+j] == 0 {
				dead = false
				break
			}
		}
		if dead == false {
			break
		}
	}
	if dead {
		s.MacroBoard[macro_row][macro_col] = 9
	}


	// Figure out next macro board opportunities
	next_macro_row := m.Row % 3
	next_macro_col := m.Col % 3
	if s.MacroBoard[next_macro_row][next_macro_col] <= 0 {
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if s.MacroBoard[i][j] <= 0 {
					s.MacroBoard[i][j] = 0
				}
			}
		}
		s.MacroBoard[next_macro_row][next_macro_col] = -1
	} else {
		macro_avail_count := 0
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				if s.MacroBoard[i][j] <= 0 {
					s.MacroBoard[i][j] = -1
					macro_avail_count++
				}
			}
		}
		if macro_avail_count == 0 {
			s.GameFinished = 3 // Game finished in a draw
		}
	}
}

func (s *State) checkMacroWin() {
	game_won := false
	for i := 0; i < 3; i++ {
		if s.MacroBoard[i][0] == s.PlayerJustMoved && s.MacroBoard[i][1] == s.PlayerJustMoved && s.MacroBoard[i][2] == s.PlayerJustMoved {
			game_won = true
		}

		if s.MacroBoard[0][i] == s.PlayerJustMoved && s.MacroBoard[1][i] == s.PlayerJustMoved && s.MacroBoard[2][i] == s.PlayerJustMoved {
			game_won = true
		}
	}
	// diags
	if s.MacroBoard[0][0] == s.PlayerJustMoved && s.MacroBoard[1][1] == s.PlayerJustMoved && s.MacroBoard[2][2] == s.PlayerJustMoved {
		game_won = true
	}

	if s.MacroBoard[2][0] == s.PlayerJustMoved && s.MacroBoard[1][1] == s.PlayerJustMoved && s.MacroBoard[0][2] == s.PlayerJustMoved {
		game_won = true
	}

	if game_won {
		s.GameFinished = s.PlayerJustMoved
	}
}
