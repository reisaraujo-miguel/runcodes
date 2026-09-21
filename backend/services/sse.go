package services

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
)

// maxSSELineBytes bounds a single SSE line; test-case user output can be large.
const maxSSELineBytes = 4 << 20

/*
SSEFrame is a single Server-Sent Events frame.
*/
type SSEFrame struct {
	Event string
	ID    string
	Data  []byte
}

/*
ParseSSE reads a Server-Sent Events stream, invoking handle for every complete
frame. Comment lines (starting with ':') are ignored, which also covers the
judge's ": ping" heartbeats. Multi-line data fields are joined with newlines.
*/
func ParseSSE(r io.Reader, handle func(SSEFrame) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxSSELineBytes)

	var (
		event string
		id    string
		data  []byte
	)

	flush := func() error {
		if len(data) == 0 && event == "" && id == "" {
			return nil
		}
		frame := SSEFrame{Event: event, ID: id, Data: data}
		event, id, data = "", "", nil
		return handle(frame)
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
		case strings.HasPrefix(line, ":"):
			// Comment / heartbeat.
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "id:"):
			id = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		case strings.HasPrefix(line, "data:"):
			value := strings.TrimPrefix(line, "data:")
			value = strings.TrimPrefix(value, " ")
			if len(data) > 0 {
				data = append(data, '\n')
			}
			data = append(data, value...)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return flush()
}

/*
FormatSSEFrame renders a single SSE frame. The sequence is only included when
it is greater than zero so snapshot frames (which have no sequence) stay clean.
*/
func FormatSSEFrame(event string, id int64, data []byte) []byte {
	var buf bytes.Buffer

	if event != "" {
		fmt.Fprintf(&buf, "event: %s\n", event)
	}
	if id > 0 {
		fmt.Fprintf(&buf, "id: %d\n", id)
	}
	buf.WriteString("data: ")
	buf.Write(data)
	buf.WriteString("\n\n")

	return buf.Bytes()
}
