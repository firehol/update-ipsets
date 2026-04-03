package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	posture, err := Collect(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "archposture: %v\n", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(posture); err != nil {
		fmt.Fprintf(os.Stderr, "archposture: encode: %v\n", err)
		os.Exit(1)
	}
}
