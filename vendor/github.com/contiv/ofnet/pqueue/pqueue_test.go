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
	"fmt"
	"math/rand"
	"sort"
	"testing"
)

// TestGetMin tests GetMin
func TestGetMin(t *testing.T) {
	testNums := make([]int, 10)

	pq := NewMinPQueue()
	// Add 10 random numbers.
	for ix := 0; ix < 10; ix++ {
		r := rand.Intn(50)
		testNums[ix] = r
		item := NewItem(fmt.Sprintf("item%d", r))
		item.priority = r
		pq.PushItem(item)
	}

	dummyItem := NewItem("dummy")
	dummyItem.index = 0
	// sort testNums and verify pq against it
	sort.Ints(testNums)
	for ix := 0; ix < 10; ix++ {
		exp := fmt.Sprintf("item%d", testNums[ix])
		res := pq.GetMin()
		if res != exp {
			fmt.Println("testNums: ", testNums)
			t.Fatalf("Got %s, expected: %s", res, exp)
		}

		// move to next item
		pq.RemoveItem(dummyItem)
	}
}

// TestIncreaseMin tests IncreaseMin
func TestIncreaseMin(t *testing.T) {
	pq := NewMinPQueue()

	for ix := 0; ix < 3; ix++ {
		item := NewItem(fmt.Sprintf("entry%d", ix))
		item.priority = ix
		pq.PushItem(item)
	}

	res := pq.GetMin()
	if res != "entry0" {
		t.Fatalf("Got %s, expected entry0", res)
	}

	// in at most two attempts, we should get entry1
	for ix := 0; ix < 2; ix++ {
		pq.IncreaseMin()
		res = pq.GetMin()
		if res == "entry1" {
			break
		}
	}

	if res != "entry1" {
		t.Fatalf("Got %s, expected entry1", res)
	}

	// in at most three attempts, we should get entry2
	for ix := 0; ix < 3; ix++ {
		pq.IncreaseMin()
		res = pq.GetMin()
		if res == "entry2" {
			break
		}
	}

	if res != "entry2" {
		t.Fatalf("Got %s, expected entry2", res)
	}

}

// TestDecreaseItem tests DecreaseItem
func TestDecreaseItem(t *testing.T) {
	pq := NewMinPQueue()

	for ix := 1; ix < 3; ix++ {
		item := NewItem(fmt.Sprintf("decr%d", ix))
		item.priority = ix
		pq.PushItem(item)
	}

	testItem := NewItem("testItem")
	testItem.priority = 3
	pq.PushItem(testItem)

	res := pq.GetMin()
	if res != "decr1" {
		t.Fatalf("Got %s, expected decr0", res)
	}

	for ix := 0; ix < 3; ix++ {
		pq.DecreaseItem(testItem)
	}

	res = pq.GetMin()
	if res != "testItem" {
		t.Fatalf("Got %s, expected testItem", res)
	}
}

// TestRemoveItem tests RemoveItem
func TestRemoveItem(t *testing.T) {
	items := make([]*Item, 3)
	pq := NewMinPQueue()

	for ix := 0; ix < 3; ix++ {
		item := NewItem(fmt.Sprintf("rem%d", ix))
		item.priority = ix
		pq.PushItem(item)
		items[ix] = item
	}

	res := pq.GetMin()
	if res != "rem0" {
		t.Fatalf("Got %s, expected rem0", res)
	}

	pq.RemoveItem(items[0])
	pq.RemoveItem(items[1])
	res = pq.GetMin()
	if res != "rem2" {
		t.Fatalf("Got %s, expected rem2", res)
	}
}
