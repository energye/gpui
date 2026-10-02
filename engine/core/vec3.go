//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package core

// Vec3 is a 3D point in game units, for true-2.5D math only (S79).
// It mirrors Vec2's style: plain numbers, no render dependency.
// Convert once at the render boundary in engine/renderconv.
type Vec3 struct {
	X, Y, Z float64
}

// V3 builds a Vec3.
func V3(x, y, z float64) Vec3 { return Vec3{X: x, Y: y, Z: z} }

// Add returns v+w.
func (v Vec3) Add(w Vec3) Vec3 { return Vec3{v.X + w.X, v.Y + w.Y, v.Z + w.Z} }

// Sub returns v-w.
func (v Vec3) Sub(w Vec3) Vec3 { return Vec3{v.X - w.X, v.Y - w.Y, v.Z - w.Z} }

// IsZero reports whether v is exactly the zero vector.
func (v Vec3) IsZero() bool { return v.X == 0 && v.Y == 0 && v.Z == 0 }
