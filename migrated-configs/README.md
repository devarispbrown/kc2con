# Conduit Pipeline Configurations

This directory contains Conduit pipeline configurations migrated from Kafka Connect.

## Structure

- `pipelines/` - Pipeline YAML files
- `deploy-pipelines.sh` - Deployment script
- `MIGRATION_GUIDE.md` - Detailed migration guide

## Quick Start

1. Review and update pipeline configurations
2. Deploy pipelines:
   ```bash
   ./deploy-pipelines.sh
   ```

## Configuration

Set the Conduit URL if not using default:
```bash
export CONDUIT_URL=http://your-conduit-host:8080
./deploy-pipelines.sh
```
