// server.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var (
	mu        sync.Mutex
	KillChan  = make(chan bool)
	brokerRPC *rpc.Client // Broker RPC client
)

// GameOfLifeOperations struct for server's RPC methods
type GameOfLifeOperations struct{}

// GOL forwards the simulation request to the Broker
func (s *GameOfLifeOperations) GOL(req stubs.Request, res *stubs.Response) (err error) {

	brokerReq := stubs.Request{
		InitialWorld: req.InitialWorld,
		ImageHeight:  req.ImageHeight,
		ImageWidth:   req.ImageWidth,
		Turns:        req.Turns,
	}
	var brokerRes stubs.Response

	// Forward the simulation request to the Broker
	err = brokerRPC.Call("Broker.HandleSimulation", brokerReq, &brokerRes)
	if err != nil {
		return fmt.Errorf("Error in Broker simulation: %w", err)
	}
	mu.Lock()
	// Populate the response
	res.FinalWorld = brokerRes.FinalWorld
	res.CompletedTurns = brokerRes.CompletedTurns
	res.AliveCellsAfterFinalState = brokerRes.AliveCellsAfterFinalState
	mu.Unlock()
	return nil
}

// Alive forwards the alive cell count request to the Broker
func (s *GameOfLifeOperations) Alive(req stubs.AliveRequest, res *stubs.AliveResponse) (err error) {
	var brokerRes stubs.AliveResponse

	// Forward the alive cell count request to the Broker
	err = brokerRPC.Call("Broker.GetAliveCells", req, &brokerRes)
	if err != nil {
		return fmt.Errorf("Error in Broker alive cell count request: %w", err)
	}
	mu.Lock()
	// Populate the response
	res.AliveCellsCount = brokerRes.AliveCellsCount
	res.Turn = brokerRes.Turn
	mu.Unlock()
	return nil
}

// KillServer forwards the kill signal to the Broker
func (s *GameOfLifeOperations) KillServer(req stubs.KillRequest, res *stubs.KillResponse) (err error) {
	err = brokerRPC.Call("Broker.KillServer", req, res)
	if err != nil {
		return fmt.Errorf("Error in Broker kill server request: %w", err)
	}
	return
}

// PressedKey forwards keypress handling to the Broker
func (s *GameOfLifeOperations) PressedKey(req stubs.KeyRequest, res *stubs.KeyResponse) (err error) {
	err = brokerRPC.Call("Broker.HandleKeyPress", req, res)
	if err != nil {
		return fmt.Errorf("Error in Broker key press handling: %w", err)
	}
	return
}

func main() {
	// Parse server configuration
	pAddr := flag.String("port", "8030", "Port to listen on")
	brokerAddr := flag.String("broker", "localhost:8031", "Broker address")
	flag.Parse()

	// Connect to the Broker
	var err error
	brokerRPC, err = rpc.Dial("tcp", *brokerAddr)
	if err != nil {
		log.Fatalf("Failed to connect to Broker: %v", err)
	}
	defer brokerRPC.Close()

	// Start the RPC server
	rpc.Register(&GameOfLifeOperations{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()
	fmt.Printf("Server started on port %s, connected to Broker at %s\n", *pAddr, *brokerAddr)
	rpc.Accept(listener)
}
