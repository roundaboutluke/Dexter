package config

func stripJSONComments(input []byte) []byte {
	out := make([]byte, 0, len(input))
	inString := false
	inSingleLine := false
	inMultiLine := false
	escaped := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if inSingleLine {
			if c == '\n' {
				inSingleLine = false
				out = append(out, c)
			}
			continue
		}

		if inMultiLine {
			if c == '*' && i+1 < len(input) && input[i+1] == '/' {
				inMultiLine = false
				i++
			}
			continue
		}

		if inString {
			out = append(out, c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inString = false
			}
			continue
		}

		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}

		if c == '/' && i+1 < len(input) {
			next := input[i+1]
			if next == '/' {
				inSingleLine = true
				i++
				continue
			}
			if next == '*' {
				inMultiLine = true
				i++
				continue
			}
		}

		out = append(out, c)
	}

	return out
}

// StripJSONComments removes // and /* */ comments from JSON content.
func StripJSONComments(input []byte) []byte {
	return stripJSONComments(input)
}
