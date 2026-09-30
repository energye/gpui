//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package animation provides Flutter-style AnimationController + curves.
//
// Default animation path is compositor-only (F09): prefer MutSetOpacity /
// MutSetOffset over re-recording pictures. Use saveLayer only when filters
// require isolation (see painting.SaveLayerBudget).
//
// Controllers auto-unregister from the TickerRegistry when one-shot animations
// complete or Stop is called (F17) — no zombie tickers / idle CPU.
package animation
