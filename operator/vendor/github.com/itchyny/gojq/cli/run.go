package cli

import (
	"io"
	"os"
)

// Run gojq.
func Run() int {
	return (&cli{
		inStream:  os.Stdin,
		outStream: os.Stdout,
		errStream: os.Stderr,
	}).run(os.Args[1:])
}

func CmdRun(in io.Reader, out io.Writer, err io.Writer, args []string) int {
	return (&cli{
		inStream:  in,
		outStream: out,
		errStream: err,
	}).run(args)
}
