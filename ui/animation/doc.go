// Package animation provides Flutter-style AnimationController + curves (P3/P5e).
//
// Default animation path is compositor-only (F09): prefer MutSetOpacity /
// MutSetOffset over re-recording pictures. Use saveLayer only when filters
// require isolation (see painting.SaveLayerBudget).
//
// Controllers auto-unregister from the TickerRegistry when one-shot animations
// complete or Stop is called (F17) — no zombie tickers / idle CPU.
package animation
