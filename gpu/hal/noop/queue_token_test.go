//go:build !(js && wasm)

package noop

import "testing"

// TestQueueCompletionToken pins the synchronous-backend contract: a token
// returned after Submit is already complete, and Wait does not block.
func TestQueueCompletionToken(t *testing.T) {
	q := &Queue{}
	if _, err := q.Submit(); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	tok, err := q.OnSubmittedWorkDone()
	if err != nil {
		t.Fatalf("OnSubmittedWorkDone: %v", err)
	}
	if !tok.Done() {
		t.Fatal("synchronous backend token must be done immediately")
	}
	if err := tok.Wait(); err != nil {
		t.Fatalf("Wait: %v", err)
	}
}
