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

## Feature: [Feature Name]

### Background
```gherkin
Background: [Common setup]
  Given [precondition 1]
  And [precondition 2]
```

### Scenario: [Scenario Title]
```gherkin
Scenario: [Description of the scenario]
  Given [precondition]
  When [user performs action]
  Then [expected result]
  And [additional expected result]
```

### Scenario Outline: [Title with Variations]
```gherkin
Scenario Outline: [Description with <param>]
  Given [precondition with <param>]
  When [action with <param>]
  Then [expected result]

  Examples:
    | param | expected |
    | value1 | result1 |
    | value2 | result2 |
```

## Example User Stories

### Story 1: [Title]
```gherkin
Feature: [Feature name]

  Scenario: [Title]
    Given [the system is in a particular state]
    When [user performs action]
    Then [outcome occurs]
    And [another outcome occurs]
```

### Story 2: [Title]
```gherkin
Feature: [Feature name]

  Scenario: [Title]
    Given [precondition]
    When [action]
    Then [result]
```

## Acceptance Criteria Mapping

| Scenario | Acceptance Criteria | Test Priority |
|----------|---------------------|---------------|
| [Scenario 1] | [Which BSD criteria this satisfies] | [Must/Should/Could] |
| [Scenario 2] | [Which BSD criteria this satisfies] | [Must/Should/Could] |