//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"time"

	"github.com/energye/gpui/gpu/hal"
)

// OnSubmittedWorkDone captures the queue's submission index. The token is
// complete once Poll (which reads the GL sync objects via Fence.GetLatest)
// reaches that index. When the queue has no fence, Poll returns the
// submission index and the token completes immediately — same contract as the
// queueless path in Submit.
func (q *Queue) OnSubmittedWorkDone() (hal.CompletionToken, error) {
	return &submissionToken{q: q, index: q.LastSubmissionIndex()}, nil
}

// submissionToken implements hal.CompletionToken over a GLES submission index.
type submissionToken struct {
	q     *Queue
	index uint64
}

func (t *submissionToken) Done() bool {
	if t == nil || t.q == nil {
		return true
	}
	return t.q.Poll() >= t.index
}

func (t *submissionToken) Wait() error {
	for !t.Done() {
		time.Sleep(time.Millisecond)
	}
	return nil
}
