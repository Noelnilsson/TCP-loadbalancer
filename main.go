package main

import (
	"fmt"
	"os"

	"tcp_lb/tui"
)

func main() {
	if err := tui.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Goodbye!")
}
