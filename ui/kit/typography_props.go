//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package kit

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
)

// TypographyKind selects one of the four antd forms sharing one Base.
type TypographyKind string

const (
	// TypographyText is inline text (default).
	TypographyText TypographyKind = "text"
	// TypographyTitle is a heading; Level picks the size step.
	TypographyTitle TypographyKind = "title"
	// TypographyParagraph is a block of body text.
	TypographyParagraph TypographyKind = "paragraph"
	// TypographyLink is a hyperlink-colored, focusable text.
	TypographyLink TypographyKind = "link"
)

// TypographyType selects the semantic text color (antd `type`).
type TypographyType string

const (
	// TypographyTypeDefault uses the body text color.
	TypographyTypeDefault TypographyType = ""
	// TypographyTypeSecondary uses the secondary text color.
	TypographyTypeSecondary TypographyType = "secondary"
	// TypographyTypeSuccess uses the success color.
	TypographyTypeSuccess TypographyType = "success"
	// TypographyTypeWarning uses the warning color.
	TypographyTypeWarning TypographyType = "warning"
	// TypographyTypeDanger uses the error color.
	TypographyTypeDanger TypographyType = "danger"
)

// TypographyActionsPlacement positions the operator row relative to the text.
type TypographyActionsPlacement string

const (
	// TypographyActionsEnd is the antd default (operators after text).
	TypographyActionsEnd TypographyActionsPlacement = "end"
	// TypographyActionsStart puts operators before text.
	TypographyActionsStart TypographyActionsPlacement = "start"
)

// TypographyCopyable configures the copy operator (antd copyable subset).
type TypographyCopyable struct {
	// On enables the copy operator.
	On bool
	// Text overrides the copied payload; empty copies Value.
	Text string
	// OnCopy fires once per successful copy with the payload.
	OnCopy func(text string)
}

// TypographyEditable configures inline editing (antd editable subset).
type TypographyEditable struct {
	// On enables the edit operator.
	On bool
	// EditingCtl when non-nil makes editing fully controlled: the component
	// mirrors it and never flips its own state.
	EditingCtl *bool
	// MaxLength caps committed text; 0 = unlimited.
	MaxLength int
	// OnChange fires once per commit (Enter) with the new text.
	OnChange func(value string)
	// OnStart fires when editing begins.
	OnStart func()
	// OnEnd fires after a successful commit.
	OnEnd func()
	// OnCancel fires on Esc (abort without commit).
	OnCancel func()
}

// TypographyEllipsis configures overflow ellipsis (antd ellipsis subset).
type TypographyEllipsis struct {
	// On enables ellipsis.
	On bool
	// Rows caps visible lines; <=0 means one line.
	Rows int
	// Expandable shows the expand operator when text overflows.
	Expandable bool
	// Collapsible lets an expanded text collapse again.
	Collapsible bool
	// ExpandedCtl when non-nil makes expansion fully controlled.
	ExpandedCtl *bool
	// DefaultExpanded seeds the uncontrolled expansion state.
	DefaultExpanded bool
	// Suffix is appended after the ellipsis mark (or kept visible in
	// middle ellipsis as the fixed tail).
	Suffix string
	// Middle cuts head+tail with a fixed suffix tail instead of binary
	// search on the head.
	Middle bool
	// OnExpand fires with the final expanded value.
	OnExpand func(expanded bool)
	// OnEllipsis fires when ellipsis appears or disappears.
	OnEllipsis func(ellipsis bool)
}

// TypographyProps is the immutable per-call configuration (Widget facet).
type TypographyProps struct {
	Kind  TypographyKind
	Value string
	// Level picks the Title size step 1..5 (Title only; default 1).
	Level int
	Type  TypographyType

	Copyable TypographyCopyable
	Editable TypographyEditable
	Ellipsis TypographyEllipsis

	Strong    bool
	Italic    bool
	Underline bool
	Delete    bool
	Code      bool
	Mark      bool
	Keyboard  bool

	Disabled bool
	// ActionsPlacement orders the operator row (default end).
	ActionsPlacement TypographyActionsPlacement

	// OnClick fires for Link (and any kind with operators) on tap.
	OnClick func()
	// AriaLabel overrides the accessible name.
	AriaLabel string

	// ClassNames/Styles are the escape hatches; merged at the same tier
	// as the component theme, never replacing the token structure.
	ClassNames map[string]string
	Styles     map[string]string
}

// DefaultTypographyProps returns the antd defaults: Text form, level 1,
// default type, all operators off, operators at end.
func DefaultTypographyProps() TypographyProps {
	return TypographyProps{
		Kind:             TypographyText,
		Level:            1,
		Type:             TypographyTypeDefault,
		ActionsPlacement: TypographyActionsEnd,
	}
}

// TypographySubscribedAspects reports the Ctx aspects the build reads:
// theme (tokens), locale (expand/collapse copy), disabled (subtree OR).
func TypographySubscribedAspects() []scope.Aspect {
	return []scope.Aspect{
		scope.AspectTheme,
		scope.AspectLocale,
		scope.AspectDisabled,
	}
}
