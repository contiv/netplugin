/***
Copyright 2016 Cisco Systems Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pqueue

import (
	"container/heap"
	"errors"
)

// An Item is what we manage the queue.
type Item struct {
	value    string // The value of the item; arbitrary.
	priority int    // The priority of the item in the queue.
	index    int    // index of the item in pq
}

// A MinPQueue implements heap.Interface and holds Items.
type MinPQueue []*Item

// Len returns the current length of the pqueue
func (pq MinPQueue) Len() int {
	return len(pq)
}

// Less returns true when the first item has higher priority
func (pq MinPQueue) Less(i, j int) bool {
	// min heap -- lower the value, higher the priority
	return pq[i].priority < pq[j].priority
}

// Swap swaps the positions of the two items
func (pq MinPQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

// Push adds the item to the end of the queue
func (pq *MinPQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.index = n
	*pq = append(*pq, item)
}

// Pop removes the last item and returns it
func (pq *MinPQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// GetMin returns the value of the root item
func (pq *MinPQueue) GetMin() string {
	queue := *pq
	return queue[0].value
}

// IncreaseMin increments the priority of the root item
func (pq *MinPQueue) IncreaseMin() {
	queue := *pq
	queue[0].priority += 1
	heap.Fix(pq, 0)
}

// DecreaseItem decrements the priority of the specified item
func (pq *MinPQueue) DecreaseItem(ip *Item) error {
	// make sure index is valid
	index := ip.index
	queue := *pq
	count := len(queue)
	if !(index < count) {
		return errors.New("Item index is invalid")
	}

	if queue[index].priority > 0 {
		queue[index].priority -= 1
	}

	heap.Fix(pq, index)
	return nil
}

// RemoveItem removes the specified item from pq
func (pq *MinPQueue) RemoveItem(ip *Item) error {
	// make sure index is valid
	index := ip.index
	count := len(*pq)
	if !(index < count) {
		return errors.New("Item index is invalid")
	}

	heap.Remove(pq, index)
	return nil
}

// PushItem adds the specified item to the pq
func (pq *MinPQueue) PushItem(item *Item) {
	heap.Push(pq, item)
}

// NewItem creates and initializes an item
func NewItem(val string) *Item {
	item := &Item{
		value:    val,
		priority: 0,
		index:    -1,
	}

	return item
}

// NewMinPQueue creates a pq and initializes it
func NewMinPQueue() *MinPQueue {
	pq := MinPQueue{}
	heap.Init(&pq)
	return &pq
}
