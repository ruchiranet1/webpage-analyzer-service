package logger

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	log := New()
	log.Info("test message", "key", "value")

	w.Close()
	os.Stdout = old

	var out bytes.Buffer
	_, _ = io.Copy(&out, r)

	assert.Contains(t, out.String(), "test message")
	assert.Contains(t, out.String(), `"key":"value"`)
}
