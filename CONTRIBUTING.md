# Contributing to DynaGo

Thank you for your interest in contributing to DynaGo! This guide will help you get started.

## Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/<your-username>/dynago.git
   cd dynago
   ```
3. Create a branch for your change:
   ```bash
   git checkout -b my-feature
   ```

## Development

### Prerequisites

- Go 1.23 or later

### Running Tests

```bash
go test ./...
```

### Linting

```bash
go vet ./...
```

## Submitting Changes

1. Ensure all tests pass and there are no lint warnings.
2. Write clear, concise commit messages.
3. Open a pull request against the `main` branch.
4. Describe what your change does and why.

## Pull Request Guidelines

- Keep PRs focused — one feature or fix per PR.
- Add tests for new functionality.
- Update documentation if your change affects the public API.
- Follow existing code style and conventions.

## Reporting Bugs

Open an [issue](https://github.com/danielMensah/dynago/issues) with:

- A clear description of the bug
- Steps to reproduce
- Expected vs. actual behavior
- Go version and OS

## Suggesting Features

Feature ideas are welcome! Open an [issue](https://github.com/danielMensah/dynago/issues) and describe the use case and proposed solution.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
