package main

import (
	"bgos/internal/cmd"
	"os"
	"path/filepath"
)

func main() {
	baseName := filepath.Base(os.Args[0])

	err := cmd.NewRootCommand(baseName).Execute()
	cmd.CheckError(err)
}
