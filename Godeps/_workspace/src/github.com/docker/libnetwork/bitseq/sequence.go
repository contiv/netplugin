// Package bitseq provides a structure and utilities for representing long bitmask
// as sequence of run-lenght encoded blocks. It operates direclty on the encoded
// representation, it does not decode/encode.
package bitseq

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/docker/libnetwork/datastore"
	"github.com/docker/libnetwork/types"
)

// block sequence constants
// If needed we can think of making these configurable
const (
	blockLen      = uint32(32)
	blockBytes    = blockLen / 8
	blockMAX      = uint32(1<<blockLen - 1)
	blockFirstBit = uint32(1) << (blockLen - 1)
	invalidPos    = blockMAX
)

var (
	errNoBitAvailable = fmt.Errorf("no bit available")
)

// Handle contains the sequece representing the bitmask and its identifier
type Handle struct {
	bits       uint32
	unselected uint32
	head       *sequence
	app        string
	id         string
	dbIndex    uint64
	dbExists   bool
	store      datastore.DataStore
	sync.Mutex
}

// NewHandle returns a thread-safe instance of the bitmask handler
func NewHandle(app string, ds datastore.DataStore, id string, numElements uint32) (*Handle, error) {
	h := &Handle{
		app:        app,
		id:         id,
		store:      ds,
		bits:       numElements,
		unselected: numElements,
		head: &sequence{
			block: 0x0,
			count: getNumBlocks(numElements),
		},
	}

	if h.store == nil {
		return h, nil
	}

	// Register for status changes
	h.watchForChanges()

	// Get the initial status from the ds if present.
	if err := h.store.GetObject(datastore.Key(h.Key()...), h); err != nil && err != datastore.ErrKeyNotFound {
		return nil, err
	}

	return h, nil
}

// sequence represents a recurring sequence of 32 bits long bitmasks
type sequence struct {
	block uint32    // block is a symbol representing 4 byte long allocation bitmask
	count uint32    // number of consecutive blocks (symbols)
	next  *sequence // next sequence
}

// String returns a string representation of the block sequence starting from this block
func (s *sequence) toString() string {
	var nextBlock string
	if s.next == nil {
		nextBlock = "end"
	} else {
		nextBlock = s.next.toString()
	}
	return fmt.Sprintf("(0x%x, %d)->%s", s.block, s.count, nextBlock)
}

// GetAvailableBit returns the position of the first unset bit in the bitmask represented by this sequence
func (s *sequence) getAvailableBit() (uint32, uint32, error) {
	if s.block == blockMAX || s.count == 0 {
		return invalidPos, invalidPos, fmt.Errorf("no available bit")
	}
	bits := uint32(0)
	bitSel := blockFirstBit
	for bitSel > 0 && s.block&bitSel != 0 {
		bitSel >>= 1
		bits++
	}
	return bits / 8, bits % 8, nil
}

// GetCopy returns a copy of the linked list rooted at this node
func (s *sequence) getCopy() *sequence {
	n := &sequence{block: s.block, count: s.count}
	pn := n
	ps := s.next
	for ps != nil {
		pn.next = &sequence{block: ps.block, count: ps.count}
		pn = pn.next
		ps = ps.next
	}
	return n
}

// Equal checks if this sequence is equal to the passed one
func (s *sequence) equal(o *sequence) bool {
	this := s
	other := o
	for this != nil {
		if other == nil {
			return false
		}
		if this.block != other.block || this.count != other.count {
			return false
		}
		this = this.next
		other = other.next
	}
	// Check if other is longer than this
	if other != nil {
		return false
	}
	return true
}

// ToByteArray converts the sequence into a byte array
func (s *sequence) toByteArray() ([]byte, error) {
	var bb []byte

	p := s
	for p != nil {
		b := make([]byte, 8)
		binary.BigEndian.PutUint32(b[0:], p.block)
		binary.BigEndian.PutUint32(b[4:], p.count)
		bb = append(bb, b...)
		p = p.next
	}

	return bb, nil
}

// fromByteArray construct the sequence from the byte array
func (s *sequence) fromByteArray(data []byte) error {
	l := len(data)
	if l%8 != 0 {
		return fmt.Errorf("cannot deserialize byte sequence of lenght %d (%v)", l, data)
	}

	p := s
	i := 0
	for {
		p.block = binary.BigEndian.Uint32(data[i : i+4])
		p.count = binary.BigEndian.Uint32(data[i+4 : i+8])
		i += 8
		if i == l {
			break
		}
		p.next = &sequence{}
		p = p.next
	}

	return nil
}

func (h *Handle) getCopy() *Handle {
	return &Handle{
		bits:       h.bits,
		unselected: h.unselected,
		head:       h.head.getCopy(),
		app:        h.app,
		id:         h.id,
		dbIndex:    h.dbIndex,
		dbExists:   h.dbExists,
		store:      h.store,
	}
}

// SetAny atomically sets the first unset bit in the sequence and returns the corresponding ordinal
func (h *Handle) SetAny() (uint32, error) {
	if h.Unselected() == 0 {
		return invalidPos, errNoBitAvailable
	}
	return h.set(0, true, false)
}

// Set atomically sets the corresponding bit in the sequence
func (h *Handle) Set(ordinal uint32) error {
	if err := h.validateOrdinal(ordinal); err != nil {
		return err
	}
	_, err := h.set(ordinal, false, false)
	return err
}

// Unset atomically unsets the corresponding bit in the sequence
func (h *Handle) Unset(ordinal uint32) error {
	if err := h.validateOrdinal(ordinal); err != nil {
		return err
	}
	_, err := h.set(ordinal, false, true)
	return err
}

// IsSet atomically checks if the ordinal bit is set. In case ordinal
// is outside of the bit sequence limits, false is returned.
func (h *Handle) IsSet(ordinal uint32) bool {
	if err := h.validateOrdinal(ordinal); err != nil {
		return false
	}
	h.Lock()
	_, _, err := checkIfAvailable(h.head, ordinal)
	h.Unlock()
	return err != nil
}

// set/reset the bit
func (h *Handle) set(ordinal uint32, any bool, release bool) (uint32, error) {
	var (
		bitPos  uint32
		bytePos uint32
		ret     uint32
		err     error
	)

	for {
		h.Lock()
		// Get position if available
		if release {
			bytePos, bitPos = ordinalToPos(ordinal)
		} else {
			if any {
				bytePos, bitPos, err = getFirstAvailable(h.head)
				ret = posToOrdinal(bytePos, bitPos)
			} else {
				bytePos, bitPos, err = checkIfAvailable(h.head, ordinal)
				ret = ordinal
			}
		}
		if err != nil {
			h.Unlock()
			return ret, err
		}

		// Create a private copy of h and work on it, also copy the current db index
		nh := h.getCopy()
		ci := h.dbIndex
		h.Unlock()

		nh.head = pushReservation(bytePos, bitPos, nh.head, release)
		if release {
			nh.unselected++
		} else {
			nh.unselected--
		}

		// Attempt to write private copy to store
		if err := nh.writeToStore(); err != nil {
			if _, ok := err.(types.RetryError); !ok {
				return ret, fmt.Errorf("internal failure while setting the bit: %v", err)
			}
			// Retry
			continue
		}

		// Unless unexpected error, save private copy to local copy
		h.Lock()
		defer h.Unlock()
		if h.dbIndex != ci {
			return ret, fmt.Errorf("unexected database index change")
		}
		h.unselected = nh.unselected
		h.head = nh.head
		h.dbExists = nh.dbExists
		h.dbIndex = nh.dbIndex
		return ret, nil
	}
}

// checks is needed because to cover the case where the number of bits is not a multiple of blockLen
func (h *Handle) validateOrdinal(ordinal uint32) error {
	if ordinal > h.bits {
		return fmt.Errorf("bit does not belong to the sequence")
	}
	return nil
}

// Destroy removes from the datastore the data belonging to this handle
func (h *Handle) Destroy() {
	h.deleteFromStore()
}

// ToByteArray converts this handle's data into a byte array
func (h *Handle) ToByteArray() ([]byte, error) {

	h.Lock()
	defer h.Unlock()
	ba := make([]byte, 8)
	binary.BigEndian.PutUint32(ba[0:], h.bits)
	binary.BigEndian.PutUint32(ba[4:], h.unselected)
	bm, err := h.head.toByteArray()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize head: %s", err.Error())
	}
	ba = append(ba, bm...)

	return ba, nil
}

// FromByteArray reads his handle's data from a byte array
func (h *Handle) FromByteArray(ba []byte) error {
	if ba == nil {
		return fmt.Errorf("nil byte array")
	}

	nh := &sequence{}
	err := nh.fromByteArray(ba[8:])
	if err != nil {
		return fmt.Errorf("failed to deserialize head: %s", err.Error())
	}

	h.Lock()
	h.head = nh
	h.bits = binary.BigEndian.Uint32(ba[0:4])
	h.unselected = binary.BigEndian.Uint32(ba[4:8])
	h.Unlock()

	return nil
}

// Bits returns the length of the bit sequence
func (h *Handle) Bits() uint32 {
	return h.bits
}

// Unselected returns the number of bits which are not selected
func (h *Handle) Unselected() uint32 {
	h.Lock()
	defer h.Unlock()
	return h.unselected
}

func (h *Handle) String() string {
	h.Lock()
	defer h.Unlock()
	return fmt.Sprintf("App: %s, ID: %s, DBIndex: 0x%x, bits: %d, unselected: %d, sequence: %s",
		h.app, h.id, h.dbIndex, h.bits, h.unselected, h.head.toString())
}

// getFirstAvailable looks for the first unset bit in passed mask
func getFirstAvailable(head *sequence) (uint32, uint32, error) {
	byteIndex := uint32(0)
	current := head
	for current != nil {
		if current.block != blockMAX {
			bytePos, bitPos, err := current.getAvailableBit()
			return byteIndex + bytePos, bitPos, err
		}
		byteIndex += current.count * blockBytes
		current = current.next
	}
	return invalidPos, invalidPos, errNoBitAvailable
}

// checkIfAvailable checks if the bit correspondent to the specified ordinal is unset
// If the ordinal is beyond the sequence limits, a negative response is returned
func checkIfAvailable(head *sequence, ordinal uint32) (uint32, uint32, error) {
	bytePos := ordinal / 8
	bitPos := ordinal % 8

	// Find the sequence containing this byte
	current, _, _, inBlockBytePos := findSequence(head, bytePos)
	if current != nil {
		// Check whether the bit corresponding to the ordinal address is unset
		bitSel := blockFirstBit >> (inBlockBytePos*8 + bitPos)
		if current.block&bitSel == 0 {
			return bytePos, bitPos, nil
		}
	}

	return invalidPos, invalidPos, fmt.Errorf("requested bit is not available")
}

// Given the byte position and the sequences list head, return the pointer to the
// sequence containing the byte (current), the pointer to the previous sequence,
// the number of blocks preceding the block containing the byte inside the current sequence.
// If bytePos is outside of the list, function will return (nil, nil, 0, invalidPos)
func findSequence(head *sequence, bytePos uint32) (*sequence, *sequence, uint32, uint32) {
	// Find the sequence containing this byte
	previous := head
	current := head
	n := bytePos
	for current.next != nil && n >= (current.count*blockBytes) { // Nil check for less than 32 addresses masks
		n -= (current.count * blockBytes)
		previous = current
		current = current.next
	}

	// If byte is outside of the list, let caller know
	if n >= (current.count * blockBytes) {
		return nil, nil, 0, invalidPos
	}

	// Find the byte position inside the block and the number of blocks
	// preceding the block containing the byte inside this sequence
	precBlocks := n / blockBytes
	inBlockBytePos := bytePos % blockBytes

	return current, previous, precBlocks, inBlockBytePos
}

// PushReservation pushes the bit reservation inside the bitmask.
// Given byte and bit positions, identify the sequence (current) which holds the block containing the affected bit.
// Create a new block with the modified bit according to the operation (allocate/release).
// Create a new sequence containing the new block and insert it in the proper position.
// Remove current sequence if empty.
// Check if new sequence can be merged with neighbour (previous/next) sequences.
//
//
// Identify "current" sequence containing block:
//                                      [prev seq] [current seq] [next seq]
//
// Based on block position, resulting list of sequences can be any of three forms:
//
//        block position                        Resulting list of sequences
// A) block is first in current:         [prev seq] [new] [modified current seq] [next seq]
// B) block is last in current:          [prev seq] [modified current seq] [new] [next seq]
// C) block is in the middle of current: [prev seq] [curr pre] [new] [curr post] [next seq]
func pushReservation(bytePos, bitPos uint32, head *sequence, release bool) *sequence {
	// Store list's head
	newHead := head

	// Find the sequence containing this byte
	current, previous, precBlocks, inBlockBytePos := findSequence(head, bytePos)
	if current == nil {
		return newHead
	}

	// Construct updated block
	bitSel := blockFirstBit >> (inBlockBytePos*8 + bitPos)
	newBlock := current.block
	if release {
		newBlock &^= bitSel
	} else {
		newBlock |= bitSel
	}

	// Quit if it was a redundant request
	if current.block == newBlock {
		return newHead
	}

	// Current sequence inevitably looses one block, upadate count
	current.count--

	// Create new sequence
	newSequence := &sequence{block: newBlock, count: 1}

	// Insert the new sequence in the list based on block position
	if precBlocks == 0 { // First in sequence (A)
		newSequence.next = current
		if current == head {
			newHead = newSequence
			previous = newHead
		} else {
			previous.next = newSequence
		}
		removeCurrentIfEmpty(&newHead, newSequence, current)
		mergeSequences(previous)
	} else if precBlocks == current.count-2 { // Last in sequence (B)
		newSequence.next = current.next
		current.next = newSequence
		mergeSequences(current)
	} else { // In between the sequence (C)
		currPre := &sequence{block: current.block, count: precBlocks, next: newSequence}
		currPost := current
		currPost.count -= precBlocks
		newSequence.next = currPost
		if currPost == head {
			newHead = currPre
		} else {
			previous.next = currPre
		}
		// No merging or empty current possible here
	}

	return newHead
}

// Removes the current sequence from the list if empty, adjusting the head pointer if needed
func removeCurrentIfEmpty(head **sequence, previous, current *sequence) {
	if current.count == 0 {
		if current == *head {
			*head = current.next
		} else {
			previous.next = current.next
			current = current.next
		}
	}
}

// Given a pointer to a sequence, it checks if it can be merged with any following sequences
// It stops when no more merging is possible.
// TODO: Optimization: only attempt merge from start to end sequence, no need to scan till the end of the list
func mergeSequences(seq *sequence) {
	if seq != nil {
		// Merge all what possible from seq
		for seq.next != nil && seq.block == seq.next.block {
			seq.count += seq.next.count
			seq.next = seq.next.next
		}
		// Move to next
		mergeSequences(seq.next)
	}
}

func getNumBlocks(numBits uint32) uint32 {
	numBlocks := numBits / blockLen
	if numBits%blockLen != 0 {
		numBlocks++
	}
	return numBlocks
}

func ordinalToPos(ordinal uint32) (uint32, uint32) {
	return ordinal / 8, ordinal % 8
}

func posToOrdinal(bytePos, bitPos uint32) uint32 {
	return bytePos*8 + bitPos
}
