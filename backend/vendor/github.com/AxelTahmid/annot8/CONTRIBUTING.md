# Contributing to Annot8

We want to make contributing to Annot8 as easy and transparent as possible, whether it's:

- Reporting a bug
- Discussing the current state of the code
- Submitting a fix
- Proposing new features
- Becoming a maintainer

## Development Process

We use GitHub to host code, to track issues and feature requests, as well as accept pull requests.

1. Fork the repo and create your branch from `main`
2. If you've added code that should be tested, add tests
3. If you've changed APIs, update the documentation
4. Ensure the test suite passes
5. Make sure your code lints
6. Issue that pull request!

## Prerequisites

- Go 1.25 or higher
- Git
- golangci-lint (for linting)

## Setting Up Development Environment

```bash
# Fork and clone the repository
git clone https://github.com/AxelTahmid/annot8.git
cd annot8

# Install dependencies
go mod download

# Run tests to ensure everything works
go test ./...
```

## Testing

We take testing seriously. Please ensure your changes include appropriate tests:

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test file
go test -v ./pkg/annot8 -run TestGenerator

# Run benchmarks
go test -bench=. ./pkg/annot8
```

### Test Categories

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test the full OpenAPI generation process
3. **Benchmark Tests**: Performance testing for critical paths

### Writing Tests

- Follow Go testing conventions
- Use table-driven tests for multiple test cases
- Include both positive and negative test cases
- Test edge cases and error conditions
- Aim for >90% test coverage

Example test structure:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    InputType
        expected ExpectedType
        wantErr  bool
    }{
        {
            name:     "valid input",
            input:    validInput,
            expected: expectedOutput,
            wantErr:  false,
        },
        {
            name:    "invalid input",
            input:   invalidInput,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := MyFunction(tt.input)

            if tt.wantErr {
                assert.Error(t, err)
                return
            }

            assert.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

## Code Style

### Formatting

- Use `gofmt` to format your code
- Follow standard Go naming conventions
- Use meaningful variable and function names
- Keep functions small and focused

### Documentation

- Document all exported functions, types, and variables
- Use complete sentences in comments
- Include examples for complex functions
- Keep comments up to date with code changes

### Error Handling

- Always handle errors appropriately
- Use meaningful error messages
- Prefer returning errors over panicking
- Use structured logging for debugging

Example:

```go
// GenerateSchema creates a JSON schema for the given type name.
// It returns an error if the type cannot be found or processed.
func GenerateSchema(typeName string) (*Schema, error) {
    if typeName == "" {
        return nil, fmt.Errorf("type name cannot be empty")
    }

    // Implementation...

    return schema, nil
}
```

## Code Quality

### Linting

We use `golangci-lint` for code quality checks:

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
golangci-lint run

# Run linter with auto-fix
golangci-lint run --fix
```

### Performance

- Use benchmarks to measure performance impact
- Avoid unnecessary allocations in hot paths
- Use profiling tools to identify bottlenecks
- Consider memory usage, especially for large projects

## Pull Request Process

1. **Create a feature branch** from `main`:

    ```bash
    git checkout -b feature/your-feature-name
    ```

2. **Make your changes** following the guidelines above

3. **Add or update tests** for your changes

4. **Update documentation** if needed:

    - Update README.md for user-facing changes
    - Update code comments for API changes
    - Add examples if helpful

5. **Ensure all checks pass**:

    ```bash
    go test ./...
    golangci-lint run
    go mod tidy
    ```

6. **Commit your changes** with a clear commit message:

    ```bash
    git commit -m "feat: add support for custom annotations

    - Add parsing for custom @Custom annotation
    - Include tests for new functionality
    - Update documentation with usage examples"
    ```

7. **Push to your fork** and create a pull request

8. **Respond to feedback** and make necessary changes

### Pull Request Guidelines

- Use a clear and descriptive title
- Include a detailed description of changes
- Reference any related issues
- Include screenshots for UI changes
- Ensure CI checks pass
- Request review from maintainers

## Reporting Bugs

We use GitHub issues to track public bugs. Report a bug by opening a new issue.

### Bug Report Template

When reporting bugs, please include:

- **Summary**: Brief description of the bug
- **Environment**: Go version, OS, relevant versions
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Expected Behavior**: What you expected to happen
- **Actual Behavior**: What actually happened
- **Code Sample**: Minimal code that reproduces the issue
- **Additional Context**: Screenshots, logs, etc.

Example:

````markdown
## Bug Report

### Summary

OpenAPI generation fails when using pointer to slice types

### Environment

-   Go version: 1.25.0
-   OS: macOS 14.1
-   OpenApi Gen version: v0.1.0

### Steps to Reproduce

1. Define a struct with `Field *[]string`
2. Generate OpenAPI spec
3. Check generated schema

### Expected Behavior

Should generate array schema with proper typing

### Actual Behavior

Generates object schema instead of array

### Code Sample

```go
type Example struct {
    Items *[]string `json:"items,omitempty"`
}
```
````

## Feature Requests

WeI welcome feature requests! Please use GitHub issues with the `enhancement` label.

### Feature Request Template

- **Summary**: Brief description of the feature
- **Motivation**: Why this feature would be useful
- **Detailed Description**: How the feature should work
- **Use Cases**: Real-world scenarios where this helps
- **Implementation Ideas**: Suggestions for implementation
- **Breaking Changes**: Any potential breaking changes

## Release Process

Our release process follows semantic versioning:

- **Major versions** (1.0.0): Breaking changes
- **Minor versions** (1.1.0): New features, backward compatible
- **Patch versions** (1.1.1): Bug fixes, backward compatible

### Release Checklist

1. Update version numbers
2. Update CHANGELOG.md
3. Run full test suite
4. Create release notes
5. Tag the release
6. Publish to GitHub

## Issue Labels

We use labels to categorize issues:

- `bug`: Something isn't working
- `enhancement`: New feature or request
- `documentation`: Improvements or additions to documentation
- `good first issue`: Good for newcomers
- `help wanted`: Extra attention is needed
- `performance`: Performance-related issues
- `breaking change`: Would break existing functionality

## Development Tips

### Useful Commands

```bash
# Run tests in watch mode (requires entr)
find . -name "*.go" | entr -c go test ./...

# Generate coverage report
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# Profile memory usage
go test -memprofile=mem.prof -bench=. ./...
go tool pprof mem.prof

# Check for race conditions
go test -race ./...

# Update all dependencies
go get -u ./...
go mod tidy
```

### Debugging

- Use the `-v` flag for verbose test output
- Use `log/slog` for structured logging
- Use the debugger (delve) for complex issues
- Add temporary print statements sparingly

### Performance Tips

- Use benchmarks to measure performance
- Profile CPU and memory usage
- Avoid premature optimization
- Focus on algorithmic improvements first

## Community Guidelines

### Code of Conduct

This project adheres to a code of conduct that we expect all participants to follow:

- Be respectful and inclusive
- Welcome newcomers and help them learn
- Focus on what's best for the community
- Show empathy towards other community members

### Communication

- Use clear, concise language
- Be patient with questions and suggestions
- Provide constructive feedback
- Celebrate contributions and achievements

## Recognition

Contributors will be recognized in:

- The project README
- Release notes for significant contributions
- GitHub contributor statistics
- Special recognition for major features

## Resources

### Go Resources

- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Go Testing](https://golang.org/pkg/testing/)

### OpenAPI Resources

- [OpenAPI Specification](https://spec.openapis.org/oas/v3.1.0)
- [Swagger Documentation](https://swagger.io/docs/)

### Project Resources

- [Project Issues](https://github.com/AxelTahmid/annot8/issues)
- [Project Discussions](https://github.com/AxelTahmid/annot8/discussions)
- [Project Wiki](https://github.com/AxelTahmid/annot8/wiki)

## Questions?

Don't hesitate to ask questions:

- Open a discussion on GitHub
- Comment on relevant issues
- Reach out to maintainers

We're here to help make your contribution experience great!

---

Thank you for contributing to Annot8! 🎉
