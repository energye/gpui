//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// TrueType bytecode interpreter — logical and comparison instructions.
//
// Implements: LT, LTEQ, GT, GTEQ, EQ, NEQ, ODD, EVEN, AND, OR, NOT.
package text

// boolToInt32 converts a boolean to a TrueType boolean value (0 or 1).
func boolToInt32(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

// opLt implements LT[] (0x50).
func (e *ttEngine) opLt() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return boolToInt32(a < b), nil
	})
}

// opLteq implements LTEQ[] (0x51).
func (e *ttEngine) opLteq() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return boolToInt32(a <= b), nil
	})
}

// opGt implements GT[] (0x52).
func (e *ttEngine) opGt() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return boolToInt32(a > b), nil
	})
}

// opGteq implements GTEQ[] (0x53).
func (e *ttEngine) opGteq() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return boolToInt32(a >= b), nil
	})
}

// opEq implements EQ[] (0x54).
func (e *ttEngine) opEq() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return boolToInt32(a == b), nil
	})
}

// opNeq implements NEQ[] (0x55).
func (e *ttEngine) opNeq() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return boolToInt32(a != b), nil
	})
}

// opOdd implements ODD[] (0x56).
// Rounds the value using the current round state, then tests if the
// integer part is odd.
func (e *ttEngine) opOdd() error {
	rs := e.graphics.roundState
	return e.valueStack.applyUnary(func(e1 int32) (int32, error) {
		rounded := rs.round(e1)
		return boolToInt32(rounded&127 == 64), nil
	})
}

// opEven implements EVEN[] (0x57).
// Rounds the value using the current round state, then tests if the
// integer part is even.
func (e *ttEngine) opEven() error {
	rs := e.graphics.roundState
	return e.valueStack.applyUnary(func(e1 int32) (int32, error) {
		rounded := rs.round(e1)
		return boolToInt32(rounded&127 == 0), nil
	})
}

// opAnd implements AND[] (0x5A).
func (e *ttEngine) opAnd() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return boolToInt32(a != 0 && b != 0), nil
	})
}

// opOr implements OR[] (0x5B).
func (e *ttEngine) opOr() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return boolToInt32(a != 0 || b != 0), nil
	})
}

// opNot implements NOT[] (0x5C).
func (e *ttEngine) opNot() error {
	return e.valueStack.applyUnary(func(e1 int32) (int32, error) {
		return boolToInt32(e1 == 0), nil
	})
}
