// Package gate hosts the F0-5 central gate: four checks every future
// component window must pass (Props coverage, State flow, wiring
// limited to L1/L2, Token via theme). Each check is pure logic over
// window-reported facts; windows collect the facts, gate judges them.
package gate
