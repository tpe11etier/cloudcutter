package style

import (
	"encoding/json"
	"fmt"
	"strings"

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

func ColorizeJSON(input string) string {
	var data any
	if err := json.Unmarshal([]byte(input), &data); err != nil {
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
		sb.WriteString(fmt.Sprintf("[%s]{[%s]\n", Active.JSONBracket, tcell.ColorReset))
		keys := 0
		for k, child := range val {
			keys++
			writeIndent(sb, indent+2)
			sb.WriteString(fmt.Sprintf(`[%s]"%s"[%s]: `, Active.JSONKey, k, tcell.ColorReset))
			ColorizeValue(child, indent+2, sb, false)
			if keys < len(val) {
				sb.WriteRune(',')
			}
			sb.WriteRune('\n')
		}
		writeIndent(sb, indent)
		sb.WriteString(fmt.Sprintf("[%s]}[%s]", Active.JSONBracket, tcell.ColorReset))
	case []interface{}:
		sb.WriteString(fmt.Sprintf("[%s][[%s]\n", Active.JSONBracket, tcell.ColorReset))
		for i, elem := range val {
			writeIndent(sb, indent+2)
			ColorizeValue(elem, indent+2, sb, false)
			if i < len(val)-1 {
				sb.WriteRune(',')
			}
			sb.WriteRune('\n')
		}
		writeIndent(sb, indent)
		sb.WriteString(fmt.Sprintf("[%s]][%s]", Active.JSONBracket, tcell.ColorReset))
	case string:
		sb.WriteString(fmt.Sprintf(`[%s]"%s"[%s]`, Active.JSONString, val, tcell.ColorReset))
	case float64:
		sb.WriteString(fmt.Sprintf(`[%s]%v[%s]`, Active.JSONNumber, val, tcell.ColorReset))
	case bool:
		sb.WriteString(fmt.Sprintf(`[%s]%t[%s]`, Active.JSONBool, val, tcell.ColorReset))
	case nil:
		sb.WriteString(fmt.Sprintf(`[%s]null[%s]`, Active.JSONNull, tcell.ColorReset))
	default:
		sb.WriteString(fmt.Sprintf("%v", val))
	}
}

// ColorizeJSONLine applies syntax highlighting to a single line of pretty-printed JSON.
// It scans left-to-right so it works on individual lines and fragments, unlike ColorizeJSON
// which requires a complete JSON document.
func ColorizeJSONLine(line string) string {
	return colorizeJSONLine(line, -1)
}

// ColorizeJSONLineWithCursor applies syntax highlighting and embeds a cursor block at cursorX.
// cursorX is a byte offset into line. If cursorX >= len(line), the cursor appears as a
// trailing space at the end.
func ColorizeJSONLineWithCursor(line string, cursorX int) string {
	return colorizeJSONLine(line, cursorX)
}

type jsonLineToken struct {
	start, end int
	color      string
}

func colorizeJSONLine(line string, cursorX int) string {
	tokens := tokenizeJSONLine(line)

	var sb strings.Builder
	cursorDone := false

	for _, tok := range tokens {
		if cursorX >= tok.start && cursorX < tok.end && !cursorDone {
			// Cursor falls inside this token — split around it.
			sb.WriteString(fmt.Sprintf("[%s]", tok.color))
			if cursorX > tok.start {
				sb.WriteString(line[tok.start:cursorX])
			}
			sb.WriteString(fmt.Sprintf("[%s:%s]", Active.CursorFg, Active.CursorBg))
			sb.WriteByte(line[cursorX])
			sb.WriteString(fmt.Sprintf("[-:-][%s]", tok.color))
			if cursorX+1 < tok.end {
				sb.WriteString(line[cursorX+1 : tok.end])
			}
			cursorDone = true
		} else {
			sb.WriteString(fmt.Sprintf("[%s]%s", tok.color, line[tok.start:tok.end]))
		}
	}

	// Cursor past end of line — show as a highlighted space.
	if cursorX >= len(line) && !cursorDone {
		sb.WriteString(fmt.Sprintf("[%s:%s] ", Active.CursorFg, Active.CursorBg))
	}

	sb.WriteString("[-:-]")
	return sb.String()
}

func tokenizeJSONLine(line string) []jsonLineToken {
	var tokens []jsonLineToken
	i, n := 0, len(line)

	fg := Active.Foreground.String()
	blue := Active.JSONKey.String()
	green := Active.JSONString.String()
	yellow := Active.JSONBracket.String()
	orange := Active.JSONNumber.String()
	purple := Active.JSONBool.String()
	red := Active.JSONNull.String()

	add := func(start, end int, color string) {
		if start < end {
			tokens = append(tokens, jsonLineToken{start, end, color})
		}
	}

	for i < n {
		ch := line[i]
		switch {
		case ch == ' ' || ch == '\t':
			start := i
			for i < n && (line[i] == ' ' || line[i] == '\t') {
				i++
			}
			add(start, i, fg)

		case ch == '"':
			start := i
			i++ // skip opening quote
			for i < n {
				if line[i] == '\\' {
					i += 2 // skip escape sequence
					continue
				}
				if line[i] == '"' {
					i++ // include closing quote
					break
				}
				i++
			}
			// Look ahead past whitespace: if ':' follows, this is a key.
			j := i
			for j < n && (line[j] == ' ' || line[j] == '\t') {
				j++
			}
			if j < n && line[j] == ':' {
				add(start, i, blue)
			} else {
				add(start, i, green)
			}

		case ch == '{' || ch == '}' || ch == '[' || ch == ']':
			add(i, i+1, yellow)
			i++

		case ch == ':' || ch == ',':
			add(i, i+1, fg)
			i++

		case ch == '-' || (ch >= '0' && ch <= '9'):
			start := i
			if ch == '-' {
				i++
			}
			for i < n {
				c := line[i]
				if c >= '0' && c <= '9' || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-' {
					i++
				} else {
					break
				}
			}
			add(start, i, orange)

		case i+4 <= n && line[i:i+4] == "true":
			add(i, i+4, purple)
			i += 4

		case i+5 <= n && line[i:i+5] == "false":
			add(i, i+5, purple)
			i += 5

		case i+4 <= n && line[i:i+4] == "null":
			add(i, i+4, red)
			i += 4

		default:
			add(i, i+1, fg)
			i++
		}
	}

	return tokens
}

func writeIndent(sb *strings.Builder, spaces int) {
	for i := 0; i < spaces; i++ {
		sb.WriteRune(' ')
	}
}
