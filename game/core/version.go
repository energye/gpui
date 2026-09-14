package core

// SchemaVersion is the core contract version every later data format
// (atlas JSON, scene JSON, save JSON, Spine/TMX subsets) checks against.
// Bump Major for a breaking change, Minor for an additive one.
var SchemaVersion = Version{Major: 1, Minor: 0}
