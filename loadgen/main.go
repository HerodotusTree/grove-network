// Command loadgen hammers a Grove node with random signed operations to
// measure its efficiency. Usage:
//
//	go run . -url http://localhost:8080 -ops 1000 -workers 4
//
// Every worker generates its own keypair at startup and signs each op
// with a strictly increasing nonce. One account per worker: sharing a
// single account across concurrent workers would deliver nonces out of
// order, and the node (correctly) rejects anything but last+1.
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

	"github.com/HerodotusTree/grove-network/identity"
)

type signer struct {
	priv    string
	account string
	// next is the nonce to try next. It advances ONLY when the server
	// accepts the op: the node advances its record only on acceptance
	// too, so a rejected attempt must be retried with the same nonce,
	// never skipped. (A client that numbers attempts instead of
	// acceptances desyncs permanently after its first rejection.)
	next uint64
}

func (s *signer) post(url, op string, amount int64) bool {
	sig, err := identity.Sign(s.priv, identity.Payload(op, s.account, amount, s.next))
	if err != nil {
		return false
	}
	body, _ := json.Marshal(map[string]any{
		"account":   s.account,
		"amount":    amount,
		"nonce":     s.next,
		"signature": sig,
	})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		s.next++
		return true
	}
	return false
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
			priv, pub, err := identity.Generate()
			if err != nil {
				for range jobs {
					atomic.AddInt64(&fail, 1)
				}
				return
			}
			s := &signer{priv: priv, account: pub, next: 1}
			for range jobs {
				amount := int64(rand.Intn(100) + 1)
				op := identity.OpDeposit
				if rand.Intn(2) == 0 {
					op = identity.OpRedeem
				}
				if s.post(*url+"/"+op, op, amount) {
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
