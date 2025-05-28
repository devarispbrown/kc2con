package main

import (
	"os"

	"github.com/devarispbrown/kc2con/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
