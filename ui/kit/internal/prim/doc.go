// Package prim hosts the F0-2 layout and decor facades (P1+P2).
//
// Every facade wraps existing ui/rendering capability only: constraints,
// boxes, align, viewport, virtual list, clip, opacity, transform, filters
// and draw helpers. No new GPU code lives here. Sizes, radii and spacing
// resolve from ui/theme through scope.Ctx; this package never hardcodes
// user-visible numbers.
package prim
