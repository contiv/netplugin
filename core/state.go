package core

type State interface {
	Read(id string) error
	ReadAll() ([]State, error)
	Write() error
	Clear() error
}

// CommonState defines the fields common to all core.State implementations.
// This struct shall be embedded as anonymous field in all structs that
// implement core.State
type CommonState struct {
	StateDriver StateDriver `json:"-"`
	Id          string      `json:"id"`
}
