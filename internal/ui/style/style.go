package style

import (
	"github.com/gdamore/tcell/v2"
)

var GruvboxMaterial = struct {
	// Basic Colors
	Background tcell.Color
	Foreground tcell.Color
	Cursor     tcell.Color
	Comment    tcell.Color
	Gray       tcell.Color

	// Dark Shades
	Dark0 tcell.Color
	Dark1 tcell.Color
	Dark2 tcell.Color

	// Accent Colors
	Red    tcell.Color
	Green  tcell.Color
	Yellow tcell.Color
	Blue   tcell.Color
	Purple tcell.Color
	Aqua   tcell.Color
	Orange tcell.Color
}{
	// Basic Colors
	Background: tcell.NewRGBColor(50, 48, 47),    // #32302f
	Foreground: tcell.NewRGBColor(213, 196, 161), // #d5c4a1
	Cursor:     tcell.NewRGBColor(213, 196, 161), // #d5c4a1
	Comment:    tcell.NewRGBColor(146, 131, 116), // #928374
	Gray:       tcell.NewRGBColor(146, 131, 116), // #928374

	// Dark Shades
	Dark0: tcell.NewRGBColor(29, 32, 33), // #1d2021
	Dark1: tcell.NewRGBColor(40, 40, 40), // #282828
	Dark2: tcell.NewRGBColor(60, 56, 54), // #3c3836

	// Accent Colors
	Red:    tcell.NewRGBColor(251, 73, 52),   // #fb4934
	Green:  tcell.NewRGBColor(184, 187, 38),  // #b8bb26
	Yellow: tcell.NewRGBColor(250, 189, 47),  // #fabd2f
	Blue:   tcell.NewRGBColor(131, 165, 152), // #83a598
	Purple: tcell.NewRGBColor(211, 134, 155), // #d3869b
	Aqua:   tcell.NewRGBColor(142, 192, 124), // #8ec07c
	Orange: tcell.NewRGBColor(254, 128, 25),  // #fe8019
}
