//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

// Conditional processing (usvg parser/switch.rs + converter is_visible_element):
//
//   - every element checks requiredExtensions / requiredFeatures /
//     systemLanguage; a failing condition removes the element with its
//     subtree (and it stays unreachable through <use>, whose re-parsed
//     shadow content goes through the same visibility check);
//   - requiredExtensions fails whenever present (usvg: has_attribute);
//   - requiredFeatures is a whitespace-separated feature list — every
//     string must be in the supported FEATURES table below (verbatim from
//     switch.rs, commented-out entries excluded);
//   - systemLanguage is a comma-separated BCP 47 list; it matches when the
//     user language equals an entry or equals the entry's prefix before
//     the first '-' (usvg is_valid_sys_lang, case-sensitive compare). The
//     user language is resvg's Options default ["en"] — a configurable
//     language option is a registered follow-up, not covered by corpus.

import (
	"encoding/xml"
	"strings"
)

// svgFeatures is the SVG 1.1 feature-string table usvg reports support
// for (switch.rs FEATURES, verbatim).
var svgFeatures = []string{
	"http://www.w3.org/TR/SVG11/feature#SVGDOM-static",
	"http://www.w3.org/TR/SVG11/feature#SVG-static",
	"http://www.w3.org/TR/SVG11/feature#CoreAttribute", // no xml:base and xml:lang
	"http://www.w3.org/TR/SVG11/feature#Structure",
	"http://www.w3.org/TR/SVG11/feature#BasicStructure",
	"http://www.w3.org/TR/SVG11/feature#ContainerAttribute", // `enable-background`
	"http://www.w3.org/TR/SVG11/feature#ConditionalProcessing",
	"http://www.w3.org/TR/SVG11/feature#Image",
	"http://www.w3.org/TR/SVG11/feature#Style",
	// "http://www.w3.org/TR/SVG11/feature#ViewportAttribute", // `clip` and `overflow`, not yet
	"http://www.w3.org/TR/SVG11/feature#Shape",
	"http://www.w3.org/TR/SVG11/feature#Text",
	"http://www.w3.org/TR/SVG11/feature#BasicText",
	"http://www.w3.org/TR/SVG11/feature#PaintAttribute", // no color-interpolation and color-rendering
	"http://www.w3.org/TR/SVG11/feature#BasicPaintAttribute",
	"http://www.w3.org/TR/SVG11/feature#OpacityAttribute",
	"http://www.w3.org/TR/SVG11/feature#GraphicsAttribute",
	"http://www.w3.org/TR/SVG11/feature#BasicGraphicsAttribute",
	"http://www.w3.org/TR/SVG11/feature#Marker",
	// "http://www.w3.org/TR/SVG11/feature#ColorProfile", // not yet
	"http://www.w3.org/TR/SVG11/feature#Gradient",
	"http://www.w3.org/TR/SVG11/feature#Pattern",
	"http://www.w3.org/TR/SVG11/feature#Clip",
	"http://www.w3.org/TR/SVG11/feature#BasicClip",
	"http://www.w3.org/TR/SVG11/feature#Mask",
	"http://www.w3.org/TR/SVG11/feature#Filter",
	"http://www.w3.org/TR/SVG11/feature#BasicFilter",
	// only xlink:href
	"http://www.w3.org/TR/SVG11/feature#XlinkAttribute",
}

// userLanguages is the render-side language preference (resvg Options
// default: ["en"]).
var userLanguages = []string{"en"}

// conditionPassed evaluates the conditional processing attributes
// (usvg is_condition_passed).
func conditionPassed(a Attrs) bool {
	if v, ok := a.Raw["requiredExtensions"]; ok {
		_ = v // any value fails, including empty (usvg has_attribute)
		return false
	}
	if feats, ok := a.Raw["requiredFeatures"]; ok {
		for _, f := range strings.Split(feats, " ") {
			found := false
			for _, supported := range svgFeatures {
				if f == supported {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return systemLanguageMatches(a)
}

// systemLanguageMatches implements usvg is_valid_sys_lang: comma-separated
// BCP 47 names; an entry matches when a user language equals it exactly or
// equals its prefix before the first '-'.
func systemLanguageMatches(a Attrs) bool {
	langs, ok := a.Raw["systemLanguage"]
	if !ok {
		return true
	}
	for _, lang := range strings.Split(langs, ",") {
		lang = strings.TrimSpace(lang)
		for _, user := range userLanguages {
			if user == lang {
				return true
			}
		}
		if idx := strings.IndexByte(lang, '-'); idx > 0 {
			prefix := lang[:idx]
			for _, user := range userLanguages {
				if user == prefix {
					return true
				}
			}
		}
	}
	return false
}

// checkConditions evaluates the visibility conditions on one parsed
// element (every element, matching converter is_visible_element):
// display="none" and the conditional-processing attributes remove the
// element with its subtree (and keep it unreachable through <use>).
func checkConditions(e Element) {
	if e == nil {
		return
	}
	a := e.attrs()
	if a == nil || a.condInvalid {
		return
	}
	a.condFailed = !conditionPassed(*a)
	if a.condFailed || strings.EqualFold(strings.TrimSpace(a.Raw["display"]), "none") {
		a.condInvalid = true
	}
}

// rootConditionFailed evaluates the conditional attributes on the root
// <svg> element (its attrs never go through parseElement).
func rootConditionFailed(se xml.StartElement) bool {
	a := Attrs{Raw: map[string]string{}}
	for _, attr := range se.Attr {
		a.Raw[attr.Name.Local] = attr.Value
	}
	return !conditionPassed(a)
}

// parseSwitchElement parses <switch> as a group wrapping only the first
// direct child whose conditions pass (usvg switch.rs convert: children
// .find(is_condition_passed) — exactly one child survives, others are
// dropped regardless of their own conditions).
func parseSwitchElement(pc *parseCtx, se xml.StartElement) (Element, error) {
	elem := &GroupElement{Attrs: parseAttrs(pc, se)}
	children, err := parseChildren(pc)
	if err != nil {
		return nil, err
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		if g, isGroup := child.(*GroupElement); isGroup && g.inert {
			// Foreign-namespace children were never candidates in usvg
			// (they never entered the tree); keep looking.
			continue
		}
		// Selection tests the conditional attributes only (usvg
		// is_condition_passed); a chosen child hidden by display="none"
		// renders nothing WITHOUT falling through to the next child.
		if a := child.attrs(); a != nil && !a.condFailed {
			elem.Children = []Element{child}
			return elem, nil
		}
	}
	return elem, nil // no child passes: renders nothing
}
