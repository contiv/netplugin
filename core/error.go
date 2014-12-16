package core

type Error struct {
	Desc string
}

func (e *Error) Error() string {
	return e.Desc
}
