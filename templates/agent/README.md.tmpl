
# {{if .Service}}{{.Service.Name}}{{else}}Service{{end}} — Swift Service Agent

## Overview

This agent manages a server-side Swift service using Vapor or Hummingbird.
It handles scaffolding, building, testing, hot-reload, and Docker deployment.

## Capabilities

- **HTTP server** with REST endpoints
- **Hot-reload** during development (file watching)
- **Docker build** for production (multi-stage: swift:slim to ubuntu:slim)
- **Kubernetes deployment** manifests (placeholder)

## File Layout

```
{{if .Service}}{{.Service.Name}}{{else}}service{{end}}/
├── code/
│   ├── Package.swift             ← SPM package definition
│   ├── Sources/
│   │   ├── App/
│   │   │   ├── configure.swift   ← Server configuration
│   │   │   └── routes.swift      ← YOUR route definitions
│   │   └── Run/
│   │       └── main.swift        ← Entry point
│   └── Tests/                    ← Your tests
└── openapi/
    └── api.swagger.json          (optional)
```

## Common Operations

### Add a new route

1. Edit `code/Sources/App/routes.swift`
2. Add your handler function
3. codefly will hot-reload automatically

### Run locally

```bash
codefly run service
```

### Run tests

```bash
codefly test service
```

## Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| `hot-reload` | true | Enable file-watching hot reload in dev mode |
| `source-dir` | code | Directory containing Swift sources |
| `framework` | vapor | Swift web framework: "vapor" or "hummingbird" |
