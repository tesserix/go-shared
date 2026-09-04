// Package mcp is the foundation every Tesserix MCP connector is built on:
// tool registration with closed input and output schemas, a GET-only upstream
// client, key verification, and per-tool metrics.
//
// # What is deliberately NOT here
//
// There is no MCP SDK import, and no protocol type, anywhere in this package
// tree. The binding to a specific SDK and protocol version lives in the
// consuming service.
//
// Two reasons. Every Go service in the estate imports go-shared, so an SDK
// dependency here enters ~30 module graphs whether those services serve MCP or
// not. And the protocol is where the movement is — the estate's registry
// records pin protocolVersion 2026-07-28 — so a protocol revision would
// otherwise force a go-shared release affecting every service to change
// something only MCP servers care about.
//
// Only the first of those is machine-enforced. boundaries_test.go checks the
// real dependency graph, tests included (via `go list -deps -test`), and fails if this package tree
// imports an MCP SDK or a product package — that is D9 and D2. The absence of
// protocol types is a design rule, not a machine-checked one: an import gate
// cannot see a hand-written struct that mirrors a protocol shape (a
// `protocolVersion` constant, a local `Tool` or `CallToolResult` lookalike).
// Keeping protocol types out of this tree is enforced by review, same as it
// would be in any package with no SDK dependency to check against.
package mcp
