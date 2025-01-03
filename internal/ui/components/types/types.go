package types

import (
	"encoding/json"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/tpelletiersophos/cloudcutter/internal/ui/help"
)

const (
	ComponentList       ComponentType = "list"
	ComponentTextView   ComponentType = "textview"
	ComponentFlex       ComponentType = "flex"
	ComponentTable      ComponentType = "table"
	ComponentInputField ComponentType = "inputfield"

	ViewDynamoDB = "dynamodb"
	ViewElastic  = "elastic"
	ViewTest     = "test"

	ModalCmdPrompt  = "modalPrompt"
	ModalFilter     = "modalFilter"
	ModalRowDetails = "modalRowDetails"
	ModalProfile    = "profileSelector"
	ModalRegion     = "regionSelector"
)

type ComponentType string

type Action struct {
	Shortcut    string
	Description string
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

type BaseStyle struct {
	Border          bool
	BorderColor     tcell.Color
	Title           string
	TitleAlign      int
	TitleColor      tcell.Color
	BackgroundColor tcell.Color
	TextColor       tcell.Color
}

type BaseProperties struct{}

type ListStyle struct {
	BaseStyle
	SelectedTextColor       tcell.Color
	SelectedBackgroundColor tcell.Color
}

type ListProperties struct {
	BaseProperties
	Items         []string
	ShowSecondary bool
	NewHelp       *help.HelpCategory
	OnFocus       func(*tview.List)
	OnBlur        func(*tview.List)
	OnChanged     func(int, string, string, rune)
	OnSelected    func(int, string, string, rune)
}

type TableStyle struct {
	BaseStyle
	SelectedTextColor       tcell.Color
	SelectedBackgroundColor tcell.Color
}

type TableProperties struct {
	BaseProperties
	OnFocus    func(*tview.Table)
	OnBlur     func(*tview.Table)
	OnSelected func(row, col int)
}

type TextViewStyle struct {
	BaseStyle
}

type TextViewProperties struct {
	BaseProperties
	Text          string
	Wrap          bool
	Scrollable    bool
	DynamicColors bool
	OnFocus       func(*tview.TextView)
	OnBlur        func(*tview.TextView)
}

type FlexStyle struct {
	BaseStyle
}

type FlexProperties struct {
	BaseProperties
	OnFocus func(*tview.Flex)
	OnBlur  func(*tview.Flex)
}

type InputFieldStyle struct {
	BaseStyle
	LabelColor           tcell.Color
	FieldBackgroundColor tcell.Color
	FieldTextColor       tcell.Color
}

type InputFieldProperties struct {
	BaseProperties
	Label       string
	FieldWidth  int
	Text        string
	DoneFunc    func(string)
	ChangedFunc func(string)
	OnFocus     func(*tview.InputField)
	OnBlur      func(*tview.InputField)
}

type HelpStyle struct {
	BaseStyle
}

type Component struct {
	ID         string
	Type       ComponentType
	Direction  int
	FixedSize  int
	Proportion int
	Focus      bool
	Style      any
	Properties any
	Children   []Component
	Help       []help.Command
	HelpProps  *help.HelpProperties
	OnCreate   func(p tview.Primitive)
}

type ESSearchHit struct {
	ID      string          `json:"_id"`
	Index   string          `json:"_index"`
	Type    string          `json:"_type"`
	Score   *float64        `json:"_score"`
	Version *int64          `json:"_version"`
	Source  json.RawMessage `json:"_source"`
}

type ESSearchResult struct {
	ScrollID string `json:"_scroll_id"`
	Source   json.RawMessage
	Hits     struct {
		Total int           `json:"total"`
		Hits  []ESSearchHit `json:"hits"`
	} `json:"hits"`
}
