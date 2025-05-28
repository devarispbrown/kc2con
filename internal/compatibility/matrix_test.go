package compatibility

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMatrix(t *testing.T) {
	matrix, err := GetMatrix("")
	require.NoError(t, err)
	assert.NotNil(t, matrix)
	assert.NotNil(t, matrix.registry)
}

func TestShowAllConnectors(t *testing.T) {
	matrix, err := GetMatrix("")
	require.NoError(t, err)

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = matrix.ShowAll()
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	// Read output
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "Compatibility Matrix")
	assert.Contains(t, output, "Summary:")
}

func TestShowSpecificConnector(t *testing.T) {
	matrix, err := GetMatrix("")
	require.NoError(t, err)

	// Capture output for valid connector
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = matrix.ShowConnector("mysql")
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	assert.Contains(t, output, "MySQL")
	assert.Contains(t, output, "Kafka Connect Class:")
}

func TestShowNonExistentConnector(t *testing.T) {
	matrix, err := GetMatrix("")
	require.NoError(t, err)

	err = matrix.ShowConnector("nonexistent-connector")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found in registry")
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "hello", truncateString("hello", 10))
	assert.Equal(t, "hello w...", truncateString("hello world", 10))
	assert.Equal(t, "test", truncateString("test", 4))
}
