package main

import (
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/bStubs"
	"uk.ac.bris.cs/gameoflife/stubs"
)

const alive = 255

var aliveCount int
var globalTurns int
var mu sync.Mutex
var globalWorld [][]byte
var workers []*rpc.Client

// Gol Logic

// RPC call to workers to calculate next state, response world passed into out channel
func makeCallWorld(client *rpc.Client, world [][]byte, ImageHeight, ImageWidth, StartY, EndY, Turns int, out chan [][]byte) {
	request := bStubs.Request{world, ImageWidth, StartY, EndY, ImageHeight, Turns, false}
	response := new(bStubs.Response)
	client.Call(bStubs.BTurnHandler, request, response)
	out <- response.World
	return
}

// RPC call to shut down workers
func closeServers(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns int) {
	request := bStubs.Request{world, ImageWidth, 0, 0, ImageHeight, Turns, true}
	response := new(bStubs.Response)
	client.Call(bStubs.BShutHandler, request, response)
	return
}

// Function to split to multiple nodes based on the number of threads on input, with a maximum of 8 nodes
func splitWorkers(req stubs.Request, world [][]byte, workers []*rpc.Client) [][]byte {
	maximum := int(math.Min(8, float64(req.Threads)))

	// Initialises the out channels
	out := make([]chan [][]byte, maximum)
	for i := range out {
		out[i] = make(chan [][]byte)
	}

	// Run all worker nodes in parallel
	for j := 0; j < maximum; j++ {
		go makeCallWorld(workers[j], world, req.Height, req.Width, j*len(world)/maximum, (j+1)*(len(world))/maximum, req.Turns, out[j])
	}

	// Outputs new world slices into newPixelData and returns the new world
	var newPixelData [][]byte
	for i := 0; i < len(out); i++ {
		newPixelData = append(newPixelData, <-out[i]...)
	}
	return newPixelData
}

// Broker Struct for distributor/client to interact with broker through stubs
type Broker struct{}

// Calculate number of alive cells in 2d slice/world and returns slice of type cells containing coordinates
func calculateAliveCells(world [][]byte) int {
	aliveCells := 0
	for i := 0; i < len(world); i++ {
		for j := 0; j < len(world[i]); j++ {
			if world[i][j] == alive {
				aliveCells++
			}
		}
	}
	return aliveCells
}

// Receives RPC call from client/distributor that splits the workers and returns the udpated world, repeats this 100 turns
func (s *Broker) CalculateNextWorld(req stubs.Request, res *stubs.Response) (err error) {
	turn := 0
	globalWorld = req.World
	// Runs for at most 100 turns to update the world
	for turn < req.Turns {
		// splitWorkers returns new world state
		mu.Lock()
		globalWorld = splitWorkers(req, globalWorld, workers)
		mu.Unlock()
		// counts number of alive cells in update world
		numAliveCount := calculateAliveCells(globalWorld)

		mu.Lock()
		aliveCount = numAliveCount
		mu.Unlock()

		turn++
		mu.Lock()
		globalTurns = turn
		mu.Unlock()
	}
	res.Turns = turn
	res.World = globalWorld
	return
}

// RPC call from client to broker to receive number of alive cells every 2s
func (s *Broker) CalculateAlive(req stubs.Request, res *stubs.Response) (err error) {
	mu.Lock()
	res.AliveCells = aliveCount
	mu.Unlock()

	mu.Lock()
	res.Turns = globalTurns
	mu.Unlock()
	return
}

// RPC call from client to broker to shut down all servers and broker
func (s *Broker) ShutServer(req stubs.Request, res *stubs.Response) (err error) {
	mu.Lock()
	for i := 0; i < 8; i++ {
		closeServers(workers[i], req.World, req.Width, req.Height, req.Turns)
	}
	mu.Unlock()
	os.Exit(3)
	return
}

// RPC call from client to broker to receive current world to be saved
func (s *Broker) Snapshot(req stubs.Request, res *stubs.Response) (err error) {
	mu.Lock()
	res.Turns = globalTurns
	res.World = globalWorld
	mu.Unlock()
	return
}

// Main function to setup the broker and port to listen on
// As well as register the Broker variable and register it
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	task := &Broker{}
	rpc.Register(task)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()

	workers = make([]*rpc.Client, 8)

	//AWS ADDRESSES
	address := make([]string, 8)
	address[0] = "44.200.132.137"
	address[1] = "44.212.47.118"
	address[2] = "44.200.201.158"
	address[3] = "44.197.193.0"
	address[4] = "44.198.171.45"
	address[5] = "3.221.127.69"
	address[6] = "3.238.147.211"
	address[7] = "54.236.241.48"

	// AWS PORT
	port := ":8030"

	// LOCAL PORT
	//address := "127.0.0.1"
	//port := ":803"

	// Dials into every address of the worker node
	for i := 0; i < 8; i++ {
		// WORKERS AWS
		fmt.Println(address[i] + port)
		workers[i], _ = rpc.Dial("tcp", address[i]+port)

		// WORKERS LOCAL - Usage: $ go run server.go -port=8031 .. 8038
		//fmt.Println(address + port + strconv.Itoa(i+1))
		//workers[i], _ = rpc.Dial("tcp", address+port+strconv.Itoa(i+1))

		defer workers[i].Close()
	}
	fmt.Println("--- PORTS LOGGED")

	rpc.Accept(listener)
}
