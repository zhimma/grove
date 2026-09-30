package main

import (
	"fmt"
	"io"
)

func writeLine(out io.Writer, line string) error {
	_, err := fmt.Fprintln(out, line)
	return err
}
