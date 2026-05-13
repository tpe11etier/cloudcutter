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
	Background: tcell.NewRGBColor(251, 241, 199), // #fbf1c7
	Foreground: tcell.NewRGBColor(60, 56, 54),    // #3c3836

	Border: tcell.NewRGBColor(69, 133, 136),  // #458588 neutral blue
	Title:  tcell.NewRGBColor(181, 118, 20),  // #b57614 faded yellow/gold
	Accent: tcell.NewRGBColor(66, 123, 88),   // #427b58 faded aqua

	FieldText: tcell.NewRGBColor(60, 56, 54),    // #3c3836 dark fg
	FieldBg:   tcell.NewRGBColor(235, 219, 178), // #ebdbb2 bg1

	SelectionFg: tcell.NewRGBColor(251, 241, 199), // #fbf1c7 light on dark
	SelectionBg: tcell.NewRGBColor(69, 133, 136),  // #458588 blue
	CursorFg:    tcell.NewRGBColor(251, 241, 199), // #fbf1c7
	CursorBg:    tcell.NewRGBColor(60, 56, 54),    // #3c3836

	VisualSelectionFg: tcell.NewRGBColor(60, 56, 54),    // #3c3836
	VisualSelectionBg: tcell.NewRGBColor(213, 196, 161), // #d5c4a1 bg2

	Subtle: tcell.NewRGBColor(146, 131, 116), // #928374

	StatusOK:    tcell.NewRGBColor(121, 116, 14), // #79740e faded green
	StatusError: tcell.NewRGBColor(157, 0, 6),    // #9d0006 faded red

	JSONKey:     tcell.NewRGBColor(7, 102, 120),   // #076678 faded blue
	JSONString:  tcell.NewRGBColor(121, 116, 14),  // #79740e faded green
	JSONNumber:  tcell.NewRGBColor(175, 58, 3),    // #af3a03 faded orange
	JSONBool:    tcell.NewRGBColor(143, 63, 113),  // #8f3f71 faded purple
	JSONNull:    tcell.NewRGBColor(157, 0, 6),     // #9d0006 faded red
	JSONBracket: tcell.NewRGBColor(181, 118, 20),  // #b57614 faded yellow
}
