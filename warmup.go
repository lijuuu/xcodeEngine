package main

import (
	"fmt"
	"math/big"
	"os"
	"runtime"
	"time"
)

func main() {
	fmt.Println("Pre-warming Go runtime...")

	// Simulate memory allocations to trigger garbage collector
	_ = make([]byte, 50*1024*1024) // Allocate 50MB

	// Execute a large computation to force runtime optimizations
	_ = big.NewInt(1).Exp(big.NewInt(2), big.NewInt(1000000), nil)

	// Force garbage collection to initialize memory pools
	runtime.GC()

	// Print CPU and memory info
	fmt.Printf("GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("HeapAlloc: %d KB\n", m.HeapAlloc/1024)
	fmt.Printf("HeapSys: %d KB\n", m.HeapSys/1024)

	// Allow time for runtime adjustments
	time.Sleep(2 * time.Second)

	// Exit after warmup
	os.Exit(0)
}
