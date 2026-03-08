# Configuration

<!--
Intent: Define all configuration options for the feature.
Scope: Environment variables, feature flags, configuration files, and their defaults.
Used by: AI agents to correctly configure the feature and understand available options.
-->

## Overview
[Summary of configuration requirements]

## Environment Variables

### Required Variables
| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| [VAR_NAME] | string | - | [Description] |
| [VAR_NAME] | integer | - | [Description] |

### Optional Variables
| Variable | Type | Default | Description |
|----------|------|---------|-------------|
| [VAR_NAME] | string | "default" | [Description] |
| [VAR_NAME] | boolean | false | [Description] |

### Example Configuration
```bash
# Required
export [VAR_NAME]="value"

# Optional
export [VAR_NAME]="default_value"
```

## Feature Flags

### Flags
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| [FLAG_NAME] | boolean | false | [Description] |
| [FLAG_NAME] | string | "default" | [Description] |

### Usage Example
```python
# In code
if feature_flags["[FLAG_NAME]"]:
    # Feature code
```

## Configuration Files

### config.yaml
```yaml
[feature]:
  enabled: true
  setting: value
  nested:
    key: value
```

### config.json
```json
{
  "[feature]": {
    "enabled": true,
    "setting": "value"
  }
}
```

## Configuration Schema
| Path | Type | Required | Default | Validation |
|------|------|----------|---------|------------|
| [path] | [type] | Yes/No | [default] | [rules] |

## Environment-Specific Configuration

### Development
```bash
[DEV_VAR]=dev_value
```

### Staging
```bash
[STAGING_VAR]=staging_value
```

### Production
```bash
[PROD_VAR]=prod_value
```

## Secrets Management
| Secret | Location | How to Rotate |
|--------|----------|----------------|
| [Secret] | [Location] | [Procedure] |

## Configuration Validation
- [ ] Validate on startup
- [ ] Log warnings for missing optional vars
- [ ] Fail on missing required vars

## Migration Guide
[How to migrate configuration when upgrading]