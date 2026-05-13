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
	Background: tcell.ColorBlack, // matches tview's default; preserves original panel appearance
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

var GruvboxLight = &Theme{
	Background: tcell.NewRGBColor(255, 255, 255), // white
	Foreground: tcell.NewRGBColor(33, 33, 33),    // #212121 near-black

	Border: tcell.NewRGBColor(189, 189, 189), // #bdbdbd neutral light gray
	Title:  tcell.NewRGBColor(0, 137, 123),   // #00897b material teal 600
	Accent: tcell.NewRGBColor(0, 137, 123),   // #00897b

	FieldText: tcell.NewRGBColor(33, 33, 33),    // #212121 near-black
	FieldBg:   tcell.NewRGBColor(245, 245, 245), // #f5f5f5 neutral light gray

	SelectionFg: tcell.NewRGBColor(255, 255, 255), // white
	SelectionBg: tcell.NewRGBColor(0, 121, 107),   // #00796b material teal 700
	CursorFg:    tcell.NewRGBColor(255, 255, 255), // white
	CursorBg:    tcell.NewRGBColor(66, 66, 66),    // #424242 dark gray

	VisualSelectionFg: tcell.NewRGBColor(33, 33, 33),     // dark text
	VisualSelectionBg: tcell.NewRGBColor(178, 223, 219),  // #b2dfdb teal 100

	Subtle: tcell.NewRGBColor(117, 117, 117), // #757575 material gray 600

	StatusOK:    tcell.NewRGBColor(56, 142, 60),  // #388e3c material green 700
	StatusError: tcell.NewRGBColor(198, 40, 40),  // #c62828 material red 800

	JSONKey:     tcell.NewRGBColor(2, 119, 189),  // #0277bd material blue 800
	JSONString:  tcell.NewRGBColor(56, 142, 60),  // #388e3c material green 700
	JSONNumber:  tcell.NewRGBColor(230, 81, 0),   // #e65100 material orange 900
	JSONBool:    tcell.NewRGBColor(106, 27, 154), // #6a1b9a material purple 800
	JSONNull:    tcell.NewRGBColor(198, 40, 40),  // #c62828 material red 800
	JSONBracket: tcell.NewRGBColor(158, 158, 158), // #9e9e9e material gray 500
}
