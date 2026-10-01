package herdr

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"
)

type SessionRecord struct {
	Type    string          `json:"type"`
	Message *SessionMessage `json:"message,omitempty"`
}

type SessionMessage struct {
	Role    string            `json:"role"`
	Content []json.RawMessage `json:"content"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// GetSessionFileOffset returns the current byte size of the session file.
func GetSessionFileOffset(filePath string) int64 {
	if filePath == "" {
		return 0
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return info.Size()
}

// ReadLatestAssistantMessage reads the latest assistant text response from an OMP session file.
// If fromOffset > 0, it reads only the portion of the file starting at fromOffset.
func ReadLatestAssistantMessage(filePath string, fromOffset int64) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if fromOffset > 0 {
		_, err = file.Seek(fromOffset, io.SeekStart)
		if err != nil {
			return "", err
		}
	}

	scanner := bufio.NewScanner(file)
	// Support long lines (up to 10MB per line for large responses/tool calls)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var lastAssistantText string

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec SessionRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		if rec.Type == "message" && rec.Message != nil && rec.Message.Role == "assistant" {
			var texts []string
			for _, rawItem := range rec.Message.Content {
				var tc TextContent
				if err := json.Unmarshal(rawItem, &tc); err == nil && tc.Type == "text" && tc.Text != "" {
					texts = append(texts, tc.Text)
				}
			}
			if len(texts) > 0 {
				lastAssistantText = strings.Join(texts, "\n\n")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return lastAssistantText, err
	}
	return lastAssistantText, nil
}
