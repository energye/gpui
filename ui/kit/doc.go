//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// Package kit hosts the flat antd component library.
//
// Every component lives directly in this package as <prefix>_*.go
// files (no per-component subpackages). Exported component names
// carry their prefix (ButtonProps, BuildButton). Foundation facades
// live here too: prim.go (layout/decor/content over ui/rendering),
// behavior.go (interaction/motion/semantics/performance/direction),
// gate.go (the four F0 central-gate checks), scope.go (Ctx,
// WidgetState, resolve helpers). Internal implementations sit under
// ui/kit/internal/...; examples/kit_* windows use only the public
// kit API and never import internal paths.
package kit
