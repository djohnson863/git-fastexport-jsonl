package fastexport

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func loadFixture(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/sample.fastexport")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return data
}

func TestReaderParsesFixture(t *testing.T) {
	data := loadFixture(t)
	r := NewReader(bytes.NewReader(data))

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("read blob 1: %v", err)
	}
	if rec.Blob == nil || rec.Blob.Mark != 1 || string(rec.Blob.Data) != "line one\n" {
		t.Fatalf("blob 1 mismatch: %+v", rec.Blob)
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("read blob 2: %v", err)
	}
	if rec.Blob == nil || rec.Blob.Mark != 2 || string(rec.Blob.Data) != "line two\n" {
		t.Fatalf("blob 2 mismatch: %+v", rec.Blob)
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("read commit 1: %v", err)
	}
	c := rec.Commit
	if c == nil {
		t.Fatalf("expected commit, got %+v", rec)
	}
	if c.Ref != "refs/heads/main" || c.Mark != 3 {
		t.Fatalf("commit 1 ref/mark mismatch: %+v", c)
	}
	if c.Author != nil {
		t.Fatalf("commit 1 should have no separate author, got %+v", c.Author)
	}
	if c.Committer.Name != "Jane Doe" || c.Committer.Email != "jane@example.com" {
		t.Fatalf("commit 1 committer mismatch: %+v", c.Committer)
	}
	if c.Message != "initial commit\n" {
		t.Fatalf("commit 1 message mismatch: %q", c.Message)
	}
	wantChange1 := FileChange{Op: "M", Mode: "100644", DataRef: ":1", Path: "hello.txt"}
	if len(c.FileChanges) != 1 || c.FileChanges[0] != wantChange1 {
		t.Fatalf("commit 1 filechanges mismatch: %+v", c.FileChanges)
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("read commit 2: %v", err)
	}
	c = rec.Commit
	if c.Mark != 4 || c.From != ":3" {
		t.Fatalf("commit 2 mark/from mismatch: %+v", c)
	}
	if c.Author == nil || c.Author.When != "1690000100 -0700" {
		t.Fatalf("commit 2 author mismatch: %+v", c.Author)
	}
	wantChanges := []FileChange{
		{Op: "M", Mode: "100644", DataRef: ":2", Path: "hello.txt"},
		{Op: "R", SrcPath: "hello.txt", Path: "sub dir/hello.txt"},
	}
	if len(c.FileChanges) != len(wantChanges) {
		t.Fatalf("commit 2 filechanges count mismatch: %+v", c.FileChanges)
	}
	for i, want := range wantChanges {
		if c.FileChanges[i] != want {
			t.Fatalf("commit 2 filechange %d mismatch: got %+v, want %+v", i, c.FileChanges[i], want)
		}
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("read reset: %v", err)
	}
	if rec.Reset == nil || rec.Reset.Ref != "refs/heads/main" || rec.Reset.From != ":4" {
		t.Fatalf("reset mismatch: %+v", rec.Reset)
	}

	rec, err = r.Read()
	if err != nil {
		t.Fatalf("read tag: %v", err)
	}
	tag := rec.Tag
	if tag == nil || tag.Name != "v1.0" || tag.Mark != 5 || tag.From != ":4" {
		t.Fatalf("tag mismatch: %+v", tag)
	}
	if tag.Tagger == nil || tag.Tagger.Email != "jane@example.com" {
		t.Fatalf("tag tagger mismatch: %+v", tag.Tagger)
	}
	if tag.Message != "release 1.0\n" {
		t.Fatalf("tag message mismatch: %q", tag.Message)
	}

	if _, err := r.Read(); err != io.EOF {
		t.Fatalf("expected EOF after last record, got %v", err)
	}
}

// TestReaderWriterRoundTrip checks that reading the fixture and writing it
// straight back out reproduces the original bytes exactly, since the
// fixture is itself already in the canonical form Writer produces.
func TestReaderWriterRoundTrip(t *testing.T) {
	data := loadFixture(t)
	src := NewReader(bytes.NewReader(data))

	var buf bytes.Buffer
	dst := NewWriter(&buf)
	for {
		rec, err := src.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading record: %v", err)
		}
		if err := dst.WriteRecord(rec); err != nil {
			t.Fatalf("writing record: %v", err)
		}
	}

	if buf.String() != string(data) {
		t.Fatalf("round trip mismatch:\ngot:\n%s\nwant:\n%s", buf.String(), string(data))
	}
}

func TestReaderStopsAtDoneMarker(t *testing.T) {
	const stream = "blob\nmark :1\ndata 4\nabcd\ndone\nblob\nmark :2\ndata 1\nx\n"
	r := NewReader(strings.NewReader(stream))

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if rec.Blob == nil || rec.Blob.Mark != 1 || string(rec.Blob.Data) != "abcd" {
		t.Fatalf("blob mismatch: %+v", rec.Blob)
	}

	if _, err := r.Read(); err != io.EOF {
		t.Fatalf("expected EOF at done marker, got %v", err)
	}
}

func TestReaderSkipsCommentsProgressAndBlankLines(t *testing.T) {
	const stream = "# a comment\nprogress loading\n\nblob\nmark :1\ndata 1\nx\n"
	r := NewReader(strings.NewReader(stream))

	rec, err := r.Read()
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if rec.Blob == nil || rec.Blob.Mark != 1 || string(rec.Blob.Data) != "x" {
		t.Fatalf("blob mismatch: %+v", rec.Blob)
	}

	if _, err := r.Read(); err != io.EOF {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestReaderUnsupportedCommand(t *testing.T) {
	const stream = "cat-blob :1\n"
	r := NewReader(strings.NewReader(stream))

	if _, err := r.Read(); err == nil {
		t.Fatal("expected an error for an unsupported command, got nil")
	}
}
