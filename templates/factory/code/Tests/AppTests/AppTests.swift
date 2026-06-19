@testable import App
import XCTVapor

final class AppTests: XCTestCase {
    func testHealthCheck() throws {
        let app = Application(.testing)
        defer { app.shutdown() }
        try configure(app)

        try app.test(.GET, "health") { res in
            XCTAssertEqual(res.status, .ok)
        }
    }

    func testHelloWorld() throws {
        let app = Application(.testing)
        defer { app.shutdown() }
        try configure(app)

        try app.test(.GET, "/") { res in
            XCTAssertEqual(res.status, .ok)
            XCTAssertTrue(res.body.string.contains("Hello"))
        }
    }
}
