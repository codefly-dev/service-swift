import Vapor

public func configure(_ app: Application) throws {
    // Read port from environment (codefly injects PORT)
    let port = Environment.get("PORT").flatMap(Int.init) ?? 8080

    app.http.server.configuration.hostname = "0.0.0.0"
    app.http.server.configuration.port = port

    try routes(app)
}
