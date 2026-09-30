//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package overlay fills the F13 overlay band with insert/remove/hit-order
// mechanics (P5d). This is a framework shell — not Ant Modal skins.
//
// Rules:
//
//   - Paint order: main band first, then overlay (bottom → top entry order).
//   - Hit order: overlay top → bottom, then main.
//   - Insert/Remove dirties only overlay-related layer ids; main DirtyLayerIDs
//     must not explode solely because a portal opened.
//
// Dependency: overlay → rendering, scene (not gpu).
package overlay
