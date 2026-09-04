package mcp_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// deps returns the full transitive import list of the mcp package tree, as the
// toolchain itself computes it. Shelling out to `go list` avoids adding
// golang.org/x/tools as a dependency purely to enforce a rule about
// dependencies.
//
// -test is load-bearing: the rule is "no MCP SDK import, anywhere, not in code
// and not in tests", and without it `go list -deps` reports only the non-test
// dependency graph — an SDK imported by a _test.go file would pass unseen.
func deps(t *testing.T) []string {
	t.Helper()
	out, err := exec.Command("go", "list", "-deps", "-test", "./...").Output()
	require.NoError(t, err, "go list must succeed")
	return strings.Fields(string(out))
}

// D9: the SDK binding lives in the consuming service, never here. If this
// fails, a protocol bump has just become a go-shared release for ~30 services.
func TestFoundationImportsNoMCPSDK(t *testing.T) {
	banned := []string{
		"modelcontextprotocol",
		"mcp-go",
		"mark3labs",
	}
	for _, dep := range deps(t) {
		for _, b := range banned {
			require.NotContains(t, dep, b,
				"go-shared/mcp must not import an MCP SDK (D9); found %s", dep)
		}
	}
}

// D2: the foundation is domain-free. A product import here would make every
// service that imports go-shared depend on one product's model.
func TestFoundationImportsNoProductPackage(t *testing.T) {
	banned := []string{
		"github.com/mark8ly/",
		"github.com/tesserix/tesserix-home",
		"github.com/tesserix/australis",
	}
	for _, dep := range deps(t) {
		for _, b := range banned {
			require.NotContains(t, dep, b,
				"go-shared/mcp must not import a product package (D2); found %s", dep)
		}
	}
}
