package wraperr

import "fmt"

type Error struct {
	Context string
	ErrStr  string
}

func (m *Error) Error() string {
	return fmt.Sprintf("func %s: %s", m.Context, m.ErrStr)
}
