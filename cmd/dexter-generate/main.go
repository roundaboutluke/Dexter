package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"dexter/internal/data"
)

func main() {
	var latest bool
	flag.BoolVar(&latest, "latest", false, "fetch latest grunts data")
	flag.Parse()
	for _, arg := range flag.Args() {
		if arg == "latest" {
			latest = true
		}
	}

	root, err := os.Getwd()
	if err != nil {
		fatalf("resolve workdir: %v", err)
	}
	root = filepath.Clean(root)
	if err := data.Generate(root, latest, logf); err != nil {
		fatalf("generate data failed: %v", err)
	}
}

func logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
