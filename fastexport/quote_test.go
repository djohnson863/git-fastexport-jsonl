package fastexport

import "testing"

func TestQuotePath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"hello.txt", "hello.txt"},
		{"dir/hello.txt", "dir/hello.txt"},
		{"has space.txt", `"has space.txt"`},
		{`quote".txt`, `"quote\".txt"`},
		{`back\slash.txt`, `"back\\slash.txt"`},
		{"line\nbreak.txt", `"line\nbreak.txt"`},
		{"tab\ttab.txt", `"tab\ttab.txt"`},
		{"\x01ctrl.txt", `"\001ctrl.txt"`},
		{"\x7fdel.txt", `"\177del.txt"`},
	}
	for _, c := range cases {
		got := quotePath(c.path)
		if got != c.want {
			t.Errorf("quotePath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestUnquotePath(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"hello.txt", "hello.txt", false},
		{`"has space.txt"`, "has space.txt", false},
		{`"quote\".txt"`, `quote".txt`, false},
		{`"back\\slash.txt"`, `back\slash.txt`, false},
		{`"line\nbreak.txt"`, "line\nbreak.txt", false},
		{`"tab\ttab.txt"`, "tab\ttab.txt", false},
		{`"\001ctrl.txt"`, "\x01ctrl.txt", false},
		{`"unterminated`, "", true},
		{`"ab\"`, "", true}, // trailing backslash once the closing quote is stripped
		{`"bad\zescape"`, "", true},
	}
	for _, c := range cases {
		got, err := unquotePath(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("unquotePath(%q): expected error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("unquotePath(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("unquotePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestQuoteUnquoteRoundTrip(t *testing.T) {
	paths := []string{
		"hello.txt",
		"dir/sub dir/hello.txt",
		`weird"name\here.txt`,
		"tabs\tand\nnewlines.txt",
		"\x01\x02control.txt",
	}
	for _, p := range paths {
		got, err := unquotePath(quotePath(p))
		if err != nil {
			t.Errorf("round trip for %q: unexpected error: %v", p, err)
			continue
		}
		if got != p {
			t.Errorf("round trip for %q: got %q", p, got)
		}
	}
}

func TestNeedsQuote(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"hello.txt", false},
		{"dir/hello.txt", false},
		{"has space.txt", true},
		{`quote".txt`, true},
		{`back\slash.txt`, true},
		{"\x01ctrl.txt", true},
	}
	for _, c := range cases {
		if got := needsQuote(c.path); got != c.want {
			t.Errorf("needsQuote(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

func TestReadPathToken(t *testing.T) {
	cases := []struct {
		in       string
		wantTok  string
		wantRest string
		wantErr  bool
	}{
		{"src.txt dst.txt", "src.txt", "dst.txt", false},
		{`"src with space.txt" dst.txt`, "src with space.txt", "dst.txt", false},
		{"onlysrc.txt", "onlysrc.txt", "", false},
		{`"unterminated`, "", "", true},
	}
	for _, c := range cases {
		tok, rest, err := readPathToken(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("readPathToken(%q): expected error, got (%q, %q)", c.in, tok, rest)
			}
			continue
		}
		if err != nil {
			t.Errorf("readPathToken(%q): unexpected error: %v", c.in, err)
			continue
		}
		if tok != c.wantTok || rest != c.wantRest {
			t.Errorf("readPathToken(%q) = (%q, %q), want (%q, %q)", c.in, tok, rest, c.wantTok, c.wantRest)
		}
	}
}
