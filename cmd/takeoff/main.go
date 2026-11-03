// Command takeoff prints the Materials Take Off for an estimate text file.
//
//	go run ./cmd/takeoff materials/test/estimate_1031.txt
package main

import (
	"fmt"
	"os"

	"colout2/takeoff"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: takeoff <estimate_file>")
		os.Exit(2)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	in, err := takeoff.ParseEstimateText(f)
	if err != nil {
		return err
	}
	res, err := takeoff.Calculate(in)
	if err != nil {
		return err
	}
	return takeoff.Render(os.Stdout, in.EstimateID, res)
}
