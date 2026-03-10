# Dependencies

<!--
Intent: Define all external dependencies required by the feature.
Scope: Libraries, services, APIs, and tools that the feature depends on.
Used by: AI agents to understand what external resources are needed and how to integrate with them.
-->

## Overview
[Summary of dependencies for this feature]

## External Services

### Service 1: [Service Name]
| Property | Value |
|----------|-------|
| Type | [Cloud Service/API/Database] |
| Provider | [Provider name] |
| Purpose | [What it's used for] |
| Cost | [Estimated cost/month] |

#### Configuration
```bash
# Environment variables
export [SERVICE_NAME]_API_KEY="[key]"
export [SERVICE_NAME]_ENDPOINT="[url]"
```

#### Integration
```python
# Example client setup
import [library]

client = [Library](
    api_key=os.environ["[SERVICE_NAME]_API_KEY"],
    endpoint=os.environ["[SERVICE_NAME]_ENDPOINT"]
)
```

#### Rate Limits
| Endpoint | Limit | Window |
|----------|-------|--------|
| [Endpoint] | [Number] | [Per minute/hour] |

#### Alternatives Considered
- **Alternative 1**: [Name] - [Why not chosen]
- **Alternative 2**: [Name] - [Why not chosen]

## Libraries

### Runtime Dependencies
| Library | Version | Purpose | License |
|---------|---------|---------|---------|
| [library1] | ^1.0.0 | [Purpose] | [License] |
| [library2] | ^2.0.0 | [Purpose] | [License] |

### Development Dependencies
| Library | Version | Purpose |
|---------|---------|---------|
| [library1] | ^1.0.0 | [Purpose] |
| [library2] | ^2.0.0 | [Purpose] |

### Dependency Management
```bash
# Add dependency
pip install [library]

# Or with poetry
poetry add [library]
```

## API Integrations

### API 1: [API Name]
| Property | Value |
|----------|-------|
| Base URL | [URL] |
| Auth Type | [OAuth/API Key/JWT] |
| Version | [v1/v2] |
| Documentation | [URL] |

#### Endpoints Used
| Endpoint | Purpose | Frequency |
|----------|---------|-----------|
| [Endpoint] | [Purpose] | [Frequency] |

#### Error Handling
| Error | Handling Strategy |
|-------|------------------|
| [Error 1] | [How handled] |
| [Error 2] | [How handled] |

#### Retry Strategy
- **Max Retries**: [Number]
- **Backoff**: [Exponential/Linear]
- **Max Backoff**: [Time]

## Infrastructure Dependencies

### Required Infrastructure
| Resource | Type | Specification | Purpose |
|----------|------|---------------|---------|
| [Resource] | [Type] | [Spec] | [Purpose] |

### Optional Infrastructure
| Resource | Type | Specification | Purpose |
|----------|------|---------------|---------|
| [Resource] | [Type] | [Spec] | [Purpose] |

## Dependency Security

### Secret Management
| Secret | How Stored | Rotation |
|--------|-----------|----------|
| [Secret] | [Vault/Env] | [Frequency] |

### Vulnerability Scanning
- [ ] Scan dependencies on build
- [ ] Automated CVE alerts
- [ ] Update policy: [Timeframe]

## Dependency Update Strategy
- Review updates: [Monthly/Quarterly]
- Security patches: [Immediate/Within X days]
- Major versions: [Evaluate per case]