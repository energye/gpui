//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package gate hosts the F0-5 central gate: four checks every future
// component window must pass (Props coverage, State flow, wiring
// limited to L1/L2, Token via theme). Each check is pure logic over
// window-reported facts; windows collect the facts, gate judges them.
package gate
