package fastexport

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Reader parses a fast-export stream one record at a time. It holds at most
// one line of lookahead plus whatever the current blob or commit message
// data block requires, so memory use stays flat no matter how large the
// repository being exported is.
type Reader struct {
	r    *bufio.Reader
	held string
	have bool
}

// NewReader wraps r as a fast-export stream. r is read incrementally by
// Read; NewReader itself does no I/O.
func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReaderSize(r, 64*1024)}
}

// Read returns the next record, or io.EOF once the stream is exhausted (or
// a `done` command is seen, which git fast-export writes as an explicit
// end marker on newer versions).
func (r *Reader) Read() (*Record, error) {
	for {
		line, err := r.nextLine()
		if err != nil {
			return nil, err
		}
		switch {
		case line == "":
			continue
		case line == "done":
			return nil, io.EOF
		case strings.HasPrefix(line, "#"), strings.HasPrefix(line, "progress "):
			continue
		case line == "blob":
			b, err := r.readBlob()
			if err != nil {
				return nil, err
			}
			return &Record{Blob: b}, nil
		case strings.HasPrefix(line, "commit "):
			c, err := r.readCommit(strings.TrimPrefix(line, "commit "))
			if err != nil {
				return nil, err
			}
			return &Record{Commit: c}, nil
		case strings.HasPrefix(line, "reset "):
			rs, err := r.readReset(strings.TrimPrefix(line, "reset "))
			if err != nil {
				return nil, err
			}
			return &Record{Reset: rs}, nil
		case strings.HasPrefix(line, "tag "):
			t, err := r.readTag(strings.TrimPrefix(line, "tag "))
			if err != nil {
				return nil, err
			}
			return &Record{Tag: t}, nil
		default:
			return nil, fmt.Errorf("fastexport: unsupported command: %q", line)
		}
	}
}

func (r *Reader) nextLine() (string, error) {
	if r.have {
		r.have = false
		return r.held, nil
	}
	line, err := r.r.ReadString('\n')
	if err != nil {
		if err == io.EOF && line != "" {
			return line, nil
		}
		return "", err
	}
	return strings.TrimSuffix(line, "\n"), nil
}

// unreadLine puts a line that turned out to belong to the next record back
// so the following nextLine call returns it.
func (r *Reader) unreadLine(line string) {
	r.held = line
	r.have = true
}

func (r *Reader) readBlob() (*Blob, error) {
	b := &Blob{}
	for {
		line, err := r.nextLine()
		if err != nil {
			return nil, err
		}
		switch {
		case strings.HasPrefix(line, "mark :"):
			n, err := strconv.Atoi(strings.TrimPrefix(line, "mark :"))
			if err != nil {
				return nil, fmt.Errorf("fastexport: bad mark %q: %w", line, err)
			}
			b.Mark = n
		case strings.HasPrefix(line, "data "):
			data, err := r.readData(line)
			if err != nil {
				return nil, err
			}
			b.Data = data
			return b, nil
		default:
			return nil, fmt.Errorf("fastexport: unexpected line in blob: %q", line)
		}
	}
}

func (r *Reader) readCommit(ref string) (*Commit, error) {
	c := &Commit{Ref: ref}
	for {
		line, err := r.nextLine()
		if err != nil {
			if err == io.EOF {
				return c, nil
			}
			return nil, err
		}
		switch {
		case line == "":
			return c, nil
		case strings.HasPrefix(line, "mark :"):
			n, err := strconv.Atoi(strings.TrimPrefix(line, "mark :"))
			if err != nil {
				return nil, fmt.Errorf("fastexport: bad mark %q: %w", line, err)
			}
			c.Mark = n
		case strings.HasPrefix(line, "author "):
			id, err := parseIdentity(strings.TrimPrefix(line, "author "))
			if err != nil {
				return nil, err
			}
			c.Author = id
		case strings.HasPrefix(line, "committer "):
			id, err := parseIdentity(strings.TrimPrefix(line, "committer "))
			if err != nil {
				return nil, err
			}
			c.Committer = *id
		case strings.HasPrefix(line, "data "):
			msg, err := r.readData(line)
			if err != nil {
				return nil, err
			}
			c.Message = string(msg)
		case strings.HasPrefix(line, "from "):
			c.From = strings.TrimPrefix(line, "from ")
		case strings.HasPrefix(line, "merge "):
			c.Merges = append(c.Merges, strings.TrimPrefix(line, "merge "))
		case strings.HasPrefix(line, "M "), strings.HasPrefix(line, "D "),
			strings.HasPrefix(line, "C "), strings.HasPrefix(line, "R "):
			fc, err := parseFileChange(line)
			if err != nil {
				return nil, err
			}
			c.FileChanges = append(c.FileChanges, fc)
		default:
			// Belongs to whatever comes next; hand it back.
			r.unreadLine(line)
			return c, nil
		}
	}
}

func (r *Reader) readReset(ref string) (*Reset, error) {
	rs := &Reset{Ref: ref}
	line, err := r.nextLine()
	if err != nil {
		if err == io.EOF {
			return rs, nil
		}
		return nil, err
	}
	if strings.HasPrefix(line, "from ") {
		rs.From = strings.TrimPrefix(line, "from ")
		return rs, nil
	}
	r.unreadLine(line)
	return rs, nil
}

// readTag reads a `tag` command. Per git-fast-import(1) the lines appear in
// a fixed order: an optional mark, then from, then an optional tagger, then
// the data block holding the tag message.
func (r *Reader) readTag(name string) (*Tag, error) {
	t := &Tag{Name: name}
	for {
		line, err := r.nextLine()
		if err != nil {
			return nil, err
		}
		switch {
		case strings.HasPrefix(line, "mark :"):
			n, err := strconv.Atoi(strings.TrimPrefix(line, "mark :"))
			if err != nil {
				return nil, fmt.Errorf("fastexport: bad mark %q: %w", line, err)
			}
			t.Mark = n
		case strings.HasPrefix(line, "from "):
			t.From = strings.TrimPrefix(line, "from ")
		case strings.HasPrefix(line, "tagger "):
			id, err := parseIdentity(strings.TrimPrefix(line, "tagger "))
			if err != nil {
				return nil, err
			}
			t.Tagger = id
		case strings.HasPrefix(line, "data "):
			msg, err := r.readData(line)
			if err != nil {
				return nil, err
			}
			t.Message = string(msg)
			return t, nil
		default:
			return nil, fmt.Errorf("fastexport: unexpected line in tag: %q", line)
		}
	}
}

// readData reads the payload of a "data <len>" line: exactly len bytes,
// read with io.ReadFull rather than buffered line-by-line, since the
// payload is arbitrary binary content that may contain newlines.
func (r *Reader) readData(line string) ([]byte, error) {
	n, err := strconv.Atoi(strings.TrimPrefix(line, "data "))
	if err != nil {
		return nil, fmt.Errorf("fastexport: bad data length %q: %w", line, err)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r.r, buf); err != nil {
		return nil, fmt.Errorf("fastexport: reading %d byte data block: %w", n, err)
	}
	// The byte count doesn't include the LF the writer adds after the block.
	if b, err := r.r.ReadByte(); err == nil && b != '\n' {
		r.r.UnreadByte()
	}
	return buf, nil
}

func parseIdentity(s string) (*Identity, error) {
	lt := strings.IndexByte(s, '<')
	gt := strings.IndexByte(s, '>')
	if lt < 0 || gt < 0 || gt < lt {
		return nil, fmt.Errorf("fastexport: malformed identity: %q", s)
	}
	return &Identity{
		Name:  strings.TrimSpace(s[:lt]),
		Email: s[lt+1 : gt],
		When:  strings.TrimSpace(s[gt+1:]),
	}, nil
}

func parseFileChange(line string) (FileChange, error) {
	op := line[:1]
	rest := line[2:]
	switch op {
	case "M":
		fields := strings.SplitN(rest, " ", 3)
		if len(fields) != 3 {
			return FileChange{}, fmt.Errorf("fastexport: malformed M line: %q", line)
		}
		return FileChange{Op: "M", Mode: fields[0], DataRef: fields[1], Path: fields[2]}, nil
	case "D":
		return FileChange{Op: "D", Path: rest}, nil
	case "C", "R":
		fields := strings.SplitN(rest, " ", 2)
		if len(fields) != 2 {
			return FileChange{}, fmt.Errorf("fastexport: malformed %s line: %q", op, line)
		}
		return FileChange{Op: op, SrcPath: fields[0], Path: fields[1]}, nil
	default:
		return FileChange{}, fmt.Errorf("fastexport: unknown file-change op: %q", line)
	}
}
