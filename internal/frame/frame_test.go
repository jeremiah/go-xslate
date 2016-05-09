package frame

import (
	"testing"

	"github.com/lestrrat/go-xslate/internal/stack"
)

func TestFrame_Lvar(t *testing.T) {
	f := New(stack.New(5))
	f.SetLvar(0, 1)
	x := f.GetLvar(0)
	i, ok := x.(int)
	if !ok {
		t.Errorf("GetLvar(0) did not return an int")
	} else {
		if i != 1 {
			t.Errorf("GetLvar(0) did not return 1, it returned %d", i)
		}
	}
}
