# Contributing to Hello World Admission Controller

Thank you for your interest in contributing! This is a learning project designed to help people understand Kubernetes admission webhooks.

## 🎯 Project Goals

This project aims to:
- Provide a simple, easy-to-understand admission controller example
- Help developers learn how admission webhooks work
- Demonstrate best practices for webhook development
- Never deny any requests (observability only)

## 🚀 Getting Started

1. Fork the repository
2. Clone your fork
3. Create a feature branch: `git checkout -b feature/your-feature`
4. Make your changes
5. Test your changes in a Kubernetes cluster
6. Commit and push your changes
7. Open a pull request

## 💻 Development Setup

### Prerequisites

- Go 1.21+
- Docker
- Kubernetes cluster (kind, minikube, or similar)
- kubectl

### Local Development

```bash
# Install dependencies
go mod download

# Build locally
go build -o admission-controller main.go

# Run tests (if we add them)
go test ./...
```

## 📝 Code Style

- Follow standard Go conventions
- Use meaningful variable and function names
- Add comments for non-obvious logic
- Keep functions focused and small
- Use emojis in log messages to make them friendly 😊

## 🧪 Testing

Before submitting a PR:

1. Build the Docker image: `make build`
2. Deploy to a test cluster: `make deploy`
3. Test with various resources: `make test`
4. Check the logs: `make logs`
5. Verify no errors occur

## 📚 Areas for Contribution

### Ideas for Enhancements

- Add more detailed logging options
- Support for additional resource types
- Metrics/monitoring integration (Prometheus)
- Better filtering options
- Configuration file support
- Unit tests
- Integration tests
- Helm chart
- Support for mutating webhooks

### Documentation

- Improve README with more examples
- Add troubleshooting guides
- Create blog posts or tutorials
- Add architecture diagrams

## ⚠️ Important Guidelines

1. **Never add denial logic** - This project is observability-only
2. **Keep it simple** - This is a learning project
3. **Maintain backwards compatibility** - Don't break existing deployments
4. **Test thoroughly** - Admission webhooks can affect cluster operations
5. **Document your changes** - Help others understand what you did

## 🐛 Bug Reports

When filing a bug report, please include:

- Kubernetes version
- Deployment method (make, kubectl, etc.)
- Error messages or logs
- Steps to reproduce
- Expected vs actual behavior

## 💡 Feature Requests

We welcome feature requests! Please:

- Check if the feature already exists
- Explain the use case
- Keep the project goals in mind (simplicity, learning)
- Be open to feedback

## 📋 Pull Request Process

1. Update the README.md with details of changes if needed
2. Update the documentation for any new features
3. Test your changes in a real Kubernetes cluster
4. Ensure the PR description clearly describes the problem and solution
5. Reference any related issues

## 🤝 Code of Conduct

- Be respectful and welcoming
- Be patient with beginners
- Provide constructive feedback
- Focus on learning and improvement

## 📄 License

By contributing, you agree that your contributions will be licensed under the same license as the project.

## 🙏 Thank You!

Every contribution, no matter how small, is appreciated!
