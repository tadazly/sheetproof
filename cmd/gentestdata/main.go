package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tadazly/sheetproof/internal/testutil"
)

func main() {
	dir := flag.String("dir", "testdata", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fatal(err)
	}
	pair, err := testutil.CreatePair(*dir)
	if err != nil {
		fatal(err)
	}
	left := filepath.Join(*dir, "left.xlsx")
	right := filepath.Join(*dir, "right.xlsx")
	if err := os.Rename(pair.Left, left); err != nil {
		fatal(err)
	}
	if err := os.Rename(pair.Right, right); err != nil {
		fatal(err)
	}
	fmt.Printf("generated:\n  %s\n  %s\n", left, right)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
