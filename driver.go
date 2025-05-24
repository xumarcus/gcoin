package main

import (
	"fmt"
	"sync"
	"time"
)

type Node[T any] struct {
	chain Chain[T]
	send  chan Chain[T]
	recv  chan Chain[T]
	data  []T
}

func main() {
	genesis := NewBlock(0)
	m := 20
	n := 6

	// Ring network
	nodes := make([]Node[int], n)
	for i := 0; i < n; i++ {
		var recv chan Chain[int]
		if i != 0 {
			recv = nodes[i-1].send
		}
		data := make([]int, m)
		for j := range data {
			data[j] = i + j*n
		}
		nodes[i] = Node[int]{
			chain: Chain[int]{genesis},
			send:  make(chan Chain[int], m*m),
			recv:  recv,
			data:  data}
	}
	nodes[0].recv = nodes[n-1].send

	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		node := &nodes[i]
		go func() {
			defer wg.Done()

			// simulate network split
			if i == 0 || i == n/2 {
				node.send <- node.chain
			} else {
				time.Sleep(1 * time.Second)
			}

			for _, d := range node.data {
				select {
				case c := <-node.recv:
					if node.chain.Less(c) {
						node.chain = c
					}
				case <-time.After(50 * time.Millisecond):
					fmt.Printf("%d recv timeout\n", i)
				}

				b := node.chain.NextBlock(d)

				// simulate faulty node
				if i == 0 {
					time.Sleep(200 * time.Millisecond)
				}

				node.chain = append(node.chain, b)
				node.send <- node.chain
			}

			// multiple rounds of broadcast for eventual convergence
			for range n * m {
				select {
				case c := <-node.recv:
					if node.chain.Less(c) {
						node.chain = c
					}
				case <-time.After(250 * time.Millisecond):
					fmt.Printf("%d pass recv timeout\n", i)
				}
				node.send <- node.chain
			}

			// consensus achieved
		}()
	}
	wg.Wait()

	for i := range nodes {
		node := &nodes[i]
		for j := range node.chain {
			b := &node.chain[j]
			fmt.Printf("%d(%d) ", b.data, b.difficulty)
		}
		fmt.Printf("[%d]\n", node.chain.CumulativeDifficulty())
		fmt.Println("---")
	}
}
