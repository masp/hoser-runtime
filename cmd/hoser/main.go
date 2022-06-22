package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/masp/hoser-runtime/osruntime"
	"github.com/masp/hoser-runtime/plan"
)

var (
	debug = flag.Bool("d", false, "Print debug information to stderr")
)

func main() {
	flag.Parse()
	if !*debug {
		log.SetOutput(io.Discard)
	} else {
		log.SetOutput(os.Stderr)
	}
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [pipes]\n", os.Args[0])
	}

	path, pipeName, err := parseFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad path: %v\n", err)
		return
	}

	planFile, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		return
	}
	pipes, err := plan.Unmarshal(planFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid hoser pipe file '%s': %v\n", path, err)
		return
	}

	var chosenPipe *plan.Pipe
	if pipeName == "" {
		chosenPipe = &pipes[0]
	} else {
		for _, pipe := range pipes {
			if pipe.Name == pipeName {
				chosenPipe = &pipe
			}
		}
		if chosenPipe == nil {
			fmt.Fprintf(os.Stderr, "no pipe with name '%s' found in '%s'\n", pipeName, path)
			return
		}
	}
	prog, err := osruntime.Build(*chosenPipe, map[string]any{
		"stdin":  os.Stdin,
		"stdout": os.Stdout,
		"stderr": os.Stderr,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v", err)
		return
	}
	err = prog.Start()
	if err != nil {
		log.Fatal(err)
	}
	prog.Wait()
}

func parseFile() (string, string, error) {
	if flag.Arg(0) == "" {
		return "", "", fmt.Errorf("no Hoser file specified\n")
	}

	parts := strings.Split(flag.Arg(0), ":")
	if len(parts) == 1 {
		return flag.Arg(0), "", nil
	} else if len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return "", "", fmt.Errorf("path has too many parts, expected only file.json:pipe")
}
