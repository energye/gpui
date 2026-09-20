// Package kit hosts the flat antd component library.
//
// Every component lives directly in this package as <prefix>_*.go
// files (no per-component subpackages). Exported component names
// carry their prefix (ButtonProps, BuildButton). Foundation context
// (Ctx, WidgetState, resolve helpers) lives in ui/kit/internal/scope;
// this package re-exports it so examples/kit_* windows use only the
// public kit API and never import internal paths.
package kit
