package observe

import (
	"errors"

	"github.com/tesserix/go-shared/mcp"
	"github.com/tesserix/go-shared/mcp/upstream"
)

// OutcomeFor classifies an error into the outcome vocabulary.
//
// It lives here so the mapping exists ONCE. Every connector would otherwise
// write the same switch, and the moment two of them disagree — "error" in one,
// "internal" in another — the estate's per-tool dashboards stop being
// comparable across services, which is the whole reason the label is a
// controlled vocabulary.
//
// A nil error is OutcomeOK. Anything not recognised is OutcomeError rather than
// a new label invented at the call site.
func OutcomeFor(err error) Outcome {
	switch {
	case err == nil:
		return OutcomeOK
	case errors.Is(err, upstream.ErrNotFound):
		return OutcomeNotFound
	case errors.Is(err, upstream.ErrDeadlineExceeded):
		return OutcomeDeadline
	case errors.Is(err, upstream.ErrUnavailable):
		return OutcomeUnavailable
	case errors.Is(err, mcp.ErrInvalidArguments):
		return OutcomeInvalidInput
	default:
		return OutcomeError
	}
}
