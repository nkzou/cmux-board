package jira

import (
	"context"
)

// fakeRunner returns a Runner that always returns the given stdout, stderr, exitCode, and err.
func fakeRunner(stdout, stderr []byte, exitCode int, err error) Runner {
	return func(_ context.Context, _ ...string) ([]byte, []byte, int, error) {
		return stdout, stderr, exitCode, err
	}
}

// capturingRunner returns a Runner that records the args it receives and returns canned output.
func capturingRunner(captured *[]string, stdout, stderr []byte, exitCode int, err error) Runner {
	return func(_ context.Context, args ...string) ([]byte, []byte, int, error) {
		*captured = append(*captured, args...)
		return stdout, stderr, exitCode, err
	}
}

// sequentialRunnerResponses allows providing different responses per call.
type runnerResponse struct {
	stdout   []byte
	stderr   []byte
	exitCode int
	err      error
}

// sequentialRunner returns responses in order; panics if more calls are made than responses provided.
func sequentialRunner(responses []runnerResponse) Runner {
	idx := 0
	return func(_ context.Context, _ ...string) ([]byte, []byte, int, error) {
		if idx >= len(responses) {
			panic("sequentialRunner: more calls than responses")
		}
		r := responses[idx]
		idx++
		return r.stdout, r.stderr, r.exitCode, r.err
	}
}

// sequentialRunnerWithArgs captures args per call alongside providing canned responses.
func sequentialRunnerWithArgs(allArgs *[][]string, responses []runnerResponse) Runner {
	idx := 0
	return func(_ context.Context, args ...string) ([]byte, []byte, int, error) {
		argsCopy := make([]string, len(args))
		copy(argsCopy, args)
		*allArgs = append(*allArgs, argsCopy)
		if idx >= len(responses) {
			panic("sequentialRunnerWithArgs: more calls than responses")
		}
		r := responses[idx]
		idx++
		return r.stdout, r.stderr, r.exitCode, r.err
	}
}
