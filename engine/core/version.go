//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package core

// SchemaVersion is the core contract version every later data format
// (atlas JSON, scene JSON, save JSON, Spine/TMX subsets) checks against.
// Bump Major for a breaking change, Minor for an additive one.
var SchemaVersion = Version{Major: 1, Minor: 0}
