package style

import (
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"strings"
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

func ColorizeJSON(input string) string {
	var data any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
		// if it's not json, just return uncolored text.
		return input
	}

	sb := &strings.Builder{}
	ColorizeValue(data, 0, sb, false)
	return sb.String()
}

// ColorizeValue recursively walks the JSON data and appends a colorized representation to sb.
func ColorizeValue(v any, indent int, sb *strings.Builder, isKey bool) {
	switch val := v.(type) {

	case map[string]interface{}:
		sb.WriteString(fmt.Sprintf("[%s]{[%s]\n", GruvboxMaterial.Yellow, tcell.ColorReset))
		keys := 0
		for k, child := range val {
			keys++
			writeIndent(sb, indent+2)
			// Color the key as a string:
			sb.WriteString(fmt.Sprintf(`[%s]"%s"[%s]: `, GruvboxMaterial.Blue, k, tcell.ColorReset))
			// Then colorize the child
			ColorizeValue(child, indent+2, sb, false)
			if keys < len(val) {
				sb.WriteRune(',')
			}
			sb.WriteRune('\n')
		}
		writeIndent(sb, indent)
		sb.WriteString(fmt.Sprintf("[%s]}[%s]", GruvboxMaterial.Yellow, tcell.ColorReset))

	case []interface{}:
		sb.WriteString(fmt.Sprintf("[%s][[%s]\n", GruvboxMaterial.Yellow, tcell.ColorReset))
		for i, elem := range val {
			writeIndent(sb, indent+2)
			ColorizeValue(elem, indent+2, sb, false)
			if i < len(val)-1 {
				sb.WriteRune(',')
			}
			sb.WriteRune('\n')
		}
		writeIndent(sb, indent)
		sb.WriteString(fmt.Sprintf("[%s]][%s]", GruvboxMaterial.Yellow, tcell.ColorReset))

	case string:
		sb.WriteString(fmt.Sprintf(`[%s]"%s"[%s]`, GruvboxMaterial.Green, val, tcell.ColorReset))

	case float64:
		// JSON numbers get our orange color
		sb.WriteString(fmt.Sprintf(`[%s]%v[%s]`, GruvboxMaterial.Orange, val, tcell.ColorReset))

	case bool:
		// true / false in purple
		sb.WriteString(fmt.Sprintf(`[%s]%t[%s]`, GruvboxMaterial.Purple, val, tcell.ColorReset))

	case nil:
		// null in red
		sb.WriteString(fmt.Sprintf(`[%s]null[%s]`, GruvboxMaterial.Red, tcell.ColorReset))

	default:
		// Fallback—just stringify
		sb.WriteString(fmt.Sprintf("%v", val))
	}
}

func writeIndent(sb *strings.Builder, spaces int) {
	for i := 0; i < spaces; i++ {
		sb.WriteRune(' ')
	}
}
