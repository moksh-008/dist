package main

import (
	"fmt"
	"log"
	"net/rpc"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// WorkerClient represents a connection to a worker.
type WorkerClient struct {
	client *rpc.Client
}

// ProcessSlice processes the slice of the world for a worker.
func (w *WorkerClient) ProcessSlice(slice [][]byte, width, height int) ([]util.Cell, error) {
	// Process the slice and return alive cell coordinates
	aliveCells := findAliveCells(slice)
	return aliveCells, nil
}

func main() {
	// Connect to the Broker
	client, err := rpc.Dial("tcp", "broker_address_here:8030")
	if err != nil {
		log.Fatal("Connection failed:", err)
	}
	defer client.Close()

	// Send a slice to the worker and receive alive cells
	slice := [][]byte{
		// Example slice of the world (each row represents a row of cells)
		{0, 255, 0},
		{255, 255, 255},
		{0, 0, 255},
	}

	width := 3
	height := 3

	// Process the slice (return alive cells coordinates)
	worker := &WorkerClient{client: client}
	aliveCells, err := worker.ProcessSlice(slice, width, height)
	if err != nil {
		log.Fatal("Error processing slice:", err)
	}

	// Output the result
	fmt.Printf("Alive cells: %v\n", aliveCells)

	// If you're returning alive cell coordinates to the broker, do this:
	response := stubs.Response{
		FinalWorld:                slice,      // Final processed world
		AliveCellsAfterFinalState: aliveCells, // Return alive cells coordinates
		CompletedTurns:            10,         // Just an example
	}

	// Call the broker or distributor to send the results
	err = client.Call("Broker.GOL", response, &response)
	if err != nil {
		log.Fatal("Error calling Broker.GOL:", err)
	}
}
