//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import "math"

// DrawRegularPolygon draws a regular polygon with n sides.
// When rotation=0, odd-sided polygons (triangle, pentagon) have a vertex
// pointing up, and even-sided polygons (square, hexagon) have a flat top.
func (c *Context) DrawRegularPolygon(n int, x, y, r, rotation float64) {
	angle := 2.0 * math.Pi / float64(n)
	rotation -= math.Pi / 2
	if n%2 == 0 {
		rotation += angle / 2
	}
	for i := 0; i < n; i++ {
		a := rotation + angle*float64(i)
		px := x + r*math.Cos(a)
		py := y + r*math.Sin(a)
		if i == 0 {
			c.MoveTo(px, py)
		} else {
			c.LineTo(px, py)
		}
	}
	c.ClosePath()
}
