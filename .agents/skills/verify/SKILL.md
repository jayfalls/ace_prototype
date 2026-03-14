---
name: verify
description: This skill should be used when the user sends a message starting with "/verify" to request confirmation that the agent has completed all required startup and setup steps before beginning any work. Use this skill to verify the agent is properly configured.
triggers:
  - /verify
  - verify
  - merged
  - next
  - commit
  - push
  - address
  - comment
  - start
  - begin
  - work
  - unit
  - design
  - plan
  - agent
  - flow
  - issue
  - pr
  - pull
  - implement
  - units
  - BSD
  - bsd
  - user_stories
  - research
  - FSD
  - fsd
  - implementation
  - architecture
  - problem
  - planning
  - design document
  - design documents
---

# Verify Skill

This skill confirms that all required setup steps have been completed before starting any work.

## CRITICAL: This skill MUST be activated on EVERY request

**BEFORE responding to any user request, you MUST:**
1. Run the setup script (if not already run this session)
2. file_editor the design documentation
3. Activate the appropriate agency specialist
4. **Explicitly state the activation** using the format below

## Activation Steps (IN ORDER)

### Step 1: Setup Script
Run: `cd /workspace/project/ace_prototype && ./.openhands/setup.sh`
- Only run once per session
- Verify: Go, Node.js, Docker, agency-agents exist

### Step 2: Design Documentation
file_editor: 
- `design/README.md` 
- `design/units/README.md`
- Relevant unit documentation

### Step 3: Agency Specialisation
**READ the agency specialisation skill file:**
- Read: `.agents/skills/agency-specialisation/SKILL.md` (this file tells you which specialist to use)
- file_editor the relevant specialist's file from `agency-agents/` directory
- Activate by stating their name with the full path

### Step 4: State Activation (MANDATORY)
**You MUST respond with this exact format:**

```
✅ Setup Verified

- ✅ Setup script executed
- ✅ Design documentation read
- ✅ AGENTS.md instructions digested
- ✅ Agency specialisation activated

<S SpecialistName > activated
```

Replace `<SpecialistName>` with the actual specialist name from Step 3.

## Examples

**Correct:**
```
User: /verify

Agent: ✅ Setup Verified

- ✅ Setup script executed
- ✅ Design documentation read  
- ✅ AGENTS.md instructions digested
- ✅ Agency specialisation activated

Trend Researcher + Tool Evaluator activated
```

**Incorrect (do NOT do this):**
- Just proceeding without verification
- Not stating the specialist name
- Skipping any step
