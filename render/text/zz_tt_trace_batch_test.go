package text

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestTmpTraceBatch: run TT hint for a batch of gids, emitting per-glyph
// stderr markers so the FT-trace comparator can align segments.
//
// Env GPUI_TT_TRACE=1 must be set to emit instruction trace.
func TestTmpTraceBatchFT(t *testing.T) {
	gidsRaw := os.Getenv("TT_GIDS")
	if gidsRaw == "" {
		t.Skip("TT_GIDS not set")
	}
	var gids []uint16
	for _, s := range strings.Split(gidsRaw, ",") {
		v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 16)
		if err != nil {
			t.Fatalf("bad gid %q", s)
		}
		gids = append(gids, uint16(v))
	}
	data, err := os.ReadFile("testdata/wqy-microhei.ttf")
	if err != nil {
		t.Fatal(err)
	}
	cache := newTTHintCache(data)
	for _, g := range gids {
		os.Stderr.WriteString("==GOGBEGIN=" + strconv.Itoa(int(g)) + "==\n")
		out, err := cache.hintGlyphOutline(g, 16)
		if err != nil {
			t.Fatalf("gid %d: %v", g, err)
		}
		t.Logf("gid %d done npts=%d", g, len(out.points))
		os.Stderr.WriteString("==GOGEND==\n")
	}
}

// ttOpcodeNames maps raw TT opcodes to Go-side instruction names for the
// FT-go trace comparator. Naming matches the engine's decoder paths
// (e.g. opIUP0/opIUP1, opPUSHB000). The comparator normalizes variants
// before diffing (see /tmp/opencode/ftrace_cmp.py norm()).
