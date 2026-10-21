package tspdnative

import "fmt"

// ErrorKind classifies a solve failure.
type ErrorKind uint8

const (
	// KindNetwork covers DNS, TLS, connection and overall timeout failures.
	KindNetwork ErrorKind = iota + 1

	// KindProxy reports an invalid proxy URL or an unsupported proxy scheme.
	KindProxy

	// KindGetFailed reports a non-2xx response to the first page request.
	KindGetFailed

	// KindNoAnswer reports that the answer cookie could not be constructed
	// from the challenge page and sensor scripts.
	KindNoAnswer

	// KindNoTSPD reports that the edge did not issue TSPD_101 after the
	// answer cookie was sent.
	KindNoTSPD
)

// String returns the stable machine-readable kind name.
func (k ErrorKind) String() string {
	switch k {
	case KindNetwork:
		return "network"
	case KindProxy:
		return "proxy"
	case KindGetFailed:
		return "get_failed"
	case KindNoAnswer:
		return "no_answer"
	case KindNoTSPD:
		return "no_tspd"
	default:
		return "unknown"
	}
}

// SolveError is returned for every expected solver failure.
//
// Inspect Kind with errors.As:
//
//	var solveErr *tspdnative.SolveError
//	if errors.As(err, &solveErr) && solveErr.Kind == tspdnative.KindNoTSPD {
//		// The edge rejected the answer cookie.
//	}
type SolveError struct {
	Kind ErrorKind
	// HTTPStatus is set for KindGetFailed and, when available, KindNoTSPD.
	HTTPStatus int
	Err        error
}

func (e *SolveError) Error() string {
	if e.HTTPStatus != 0 {
		return fmt.Sprintf("tspdnative: %s: %v (HTTP %d)", e.Kind, e.Err, e.HTTPStatus)
	}
	return fmt.Sprintf("tspdnative: %s: %v", e.Kind, e.Err)
}

// Unwrap exposes the underlying cause for errors.Is/errors.As.
func (e *SolveError) Unwrap() error { return e.Err }

func solveError(kind ErrorKind, err error) *SolveError {
	return &SolveError{Kind: kind, Err: err}
}
