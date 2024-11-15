package gol

import (
	"fmt"
	"log"
	"net/rpc"
	"strconv"
	"time"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	ioKeypress <-chan rune
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// Initialize a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	file := strconv.Itoa(p.ImageWidth)
	file = file + "x" + file
	c.ioCommand <- ioInput
	c.ioFilename <- file

	// Populate the world with data read from the input PGM file.
	for y := range world {
		for x := range world[y] {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	// Connect to the Broker
	client, err := rpc.Dial("tcp", "BROKER_IP:BROKER_PORT")
	if err != nil {
		fmt.Println("Error connecting to Broker:", err)
		return
	}
	defer client.Close()

	// Prepare a request to send to the Broker
	request := stubs.Request{
		InitialWorld: world,
		ImageWidth:   p.ImageWidth,
		ImageHeight:  p.ImageHeight,
		Turns:        p.Turns,
	}

	// Periodic ticker to fetch alive cells
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	done := make(chan bool)

	go func() {
		for {
			select {
			case <-ticker.C:
				aliveResponse := new(stubs.AliveResponse)
				err := client.Call(stubs.AliveCellReport, stubs.AliveRequest{}, aliveResponse)
				if err != nil {
					fmt.Println("Error in Alive RPC call:", err)
					continue
				}
				c.events <- AliveCellsCount{
					CompletedTurns: aliveResponse.Turn,
					CellsCount:     aliveResponse.AliveCellsCount,
				}
			case <-done:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case command := <-c.ioKeypress:
				keyRequest := stubs.KeyRequest{Key: command}
				keyResponse := new(stubs.KeyResponse)
				err := client.Call(stubs.KeyPresshandler, keyRequest, keyResponse)
				if err != nil {
					log.Fatal("Key Press Call Error:", err)
				}
				switch command {
				case 's':
					outFileName := file + "x" + strconv.Itoa(keyResponse.Turns)
					savePGMImage(c, keyResponse.World, outFileName, p.ImageHeight, p.ImageWidth)
				case 'k':
					done <- true
					c.events <- StateChange{keyResponse.Turns, Quitting}
				case 'q':
					done <- true
					c.events <- StateChange{keyResponse.Turns, Quitting}
				case 'p':
					c.events <- StateChange{keyResponse.Turns, Paused}
					for <-c.ioKeypress == 'p' {
						c.events <- StateChange{keyResponse.Turns, Executing}
						break
					}
				}
			}
		}
	}()

	// Start the simulation
	response := new(stubs.Response)
	err = client.Call(stubs.ServerHandler, request, response)
	if err != nil {
		fmt.Println("Error in GOL RPC call:", err)
		return
	}

	c.events <- FinalTurnComplete{
		CompletedTurns: response.CompletedTurns,
		Alive:          response.AliveCellsAfterFinalState,
	}
	outputPGM(p, c, response.FinalWorld, response.CompletedTurns)
	done <- true
}

func savePGMImage(c distributorChannels, w [][]byte, file string, imageHeight, imageWidth int) {
	c.ioCommand <- ioOutput
	c.ioFilename <- file
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			c.ioOutput <- w[y][x]
		}
	}
}
func outputPGM(p Params, c distributorChannels, world [][]byte, turn int) {
	// Construct the output filename
	filename := fmt.Sprintf("%dx%d-%d", p.ImageWidth, p.ImageHeight, turn)

	// Signal the IO goroutine to prepare for output
	c.ioCommand <- ioOutput

	// Send the filename to the IO goroutine
	c.ioFilename <- filename

	// Write the world's data to the IO channel row by row
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	// Notify the IO goroutine that we're done
	fmt.Printf("Output written to file: %s.pgm\n", filename)
}
