//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// fontSourcePurge wires the process-wide font source cache into the OOM
// purge chain (O9): under texture-OOM pressure the purge recovery path can
// drop cached font sources (and their file mappings) and rebuild them on the
// next load. render/text cannot import render (render imports text), so the
// adapter lives here where both directions are legal — same pattern as the
// image-cache registration below.
type fontSourcePurge struct{}

func (fontSourcePurge) PurgeEvictable() int64 {
	before := text.FontSourceCacheSnapshot().Entries
	text.PurgeFontSourceCache()
	after := text.FontSourceCacheSnapshot().Entries
	return int64(before - after)
}

func init() {
	render.RegisterPurgeEvictable("font-sources", fontSourcePurge{})
}
