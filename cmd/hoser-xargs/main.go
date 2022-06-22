package main

import (
	"bufio"
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// hoser-xargs takes in a stream of arguments and creates a process for each line
// passing that line as an argument. The stdout and stderr of each process is written
// as filenames to stdout. If any errors occur, they are written to stderr.

const MaxLineSize = 5024

var (
	replacementToken = flag.String("I", "{}", "replacement token (token will be replaced with line in stdin)")
)

func main() {
	log.SetOutput(os.Stderr)

	cmdArgs := flag.Args()

	buf := bufio.NewReaderSize(os.Stdin, MaxLineSize)
	var wg sync.WaitGroup
	defer wg.Wait()
	for {
		newInput, err := buf.ReadString('\n')
		if err != nil {
			log.Printf("stdin closed: %v", err)
			return
		}
		newInput = strings.TrimSuffix(newInput, "\n")
		wg.Add(1)
		line := 1
		go func() {
			defer wg.Done()
			cmd := execLine(cmdArgs, newInput)
			_, stdoutW, err := os.Pipe()
			if err != nil {
				log.Printf("[line %d]: could not create pipe for stdout", line)
			}
			_, stderrW, err := os.Pipe()
			if err != nil {
				log.Printf("[line %d]: could not create pipe for stderr", line)
			}
			cmd.Stdout = stdoutW
			cmd.Stderr = stderrW
			err = cmd.Run()
			if err != nil {
				log.Printf("[line %d] pid %d exited: %v", line, cmd.Process.Pid, err)
			}
		}()
	}
}

func execLine(cmdArgs []string, replacement string) *exec.Cmd {
	for i := range cmdArgs {
		cmdArgs[i] = strings.ReplaceAll(cmdArgs[i], *replacementToken, replacement)
	}
	return exec.Command(cmdArgs[0], cmdArgs[1:]...)
}
