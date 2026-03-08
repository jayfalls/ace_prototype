# API Specification

<!--
Intent: Define all API endpoints exposed by the feature.
Scope: REST/GraphQL endpoints, request/response schemas, authentication, and error codes.
Used by: AI agents to implement and consume the API correctly.
-->

## Overview
[Summary of API for this feature]

## Authentication
| Method | Header | Description |
|--------|--------|-------------|
| Bearer Token | Authorization: Bearer \<token\> | JWT token authentication |

## Base URL
```
Production: https://api.example.com/v1
Staging: https://api-staging.example.com/v1
```

## Endpoints

### Resource: [Resource Name]

#### GET /api/v1/[resources]
Get a list of [resources].

**Query Parameters**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| page | integer | No | 1 | Page number |
| limit | integer | No | 20 | Items per page (max 100) |
| sort | string | No | created_at | Sort field |
| order | string | No | desc | Sort order (asc/desc) |
| [filter] | string | No | - | Filter by field |

**Response 200**
```json
{
  "data": [
    {
      "id": "uuid",
      "name": "string",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total": 100,
    "total_pages": 5
  }
}
```

#### POST /api/v1/[resources]
Create a new [resource].

**Request Body**
```json
{
  "name": "string (required)",
  "description": "string (optional)"
}
```

**Response 201**
```json
{
  "data": {
    "id": "uuid",
    "name": "string",
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

#### GET /api/v1/[resources]/{id}
Get a single [resource] by ID.

**Response 200**
```json
{
  "data": {
    "id": "uuid",
    "name": "string"
  }
}
```

**Response 404**
```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "[Resource] not found"
  }
}
```

#### PUT /api/v1/[resources]/{id}
Update a [resource].

**Request Body**
```json
{
  "name": "string",
  "description": "string"
}
```

#### DELETE /api/v1/[resources]/{id}
Delete a [resource].

**Response 204**: No content

## Error Responses

| Status Code | Code | Description |
|-------------|------|-------------|
| 400 | VALIDATION_ERROR | Invalid request body |
| 401 | UNAUTHORIZED | Missing or invalid token |
| 403 | FORBIDDEN | Insufficient permissions |
| 404 | NOT_FOUND | Resource not found |
| 422 | VALIDATION_ERROR | Business logic validation failed |
| 500 | INTERNAL_ERROR | Server error |

## Rate Limiting
- **Limit**: 100 requests per minute
- **Headers**: X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset

## Versioning
- **Current Version**: v1
- **Version in URL**: Yes (/v1/)
- **Deprecation Policy**: 6 months notice