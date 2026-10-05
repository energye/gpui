//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package nav freezes the CPU-side grid pathfinding every 2.5D crowd feeds.
//
// Frozen 2026-10-05 (S87): Grid, NewGrid, Bake, Encode, Decode, Equal,
// Cell, Status, FindPath, CellToWorld, WorldToCell, Agent, NewAgent,
// DefaultSpeed, ArriveTol, Queue, NewQueue, DefaultPerFrame, Tuning,
// LoadTuning, DrainFrames. Additive changes only.
//
// This package only computes numbers; it draws nothing and steps nothing.
// The caller bakes one Grid, queues path requests through Queue, then
// moves Agents along the returned waypoints. Only core numbers are used;
// no new Vec2/Rect is defined here.
//
// Godot mapping (only the approach is copied, no code is moved):
//   - AStarGrid2D 8-way default feel (G27): orthogonal cost 1, diagonal
//     cost sqrt(2), octile heuristic, diagonal steps never cut wall
//     corners (both orthogonal neighbours must be walkable).
//   - NavigationAgent2D target/next_path/desired_distance/max_speed:
//     Agent holds the waypoint list, Next is the next_path cursor,
//     ArriveTol is desired_distance, Speed is max_speed.
//   - NavigationPolygon bake: Bake builds the walkable grid from the
//     caller-supplied solid predicate; Encode is the byte-identical
//     baked artifact (row-major 0/1, same input yields same bytes).
//
// Window intent: examples/engine/nav; pure grid math, offscreen golden only.
package nav
