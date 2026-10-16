// Command loadgen hammers a Grove node with random operations to
// measure its efficiency. Usage:
//
//	go run . -url http://localhost:8080 -ops 1000 -workers 4
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

func post(url string, amount int64) bool {
	body, _ := json.Marshal(map[string]int64{"amount": amount})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func main() {
	url := flag.String("url", "http://localhost:8080", "node base URL")
	ops := flag.Int("ops", 1000, "total operations")
	workers := flag.Int("workers", 4, "concurrent workers")
	flag.Parse()

	var ok, fail int64
	jobs := make(chan int, *ops)
	for i := 0; i < *ops; i++ {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < *workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range jobs {
				amount := int64(rand.Intn(100) + 1)
				var good bool
				if rand.Intn(2) == 0 {
					good = post(*url+"/deposit", amount)
				} else {
					good = post(*url+"/redeem", amount)
				}
				if good {
					atomic.AddInt64(&ok, 1)
				} else {
					atomic.AddInt64(&fail, 1)
				}
			}
		}()
	}
	start := time.Now()
	wg.Wait()
	elapsed := time.Since(start)

	var m map[string]any
	if resp, err := http.Get(*url + "/metrics"); err == nil {
		json.NewDecoder(resp.Body).Decode(&m)
		resp.Body.Close()
	}

	fmt.Printf("loadgen: %d ops in %v (%.0f ops/sec)\n", *ops, elapsed.Round(time.Millisecond), float64(*ops)/elapsed.Seconds())
	fmt.Printf("loadgen: accepted=%d rejected-or-failed=%d\n", ok, fail)
	fmt.Printf("node metrics: %+v\n", m)
}
