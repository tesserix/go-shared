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
// boundaries_test.go enforces both halves of that. See the design's D9.
package mcp
