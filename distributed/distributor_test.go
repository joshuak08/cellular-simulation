package main

import (
	"fmt"
	"os"
	"testing"
	"uk.ac.bris.cs/gameoflife/gol"
)

const benchLength = 1000

func BenchmarkGol(b *testing.B) {
	fmt.Println("Benchmarking...")
	for threads := 1; threads <= 8; threads++ {
		fmt.Println("Thread")
		os.Stdout = nil // Disable all program output apart from benchmark results
		p := gol.Params{
			Turns:       benchLength,
			Threads:     threads,
			ImageWidth:  16,
			ImageHeight: 16,
		}
		name := fmt.Sprintf("%d_workers", p.Threads)
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				events := make(chan gol.Event)
				go gol.Run(p, events, nil)
				for range events {

				}
			}
		})
	}
}