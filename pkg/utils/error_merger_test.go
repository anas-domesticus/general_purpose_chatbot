package utils

import (
	"errors"
	"testing"
	"time"
)

func TestMergeErrorChans(t *testing.T) {
	// Create test channels
	ch1 := make(chan error, 1)
	ch2 := make(chan error, 1)
	
	// Merge them
	merged := MergeErrorChans(ch1, ch2)
	
	// Send test errors
	ch1 <- errors.New("error 1")
	ch2 <- errors.New("error 2")
	
	// Close source channels
	close(ch1)
	close(ch2)
	
	// Collect errors
	var receivedErrors []string
	timeout := time.After(time.Second)
	
	for {
		select {
		case err, ok := <-merged:
			if !ok {
				// Channel closed, we're done
				goto done
			}
			receivedErrors = append(receivedErrors, err.Error())
		case <-timeout:
			t.Fatal("Timeout waiting for errors")
		}
	}
	
done:
	// Verify we received both errors
	if len(receivedErrors) != 2 {
		t.Fatalf("Expected 2 errors, got %d", len(receivedErrors))
	}
	
	// The order might vary, so check both are present
	errorMap := make(map[string]bool)
	for _, err := range receivedErrors {
		errorMap[err] = true
	}
	
	if !errorMap["error 1"] || !errorMap["error 2"] {
		t.Fatalf("Missing expected errors. Got: %v", receivedErrors)
	}
}