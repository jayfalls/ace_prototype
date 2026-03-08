# Security Considerations

<!--
Intent: Define security requirements, threat modeling, and controls for the feature.
Scope: Authentication, authorization, data protection, input validation, and compliance.
Used by: AI agents to ensure the feature is built securely.
-->

## Security Overview
[High-level security approach for this feature]

## Authentication
| Method | Description | Implementation |
|--------|-------------|----------------|
| [Auth Method] | [Description] | [How it's implemented] |

## Authorization
| Resource | Permission | Access Control |
|----------|------------|----------------|
| [Resource 1] | [read/write] | [How controlled] |

## Data Protection

### Sensitive Data
| Data | Classification | Protection |
|------|---------------|------------|
| [Data 1] | [PII/Secret] | [How protected] |

### Encryption
- **At Rest**: [Encryption method]
- **In Transit**: [TLS version]

## Input Validation
| Input | Validation Rules | Error Response |
|-------|-----------------|----------------|
| [Input 1] | [Rules] | [Error message] |

## Threat Modeling

### STRIDE Analysis
| Threat | Description | Mitigation |
|--------|-------------|------------|
| [Threat 1] | [Description] | [Mitigation] |
| [Threat 2] | [Description] | [Mitigation] |

### Attack Surfaces
- [Attack surface 1]
- [Attack surface 2]

## Security Controls
| Control | Type | Implementation |
|---------|------|----------------|
| [Control 1] | [Preventative/Detective] | [How implemented] |

## Security Testing
- [ ] Unit tests for input validation
- [ ] Penetration testing checklist
- [ ] Vulnerability scanning

## Compliance
- [GDPR/CCPA/SOC2]: [How compliant]

## Incident Response
[How to respond to security incidents related to this feature]