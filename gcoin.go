package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// https://lhartikk.github.io/

type Block[T any] struct {
	index        int64
	timestamp    time.Time
	data         T
	previousHash [32]byte
	hash         [32]byte
}

func computeHash[T any](b *Block[T]) [32]byte {
	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, b.index)
	binary.Write(&buf, binary.BigEndian, b.timestamp.UnixNano())
	binary.Write(&buf, binary.BigEndian, b.data)
	binary.Write(&buf, binary.BigEndian, b.previousHash)
	return sha256.Sum256(buf.Bytes())
}

func NewBlock[T any](data T) Block[T] {
	b := Block[T]{
		index:        0,
		timestamp:    time.Now(),
		data:         data,
		previousHash: [32]byte{},
		hash:         [32]byte{}}
	b.hash = computeHash(&b)
	return b
}

func NextBlock[T any](prev *Block[T], data T) Block[T] {
	b := Block[T]{
		index:        prev.index + 1,
		timestamp:    time.Now(),
		data:         data,
		previousHash: prev.hash,
		hash:         [32]byte{}}
	b.hash = computeHash(&b)
	return b
}

func isValid[T any](chain []Block[T]) bool {
	for i, b := range chain {
		if int64(i) != b.index {
			return false
		}
		if i != 0 && chain[i-1].hash != b.previousHash {
			return false
		}
		if computeHash(&b) != b.hash {
			return false
		}
	}
	return true
}

type Node[T any] struct {
	chain []Block[T]
	send  chan []Block[T]
	recv  chan []Block[T]
	data  []T
}

func main() {
	genesis := NewBlock(0)
	n := 4

	// Ring network
	nodes := make([]Node[int], n)
	for i := 0; i < n; i++ {
		var recv chan []Block[int]
		if i != 0 {
			recv = nodes[i-1].send
		}
		nodes[i] = Node[int]{
			chain: []Block[int]{genesis},
			send:  make(chan []Block[int], n),
			recv:  recv,
			data:  []int{i, i + n, i + 2*n}}
	}
	nodes[0].recv = nodes[n-1].send

	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		node := &nodes[i]
		go func() {
			defer wg.Done()

			if i == n-1 {
				node.send <- node.chain
			}
			for _, d := range node.data {
				// time.Sleep(time.Duration(d) * 100 * time.Millisecond)
				c := <-node.recv

				// no consensus
				if len(node.chain) < len(c) {
					node.chain = c
				}
				last := node.chain[len(node.chain)-1]
				b := NextBlock(&last, d)
				node.chain = append(node.chain, b)
				node.send <- node.chain
			}
		}()
	}
	wg.Wait()

	for i := range nodes {
		node := &nodes[i]
		for j := range node.chain {
			b := &node.chain[j]
			fmt.Println(b.data)
		}
		fmt.Println("---")
	}
}
