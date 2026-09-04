// Package fastexport parses the stream format produced by `git fast-export`.
//
// The format is documented in git-fast-import(1). It's a sequence of
// commands (blob, commit, reset, tag, ...) written as plain text lines,
// except for the payload of a `data` command, which is an exact byte count
// followed by that many raw bytes. That's the part that matters for
// streaming: you cannot use a line scanner for the whole stream, because a
// blob's data can contain newlines, and a single blob can be gigabytes.
package fastexport

// Record is one command read from a fast-export stream. Exactly one field
// is set, matching whichever command was parsed.
type Record struct {
	Blob   *Blob   `json:"blob,omitempty"`
	Commit *Commit `json:"commit,omitempty"`
	Reset  *Reset  `json:"reset,omitempty"`
}

// Blob is a `blob` command: file content addressed by mark instead of path.
// Commits reference it later through a FileChange.DataRef of ":<mark>".
type Blob struct {
	Mark int    `json:"mark,omitempty"`
	Data []byte `json:"data"` // encoding/json base64-encodes this, which keeps it binary safe
}

// Identity is the "<name> <email> <epoch> <tz-offset>" form used by both
// `author` and `committer` lines. When is kept as the raw string rather than
// parsed into a time.Time so round-tripping doesn't lose the original
// timezone offset formatting.
type Identity struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	When  string `json:"when"`
}

// FileChange is one line describing how a commit touches the tree: M
// (modify), D (delete), C (copy) or R (rename).
type FileChange struct {
	Op      string `json:"op"`
	Mode    string `json:"mode,omitempty"`    // set for M
	DataRef string `json:"dataRef,omitempty"` // set for M: a ":<mark>" or a sha1
	Path    string `json:"path"`
	SrcPath string `json:"srcPath,omitempty"` // set for C and R
}

// Commit is a `commit` command and everything nested under it up to the
// next top-level command.
type Commit struct {
	Ref         string       `json:"ref"`
	Mark        int          `json:"mark,omitempty"`
	Author      *Identity    `json:"author,omitempty"` // absent when author == committer
	Committer   Identity     `json:"committer"`
	Message     string       `json:"message"`
	From        string       `json:"from,omitempty"`
	Merges      []string     `json:"merges,omitempty"`
	FileChanges []FileChange `json:"fileChanges,omitempty"`
}

// Reset points a ref at a commit (or deletes it, when From is empty). Git
// fast-export emits one of these before the first commit on each branch.
type Reset struct {
	Ref  string `json:"ref"`
	From string `json:"from,omitempty"`
}
