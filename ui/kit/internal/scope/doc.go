//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package scope carries the F0-1 theme-and-range context (P5).
//
// Ctx is the explicit parameter threaded through Build/Layout/Paint,
// mirroring the InheritedWidget data flow without globals.
// WidgetState plus StateResolver cover per-state value lookup,
// mirroring WidgetStateProperty. Three-level merge (this call's
// props, then the Ctx component theme, then the global seed default)
// mirrors ButtonStyle resolution. This package only reads ui/theme;
// it never touches render or gpu.
package scope
