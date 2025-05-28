package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/devarispbrown/kc2con/internal/migration/mappers"
	"github.com/devarispbrown/kc2con/internal/parser"
)

// Note: Using type aliases for migration package types
// ConduitPipeline, Pipeline, Connector, Processor, and Metrics
// are aliased in types.go from mappers package

// Orchestrator manages complex migration scenarios
type Orchestrator struct {
	engine       *Engine
	parser       *parser.Parser
	workerConfig *parser.WorkerConfig
	configCache  sync.Map // Cache for parsed configurations
}

// Plan represents a complete migration plan
type Plan struct {
	Connectors      []*ConnectorMigration
	WorkerConfig    *parser.WorkerConfig
	GlobalSettings  map[string]interface{}
	EstimatedEffort string
	Dependencies    map[string][]string // connector dependencies
}

// ConnectorMigration represents a single connector migration
type ConnectorMigration struct {
	SourcePath   string
	Config       *parser.ConnectorConfig
	Priority     int      // Migration order priority
	Dependencies []string // Other connectors this depends on
}

// NewOrchestrator creates a new migration orchestrator
func NewOrchestrator(registryPath string) (*Orchestrator, error) {
	engine, err := NewEngine(registryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create engine: %w", err)
	}

	return &Orchestrator{
		engine: engine,
		parser: parser.New(),
	}, nil
}

// CreateMigrationPlan analyzes a directory and creates a comprehensive migration plan
func (o *Orchestrator) CreateMigrationPlan(configDir string) (*Plan, error) {
	log.Debug("Creating migration plan", "dir", configDir)

	plan := &Plan{
		Connectors:     []*ConnectorMigration{},
		GlobalSettings: make(map[string]interface{}),
		Dependencies:   make(map[string][]string),
	}

	// Find and parse all configurations
	configs, err := o.findAllConfigs(configDir)
	if err != nil {
		return nil, err
	}

	// Find worker configuration
	workerConfig, err := o.findWorkerConfig(configDir)
	if err != nil {
		log.Warn("No worker configuration found", "error", err)
	} else {
		plan.WorkerConfig = workerConfig
		o.workerConfig = workerConfig
	}

	// Analyze each connector
	for path, config := range configs {
		migration := &ConnectorMigration{
			SourcePath: path,
			Config:     config,
			Priority:   o.calculatePriority(config),
		}

		// Detect dependencies
		deps := o.detectDependencies(config, configs)
		if len(deps) > 0 {
			migration.Dependencies = deps
			plan.Dependencies[config.Name] = deps
		}

		plan.Connectors = append(plan.Connectors, migration)
	}

	// Sort by priority and dependencies
	o.sortMigrations(plan.Connectors)

	// Extract global settings
	o.extractGlobalSettings(plan)

	// Estimate effort
	plan.EstimatedEffort = o.estimateEffort(plan)

	return plan, nil
}

// MigrateBatch performs a batch migration of multiple connectors with concurrency support
func (o *Orchestrator) MigrateBatch(plan *Plan, outputDir string, dryRun bool, maxConcurrency int) (*BatchMigrationResult, error) {
	result := &BatchMigrationResult{
		Successful: []*Result{},
		Failed:     []*FailedMigration{},
		Warnings:   []string{},
		Metrics: &mappers.MigrationMetrics{
			StartTime:      time.Now().Unix(),
			ConnectorTypes: make(map[string]int),
		},
	}

	// Ensure maxConcurrency is reasonable
	if maxConcurrency <= 0 {
		maxConcurrency = 1
	} else if maxConcurrency > 10 {
		maxConcurrency = 10 // Limit to prevent resource exhaustion
	}

	// Create a combined pipeline if connectors are related
	combinedPipeline := o.shouldCombinePipelines(plan)

	if combinedPipeline {
		// Migrate as a single pipeline with multiple connectors
		combined, err := o.migrateCombined(plan)
		if err != nil {
			result.Failed = append(result.Failed, &FailedMigration{
				ConnectorName: "combined-pipeline",
				Error:         err,
				Timestamp:     time.Now(),
			})
		} else {
			result.Successful = append(result.Successful, combined)
		}
	} else {
		// Migrate connectors concurrently
		var wg sync.WaitGroup
		var mu sync.Mutex

		// Create semaphore for concurrency control
		sem := make(chan struct{}, maxConcurrency)

		for _, migration := range plan.Connectors {
			wg.Add(1)
			go func(m *ConnectorMigration) {
				defer wg.Done()

				// Acquire semaphore
				sem <- struct{}{}
				defer func() { <-sem }()

				migResult, err := o.engine.MigrateConnector(m.SourcePath)

				mu.Lock()
				defer mu.Unlock()

				if err != nil {
					result.Failed = append(result.Failed, &FailedMigration{
						ConnectorName: m.Config.Name,
						SourcePath:    m.SourcePath,
						Error:         err,
						Timestamp:     time.Now(),
					})
					result.Metrics.Failed++
					return
				}

				// Apply global settings
				o.applyGlobalSettings(migResult.Pipeline, plan.GlobalSettings)
				result.Successful = append(result.Successful, migResult)
				result.Metrics.Successful++

				// Track connector type
				if m.Config.Class != "" {
					result.Metrics.ConnectorTypes[m.Config.Class]++
				}
			}(migration)
		}

		wg.Wait()
	}

	// Calculate final metrics
	result.Metrics.EndTime = time.Now().Unix()
	result.Metrics.TotalConfigs = len(plan.Connectors)
	result.Metrics.WithWarnings = 0
	for _, r := range result.Successful {
		if len(r.Warnings) > 0 {
			result.Metrics.WithWarnings++
		}
	}
	result.Metrics.Duration = result.Metrics.EndTime - result.Metrics.StartTime

	// Generate deployment scripts if not dry run
	if !dryRun && len(result.Successful) > 0 {
		scripts := o.generateDeploymentScripts(result.Successful, outputDir)
		result.DeploymentScripts = scripts
	}

	return result, nil
}

// Helper methods

func (o *Orchestrator) findAllConfigs(dir string) (map[string]*parser.ConnectorConfig, error) {
	configs := make(map[string]*parser.ConnectorConfig)

	// Use a worker pool for parallel file parsing
	type parseResult struct {
		path   string
		config *parser.ConnectorConfig
		err    error
	}

	filePaths := []string{}

	// First, collect all potential config files
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".json" || ext == ".properties" {
			filePaths = append(filePaths, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Process files concurrently
	resultChan := make(chan parseResult, len(filePaths))
	var wg sync.WaitGroup

	// Limit concurrent file operations
	sem := make(chan struct{}, 5)

	for _, path := range filePaths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// Check cache first
			if cached, ok := o.configCache.Load(p); ok {
				if config, ok := cached.(*parser.ConnectorConfig); ok {
					resultChan <- parseResult{path: p, config: config}
					return
				}
			}

			// Parse the file
			config, err := o.parser.ParseConnectorConfig(p)
			if err == nil && config.Class != "" {
				// Cache the result
				o.configCache.Store(p, config)
				resultChan <- parseResult{path: p, config: config}
			} else {
				resultChan <- parseResult{path: p, err: err}
			}
		}(path)
	}

	// Close result channel when all workers are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		if result.err == nil && result.config != nil {
			configs[result.path] = result.config
		}
	}

	return configs, nil
}

func (o *Orchestrator) findWorkerConfig(dir string) (*parser.WorkerConfig, error) {
	var workerConfig *parser.WorkerConfig

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		name := strings.ToLower(info.Name())
		if strings.Contains(name, "worker") || strings.Contains(name, "connect-") {
			if strings.HasSuffix(name, ".properties") {
				config, err := o.parser.ParseWorkerConfig(path)
				if err == nil {
					workerConfig = config
					return filepath.SkipDir // Stop after finding first worker config
				}
			}
		}

		return nil
	})

	if err != nil && err != filepath.SkipDir {
		return nil, err
	}

	return workerConfig, nil
}

func (o *Orchestrator) calculatePriority(config *parser.ConnectorConfig) int {
	// Source connectors have higher priority
	if strings.Contains(strings.ToLower(config.Class), "source") {
		return 1
	}

	// CDC connectors are critical
	if strings.Contains(strings.ToLower(config.Class), "debezium") {
		return 0
	}

	// Sink connectors
	if strings.Contains(strings.ToLower(config.Class), "sink") {
		return 2
	}

	return 3
}

func (o *Orchestrator) detectDependencies(config *parser.ConnectorConfig, allConfigs map[string]*parser.ConnectorConfig) []string {
	var deps []string

	// Check if this sink depends on any sources
	if strings.Contains(strings.ToLower(config.Class), "sink") {
		// Look for sources with matching topics
		for _, topic := range config.Topics {
			for _, other := range allConfigs {
				if other.Name == config.Name {
					continue
				}

				// Check if other connector produces to this topic
				if strings.Contains(strings.ToLower(other.Class), "source") {
					for _, otherTopic := range other.Topics {
						if otherTopic == topic {
							deps = append(deps, other.Name)
						}
					}
				}
			}
		}
	}

	return deps
}

func (o *Orchestrator) sortMigrations(migrations []*ConnectorMigration) {
	sort.Slice(migrations, func(i, j int) bool {
		// First by priority
		if migrations[i].Priority != migrations[j].Priority {
			return migrations[i].Priority < migrations[j].Priority
		}

		// Then by dependencies (connectors with no deps first)
		if len(migrations[i].Dependencies) != len(migrations[j].Dependencies) {
			return len(migrations[i].Dependencies) < len(migrations[j].Dependencies)
		}

		// Finally by name
		return migrations[i].Config.Name < migrations[j].Config.Name
	})
}

func (o *Orchestrator) extractGlobalSettings(plan *Plan) {
	if plan.WorkerConfig != nil {
		// Extract schema registry settings
		if plan.WorkerConfig.SchemaRegistry != "" {
			plan.GlobalSettings["schema.registry.url"] = plan.WorkerConfig.SchemaRegistry
		}

		// Extract security settings
		if plan.WorkerConfig.SecurityProtocol != "" {
			plan.GlobalSettings["security.protocol"] = plan.WorkerConfig.SecurityProtocol
		}

		// Extract converter settings
		if plan.WorkerConfig.KeyConverter != "" {
			plan.GlobalSettings["key.converter"] = plan.WorkerConfig.KeyConverter
		}
		if plan.WorkerConfig.ValueConverter != "" {
			plan.GlobalSettings["value.converter"] = plan.WorkerConfig.ValueConverter
		}
	}
}

func (o *Orchestrator) estimateEffort(plan *Plan) string {
	totalMinutes := 0

	for _, migration := range plan.Connectors {
		// Look up connector in registry
		info, found := o.engine.registry.Lookup(migration.Config.Class)
		if !found {
			totalMinutes += 240 // Unknown connector, assume 4 hours
			continue
		}

		// Parse effort estimate
		effort := info.EstimatedEffort
		if strings.Contains(effort, "minutes") {
			var minutes int
			_, err := fmt.Sscanf(effort, "%d minutes", &minutes)
			if err == nil {
				totalMinutes += minutes
			}
		} else if strings.Contains(effort, "hour") {
			var hours int
			_, err := fmt.Sscanf(effort, "%d hour", &hours)
			if err == nil {
				totalMinutes += hours * 60
			}
		} else {
			totalMinutes += 60 // Default 1 hour
		}
	}

	// Add time for testing and validation
	totalMinutes = int(float64(totalMinutes) * 1.5)

	if totalMinutes < 60 {
		return fmt.Sprintf("%d minutes", totalMinutes)
	} else if totalMinutes < 480 {
		return fmt.Sprintf("%.1f hours", float64(totalMinutes)/60)
	} else {
		return fmt.Sprintf("%.1f days", float64(totalMinutes)/480)
	}
}

func (o *Orchestrator) shouldCombinePipelines(plan *Plan) bool {
	// Check if connectors are part of a data flow
	if len(plan.Dependencies) > 0 {
		return true
	}

	// Check if all connectors share the same topics
	var commonTopics []string
	for i, migration := range plan.Connectors {
		if i == 0 {
			commonTopics = migration.Config.Topics
		} else {
			// Check intersection
			hasCommon := false
			for _, topic := range migration.Config.Topics {
				for _, common := range commonTopics {
					if topic == common {
						hasCommon = true
						break
					}
				}
			}
			if hasCommon {
				return true
			}
		}
	}

	return false
}

func (o *Orchestrator) migrateCombined(plan *Plan) (*Result, error) {
	// Create a combined pipeline with multiple connectors
	pipeline := &mappers.ConduitPipeline{
		Version:    "2.2",
		Pipelines:  []mappers.Pipeline{},
		Connectors: []mappers.Connector{},
		Processors: []mappers.Processor{},
	}

	pipelineID := "combined-migration-pipeline"
	mainPipeline := mappers.Pipeline{
		ID:          pipelineID,
		Status:      "running",
		Name:        "Combined Migration Pipeline",
		Description: "Migrated from multiple Kafka Connect connectors",
		Connectors:  []string{},
	}

	// Migrate each connector
	for i, migration := range plan.Connectors {
		_, _ = o.engine.registry.Lookup(migration.Config.Class)

		// Map the connector using the engine's built-in mapping
		subPipeline, err := o.engine.MigrateConnector(migration.SourcePath)
		if err != nil {
			return nil, fmt.Errorf("failed to map %s: %w", migration.Config.Name, err)
		}

		// Extract connectors and add to combined pipeline
		for _, conn := range subPipeline.Pipeline.Connectors {
			conn.ID = fmt.Sprintf("%s-%d", conn.ID, i)
			pipeline.Connectors = append(pipeline.Connectors, conn)
			mainPipeline.Connectors = append(mainPipeline.Connectors, conn.ID)
		}

		// Extract processors
		for _, proc := range subPipeline.Pipeline.Processors {
			proc.ID = fmt.Sprintf("%s-%d", proc.ID, i)
			pipeline.Processors = append(pipeline.Processors, proc)
			mainPipeline.Processors = append(mainPipeline.Processors, proc.ID)
		}
	}

	pipeline.Pipelines = []mappers.Pipeline{mainPipeline}

	return &Result{
		Pipeline:   pipeline,
		SourceFile: "combined",
		Warnings:   []string{"Combined multiple connectors into a single pipeline"},
	}, nil
}

func (o *Orchestrator) applyGlobalSettings(pipeline *mappers.ConduitPipeline, globalSettings map[string]interface{}) {
	// Apply global settings to each connector
	for i := range pipeline.Connectors {
		// Schema registry settings
		if url, ok := globalSettings["schema.registry.url"].(string); ok {
			pipeline.Connectors[i].Settings["schema.registry.url"] = url
		}

		// Security settings
		if protocol, ok := globalSettings["security.protocol"].(string); ok {
			pipeline.Connectors[i].Settings["security.protocol"] = protocol
		}
	}
}

func (o *Orchestrator) generateDeploymentScripts(results []*Result, outputDir string) []DeploymentScript {
	var scripts []DeploymentScript

	// Generate import script
	importScript := DeploymentScript{
		Name: "import-pipelines.sh",
		Type: "bash",
		Content: `#!/bin/bash
# Conduit Pipeline Import Script
# Generated by kc2con

set -e

CONDUIT_URL="${CONDUIT_URL:-http://localhost:8080}"

echo "Importing Conduit pipelines..."

`,
	}

	for _, result := range results {
		if len(result.Pipeline.Pipelines) == 0 {
			continue
		}

		pipelineName := result.Pipeline.Pipelines[0].Name
		// Sanitize pipeline name for shell safety
		safeName := shellEscape(pipelineName)
		filename := GeneratePipelineID(pipelineName) + "-pipeline.yaml"
		safeFilename := shellEscape(filename)

		importScript.Content += fmt.Sprintf(`echo "Importing %s..."
conduit pipelines import --url "$CONDUIT_URL" %s || {
    echo "Failed to import %s"
    exit 1
}
`, safeName, safeFilename, safeName)
	}

	importScript.Content += `
echo "All pipelines imported successfully!"
`

	scripts = append(scripts, importScript)

	// Generate validation script
	validationScript := DeploymentScript{
		Name: "validate-pipelines.sh",
		Type: "bash",
		Content: `#!/bin/bash
# Conduit Pipeline Validation Script
# Generated by kc2con

set -e

CONDUIT_URL="${CONDUIT_URL:-http://localhost:8080}"

echo "Validating Conduit pipelines..."

failed=0

`,
	}

	for _, result := range results {
		if len(result.Pipeline.Pipelines) == 0 {
			continue
		}

		pipelineID := result.Pipeline.Pipelines[0].ID
		// Sanitize pipeline ID for shell safety
		safeID := shellEscape(pipelineID)

		validationScript.Content += fmt.Sprintf(`echo "Checking pipeline %s..."
STATUS=$(conduit pipelines get --url "$CONDUIT_URL" %s --format json | jq -r .status || echo "error")
if [ "$STATUS" != "running" ]; then
    echo "WARNING: Pipeline %s is not running (status: $STATUS)"
    failed=$((failed + 1))
fi
`, safeID, safeID, safeID)
	}

	validationScript.Content += `
if [ $failed -gt 0 ]; then
    echo "WARNING: $failed pipeline(s) are not in running state"
    exit 1
else
    echo "All pipelines validated successfully!"
fi
`

	scripts = append(scripts, validationScript)

	// Generate rollback script
	rollbackScript := DeploymentScript{
		Name: "rollback-pipelines.sh",
		Type: "bash",
		Content: `#!/bin/bash
# Conduit Pipeline Rollback Script
# Generated by kc2con

set -e

CONDUIT_URL="${CONDUIT_URL:-http://localhost:8080}"

echo "Rolling back Conduit pipelines..."

`,
	}

	for _, result := range results {
		if len(result.Pipeline.Pipelines) == 0 {
			continue
		}

		pipelineID := result.Pipeline.Pipelines[0].ID
		safeID := shellEscape(pipelineID)

		rollbackScript.Content += fmt.Sprintf(`echo "Removing pipeline %s..."
conduit pipelines delete --url "$CONDUIT_URL" %s || echo "Pipeline may not exist"
`, safeID, safeID)
	}

	rollbackScript.Content += `
echo "Rollback completed!"
`

	scripts = append(scripts, rollbackScript)

	// Save scripts to output directory
	for _, script := range scripts {
		scriptPath := filepath.Join(outputDir, script.Name)
		if err := os.WriteFile(scriptPath, []byte(script.Content), 0o600); err != nil {
			log.Warn("Failed to save deployment script", "script", script.Name, "error", err)
		}
	}

	return scripts
}

// shellEscape escapes a string for safe use in shell commands
func shellEscape(s string) string {
	// Replace single quotes with '\''
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	// Wrap in single quotes
	return "'" + escaped + "'"
}
