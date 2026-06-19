import Vapor

func routes(_ app: Application) throws {
    // Health check endpoint (used by codefly for readiness)
    app.get("health") { req -> HTTPStatus in
        return .ok
    }

    // Example route
    app.get { req -> String in
        return "Hello from {{.Service.Name}}!"
    }

    app.get("hello", ":name") { req -> String in
        let name = req.parameters.get("name")!
        return "Hello, \(name)!"
    }
}
