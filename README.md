# kc2con - Kafka Connect to Conduit Migration Tool

🚀 **Easily migrate your Kafka Connect configurations to Conduit pipelines.**

Analyze your existing setup, plan your migration, and execute with confidence.

## Features

✅ **Comprehensive Analysis**
- Parse JSON and Properties configuration files
- 15+ built-in connector compatibility mappings
- Cross-connector validation and conflict detection
- Performance analysis and resource estimation
- Security configuration validation
- Schema registry consistency checks

✅ **Migration Planning**
- Dependency-aware migration ordering
- Migration readiness scoring (0-100%)
- Effort estimation and timeline planning
- Bottleneck identification and mitigation
- Actionable recommendations

✅ **Beautiful CLI**
- Colorful, easy-to-read output
- Multiple output formats (table/JSON/YAML)
- Detailed issue categorization
- Progress tracking and validation

## Quick Start

```bash
# Build the tool
go build -o kc2con

# Analyze your Kafka Connect configurations
./kc2con analyze --config-dir ./kafka-connect-configs

# Check connector compatibility
./kc2con compatibility

# Get help
./kc2con --help
```

## Commands

- `analyze` - Analyze Kafka Connect configurations for migration
- `compatibility` - Show connector compatibility matrix
- `migrate` - Generate Conduit pipeline configurations (coming soon)
- `validate` - Validate generated Conduit configurations (coming soon)

## Current Status

- ✅ **Analysis Engine: 85% Complete** - Production ready
- ⚠️ **Migration Generation: 20% Complete** - In development
- ⚠️ **Validation: 10% Complete** - Planned

The analysis functionality is comprehensive and ready for production use!

## Installation

```bash
git clone https://github.com/conduit-io/kc2con
cd kc2con
go mod tidy
go build -o kc2con
```

## Requirements

- Go 1.21+
- Kafka Connect configuration files

## License

MIT License