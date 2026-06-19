package temporal

import (
	"fmt"
	"strings"
)

// Naming conventions (ADR tesserix/Home-Chef-App#118): task queues are
// "<product>-<domain>" and workflow IDs are "<product>:<domain>:<entityID>".
// Using the helpers keeps every product consistent and makes workflow IDs
// idempotent (reusing the same id for the same entity makes a producer-side
// retry a no-op).

// TaskQueue builds a task-queue name: TaskQueue("homechef","orders") →
// "homechef-orders".
func TaskQueue(product, domain string) string {
	return strings.ToLower(product) + "-" + strings.ToLower(domain)
}

// WorkflowID builds an idempotent workflow id: WorkflowID("homechef","order",id)
// → "homechef:order:<id>".
func WorkflowID(product, domain, entityID string) string {
	return fmt.Sprintf("%s:%s:%s", strings.ToLower(product), strings.ToLower(domain), entityID)
}
