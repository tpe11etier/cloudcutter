package style

import "github.com/gdamore/tcell/v2"

type Theme struct {
	Background tcell.Color
	Foreground tcell.Color

	Border tcell.Color
	Title  tcell.Color
	Accent tcell.Color

	FieldText tcell.Color
	FieldBg   tcell.Color

	SelectionFg tcell.Color
	SelectionBg tcell.Color
	CursorFg    tcell.Color
	CursorBg    tcell.Color

	VisualSelectionFg tcell.Color
	VisualSelectionBg tcell.Color

	Subtle      tcell.Color

	StatusOK    tcell.Color
	StatusError tcell.Color

	JSONKey     tcell.Color
	JSONString  tcell.Color
	JSONNumber  tcell.Color
	JSONBool    tcell.Color
	JSONNull    tcell.Color
	JSONBracket tcell.Color
}

var Active *Theme

func SetActive(t *Theme) { Active = t }

func init() { Active = GruvboxDark }

var GruvboxDark = &Theme{
	Background: GruvboxMaterial.Background,
	Foreground: GruvboxMaterial.Foreground,
	Border:     tcell.ColorMediumTurquoise,
	Title:      GruvboxMaterial.Yellow,
	Accent:     tcell.ColorTeal,

	FieldText: tcell.ColorBeige,
	FieldBg:   tcell.ColorBlack,

	SelectionFg: tcell.ColorLightYellow,
	SelectionBg: tcell.ColorDarkCyan,
	CursorFg:    tcell.ColorWhite,
	CursorBg:    tcell.ColorBlue,

	VisualSelectionFg: tcell.ColorBlack,
	VisualSelectionBg: tcell.ColorYellow,

	Subtle:      GruvboxMaterial.Gray,

	StatusOK:    GruvboxMaterial.Green,
	StatusError: GruvboxMaterial.Red,

	JSONKey:     GruvboxMaterial.Blue,
	JSONString:  GruvboxMaterial.Green,
	JSONNumber:  GruvboxMaterial.Orange,
	JSONBool:    GruvboxMaterial.Purple,
	JSONNull:    GruvboxMaterial.Red,
	JSONBracket: GruvboxMaterial.Yellow,
}

// GruvboxLight is a placeholder; set to a Theme to enable the Ctrl+L toggle.
var GruvboxLight *Theme
