package main

import (
	"fmt"
	"log"
	"os"

	"github.com/inhies/go-bytesize"
	"github.com/spf13/pflag"
	"golang.org/x/sys/unix"

	"gpu-memory-limiter/gml"
)

func usage() {
	_, _ = fmt.Fprintf(os.Stderr, "Usage: %s [--limit LIMIT] [--signal SIGNUM] -- COMMAND\n", os.Args[0])
	pflag.PrintDefaults()
}

func main() {
	var limitFlag = pflag.StringP("limit", "l", "1MB", "total GPU memory limit for the command")
	var signalFlag = pflag.StringP("signal", "s", "SIGKILL", "signal that will be sent to the process if the memory exceeds the limit")
	var helpFlag = pflag.Bool("help", false, "show help message")
	pflag.Parse()

	if *helpFlag {
		usage()
		os.Exit(0)
	}

	limit, err := bytesize.Parse(*limitFlag)
	if err != nil {
		log.Fatalf("failed to parse the memory limit \"%s\": %v", *limitFlag, err)
	}

	signum := unix.SignalNum(*signalFlag)
	if signum == 0 {
		log.Fatalf("signal \"%s\" not found", *signalFlag)
	}

	subCommandIdx := -1
	for i, arg := range os.Args {
		if arg == "--" {
			subCommandIdx = i + 1
			break
		}
	}
	if subCommandIdx == -1 {
		usage()
		os.Exit(-1)
	}

	subCommand := os.Args[subCommandIdx:]

	exitCode, err := gml.Run(subCommand, uint64(limit), signum)

	if err != nil {
		log.Println(err.Error())
	}
	os.Exit(exitCode)
}
