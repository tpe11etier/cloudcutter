package types

import (
	"github.com/gdamore/tcell/v2"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/help"
)

type Action struct {
	Shortcut    string
	Description string
}

type ComponentType string

type Style struct {
	Border          bool
	BorderColor     tcell.Color
	Title           string
	TitleAlign      int
	TitleColor      tcell.Color
	BackgroundColor tcell.Color
	TextColor       tcell.Color
}

type Component struct {
	ID         string
	Type       ComponentType
	Direction  int
	FixedSize  int
	Proportion int
	Focus      bool
	Style      Style
	Properties map[string]any
	Children   []Component
	Help       []help.Command
}

type LayoutConfig struct {
	Title      string
	Components []Component
	Direction  int
}

type SummaryItem struct {
	Key   string
	Value string
}

const (
	ComponentList       ComponentType = "list"
	ComponentTextView   ComponentType = "textview"
	ComponentFlex       ComponentType = "flex"
	ComponentTable      ComponentType = "table"
	ComponentInputField ComponentType = "inputfield"
	ComponentHelp       ComponentType = "help"
)
