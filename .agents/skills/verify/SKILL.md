---
name: verify
description: This skill should be used when the user sends a message starting with "/verify" to request confirmation that the agent has completed all required startup and setup steps before beginning any work. Use this skill to verify the agent is properly configured.
---

# Verify Skill

This skill confirms that all required setup steps have been completed before starting any work.

## Activation

When the user sends a message starting with `/verify`, activate this skill and respond with the verification checklist.

## Verification Steps

Before responding to any user request, verify all of the following:

1. **Setup Script Executed**: Confirm that `cd /workspace/project/ace_prototype && ./.openhands/setup.sh` has been run
   - Check if Go, Node.js, Docker are installed
   - Verify agency-agents directory exists

2. **Design Documentation file_editor**: Confirm that `design/README.md` and relevant unit documentation has been read
   - For unit-specific work, read `design/units/<unit-name>/` files

3. **Instructions Digested**: Confirm all AGENTS.md instructions are understood:
   - CRITICAL startup steps
   - Documentation update requirements
   - GitHub workflow rules
   - Coding best practices

4. **Agency Specialisation Activated**: Confirm the relevant specialist agent is loaded
   - Reference `.agents/skills/agency-specialisation/SKILL.md` for current workflow stage
   - Map the task to the appropriate specialist

## Response Format

Respond with the following format:

```
✅ Setup Verified

- ✅ Setup script executed
- ✅ Design documentation read
- ✅ AGENTS.md instructions digested
- ✅ Agency specialisation activated

<AgentName> activated
```

Replace `<AgentName>` with the appropriate specialist agent name based on the current workflow stage.

## Examples

**Example 1 - Initial verification:**
```
User: /verify

Agent: ✅ Setup Verified

- ✅ Setup script executed
- ✅ Design documentation read
- ✅ AGENTS.md instructions digested
- ✅ Agency specialisation activated

Sprint Prioritizer activated
```

**Example 2 - During unit work:**
```
User: /verify

Agent: ✅ Setup Verified

- ✅ Setup script executed
- ✅ Design documentation read
- ✅ Observability unit documentation read
- ✅ AGENTS.md instructions digested
- ✅ Agency specialisation activated

Trend Researcher activated
```

## Notes

- This skill should trigger on any message starting with `/verify`
- The response should be brief and formatted consistently
- Include the relevant specialist agent name based on the current task context
