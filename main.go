// Command gitstream converts a git fast-export stream into JSON Lines, one
// record (blob, commit, or reset) per line, and back again.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"gitstream/fastexport"
)

func main() {
	in := flag.String("in", "-", "input file to read (- for stdin)")
	out := flag.String("out", "-", "output file to write (- for stdout)")
	reverse := flag.Bool("reverse", false, "read JSON Lines and write a fast-import stream, instead of the other way around")
	flag.Parse()

	if err := run(*in, *out, *reverse); err != nil {
		fmt.Fprintln(os.Stderr, "gitstream:", err)
		os.Exit(1)
	}
}

func run(inPath, outPath string, reverse bool) error {
	r, closeIn, err := openInput(inPath)
	if err != nil {
		return err
	}
	defer closeIn()

	w, closeOut, err := createOutput(outPath)
	if err != nil {
		return err
	}
	defer closeOut()

	bw := bufio.NewWriter(w)
	defer bw.Flush()

	if reverse {
		return jsonlToFastImport(r, bw)
	}
	return fastExportToJSONL(r, bw)
}

func fastExportToJSONL(r io.Reader, w io.Writer) error {
	src := fastexport.NewReader(r)
	enc := json.NewEncoder(w)

	for {
		rec, err := src.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// Encode and move on immediately: nothing about a prior record
		// or an upcoming one is kept around, so a multi-gigabyte
		// history streams through in constant memory.
		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
}

func jsonlToFastImport(r io.Reader, w io.Writer) error {
	dec := json.NewDecoder(r)
	dst := fastexport.NewWriter(w)

	for {
		var rec fastexport.Record
		if err := dec.Decode(&rec); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := dst.WriteRecord(&rec); err != nil {
			return err
		}
	}
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "-" {
		return os.Stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

func createOutput(path string) (io.Writer, func(), error) {
	if path == "-" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}
