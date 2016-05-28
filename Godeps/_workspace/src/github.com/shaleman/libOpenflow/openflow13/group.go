package openflow13

// This file has all group related defs

import (
	"encoding/binary"

	log "github.com/Sirupsen/logrus"
	"github.com/shaleman/libOpenflow/common"
)

const (
	OFPG_MAX = 0xffffff00 /* Last usable group number. */
	/* Fake groups. */
	OFPG_ALL = 0xfffffffc /* Represents all groups for group delete commands. */
	OFPG_ANY = 0xffffffff /* Wildcard group used only for flow stats requests. Selects all flows regardless of group (including flows with no group).
	 */
)

const (
	OFPGC_ADD    = 0 /* New group. */
	OFPGC_MODIFY = 1 /* Modify all matching groups. */
	OFPGC_DELETE = 2 /* Delete all matching groups. */
)

const (
	OFPGT_ALL      = 0 /* All (multicast/broadcast) group. */
	OFPGT_SELECT   = 1 /* Select group. */
	OFPGT_INDIRECT = 2 /* Indirect group. */
	OFPGT_FF       = 3 /* Fast failover group. */
)

// GroupMod message
type GroupMod struct {
	common.Header
	Command uint16   /* One of OFPGC_*. */
	Type    uint8    /* One of OFPGT_*. */
	pad     uint8    /* Pad to 64 bits. */
	GroupId uint32   /* Group identifier. */
	Buckets []Bucket /* List of buckets */
}

// Create a new group mode message
func NewGroupMod() *GroupMod {
	g := new(GroupMod)
	g.Header = NewOfp13Header()
	g.Header.Type = Type_GroupMod

	g.Command = OFPGC_ADD
	g.Type = OFPGT_ALL
	g.GroupId = 0
	g.Buckets = make([]Bucket, 0)
	return g
}

// Add a bucket to group mod
func (g *GroupMod) AddBucket(bkt Bucket) {
	g.Buckets = append(g.Buckets, bkt)
}

func (g *GroupMod) Len() (n uint16) {
	n = g.Header.Len()
	n += 8
	if g.Command == OFPGC_DELETE {
		return
	}

	for _, b := range g.Buckets {
		n += b.Len()
	}

	return
}

func (g *GroupMod) MarshalBinary() (data []byte, err error) {
	g.Header.Length = g.Len()
	data, err = g.Header.MarshalBinary()

	bytes := make([]byte, 8)
	n := 0
	binary.BigEndian.PutUint16(bytes[n:], g.Command)
	n += 2
	bytes[n] = g.Type
	n += 1
	bytes[n] = g.pad
	n += 1
	binary.BigEndian.PutUint32(bytes[n:], g.GroupId)
	n += 4
	data = append(data, bytes...)

	for _, bkt := range g.Buckets {
		bytes, err = bkt.MarshalBinary()
		data = append(data, bytes...)
		log.Debugf("Groupmod bucket: %v", bytes)
	}

	log.Debugf("GroupMod(%d): %v", len(data), data)

	return
}

func (g *GroupMod) UnmarshalBinary(data []byte) error {
	n := 0
	g.Header.UnmarshalBinary(data[n:])
	n += int(g.Header.Len())

	g.Command = binary.BigEndian.Uint16(data[n:])
	n += 2
	g.Type = data[n]
	n += 1
	g.pad = data[n]
	n += 1
	g.GroupId = binary.BigEndian.Uint32(data[n:])
	n += 4

	for n < int(g.Header.Length) {
		bkt := new(Bucket)
		bkt.UnmarshalBinary(data[n:])
		g.Buckets = append(g.Buckets, *bkt)
		n += int(bkt.Len())
	}

	return nil
}

type Bucket struct {
	Length     uint16   /* Length the bucket in bytes, including this header and any padding to make it 64-bit aligned. */
	Weight     uint16   /* Relative weight of bucket. Only defined for select groups. */
	WatchPort  uint32   /* Used for FRR groups */
	WatchGroup uint32   /* Used for FRR groups */
	pad        []byte   /* 4 bytes */
	Actions    []Action /* zero or more actions */
}

// Create a new Bucket
func NewBucket() *Bucket {
	bkt := new(Bucket)

	bkt.Weight = 1
	bkt.pad = make([]byte, 4)
	bkt.Actions = make([]Action, 0)
	bkt.WatchPort = P_ANY
	bkt.WatchGroup = OFPG_ANY
	bkt.Length = bkt.Len()

	return bkt
}

// Add an action to the bucket
func (b *Bucket) AddAction(act Action) {
	b.Actions = append(b.Actions, act)
}

func (b *Bucket) Len() (n uint16) {
	n = 16

	for _, a := range b.Actions {
		n += a.Len()
	}

	// Round it to closest multiple of 8
	n = ((n + 7) / 8) * 8
	return
}

func (b *Bucket) MarshalBinary() (data []byte, err error) {
	bytes := make([]byte, 16)
	n := 0
	b.Length = b.Len() // Calculate length first
	binary.BigEndian.PutUint16(bytes[n:], b.Length)
	n += 2
	binary.BigEndian.PutUint16(bytes[n:], b.Weight)
	n += 2
	binary.BigEndian.PutUint32(bytes[n:], b.WatchPort)
	n += 4
	binary.BigEndian.PutUint32(bytes[n:], b.WatchGroup)
	n += 4
	data = append(data, bytes...)

	for _, a := range b.Actions {
		bytes, err = a.MarshalBinary()
		data = append(data, bytes...)
	}

	return
}

func (b *Bucket) UnmarshalBinary(data []byte) error {
	n := 0
	b.Length = binary.BigEndian.Uint16(data[n:])
	n += 2
	b.Weight = binary.BigEndian.Uint16(data[n:])
	n += 2
	b.WatchPort = binary.BigEndian.Uint32(data[n:])
	n += 4
	b.WatchGroup = binary.BigEndian.Uint32(data[n:])
	n += 4
	n += 4 // for padding

	for n < int(b.Length) {
		a := DecodeAction(data[n:])
		b.Actions = append(b.Actions, a)
		n += int(a.Len())
	}

	return nil
}
