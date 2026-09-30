//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !linux

package scheduler

// ReadRSSKB is unavailable off Linux; returns 0.
func ReadRSSKB() int64 { return 0 }

func readCPUTime() (jiffies float64, ok bool) { return 0, false }

func clockTicks() float64 { return 100 }
