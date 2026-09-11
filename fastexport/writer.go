package fastexport

import (
	"fmt"
	"io"
)

// Writer serializes Records back into fast-import stream syntax, the
// format documented in git-fast-import(1) and consumed by `git
// fast-import`. It's the inverse of Reader: one WriteRecord call emits
// exactly the command that a matching Reader.Read call would have parsed.
type Writer struct {
	w io.Writer
}

// NewWriter wraps w as the destination for a fast-import stream.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// WriteRecord writes rec's command (blob, commit, or reset) to the stream.
func (w *Writer) WriteRecord(rec *Record) error {
	switch {
	case rec.Blob != nil:
		return w.writeBlob(rec.Blob)
	case rec.Commit != nil:
		return w.writeCommit(rec.Commit)
	case rec.Reset != nil:
		return w.writeReset(rec.Reset)
	case rec.Tag != nil:
		return w.writeTag(rec.Tag)
	default:
		return fmt.Errorf("fastexport: record has no command set")
	}
}

func (w *Writer) writeBlob(b *Blob) error {
	if _, err := fmt.Fprintln(w.w, "blob"); err != nil {
		return err
	}
	if b.Mark != 0 {
		if _, err := fmt.Fprintf(w.w, "mark :%d\n", b.Mark); err != nil {
			return err
		}
	}
	return w.writeData(b.Data)
}

func (w *Writer) writeCommit(c *Commit) error {
	if _, err := fmt.Fprintf(w.w, "commit %s\n", c.Ref); err != nil {
		return err
	}
	if c.Mark != 0 {
		if _, err := fmt.Fprintf(w.w, "mark :%d\n", c.Mark); err != nil {
			return err
		}
	}
	if c.Author != nil {
		if _, err := fmt.Fprintf(w.w, "author %s\n", formatIdentity(*c.Author)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w.w, "committer %s\n", formatIdentity(c.Committer)); err != nil {
		return err
	}
	if err := w.writeData([]byte(c.Message)); err != nil {
		return err
	}
	if c.From != "" {
		if _, err := fmt.Fprintf(w.w, "from %s\n", c.From); err != nil {
			return err
		}
	}
	for _, m := range c.Merges {
		if _, err := fmt.Fprintf(w.w, "merge %s\n", m); err != nil {
			return err
		}
	}
	for _, fc := range c.FileChanges {
		if err := w.writeFileChange(fc); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) writeFileChange(fc FileChange) error {
	var err error
	switch fc.Op {
	case "M":
		_, err = fmt.Fprintf(w.w, "M %s %s %s\n", fc.Mode, fc.DataRef, fc.Path)
	case "D":
		_, err = fmt.Fprintf(w.w, "D %s\n", fc.Path)
	case "C", "R":
		_, err = fmt.Fprintf(w.w, "%s %s %s\n", fc.Op, fc.SrcPath, fc.Path)
	default:
		return fmt.Errorf("fastexport: unknown file-change op: %q", fc.Op)
	}
	return err
}

func (w *Writer) writeReset(rs *Reset) error {
	if _, err := fmt.Fprintf(w.w, "reset %s\n", rs.Ref); err != nil {
		return err
	}
	if rs.From != "" {
		if _, err := fmt.Fprintf(w.w, "from %s\n", rs.From); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) writeTag(t *Tag) error {
	if _, err := fmt.Fprintf(w.w, "tag %s\n", t.Name); err != nil {
		return err
	}
	if t.Mark != 0 {
		if _, err := fmt.Fprintf(w.w, "mark :%d\n", t.Mark); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w.w, "from %s\n", t.From); err != nil {
		return err
	}
	if t.Tagger != nil {
		if _, err := fmt.Fprintf(w.w, "tagger %s\n", formatIdentity(*t.Tagger)); err != nil {
			return err
		}
	}
	return w.writeData([]byte(t.Message))
}

// writeData writes a "data <len>" line followed by data itself and the
// trailing LF that fast-import expects but doesn't count in <len>.
func (w *Writer) writeData(data []byte) error {
	if _, err := fmt.Fprintf(w.w, "data %d\n", len(data)); err != nil {
		return err
	}
	if _, err := w.w.Write(data); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w.w)
	return err
}

func formatIdentity(id Identity) string {
	return fmt.Sprintf("%s <%s> %s", id.Name, id.Email, id.When)
}
