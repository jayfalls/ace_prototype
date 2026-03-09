# Business Specification Document

## Unit Name
Core Infrastructure

## Problem Statement
The ACE Framework lacks defined data models and APIs. Without these:
- Frontend cannot communicate with backend
- Data cannot be persisted or retrieved consistently
- Services cannot interoperate
- Development of frontend and backend cannot proceed in parallel

## Solution
Define the data model and API layer that enables:
- Structured data persistence in PostgreSQL
- Type-safe database access via SQLC
- REST API for CRUD operations
- WebSocket for real-time communication
- Clear data flow between frontend, API, and persistence

The core infrastructure must support:
- **Data Model**: PostgreSQL schema for agents, memories, configurations, sessions
- **API**: REST endpoints for all operations, WebSocket for real-time updates
- **Type Safety**: SQLC for compile-time SQL safety

## In Scope
- Define core entities and their relationships
- Define REST API endpoints (structure, not implementation)
- Define WebSocket message types
- Database schema design (high-level)
- API authentication approach
- Data validation requirements

## Out of Scope
- Detailed database migration scripts
- Full API implementation code
- Frontend implementation
- Specific LLM provider integrations
- Monitoring/observability details
- CI/CD pipelines

## Value Proposition
Well-defined core infrastructure enables:
- **Parallel Development**: Frontend and backend teams can work simultaneously
- **Type Safety**: SQLC ensures compile-time database type safety
- **Clear Contracts**: API definitions enable independent service development
- **Real-time Updates**: WebSocket support enables live cognitive state streaming
- **Data Integrity**: Well-defined schemas ensure consistent data storage
- **Security**: JWT authentication and data validation

## Success Criteria
| Criterion | Metric | Target |
|-----------|--------|--------|
| Entities Defined | Core entities identified | Complete entity list |
| API Structure | REST endpoints defined | Documented endpoint structure |
| Real-time Support | WebSocket message types defined | Complete message type list |
| Schema Design | Database schema outlined | High-level schema documented |
| Auth Strategy | Authentication approach defined | JWT-based auth confirmed |
| Data Validation | Validation rules defined | Documented per endpoint |

## Key Requirements
- **PostgreSQL**: Primary data store
- **SQLC**: Type-safe SQL access
- **REST**: Standard REST API patterns
- **WebSocket**: Real-time thought streaming
- **JWT**: Token-based authentication
- **Validation**: Input validation on all endpoints
