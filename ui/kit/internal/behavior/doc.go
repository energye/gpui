//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package behavior hosts the F0-4 interaction facades.
//
// Every facade wraps existing engine capability only: gestures for tap
// arbitration, focus for keyboard traversal, overlay for placement and
// outside-close, textinput concepts via a small controller interface.
// No new GPU code lives here. Behavior never draws; it owns state
// transitions and event routing, while prim owns nodes and paint.
package behavior
