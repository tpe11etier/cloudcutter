package manager

import (
	"context"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/tpe11etier/cloudcutter/internal/ui/views"
)

type fakeView struct {
	name   string
	shown  bool
	hidden bool
}

func (v *fakeView) Name() string                                        { return v.name }
func (v *fakeView) Content() tview.Primitive                            { return tview.NewBox() }
func (v *fakeView) Show()                                               { v.shown = true; v.hidden = false }
func (v *fakeView) Hide()                                               { v.hidden = true; v.shown = false }
func (v *fakeView) InputHandler() func(*tcell.EventKey) *tcell.EventKey { return nil }

func TestCenteredModal(t *testing.T) {
	modal := centeredModal(tview.NewBox(), 60, 10)
	_, ok := modal.(*tview.Grid)
	assert.True(t, ok, "centeredModal should return a *tview.Grid")
}

func TestTitleCase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"username", "Username"},
		{"password", "Password"},
		{"a", "A"},
		{"UPPER", "UPPER"},
		{"", ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, titleCase(c.in), "titleCase(%q)", c.in)
	}
}

func TestRegisterView(t *testing.T) {
	vm := &Manager{views: make(map[string]views.View)}

	v1 := &fakeView{name: "alpha"}
	assert.NoError(t, vm.RegisterView(v1))
	assert.Same(t, v1, vm.views["alpha"])

	v2 := &fakeView{name: "alpha"}
	err := vm.RegisterView(v2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
	assert.Same(t, v1, vm.views["alpha"], "original view should not be replaced")
}

func TestViewContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	vm := &Manager{ctx: ctx}
	assert.Equal(t, ctx, vm.ViewContext())
}
