//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// TrueType bytecode interpreter — arithmetic and math instructions.
//
// Implements: ADD, SUB, DIV, MUL, ABS, NEG, FLOOR, CEILING, MAX, MIN.
package text

// opAdd implements ADD[] (0x60).
func (e *ttEngine) opAdd() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return a + b, nil
	})
}

// opSub implements SUB[] (0x61).
func (e *ttEngine) opSub() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return a - b, nil
	})
}

// opDiv implements DIV[] (0x62).
// Division in 26.6: a * 64 / b (since both are 26.6).
func (e *ttEngine) opDiv() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		if b == 0 {
			return 0, ttErrDivideByZero
		}
		return ttMulDivNoRound(a, 64, b), nil
	})
}

// opMul implements MUL[] (0x63).
// Multiply in 26.6: a * b / 64.
func (e *ttEngine) opMul() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		return ttMulDiv(a, b, 64), nil
	})
}

// opAbs implements ABS[] (0x64).
func (e *ttEngine) opAbs() error {
	return e.valueStack.applyUnary(func(n int32) (int32, error) {
		if n < 0 {
			return -n, nil
		}
		return n, nil
	})
}

// opNeg implements NEG[] (0x65).
func (e *ttEngine) opNeg() error {
	return e.valueStack.applyUnary(func(n int32) (int32, error) {
		return -n, nil
	})
}

// opFloor implements FLOOR[] (0x66).
func (e *ttEngine) opFloor() error {
	return e.valueStack.applyUnary(func(n int32) (int32, error) {
		return ttFloor26Dot6(n), nil
	})
}

// opCeiling implements CEILING[] (0x67).
func (e *ttEngine) opCeiling() error {
	return e.valueStack.applyUnary(func(n int32) (int32, error) {
		return ttCeil26Dot6(n), nil
	})
}

// opMax implements MAX[] (0x8B).
func (e *ttEngine) opMax() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		if a > b {
			return a, nil
		}
		return b, nil
	})
}

// opMin implements MIN[] (0x8C).
func (e *ttEngine) opMin() error {
	return e.valueStack.applyBinary(func(a, b int32) (int32, error) {
		if a < b {
			return a, nil
		}
		return b, nil
	})
}

// opRound implements ROUND[ab] (0x68-0x6B).
func (e *ttEngine) opRound() error {
	rs := e.graphics.roundState
	return e.valueStack.applyUnary(func(n int32) (int32, error) {
		return rs.round(n), nil
	})
}
