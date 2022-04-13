package main

import (
	"os/exec"
	"runtime"
	"fmt"
)


var Command string

func main() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", "start", Command)
	case "linux":
		cmd = exec.Command("x-terminal-emulator", "-e", Command)
	default:
		panic("unknown runtime")
	}

	err := cmd.Start()
	if err != nil {
		panic(fmt.Sprintf("%+v", err))
	}
}