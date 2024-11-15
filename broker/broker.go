package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// WorkerClient represents a connection to a worker.
type WorkerClient struct {
	client *rpc.Client
}

// Broker manages communication between the Local Controller and the workers.
type Broker struct {
	mu      sync.Mutex
	workers []*WorkerClient
	world   [][]byte
	turns   int
	width   int
	height  int
}

// SplitWorld divides the world into slices for each worker.
func (b *Broker) SplitWorld(world [][]byte, numWorkers int) [][][]byte {
	height := len(world)
	sliceHeight := height / numWorkers
	slices := make([][][]byte, numWorkers)

	for i := 0; i < numWorkers; i++ {
		start := i * sliceHeight
		end := start + sliceHeight
		if i == numWorkers-1 {
			end = height
		}
		slices[i] = world[start:end]
	}
	return slices
}

// MergeSlices combines slices from workers into the final world.
func (b *Broker) MergeSlices(slices [][][]byte) [][]byte {
	finalWorld := make([][]byte, 0)
	for _, slice := range slices {
		finalWorld = append(finalWorld, slice...)
	}
	return finalWorld
}

// RegisterWorker adds a new worker to the Broker's pool.
func (b *Broker) RegisterWorker(workerAddress string) error {
	client, err := rpc.Dial("tcp", workerAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to worker: %v", err)
	}
	b.mu.Lock()
	b.workers = append(b.workers, &WorkerClient{client: client})
	b.mu.Unlock()
	return nil
}

// GOL handles the Game of Life simulation request from the Local Controller.
func (b *Broker) GOL(req stubs.Request, res *stubs.Response) error {
	b.mu.Lock()
	b.world = req.InitialWorld
	b.turns = req.Turns
	b.width = req.ImageWidth
	b.height = req.ImageHeight
	numWorkers := len(b.workers)
	b.mu.Unlock()

	if numWorkers == 0 {
		return fmt.Errorf("no workers available")
	}

	for turn := 0; turn < b.turns; turn++ {

		slices := b.SplitWorld(b.world, numWorkers)
		var wg sync.WaitGroup
		results := make([][][]byte, numWorkers)

		for i, worker := range b.workers {
			wg.Add(1)
			go func(i int, worker *WorkerClient, slice [][]byte) {
				defer wg.Done()
				request := stubs.Request{
					InitialWorld: slice,
					ImageHeight:  len(slice),
					ImageWidth:   b.width,
				}
				response := new(stubs.Response)
				err := worker.client.Call(stubs.ServerHandler, request, response)
				if err != nil {
					log.Printf("Worker %d failed: %v", i, err)
					return
				}
				results[i] = response.FinalWorld
			}(i, worker, slices[i])
		}

		wg.Wait()
		b.mu.Lock()
		b.world = b.MergeSlices(results)
		b.mu.Unlock()
	}

	b.mu.Lock()
	res.FinalWorld = b.world
	res.CompletedTurns = b.turns
	res.AliveCellsAfterFinalState = findAliveCells(b.world)
	b.mu.Unlock()
	return nil
}

// Alive handles alive cell count requests.
func (b *Broker) Alive(req stubs.AliveRequest, res *stubs.AliveResponse) error {
	b.mu.Lock()
	res.Turn = b.turns
	res.AliveCellsCount = countAliveCells(b.world)
	b.mu.Unlock()
	return nil
}

// Helper function to count alive cells.
func countAliveCells(world [][]byte) int {
	count := 0
	for _, row := range world {
		for _, cell := range row {
			if cell == 255 {
				count++
			}
		}
	}
	return count
}

// Helper function to find alive cell coordinates.
func findAliveCells(world [][]byte) []util.Cell {
	aliveCells := []util.Cell{}
	for y, row := range world {
		for x, cell := range row {
			if cell == 255 {
				aliveCells = append(aliveCells, util.Cell{X: x, Y: y})
			}
		}
	}
	return aliveCells
}

func main() {
	port := "8030"
	fmt.Printf("Broker starting on port %s...\n", port)
	broker := &Broker{}

	// Register the broker's RPC methods.
	rpc.Register(broker)

	// Start listening for incoming RPC connections.
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to start broker: %v", err)
	}
	defer listener.Close()

	fmt.Println("Broker ready to accept connections.")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Connection error: %v", err)
			continue
		}
		go rpc.ServeConn(conn)
	}
}
