package core

// State identifies data uniquely identifiable by 'id' and stored in a
// (distributed) key-value store implemented by core.StateDriver.
type State interface {
	Read(id string) error
	ReadAll() ([]State, error)
	Write() error
	Clear() error
}

// WatchableState allows for the rest of core.State, plus the WatchAll call
// which allows the implementor to yield changes to a channel.
type WatchableState interface {
	State
	WatchAll(rsps chan WatchState) error
}

// CommonState defines the fields common to all core.State implementations.
// This struct shall be embedded as anonymous field in all structs that
// implement core.State
type CommonState struct {
	StateDriver StateDriver `json:"-"`
	ID          string      `json:"id"`
}
