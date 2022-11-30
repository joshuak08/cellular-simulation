package main

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"net/rpc"
	"os"
	"sync"
	"time"
	"uk.ac.bris.cs/gameoflife/bStubs"
	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

const alive = 255
const dead = 0
const numNodes = 8

var aliveCount int
var globalTurns int
var mu sync.Mutex
var globalWorld [][]byte
var workers []*rpc.Client
var working []bool

// Gol Logic

func makeCallWorld(client *rpc.Client, world [][]byte, ImageHeight, ImageWidth, StartY, EndY, Turns int, out chan [][]byte) {
	request := bStubs.Request{world, ImageWidth, StartY, EndY, ImageHeight, Turns, false}
	response := new(bStubs.Response)
	client.Call(bStubs.BTurnHandler, request, response)
	out <- response.World
	return
}

func closeServers(client *rpc.Client, world [][]byte, ImageWidth, ImageHeight, Turns int) {
	request := bStubs.Request{world, ImageWidth, 0, 0, ImageHeight, Turns, true}
	response := new(bStubs.Response)
	client.Call(bStubs.BShutHandler, request, response)
	return
}

func splitWorkers(req stubs.Request, world [][]byte, workers []*rpc.Client) [][]byte {
	//minimum := int(math.Min(numNodes, float64(req.Threads)))
	out := make([]chan [][]byte, numNodes)
	for i := range out {
		out[i] = make(chan [][]byte)
	}

	for j := 0; j < numNodes; j++ {
		go makeCallWorld(workers[j], world, req.Height, req.Width, j*len(world)/numNodes, (j+1)*(len(world))/numNodes, req.Turns, out[j])
	}
	var newPixelData [][]byte
	for i := 0; i < len(out); i++ {
		newPixelData = append(newPixelData, <-out[i]...)
	}
	return newPixelData
}

type Broker struct {
	shut chan bool
}

func calculateNextState(world [][]byte, startY, endY, ImageHeight, ImageWidth int) [][]byte {
	// Make allocates an array and returns a slice that refers to that array
	height := endY - startY
	newGrid := make([][]byte, height)
	for i := range newGrid {
		// Allocate each []byte within [][]byte
		newGrid[i] = make([]byte, ImageWidth)
	}
	for i := startY; i < endY; i++ {
		for j := 0; j < ImageWidth; j++ {
			neighbours := countNeighbours(i, j, world, ImageHeight, ImageWidth)
			state := world[i][j]
			if state == dead && neighbours == 3 {
				newGrid[i-startY][j] = alive
			} else if state == alive && (neighbours < 2 || neighbours > 3) {
				newGrid[i-startY][j] = dead
			} else {
				newGrid[i-startY][j] = state
			}
		}
	}
	return newGrid
}

func countNeighbours(x, y int, world [][]byte, ImageHeight, ImageWidth int) int {
	var aliveCount = 0
	for i := -1; i < 2; i++ {
		for j := -1; j < 2; j++ {
			// Don't count self as neighbour
			if i == 0 && j == 0 {
				continue
			}
			// Wraparound. Add height and width for negative values
			r := (x + i + ImageWidth) % ImageWidth
			c := (y + j + ImageHeight) % ImageHeight
			if world[r][c] == alive {
				aliveCount++
			}
		}
	}
	return aliveCount
}

func calculateAliveCells(world [][]byte) []util.Cell {
	var aliveCells []util.Cell
	for i := 0; i < len(world); i++ {
		for j := 0; j < len(world[i]); j++ {
			if world[i][j] == alive {
				newCell := util.Cell{X: j, Y: i}
				aliveCells = append(aliveCells, newCell)
			}
		}
	}
	return aliveCells
}

func (s *Broker) CalculateNextWorld(req stubs.Request, res *stubs.Response) (err error) {
	turn := 0
	globalWorld = req.World
	for turn < req.Turns {
		mu.Lock()
		globalWorld = splitWorkers(req, globalWorld, workers)
		mu.Unlock()
		lenAliveCount := len(calculateAliveCells(globalWorld))

		mu.Lock()
		aliveCount = lenAliveCount
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

func (s *Broker) CalculateAlive(req stubs.Request, res *stubs.Response) (err error) {
	mu.Lock()
	res.AliveCells = aliveCount
	mu.Unlock()

	mu.Lock()
	res.Turns = globalTurns
	mu.Unlock()
	return
}

func (s *Broker) ShutServer(req stubs.Request, res *stubs.Response) (err error) {
	mu.Lock()
	for i := 0; i < numNodes; i++ {
		closeServers(workers[i], req.World, req.Width, req.Height, req.Turns)
	}
	mu.Unlock()
	os.Exit(3)
	return
}

func (s *Broker) Snapshot(req stubs.Request, res *stubs.Response) (err error) {
	mu.Lock()
	res.Turns = globalTurns
	res.World = globalWorld
	mu.Unlock()
	return
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	task := &Broker{make(chan bool)}
	rpc.Register(task)
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()

	workers = make([]*rpc.Client, numNodes)
	working = make([]bool, 8)
	address := make([]string, 8)
	address[0] = "44.198.178.240"
	address[1] = "3.239.200.67"
	address[2] = "34.232.210.185"
	address[3] = "44.211.60.221"
	address[4] = "44.204.225.136"
	address[5] = "3.231.152.80"
	address[6] = "3.220.174.219"
	address[7] = "44.200.208.37"
	//address := "127.0.0.1"
	port := ":8030"

	for i := 0; i < numNodes; i++ {
		//end := strconv.Itoa(i + 1)
		//err := error()
		fmt.Println("address", address[i])
		workers[i], _ = rpc.Dial("tcp", address[i]+port)

		//if err != nil {
		//	working[i] = false
		//} else {
		//	working[i] = true
		//}

		//workers[i] = worker
		defer workers[i].Close()
	}
	fmt.Println("reach")
	//wg.Wait()
	fmt.Println("reach1")

	rpc.Accept(listener)
}
