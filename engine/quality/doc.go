//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package quality carries the quality-tier math every 2.5D play follows.
//
// Frozen 2026-09-16: Level, LevelHigh,
// LevelMedium, LevelLow, ParseLevel, Spec, SpecFor, Quality, NewQuality,
// ParseQuality, LoadQuality, Encode, StoreQuality, Level, Spec, Equal,
// Switch. Moved from engine/save in 2026-09-30 (E02); save keeps only
// slot/version migration. Additive changes only.
//
// Three tiers, frozen: high maps to 1000 particles plus 8 lights plus
// scale 1.00, medium maps to 500 particles plus 4 lights plus scale 0.75,
// low maps to 200 particles plus 2 lights plus scale 0.50. High is the
// full-fidelity tier; medium halves particles and lights and renders at
// three quarters resolution; low keeps one fifth of the particles, a
// quarter of the lights, and renders at half resolution. The numbers
// live in engine/quality/testdata/quality_cases.json; the code only replays
// them, so tuning a tier means editing that file first.
//
// The caller builds a Quality with New or Parse, reads the frozen Spec
// with Spec, and changes tiers with Switch. Switching only replaces the
// level: the same scene keeps running, nothing reloads, nothing flashes.
// Parse levels by design: only version "1.0" files parse; empty input is
// InvalidArg, torn shapes are BadData, foreign or newer versions are
// VersionMismatch, oversized files are OutOfMemory. Constructor-style
// calls (New, ParseLevel) report bad tiers as InvalidArg; file Parse
// reports the same bad tier as BadData. Nil receivers report InvalidArg
// and change nothing. Only core numbers cross the boundary; render types
// are converted at the call site, never here.
//
// File (self JSON, frozen): version "1.0", quality "high"|"medium"|"low".
// Unknown fields are ignored so a newer minor file still decodes its
// tier; a missing quality is BadData, never a guessed tier.
package quality
