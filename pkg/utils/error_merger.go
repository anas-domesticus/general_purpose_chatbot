// Package utils provides common utility functions for Go services.
// It includes error handling, server utilities, and channel management.
package utils //nolint:revive // var-naming: utils is an acceptable package name for shared utilities

import "sync"

// MergeErrorChans merges multiple error channels into a single output channel.
// It starts a goroutine for each input channel to forward errors to the output.
// The output channel is closed when all input channels are closed.
// This is useful for aggregating errors from multiple concurrent operations.
//
// Example:
//
//	err1 := make(chan error)
//	err2 := make(chan error)
//	merged := MergeErrorChans(err1, err2)
//	// Handle errors from merged channel
//	for err := range merged {
//		log.Error("Operation failed", logger.StringField("error", err.Error()))
//	}
func MergeErrorChans(channels ...chan error) chan error {
	out := make(chan error)
	var wg sync.WaitGroup

	// Start a goroutine for each input channel
	for _, ch := range channels {
		wg.Add(1)
		go func(c chan error) {
			defer wg.Done()
			for err := range c {
				out <- err
			}
		}(ch)
	}

	// Close output channel when all input channels are closed
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
