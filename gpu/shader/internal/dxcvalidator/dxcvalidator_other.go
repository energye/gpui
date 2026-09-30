//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !windows

package dxcvalidator

// validatorImpl is empty on non-Windows builds.
type validatorImpl struct{}

func (*validatorImpl) validate([]byte) (Result, error) {
	return Result{}, ErrUnsupported
}

func runImpl(func(*Validator) error) error {
	return ErrUnsupported
}
