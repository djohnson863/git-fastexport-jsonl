package fastexport

import (
	"fmt"
	"strings"
)

// quotePath renders s as a path field for an M/D/C/R line: unquoted if it
// contains none of the characters that would make the line ambiguous or
// unsafe to parse, otherwise as a C-style quoted string the way git itself
// quotes paths in fast-export output.
func quotePath(s string) string {
	if !needsQuote(s) {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(&b, `\%03o`, c)
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// needsQuote reports whether s must be quoted to appear as a single path
// field. A bare space would make C/R lines ambiguous between the two paths
// they carry, so it counts as needing quoting even though it wouldn't
// confuse an M or D line on its own.
func needsQuote(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '"' || c == '\\' || c == ' ' || c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

// unquotePath decodes s, which is either a plain path or a C-style quoted
// one as written by quotePath (or by git fast-export itself), and must
// consume all of s.
func unquotePath(s string) (string, error) {
	if !strings.HasPrefix(s, `"`) {
		return s, nil
	}
	if len(s) < 2 || s[len(s)-1] != '"' {
		return "", fmt.Errorf("unterminated quoted path %q", s)
	}
	body := s[1 : len(s)-1]
	var b strings.Builder
	for i := 0; i < len(body); i++ {
		c := body[i]
		if c != '\\' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(body) {
			return "", fmt.Errorf("trailing backslash in quoted path %q", s)
		}
		switch e := body[i]; e {
		case '"', '\\':
			b.WriteByte(e)
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case 'r':
			b.WriteByte('\r')
		default:
			if e < '0' || e > '7' {
				return "", fmt.Errorf("unknown escape \\%c in quoted path %q", e, s)
			}
			n := int(e - '0')
			for k := 0; k < 2 && i+1 < len(body) && body[i+1] >= '0' && body[i+1] <= '7'; k++ {
				i++
				n = n*8 + int(body[i]-'0')
			}
			b.WriteByte(byte(n))
		}
	}
	return b.String(), nil
}

// readPathToken parses one path field, quoted or not, from the start of s
// and returns whatever follows the single space that terminates it. Used
// for the first of the two paths on a C or R line; the second runs to the
// end of the line and is decoded directly with unquotePath instead.
func readPathToken(s string) (token, rest string, err error) {
	if strings.HasPrefix(s, `"`) {
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				tok, err := unquotePath(s[:i+1])
				if err != nil {
					return "", "", err
				}
				return tok, strings.TrimPrefix(s[i+1:], " "), nil
			}
		}
		return "", "", fmt.Errorf("unterminated quoted path %q", s)
	}
	if idx := strings.IndexByte(s, ' '); idx >= 0 {
		return s[:idx], s[idx+1:], nil
	}
	return s, "", nil
}
