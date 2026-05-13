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
	Foreground: tcell.NewRGBColor(29, 32, 33),    // #1d2021 gruvbox dark0_hard

	Border: tcell.NewRGBColor(168, 153, 132), // #a89984 gruvbox fg4 — subtle warm gray
	Title:  tcell.NewRGBColor(7, 102, 120),   // #076678 faded blue — dark, readable on white
	Accent: tcell.NewRGBColor(66, 123, 88),   // #427b58 faded aqua

	FieldText: tcell.NewRGBColor(29, 32, 33),     // #1d2021 near-black
	FieldBg:   tcell.NewRGBColor(242, 238, 230),  // very light warm gray

	SelectionFg: tcell.NewRGBColor(251, 241, 199), // #fbf1c7 cream on dark
	SelectionBg: tcell.NewRGBColor(7, 102, 120),   // #076678 dark teal
	CursorFg:    tcell.NewRGBColor(255, 255, 255), // white
	CursorBg:    tcell.NewRGBColor(29, 32, 33),    // near-black

	VisualSelectionFg: tcell.NewRGBColor(29, 32, 33),    // dark text
	VisualSelectionBg: tcell.NewRGBColor(213, 196, 161), // #d5c4a1 gruvbox bg2

	Subtle: tcell.NewRGBColor(146, 131, 116), // #928374 gruvbox gray

	StatusOK:    tcell.NewRGBColor(66, 123, 88), // #427b58 faded green
	StatusError: tcell.NewRGBColor(157, 0, 6),   // #9d0006 faded red

	JSONKey:     tcell.NewRGBColor(7, 102, 120),   // #076678 dark blue
	JSONString:  tcell.NewRGBColor(66, 123, 88),   // #427b58 dark green
	JSONNumber:  tcell.NewRGBColor(175, 58, 3),    // #af3a03 dark orange
	JSONBool:    tcell.NewRGBColor(143, 63, 113),  // #8f3f71 dark purple
	JSONNull:    tcell.NewRGBColor(157, 0, 6),     // #9d0006 dark red
	JSONBracket: tcell.NewRGBColor(146, 131, 116), // #928374 gray — subtle brackets
}
