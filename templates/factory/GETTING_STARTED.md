# Swift Service

Server-side Swift service managed by codefly.

## Structure

- `code/Package.swift` — SPM package definition
- `code/Sources/App/configure.swift` — Server configuration
- `code/Sources/App/routes.swift` — Your route definitions
- `code/Sources/Run/main.swift` — Entry point

## Commands

- Run: `codefly run service`
- Test: `codefly test service`
- Build: `codefly build service`

Add your routes in `code/Sources/App/routes.swift` and your business logic alongside.
