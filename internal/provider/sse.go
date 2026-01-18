// Package provider implements LLM provider clients.
package provider

import (
	"bufio"
	"io"
	"strings"
)

// SSEEvent represents a Server-Sent Event
type SSEEvent struct {
	Event string
	Data  string
	ID    string
	Retry int
}

// SSEReader reads SSE events from a stream
type SSEReader struct {
	reader *bufio.Reader
}

// NewSSEReader creates a new SSE reader
func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{
		reader: bufio.NewReader(r),
	}
}

// ReadEvent reads the next SSE event
func (r *SSEReader) ReadEvent() (*SSEEvent, error) {
	event := &SSEEvent{}

	for {
		line, err := r.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimRight(line, "\r\n")

		if line == "" {
			// Empty line means end of event
			if event.Data != "" {
				return event, nil
			}
			continue
		}

		if strings.HasPrefix(line, ":") {
			// Comment, ignore
			continue
		}

		colonIdx := strings.Index(line, ":")
		var field, value string
		if colonIdx == -1 {
			field = line
			value = ""
		} else {
			field = line[:colonIdx]
			value = strings.TrimPrefix(line[colonIdx+1:], " ")
		}

		switch field {
		case "event":
			event.Event = value
		case "data":
			if event.Data != "" {
				event.Data += "\n"
			}
			event.Data += value
		case "id":
			event.ID = value
		}
	}
}
