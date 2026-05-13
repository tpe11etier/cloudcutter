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

	Subtle tcell.Color

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

func init() {
	Active = CloudCutterModern
}

var CloudCutterModern = &Theme{
	Background: tcell.NewRGBColor(243, 241, 236), // #f3f1ec warm paper
	Foreground: tcell.NewRGBColor(47, 53, 66),    // #2f3542

	Border: tcell.NewRGBColor(216, 212, 204), // #d8d4cc
	Title:  tcell.NewRGBColor(23, 179, 163),  // #17b3a3
	Accent: tcell.NewRGBColor(23, 179, 163),  // #17b3a3

	FieldText: tcell.NewRGBColor(58, 65, 78),    // #3a414e
	FieldBg:   tcell.NewRGBColor(248, 247, 244), // #f8f7f4

	SelectionFg: tcell.NewRGBColor(255, 255, 255),
	SelectionBg: tcell.NewRGBColor(45, 140, 130),  // #2d8c82
	CursorFg:    tcell.NewRGBColor(47, 53, 66),
	CursorBg:    tcell.NewRGBColor(215, 243, 238), // #d7f3ee

	VisualSelectionFg: tcell.NewRGBColor(47, 53, 66),
	VisualSelectionBg: tcell.NewRGBColor(224, 247, 243), // #e0f7f3

	Subtle: tcell.NewRGBColor(154, 163, 175), // #9aa3af

	StatusOK:    tcell.NewRGBColor(46, 125, 50),   // muted green
	StatusError: tcell.NewRGBColor(239, 107, 115), // soft red

	JSONKey:     tcell.NewRGBColor(95, 141, 211),  // soft blue
	JSONString:  tcell.NewRGBColor(46, 125, 50),   // green
	JSONNumber:  tcell.NewRGBColor(233, 185, 73),  // amber
	JSONBool:    tcell.NewRGBColor(126, 87, 194),  // muted purple
	JSONNull:    tcell.NewRGBColor(239, 107, 115), // soft red
	JSONBracket: tcell.NewRGBColor(189, 189, 189), // subtle gray
}

var CloudCutterDark = &Theme{
	Background: tcell.NewRGBColor(24, 28, 34),
	Foreground: tcell.NewRGBColor(222, 226, 230),

	Border: tcell.NewRGBColor(58, 63, 72),
	Title:  tcell.NewRGBColor(77, 208, 193),
	Accent: tcell.NewRGBColor(77, 208, 193),

	FieldText: tcell.NewRGBColor(222, 226, 230),
	FieldBg:   tcell.NewRGBColor(31, 36, 43),

	SelectionFg: tcell.NewRGBColor(255, 255, 255),
	SelectionBg: tcell.NewRGBColor(38, 120, 110),
	CursorFg:    tcell.NewRGBColor(255, 255, 255),
	CursorBg:    tcell.NewRGBColor(52, 73, 94),

	VisualSelectionFg: tcell.NewRGBColor(255, 255, 255),
	VisualSelectionBg: tcell.NewRGBColor(44, 62, 80),

	Subtle: tcell.NewRGBColor(130, 138, 145),

	StatusOK:    tcell.NewRGBColor(102, 187, 106),
	StatusError: tcell.NewRGBColor(239, 107, 115),

	JSONKey:     tcell.NewRGBColor(100, 181, 246),
	JSONString:  tcell.NewRGBColor(129, 199, 132),
	JSONNumber:  tcell.NewRGBColor(255, 213, 79),
	JSONBool:    tcell.NewRGBColor(179, 157, 219),
	JSONNull:    tcell.NewRGBColor(239, 107, 115),
	JSONBracket: tcell.NewRGBColor(120, 120, 120),
}

var GruvboxDark = &Theme{
	Background: tcell.ColorBlack,
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

	Subtle: GruvboxMaterial.Gray,

	StatusOK:    GruvboxMaterial.Green,
	StatusError: GruvboxMaterial.Red,

	JSONKey:     GruvboxMaterial.Blue,
	JSONString:  GruvboxMaterial.Green,
	JSONNumber:  GruvboxMaterial.Orange,
	JSONBool:    GruvboxMaterial.Purple,
	JSONNull:    GruvboxMaterial.Red,
	JSONBracket: GruvboxMaterial.Yellow,
}
