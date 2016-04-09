package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"io"
	"os/exec"
	"sync"
	"time"
)

type game_state_t struct {
	max_timebank time.Duration
	time_per_move time.Duration
	players []*player_settings
	whoseturn int
	board_state *State
	round_num int
	move_num int
}

type player_settings struct {
	player_name string
	player_id int8
	timebank time.Duration
	cmd *exec.Cmd
	stdin io.Writer
	stdout *bufio.Reader
	stderr io.Reader
}

var game_state game_state_t

// Command line flags
type flags_t struct {
	Player1	*string
	Player2 *string
}
var flags flags_t

func parseFlags() {
	flags.Player1 = flag.String("player1", "", "Path to the player 1 bot")
	flags.Player2 = flag.String("player2", "", "Path to the player 2 bot")
	flag.Parse()

	if *flags.Player1 == "" || *flags.Player2 == "" {
		log.Fatal("Both player1 and player2 flags are required")
	}
}

func main() {
	parseFlags()

	max_timebank, _ := time.ParseDuration("10s")
	time_per_move, _ := time.ParseDuration("500ms")

	game_state = game_state_t{max_timebank: max_timebank, time_per_move: time_per_move, players: make([]*player_settings, 2), whoseturn: 1, board_state: &State{PlayerJustMoved: 2}}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			game_state.board_state.MacroBoard[i][j] = -1
		}
	}
	//bot1cmd := exec.Command("tee", "error.log")
	bot1cmd := exec.Command(*flags.Player1)
	bot1in, _ := bot1cmd.StdinPipe()
	defer bot1in.Close()
	bot1out, _ := bot1cmd.StdoutPipe()
	defer bot1out.Close()
	bot1err, _ := bot1cmd.StderrPipe()
	defer bot1err.Close()

	player_one := player_settings{player_name: *flags.Player1, player_id: 1, timebank: game_state.max_timebank, cmd: bot1cmd, stdin: bot1in, stdout: bufio.NewReader(bot1out), stderr: bot1err}


	bot2cmd := exec.Command(*flags.Player2)
	bot2in, _ := bot2cmd.StdinPipe()
	defer bot2in.Close()
	bot2out, _ := bot2cmd.StdoutPipe()
	defer bot2out.Close()
	bot2err, _ := bot2cmd.StderrPipe()
	defer bot2err.Close()

	player_two := player_settings{player_name: *flags.Player2, player_id: 2, timebank: game_state.max_timebank, cmd: bot2cmd, stdin: bot2in, stdout: bufio.NewReader(bot2out), stderr: bot2err}

	go logBotsStdErr(player_one, player_two)
	game_state.players[0] = &player_one
	game_state.players[1] = &player_two

	//Ready to begin... Start the bots!
	err := player_one.cmd.Start()
	if err != nil {
		log.Fatal(err)
	}
	err = player_two.cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	sendSettings(player_one)
	sendSettings(player_two)

	// Play the game!
	for ;game_state.board_state.GameFinished == 0; {
		// Darn 0-based counting
		current_player := game_state.players[game_state.whoseturn - 1]
		sendState(current_player)
		move := getMove(current_player)
		game_state.board_state.DoMove(move)
		game_state.board_state.LogState()

		// Get ready for next turn
		game_state.move_num++
		if game_state.whoseturn == 1 {
			game_state.whoseturn = 2
		} else {
			game_state.whoseturn = 1
			game_state.round_num++
		}
	}

	// Game Over!
	if game_state.board_state.GameFinished == 3 {
		fmt.Println("It's a draw!")
	} else {
		fmt.Printf("Player %d wins! (%s)\n", game_state.board_state.GameFinished, game_state.players[game_state.board_state.GameFinished-1].player_name)
	}
}


func sendSettings(player player_settings) {
	pipe := player.stdin
	io.WriteString(pipe, fmt.Sprintf("settings timebank %d\n", toMs(game_state.max_timebank)))
	io.WriteString(pipe, fmt.Sprintf("settings time_per_move %d\n", toMs(game_state.time_per_move)))
	io.WriteString(pipe, fmt.Sprintf("settings player_names %s,%s\n", game_state.players[0].player_name, game_state.players[1].player_name))
	io.WriteString(pipe, fmt.Sprintf("settings your_bot %s\n", player.player_name))
	io.WriteString(pipe, fmt.Sprintf("settings your_botid %d\n", player.player_id))
}

func toMs(d time.Duration) int64 {
	return int64(d / time.Millisecond)
}

func sendState(player *player_settings) {
	pipe := player.stdin
	io.WriteString(pipe, fmt.Sprintf("update game round %d\n", game_state.round_num))
	io.WriteString(pipe, fmt.Sprintf("update game move %d\n", game_state.move_num))

	board := game_state.board_state
	var field bytes.Buffer
	field.WriteString("update game field ")
	for i := 0; i < 9; i++ {
		for j := 0; j < 9; j++ {
			if i+j == 0 {
				field.WriteString(fmt.Sprintf("%d", board.Field[i][j]))
			} else {
				field.WriteString(fmt.Sprintf(",%d", board.Field[i][j]))
			}
		}
	}
	field.WriteString("\n")
	pipe.Write(field.Bytes())

	var macro bytes.Buffer
	macro.WriteString("update game macroboard ")
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i+j == 0 {
				macro.WriteString(fmt.Sprintf("%d", board.MacroBoard[i][j]))
			} else {
				macro.WriteString(fmt.Sprintf(",%d", board.MacroBoard[i][j]))
			}
		}
	}
	macro.WriteString("\n")
	pipe.Write(macro.Bytes())
}

func getMove(player *player_settings) (*Move) {
	player.timebank += game_state.time_per_move
	if player.timebank > game_state.max_timebank {
		player.timebank = game_state.max_timebank
	}
	log.Printf("Requesting move from %s with %fs", player.player_name, player.timebank.Seconds())
	io.WriteString(player.stdin, fmt.Sprintf("action move %d\n", toMs(player.timebank)))
	start_time := time.Now()
	line, err := player.stdout.ReadString('\n')
	elapsed := time.Since(start_time)
	if elapsed > player.timebank {
		log.Fatalf("Sorry, too slow %s", player.player_name)
	}
	if err != nil {
		log.Fatalf("Error reading move from %s", player.player_name)
	}

	player.timebank -= elapsed
	var col int
	var row int
	fmt.Sscanf(line, "place_move %d %d\n", &col, &row)
	return &Move{Row: row, Col:col}
}

func logBotsStdErr(bot1 player_settings, bot2 player_settings) {
	bot1err := bufio.NewReader(bot1.stderr)
	bot2err := bufio.NewReader(bot2.stderr)
	var wg sync.WaitGroup
	wg.Add(2)
	merged := make(chan string, 10)
	go logReader(bot1.player_name + " log: ", bot1err, merged, wg)
	go logReader(bot2.player_name + " log: ", bot2err, merged, wg)

	// Signal close of the channel when wg is empty
	go func() {
		wg.Wait()
		close(merged)
	}()

	// Read until close
	for line := range merged {
		log.Println(line)
	}
}

func logReader(prefix string, reader io.Reader, output chan string, wg sync.WaitGroup) {
	defer wg.Done()
	bufReader := bufio.NewReader(reader)
	for {
		line, err := bufReader.ReadString('\n')
		if err != nil {
			break
		}
		if len(line) > 0 {
			line = line[:len(line)-1]
		}
		output <- prefix + line
	}
}
