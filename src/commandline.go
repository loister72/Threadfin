package src

import (
	"fmt"
	"strings"
)

func splitCommandLine(input string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		switch {
		case r == '\\':
			if i+1 >= len(runes) {
				current.WriteRune(r)
				continue
			}

			next := runes[i+1]
			if next == '\\' || next == '\'' || next == '"' || next == ' ' || next == '\t' || next == '\n' || next == '\r' {
				current.WriteRune(next)
				i++
				continue
			}

			current.WriteRune(r)
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if quote != 0 {
		return nil, fmt.Errorf("unterminated quoted argument")
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}

	return args, nil
}
