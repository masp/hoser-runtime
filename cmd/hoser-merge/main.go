package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

const (
	MaxRecordSize     = 1024 * 1024 * 48 // 48MB max record size
	MaxStreamNameSize = 1024             // 1KB
)

var (
	sepCh byte
	sep   = flag.String("sep", "\n", "the separator for continuous strings that will be copied atomically to stdout")
)

func main() {
	flag.Parse()
	log.SetOutput(os.Stderr)

	if len(*sep) > 1 {
		log.Fatalf("sep only 1 byte")
	}
	sepCh = (*sep)[0]

	var inputs []*os.File
	for _, stream := range flag.Args() {
		fd, err := os.Open(stream)
		if err != nil {
			log.Fatalf("File invalid: %v", err)
		}
		inputs = append(inputs, fd)
	}
	defer func() {
		for _, input := range inputs {
			input.Close()
		}
	}()

	records := make(chan []byte)
	var wg sync.WaitGroup
	for _, input := range inputs {
		wg.Add(1)
		go func(input *os.File) {
			defer wg.Done()
			copy(input.Name(), input, records)
		}(input)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		readStdin(&wg, records)
	}()

	go func() {
		wg.Wait()
		close(records)
	}()

	for record := range records {
		n, err := os.Stdout.Write(record)
		if err != nil {
			log.Printf("error: output write (%d/%d): %v", n, len(record), err)
			return
		}
	}
}

func readStdin(wg *sync.WaitGroup, records chan []byte) {
	buf := bufio.NewReaderSize(os.Stdin, MaxStreamNameSize)
	for {
		newSrc, err := buf.ReadString('\n')
		newSrc = strings.TrimSuffix(newSrc, "\n")
		if err != nil && err != io.EOF {
			return
		}

		switch newSrc {
		case "":
			break
		default:
			// Treat it as a filename (unix)
			fd, err := os.Open(newSrc)
			if err != nil {
				log.Printf("error: open file '%s' to merge: %v", newSrc, err)
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				copy(newSrc, fd, records)
			}()
		}

		if err == io.EOF {
			return
		}
	}
}

func copy(name string, from io.Reader, out chan []byte) {
	bufrd := bufio.NewReaderSize(from, MaxRecordSize)
	for {
		record, err := bufrd.ReadBytes(sepCh)
		if err != nil && err != io.EOF {
			log.Printf("copy: read '%s': %v", name, err)
			return
		}

		if len(record) > 0 {
			if record[len(record)-1] != sepCh {
				record = append(record, sepCh) // for EOF, if there is no trailing sep, we add here
			}
			out <- record
		}

		if err == io.EOF {
			break
		}
	}
}
