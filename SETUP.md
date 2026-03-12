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
docker compose up -d postgres nats

# Verify services are running
docker compose ps
# Should show: postgres (port 5432), nats (port 4222)
```

---

## 2. Set Up Backend

```bash
# Run database migrations
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
export PATH=$PATH:$(go env GOPATH)/bin
migrate -path backend/db/migrations -database "postgres://ace:ace@localhost:5432/ace_framework?sslmode=disable" up

cd backend

# Install Go dependencies
go mod tidy

# Start the backend server
DATABASE_URL="postgres://ace:ace@localhost:5432/ace_framework?sslmode=disable" go run cmd/server/main.go
# Backend runs on http://localhost:8080
```

---

## 3. Set Up Frontend (Optional - for UI)

```bash
cd frontend

# Install dependencies
npm install

# Start development server
npm run dev -- --host
# Frontend runs on http://localhost:3000
```

---

## 5. Testing the API

### Using the Visualization Page (Recommended)

1. **Start the frontend**:
   ```bash
   cd /workspace/project/ace_prototype/frontend
   npm run dev -- --host
   # Frontend runs on http://localhost:3000
   ```

2. **Create your agent** (from terminal):
   ```bash
   # Register and create agent (run all at once)
   TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/register \
     -H "Content-Type: application/json" \
     -d '{"email":"test@test.com","password":"password123","name":"Test User"}' | \
     jq -r '.data.access_token')
   
   PROVIDER_ID=$(curl -s -X POST "http://localhost:8080/api/v1/providers" \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"name":"OpenRouter","provider_type":"openrouter","api_key":"YOUR_OPENROUTER_KEY","base_url":"https://openrouter.ai/api/v1","model":"openrouter/free"}' | \
     jq -r '.data.id')
   
   AGENT_ID=$(curl -s -X POST "http://localhost:8080/api/v1/agents" \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d "{\"name\":\"Test Agent\",\"provider_id\":\"$PROVIDER_ID\"}" | \
     jq -r '.data.id')
   
   SESSION_ID=$(curl -s -X POST "http://localhost:8080/api/v1/sessions" \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d "{\"agent_id\":\"$AGENT_ID\"}" | \
     jq -r '.data.id')
   
   echo "SESSION_ID: $SESSION_ID"
   echo "AGENT_ID: $AGENT_ID"
   ```

3. **Open visualization page**:
   ```
   http://localhost:3000/visualizations?session=<SESSION_ID>&agent=<AGENT_ID>
   ```

4. **Send a chat message** (from another terminal):
   ```bash
   curl -s -X POST "http://localhost:8080/api/v1/chats" \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d "{\"session_id\":\"$SESSION_ID\",\"message\":\"What is Python?\"}"
   ```

5. **Watch** the 6 cognitive layers process your message in real-time!

---

## 6. Verify LLM Integration

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
