package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devarispbrown/kc2con/internal/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryLookup(t *testing.T) {
	registry := New()

	// Test exact match
	info, found := registry.Lookup("io.debezium.connector.mysql.MySqlConnector")
	assert.True(t, found)
	assert.Equal(t, "MySQL CDC (Debezium)", info.Name)
	assert.Equal(t, StatusSupported, info.Status)

	// Test partial match
	info, found = registry.Lookup("mysql.MySqlConnector")
	assert.True(t, found)
	assert.Equal(t, "MySQL CDC (Debezium)", info.Name)

	// Test not found
	_, found = registry.Lookup("com.nonexistent.Connector")
	assert.False(t, found)
}

func TestRegistryGetByStatus(t *testing.T) {
	registry := New()

	supported := registry.GetByStatus(StatusSupported)
	assert.Greater(t, len(supported), 0)

	partial := registry.GetByStatus(StatusPartial)
	assert.GreaterOrEqual(t, len(partial), 0)

	manual := registry.GetByStatus(StatusManual)
	assert.GreaterOrEqual(t, len(manual), 0)

	unsupported := registry.GetByStatus(StatusUnsupported)
	assert.GreaterOrEqual(t, len(unsupported), 0)
}

func TestAnalyzeConnector(t *testing.T) {
	registry := New()

	config := &parser.ConnectorConfig{
		Name:  "test-mysql",
		Class: "io.debezium.connector.mysql.MySqlConnector",
		Config: map[string]interface{}{
			"database.hostname":    "localhost",
			"database.port":        3306,
			"database.user":        "test",
			"database.password":    "test",
			"database.server.name": "testserver",
		},
		Transforms: []parser.TransformConfig{
			{
				Name:  "unwrap",
				Class: "io.debezium.transforms.ExtractNewRecordState",
			},
		},
	}

	analysis := registry.AnalyzeConnector(config)

	assert.Equal(t, "test-mysql", analysis.ConnectorName)
	assert.Equal(t, "io.debezium.connector.mysql.MySqlConnector", analysis.ConnectorClass)
	assert.Equal(t, StatusSupported, analysis.ConnectorInfo.Status)
	assert.Equal(t, "mysql-cdc", analysis.ConnectorInfo.ConduitEquivalent)
}

func TestAnalyzeUnknownConnector(t *testing.T) {
	registry := New()

	config := &parser.ConnectorConfig{
		Name:  "unknown-connector",
		Class: "com.unknown.UnknownConnector",
	}

	analysis := registry.AnalyzeConnector(config)

	assert.Equal(t, StatusManual, analysis.ConnectorInfo.Status)
	assert.Equal(t, "manual-implementation-required", analysis.ConnectorInfo.ConduitEquivalent)
	assert.Greater(t, len(analysis.Issues), 0)
}

func TestImprovedRegistryWithCustomConfig(t *testing.T) {
	// Create temporary custom config
	customConfig := `
version: "1.0"
description: "Test Config"
updated: "2024-12-19"

categories:
  test:
    name: "Test Category"
    description: "Test connectors"

connectors:
  - kafka_connect_class: "com.test.TestConnector"
    name: "Test Connector"
    category: "test"
    conduit_equivalent: "test"
    conduit_type: "source"
    status: "supported"
    confidence: "high"
    estimated_effort: "15 minutes"

transforms: []
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(configPath, []byte(customConfig), 0o644)
	require.NoError(t, err)

	registry, err := NewImproved(configPath)
	require.NoError(t, err)

	// Test that custom connector is loaded
	info, found := registry.Lookup("com.test.TestConnector")
	assert.True(t, found)
	assert.Equal(t, "Test Connector", info.Name)

	// Test categories
	categories := registry.GetCategories()
	assert.Contains(t, categories, "test")
	assert.Equal(t, "Test Category", categories["test"].Name)
}

func TestRegistryReload(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Initial config
	initialConfig := `
version: "1.0"
description: "Initial"
categories: {}
connectors:
  - kafka_connect_class: "com.test.Initial"
    name: "Initial Connector"
    category: "test"
    conduit_equivalent: "initial"
    conduit_type: "source"
    status: "supported"
transforms: []
`

	err := os.WriteFile(configPath, []byte(initialConfig), 0o644)
	require.NoError(t, err)

	registry, err := NewImproved(configPath)
	require.NoError(t, err)

	// Verify initial state
	_, found := registry.Lookup("com.test.Initial")
	assert.True(t, found)

	// Update config
	updatedConfig := `
version: "1.0"
description: "Updated"
categories: {}
connectors:
  - kafka_connect_class: "com.test.Updated"
    name: "Updated Connector"
    category: "test"
    conduit_equivalent: "updated"
    conduit_type: "source"
    status: "supported"
transforms: []
`

	err = os.WriteFile(configPath, []byte(updatedConfig), 0o644)
	require.NoError(t, err)

	// Reload
	err = registry.Reload()
	require.NoError(t, err)

	// Verify updated state
	_, found = registry.Lookup("com.test.Initial")
	assert.False(t, found)

	_, found = registry.Lookup("com.test.Updated")
	assert.True(t, found)
}
