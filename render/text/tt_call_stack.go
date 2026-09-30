//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// TrueType bytecode interpreter — call stack.
//
// Tracks nested function/instruction definition calls.
package text

// ttCallStackMaxDepth is the maximum nesting depth for function calls.
const ttCallStackMaxDepth = 32

// ttCallRecord is a record of an active function or instruction invocation.
type ttCallRecord struct {
	callerProgram ttProgramType
	returnPC      int
	currentCount  int32 // remaining loop iterations
	definition    ttDefinition
}

// ttCallStack tracks nested active function or instruction calls.
// Fixed-size array matching FreeType's 32-deep call stack.
type ttCallStack struct {
	records [ttCallStackMaxDepth]ttCallRecord
	top     int
}

// clear resets the call stack.
func (s *ttCallStack) clear() {
	s.top = 0
}

// push adds a call record to the stack.
func (s *ttCallStack) push(record ttCallRecord) error {
	if s.top >= ttCallStackMaxDepth {
		return ttErrCallStackOverflow
	}
	s.records[s.top] = record
	s.top++
	return nil
}

// peek returns the top record without removing it.
func (s *ttCallStack) peek() (ttCallRecord, bool) {
	if s.top <= 0 {
		return ttCallRecord{}, false
	}
	return s.records[s.top-1], true
}

// pop removes and returns the top record.
func (s *ttCallStack) pop() (ttCallRecord, error) {
	r, ok := s.peek()
	if !ok {
		return r, ttErrCallStackUnderflow
	}
	s.top--
	return r, nil
}
