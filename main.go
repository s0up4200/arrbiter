package main

import (
	"github.com/s0up4200/arrbiter/cmd"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cmd.SetVersion(version, buildTime)
	cmd.Execute()
}
