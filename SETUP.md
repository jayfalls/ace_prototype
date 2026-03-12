# ACE Framework - Local Setup Guide

## Prerequisites

- **Go 1.23+** - Backend runtime
- **Node.js 22+** - Frontend runtime  
- **Docker & Docker Compose** - For PostgreSQL and NATS services

---

## 1. Start Docker Services

```bash
cd /workspace/project/ace_prototype

# Start PostgreSQL and NATS
docker compose up -d

# Verify services are running
docker compose ps
# Should show: postgres (port 5432), nats (port 4222)
```

---

## 2. Set Up Backend

```bash
cd /workspace/project/ace_prototype/backend

# Install Go dependencies
go mod tidy

# Run database migrations
go run cmd/migrate/main.go

# Start the backend server
DATABASE_URL="postgres://ace:ace123@localhost:5432/ace_framework?sslmode=disable" go run cmd/server/main.go
# Backend runs on http://localhost:8080
```

---

## 3. Set Up Frontend (Optional - for UI)

```bash
cd /workspace/project/ace_prototype/frontend

# Install dependencies
npm install

# Start development server
npm run dev -- --host
# Frontend runs on http://localhost:3000
```

---

## 4. Testing the API

### Register and Create Agent

```bash
# Register a user
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@test.com","password":"password123","name":"Test User"}' | \
  jq -r '.data.access_token')

# Create a provider (use your OpenRouter API key)
PROVIDER_ID=$(curl -s -X POST "http://localhost:8080/api/v1/providers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"OpenRouter","provider_type":"openrouter","api_key":"YOUR_OPENROUTER_API_KEY","base_url":"https://openrouter.ai/api/v1","model":"openrouter/free"}' | \
  jq -r '.data.id')

# Create an agent
AGENT_ID=$(curl -s -X POST "http://localhost:8080/api/v1/agents" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"Test Agent\",\"provider_id\":\"$PROVIDER_ID\"}" | \
  jq -r '.data.id')

# Create a session
SESSION_ID=$(curl -s -X POST "http://localhost:8080/api/v1/sessions" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"agent_id\":\"$AGENT_ID\"}" | \
  jq -r '.data.id')

echo "Session ID: $SESSION_ID"
```

### Send a Chat Message

```bash
# Send a chat message
curl -s -X POST "http://localhost:8080/api/v1/chats" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"session_id\":\"$SESSION_ID\",\"message\":\"What is Python?\"}"

# Wait for processing
sleep 5

# Get thoughts from all 6 cognitive layers
curl -s "http://localhost:8080/api/v1/thoughts?session_id=$SESSION_ID" \
  -H "Authorization: Bearer $TOKEN" | jq
```

---

## 5. Verify LLM Integration

Check the thoughts - you should see real LLM responses from OpenRouter:

- **cognitive_control**: Detailed explanation of the topic
- **executive_function**: Task management perspective
- **global_strategy**: Strategic planning perspective
- **aspirational**: Ethical implications
- **agent_model**: Self-model updates
- **task_prosecution**: Execution perspective

### Notes
- OpenRouter free tier: 50 requests/day
- After hitting the limit, you'll see 429 errors in some layer responses
- First few layers will succeed with real LLM responses

---

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/api/v1/auth/register` | POST | Register user |
| `/api/v1/auth/login` | POST | Login user |
| `/api/v1/providers` | GET/POST | List/Create providers |
| `/api/v1/agents` | GET/POST | List/Create agents |
| `/api/v1/sessions` | GET/POST | List/Create sessions |
| `/api/v1/chats` | POST | Send chat message |
| `/api/v1/thoughts` | GET | Get layer thoughts |

---

## Troubleshooting

### PostgreSQL Connection Error
```bash
# Check PostgreSQL is running
docker compose ps

# Recreate database
docker compose down -v
docker compose up -d
sleep 5
go run cmd/migrate/main.go
```

### NATS Connection Error
```bash
# Check NATS is running
docker compose ps

# Restart NATS
docker compose restart nats
```

### Rate Limiting (429)
- This is normal for OpenRouter free tier
- Wait 24 hours or add credits to your OpenRouter account
