//go:build script

package internal

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Run prints and executes the given command.
func Run(args ...string) error {
	exe, err := exec.LookPath(args[0])
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	announce(args...)

	cmd := exec.Command(exe, args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func announce(args ...string) {
	fmt.Println(quote(args))
}

func quote(args []string) string {
	fmtArgs := make([]string, len(args))

	for i, arg := range args {
		if strings.ContainsAny(arg, " \t'\"") {
			fmtArgs[i] = fmt.Sprintf("%q", arg)
		} else {
			fmtArgs[i] = arg
		}
	}

	return strings.Join(fmtArgs, " ")
}
