package runner

import (
	"fmt"
)

// OutputSink receives command output.
type OutputSink interface {
	// Write writes a line of output.
	Write(line string) error
	// Close closes the sink.
	Close() error
}

// StdioSink is an OutputSink that writes to stdout/stderr.
type StdioSink struct{}

// NewStdioSink creates a new sink that writes to stdout/stderr.
func NewStdioSink() *StdioSink {
	return &StdioSink{}
}

// Write writes a line to stdout.
func (s *StdioSink) Write(line string) error {
	fmt.Println(line)
	return nil
}

// Close closes the sink (no-op for StdioSink).
func (s *StdioSink) Close() error {
	return nil
}
