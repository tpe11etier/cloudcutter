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
	Foreground: tcell.NewRGBColor(33, 33, 33),    // #212121

	Border: tcell.NewRGBColor(224, 224, 224), // #e0e0e0 barely-there gray
	Title:  tcell.NewRGBColor(0, 105, 92),    // #00695c teal 800 — refined, not garish
	Accent: tcell.NewRGBColor(0, 105, 92),    // #00695c

	FieldText: tcell.NewRGBColor(33, 33, 33),    // #212121
	FieldBg:   tcell.NewRGBColor(250, 250, 250), // #fafafa near-white

	SelectionFg: tcell.NewRGBColor(255, 255, 255), // white
	SelectionBg: tcell.NewRGBColor(0, 121, 107),   // #00796b teal 700
	CursorFg:    tcell.NewRGBColor(255, 255, 255), // white
	CursorBg:    tcell.NewRGBColor(69, 90, 100),   // #455a64 blue-gray 700

	VisualSelectionFg: tcell.NewRGBColor(0, 77, 64),      // #004d40 teal 900
	VisualSelectionBg: tcell.NewRGBColor(224, 242, 241),  // #e0f2f1 teal 50 — very subtle

	Subtle: tcell.NewRGBColor(158, 158, 158), // #9e9e9e gray 500

	StatusOK:    tcell.NewRGBColor(46, 125, 50),  // #2e7d32 green 800
	StatusError: tcell.NewRGBColor(183, 28, 28),  // #b71c1c red 900

	JSONKey:     tcell.NewRGBColor(21, 101, 192), // #1565c0 blue 800
	JSONString:  tcell.NewRGBColor(46, 125, 50),  // #2e7d32 green 800
	JSONNumber:  tcell.NewRGBColor(191, 54, 12),  // #bf360c deep-orange 900
	JSONBool:    tcell.NewRGBColor(106, 27, 154), // #6a1b9a purple 900
	JSONNull:    tcell.NewRGBColor(183, 28, 28),  // #b71c1c red 900
	JSONBracket: tcell.NewRGBColor(189, 189, 189), // #bdbdbd gray 400 — very subtle
}
