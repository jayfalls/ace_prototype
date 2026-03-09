# User Stories

<!--
Intent: Define user-facing behavior in executable format that can drive tests.
Scope: All user interactions and flows expressed in Gherkin syntax.
Used by: AI agents to generate acceptance tests and ensure feature meets user expectations.
-->

## Gherkin Syntax Guide
- **Feature**: A logical grouping of scenarios
- **Scenario**: A specific example of the feature working
- **Given**: Preconditions (what must be true before)
- **When**: The action being performed
- **Then**: Expected outcome (assertions)
- **And/But**: Chain multiple conditions or steps
- **Background**: Steps that run before each scenario
- **Scenario Outline**: Parametrized scenarios with `<variable>` syntax

## Feature: User Registration and Authentication

### Background
```gherkin
Background: System is ready for user registration
  Given the system has no existing user with email "test@example.com"
```

### Scenario: User registers successfully
```gherkin
Scenario: New user creates an account
  Given the system has no existing user with email "newuser@example.com"
  When I register with email "newuser@example.com" and name "New User"
  Then the user "newuser@example.com" should exist in the system
  And I should receive a JWT token
  And my password should be securely hashed
```

### Scenario: User login with valid credentials
```gherkin
Scenario: Registered user logs in
  Given a user exists with email "user@example.com" and password "securePassword123"
  When I login with email "user@example.com" and password "securePassword123"
  Then I should receive a valid JWT token
  And the token should contain my user ID
```

### Scenario: User login with invalid credentials
```gherkin
Scenario: Login fails with wrong password
  Given a user exists with email "user@example.com" and password "correctPassword"
  When I login with email "user@example.com" and password "wrongPassword"
  Then I should receive an authentication error
  And I should not receive a token
```

### Scenario: Duplicate email registration fails
```gherkin
Scenario: Cannot register with existing email
  Given a user exists with email "existing@example.com"
  When I register with email "existing@example.com" and name "Duplicate User"
  Then I should receive an error indicating the email already exists
```

## Feature: Agent Management

### Background
```gherkin
Background: User is authenticated
  Given I am authenticated as a valid user
  And I have no agents in the system
```

### Scenario: Create a new agent
```gherkin
Scenario: User creates a new agent
  Given I am authenticated as a valid user
  When I create an agent with name "My Assistant" and description "A helpful assistant"
  Then the agent should be created with status "idle"
  And the agent should belong to my user account
  And the agent should have default configuration
```

### Scenario: List all agents
```gherkin
Scenario: User lists their agents
  Given I have created 3 agents named "Agent1", "Agent2", "Agent3"
  When I list all agents
  Then I should receive a list of 3 agents
  And each agent should include its name, description, and status
```

### Scenario: Get agent details
```gherkin
Scenario: User retrieves specific agent
  Given I have created an agent named "TestAgent" with description "Testing agent"
  When I get the agent details
  Then I should see the agent name "TestAgent"
  And I should see the description "Testing agent"
  And I should see the current status
```

### Scenario: Update agent
```gherkin
Scenario: User updates agent information
  Given I have created an agent named "OldName" with description "Old description"
  When I update the agent to name "NewName" and description "New description"
  Then the agent should have name "NewName"
  And the agent should have description "New description"
  And the updated_at timestamp should be recent
```

### Scenario: Delete agent
```gherkin
Scenario: User deletes an agent
  Given I have created an agent named "ToDelete"
  When I delete the agent
  Then the agent should no longer exist
  And associated memories should be deleted
  And associated sessions should be deleted
```

### Scenario: Start agent
```gherkin
Scenario: User starts an agent
  Given I have created an agent with status "idle"
  When I start the agent
  Then the agent status should change to "running"
```

### Scenario: Stop agent
```gherkin
Scenario: User stops a running agent
  Given I have created an agent with status "running"
  When I stop the agent
  Then the agent status should change to "idle"
```

## Feature: Memory Management

### Background
```gherkin
Background: Agent exists with memories
  Given I am authenticated as a valid user
  And I have an agent named "TestAgent" with id "agent-123"
  And the agent has no memories
```

### Scenario: Create a memory
```gherkin
Scenario: User creates a memory for an agent
  Given I have an agent with id "agent-123"
  When I create a memory with content "Important information" and type "fact"
  Then the memory should be created
  And the memory should be associated with agent "agent-123"
  And the memory should have default importance level of 5
```

### Scenario: List agent memories
```gherkin
Scenario: User lists memories for an agent
  Given my agent has 3 memories with types "fact", "experience", and "pattern"
  When I list all memories for the agent
  Then I should receive a list of 3 memories
  And each memory should include its content, type, and tags
```

### Scenario: Get specific memory
```gherkin
Scenario: User retrieves a specific memory
  Given my agent has a memory with content "Specific memory content"
  When I get that memory by ID
  Then I should see the content "Specific memory content"
  And I should see the memory type and metadata
```

### Scenario: Update memory
```gherkin
Scenario: User updates a memory
  Given my agent has a memory with content "Old content" and importance 5
  When I update the memory to content "New content" and importance 8
  Then the memory should have content "New content"
  And the memory should have importance 8
```

### Scenario: Delete memory
```gherkin
Scenario: User deletes a memory
  Given my agent has a memory with content "To be deleted"
  When I delete that memory
  Then the memory should no longer exist
```

### Scenario: Search memories by tags
```gherkin
Scenario: User searches memories by tags
  Given my agent has memories with tags ["important", "work"] and ["personal"]
  When I search for memories with tag "important"
  Then I should receive only memories with tag "important"
```

### Scenario: Create hierarchical memory
```gherkin
Scenario: User creates parent-child memory structure
  Given my agent has a memory with content "Parent memory"
  When I create a child memory with content "Child memory" referencing the parent
  Then the child memory should have a reference to the parent
  And the parent should show the child in its children
```

## Feature: Session Management

### Background
```gherkin
Background: User has an agent
  Given I am authenticated as a valid user
  And I have an agent named "TestAgent"
```

### Scenario: Create a session
```gherkin
Scenario: User starts a new session with an agent
  Given I have an agent named "TestAgent"
  When I create a new session with the agent
  Then the session should be created with status "active"
  And the session should have a start timestamp
  And the session should be associated with my user account
```

### Scenario: Get session details
```gherkin
Scenario: User retrieves session information
  Given I have an active session with id "session-123"
  When I get the session details
  Then I should see the session status "active"
  And I should see the start timestamp
  And I should see the associated agent
```

### Scenario: End a session
```gherkin
Scenario: User ends an active session
  Given I have an active session
  When I end the session
  Then the session status should change to "completed"
  And the session should have an end timestamp
```

### Scenario: List user sessions
```gherkin
Scenario: User lists their sessions
  Given I have created 5 sessions with various agents
  When I list all my sessions
  Then I should receive a list of 5 sessions
  And each session should show its status and timestamps
```

## Feature: Real-time Thought Streaming

### Background
```gherkin
Background: WebSocket connection established
  Given I am authenticated as a valid user
  And I have an active session with id "session-123"
  And I am connected via WebSocket
```

### Scenario: Subscribe to agent updates
```gherkin
Scenario: User subscribes to agent thought stream
  Given I am connected via WebSocket
  And I have an agent with id "agent-123"
  When I subscribe to agent "agent-123" updates
  Then I should receive a subscription confirmation
```

### Scenario: Receive thought updates
```gherkin
Scenario: Thoughts are streamed in real-time
  Given I am subscribed to agent "agent-123"
  When the agent processes a thought at layer 1
  Then I should receive a thought.update message with layer 1 content
```

### Scenario: Receive thought complete notification
```gherkin
Scenario: Thought cycle completes
  Given I am subscribed to agent "agent-123"
  When the agent completes a thought cycle
  Then I should receive a thought.complete message
  And the message should contain the final thought content
```

### Scenario: Receive agent status updates
```gherkin
Scenario: Agent status changes
  Given I am subscribed to agent "agent-123"
  When the agent status changes to "running"
  Then I should receive an agent.status message with status "running"
```

## Feature: LLM Provider Configuration

### Background
```gherkin
Background: User is authenticated
  Given I am authenticated as a valid user
```

### Scenario: Create LLM provider
```gherkin
Scenario: User adds an LLM provider
  Given no LLM provider with name "openai" exists
  When I create an LLM provider with name "openai", api_key "sk-xxx", and default_model "gpt-4"
  Then the provider should be created
  And the api_key should be stored securely
```

### Scenario: List LLM providers
```gherkin
Scenario: User lists available providers
  Given I have created providers "openai" and "anthropic"
  When I list all LLM providers
  Then I should receive a list of 2 providers
  And each provider should show name, base_url, and default_model
```

### Scenario: Update LLM provider
```gherkin
Scenario: User updates provider configuration
  Given I have an LLM provider with default_model "gpt-3.5-turbo"
  When I update the default_model to "gpt-4"
  Then the provider should have default_model "gpt-4"
```

### Scenario: Delete LLM provider
```gherkin
Scenario: User removes an LLM provider
  Given I have an LLM provider with name "old-provider"
  When I delete the provider
  Then the provider should no longer exist
```

### Scenario: Attach LLM to agent layer
```gherkin
Scenario: User configures LLM for agent layer
  Given I have an agent and an LLM provider
  When I create an LLM attachment for layer 1 with model "gpt-4"
  Then the agent should have an LLM configured for layer 1
  And the attachment should be retrievable
```

### Scenario: Update agent LLM attachment
```gherkin
Scenario: User changes agent's LLM configuration
  Given my agent has an LLM attachment for layer 1 with model "gpt-3.5"
  When I update the attachment to model "gpt-4"
  Then the attachment should use model "gpt-4"
```

### Scenario: Remove LLM attachment from agent
```gherkin
Scenario: User removes LLM from agent layer
  Given my agent has an LLM attachment for layer 2
  When I delete the attachment
  Then the agent should no longer have an LLM for layer 2
```

## Feature: Settings Management

### Background
```gherkin
Background: User is authenticated
  Given I am authenticated as a valid user
  And I have an agent named "TestAgent"
```

### Scenario: Get agent settings
```gherkin
Scenario: User retrieves agent settings
  Given my agent has settings for "max_tokens" and "temperature"
  When I get the agent settings
  Then I should see setting "max_tokens" with value 1000
  And I should see setting "temperature" with value 0.7
```

### Scenario: Update agent settings
```gherkin
Scenario: User modifies agent settings
  Given my agent has setting "max_tokens" with value 1000
  When I update the setting "max_tokens" to value 2000
  Then the setting should have value 2000
  And the updated_at timestamp should be recent
```

### Scenario: Get system settings (admin)
```gherkin
Scenario: Admin retrieves system settings
  Given I am an admin user
  When I get system settings
  Then I should see all system configuration values
  And secret values should be masked
```

### Scenario: Update system settings (admin)
```gherkin
Scenario: Admin updates system settings
  Given I am an admin user
  When I update system setting "max_sessions" to value 100
  Then the setting should have value 100
```

## Feature: Tool Whitelist Management

### Background
```gherkin
Background: User has an agent
  Given I am authenticated as a valid user
  And I have an agent named "TestAgent"
```

### Scenario: List available tool sources
```gherkin
Scenario: User sees available tool sources
  When I get available tool sources
  Then I should see "hardcoded" as a source
  And I should see "mcp" as a source
  And I should see "skill" as a source
```

### Scenario: List agent tool whitelist
```gherkin
Scenario: User lists enabled tools for agent
  Given my agent has tools "filesystem" and "http" enabled from "hardcoded"
  When I list my agent's tool whitelist
  Then I should see tool "filesystem" is enabled
  And I should see tool "http" is enabled
```

### Scenario: Update agent tool whitelist
```gherkin
Scenario: User enables a tool for agent
  Given my agent has no tools enabled
  When I enable tool "http" from source "hardcoded"
  Then tool "http" should be enabled for my agent
  And I should be able to use it
```

### Scenario: Disable tool for agent
```gherkin
Scenario: User disables a tool
  Given my agent has tool "filesystem" enabled
  When I disable tool "filesystem"
  Then tool "filesystem" should be disabled for my agent
  And I should not be able to use it
```

## Feature: API Security

### Scenario: Unauthenticated access denied
```gherkin
Scenario: Accessing protected route without token
  Given I am not authenticated
  When I try to access a protected endpoint
  Then I should receive a 401 Unauthorized response
```

### Scenario: Invalid token rejected
```gherkin
Scenario: Using invalid JWT token
  Given I have an invalid JWT token
  When I try to access a protected endpoint
  Then I should receive a 401 Unauthorized response
```

### Scenario: Expired token rejected
```gherkin
Scenario: Using expired JWT token
  Given I have an expired JWT token
  When I try to access a protected endpoint
  Then I should receive a 401 Unauthorized response
```

## Acceptance Criteria Mapping

| Scenario | Acceptance Criteria | Test Priority |
|----------|---------------------|---------------|
| User registers successfully | Entities Defined, Auth Strategy | Must |
| User login with valid credentials | Auth Strategy | Must |
| User login with invalid credentials | Auth Strategy | Must |
| Create a new agent | Entities Defined, API Structure | Must |
| List all agents | Entities Defined, API Structure | Must |
| Get agent details | Entities Defined, API Structure | Must |
| Update agent | Entities Defined, API Structure | Must |
| Delete agent | Entities Defined, API Structure | Must |
| Start/stop agent | Entities Defined, API Structure | Must |
| Create a memory | Entities Defined, API Structure | Must |
| List agent memories | Entities Defined, API Structure | Must |
| Search memories by tags | Entities Defined, API Structure | Should |
| Create session | Entities Defined, API Structure | Must |
| End session | Entities Defined, API Structure | Must |
| Receive thought updates | Real-time Support | Must |
| Create LLM provider | Entities Defined, API Structure | Must |
| Attach LLM to agent | Entities Defined, API Structure | Must |
| Update agent settings | Entities Defined, API Structure | Must |
| Update system settings | Entities Defined, API Structure | Must |
| Tool whitelist management | Entities Defined, API Structure | Should |
| Unauthenticated access denied | Auth Strategy | Must |
