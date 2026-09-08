# gitstream

`git fast-export` is the closest thing git has to a portable dump of a
repository's history, but it's a bespoke text/binary hybrid format that's
awkward to feed into anything outside of `git fast-import`. This converts
that stream into JSON Lines instead, so history can be piped into normal
tools: `jq`, a log pipeline, a script that audits commit messages, whatever.

The point of doing this in Go with a hand-written streaming parser, instead
of just slurping the export and running it through a regex, is that a real
repository's fast-export stream can be gigabytes of blob data. The parser
reads the stream one command at a time - a blob, a commit, a reset - and
each command is written out and discarded before the next one is read. At
no point does it hold more than one record's worth of data in memory, so
converting a large repository's full history doesn't require a large
amount of RAM.

## Usage

```sh
git fast-export --all | go run . > history.jsonl
```

Or build it once and use files directly:

```sh
go build -o gitstream .
git fast-export --all -C /path/to/repo > repo.fastexport
./gitstream -in repo.fastexport -out repo.jsonl
```

`-reverse` runs the conversion the other way, turning JSON Lines back into
a fast-import stream that `git fast-import` can replay:

```sh
./gitstream -reverse -in repo.jsonl -out repo.fastexport
git fast-import --force < repo.fastexport
```

Each line of the output is one JSON object with a `blob`, `commit`, or
`reset` key, mirroring the corresponding fast-export command. For example, a
commit line looks like:

```json
{"commit":{"ref":"refs/heads/main","mark":3,"committer":{"name":"Jane Doe","email":"jane@example.com","when":"1690000000 -0700"},"message":"fix off-by-one in the retry loop\n","from":":2","fileChanges":[{"op":"M","mode":"100644","dataRef":":1","path":"retry.go"}]}}
```

Blob content is base64-encoded in the `data` field (via Go's standard
`encoding/json` byte-slice handling), which keeps binary files representable
in a text format without a separate encoding step.

## What's implemented

The parser currently understands `blob`, `commit`, and `reset` commands,
which covers the bulk of what `git fast-export` emits for a typical
history. `tag` and `cat-blob` aren't handled yet - see the roadmap in the
issue tracker for what's planned next.

Conversion back to fast-import format (`-reverse`) covers the same three
commands. Unlike the forward direction, it doesn't hold memory flat: JSON
has no way to stream a value's bytes incrementally, so each line is decoded
whole before being written out, meaning one blob's worth of base64 sits in
memory at a time rather than being copied through in fixed-size chunks.

## Why not just use `git log --format=...`?

`git log` gives you commit metadata, but not the tree changes and blob
content in a form you can reconstruct a repository from. fast-export is the
only format that's actually complete, which is also why it's worth having a
converter for it rather than working around it.
