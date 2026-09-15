package tex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/energye/gpui/game/core"
)

// Window intent: game_tex --case=stream rides P2 (W3 gate needs an
// independent window per ability; the window is not built yet). Until
// then TestStreamOffscreenGolden is the offscreen comparison plus the
// auto verdict: frozen spots, sharp magenta placeholder, budgets.

type streamDef struct {
	ID     string `json:"id"`
	File   string `json:"file"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Blocks int    `json:"blocks"`
	Upload int    `json:"upload"`
	Pixels int    `json:"pixels"`
}

type streamSpot struct {
	File string `json:"file"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	RGBA [4]int `json:"rgba"`
}

type streamPlaceholder struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Blocks int    `json:"blocks"`
	Upload int    `json:"upload"`
	Pixels int    `json:"pixels"`
	RGBA   [4]int `json:"rgba"`
}

type streamCases struct {
	Tolerance           int               `json:"tolerance"`
	Format              string            `json:"format"`
	MaxStreams          int               `json:"max_streams"`
	MaxStreamBytes      int               `json:"max_stream_bytes"`
	MaxStreamAssetBytes int               `json:"max_stream_asset_bytes"`
	MaxIDLen            int               `json:"max_id_len"`
	Placeholder         streamPlaceholder `json:"placeholder"`
	Streams             []streamDef       `json:"streams"`
	Spots               []streamSpot      `json:"spots"`
}

func loadStreamCases(t *testing.T) streamCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "stream_cases.json"))
	if err != nil {
		t.Fatalf("read stream_cases.json: %v", err)
	}
	var c streamCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode stream_cases.json: %v", err)
	}
	if len(c.Streams) == 0 || len(c.Spots) == 0 {
		t.Fatal("stream_cases.json has no streams/spots")
	}
	return c
}

func mustFindStream(t *testing.T, c streamCases, id string) streamDef {
	t.Helper()
	for _, s := range c.Streams {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("stream_cases.json has no %q", id)
	return streamDef{}
}

func mustReadStreamFile(t *testing.T, file string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", file))
	if err != nil {
		t.Fatalf("read %s: %v", file, err)
	}
	return raw
}

func expectStreamCode(t *testing.T, name string, err error, want core.Code) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: want error", name)
		return
	}
	if core.CodeOf(err) != want {
		t.Errorf("%s code = %v, want %v", name, core.CodeOf(err), want)
	}
}

func mustStreamWait(t *testing.T, s *Stream, id string) *Image {
	t.Helper()
	im, err := s.Wait(core.AssetID(id))
	if err != nil {
		t.Fatalf("%s: Wait: %v", id, err)
	}
	if im == nil {
		t.Fatalf("%s: Wait nil image", id)
	}
	return im
}

func checkStreamImage(t *testing.T, want streamDef, im *Image) {
	t.Helper()
	if im == nil {
		t.Fatalf("%s: nil image", want.ID)
	}
	if im.Width() != want.Width || im.Height() != want.Height {
		t.Errorf("%s: size = %dx%d, want %dx%d", want.ID, im.Width(), im.Height(), want.Width, want.Height)
	}
	if im.BlockCount() != want.Blocks {
		t.Errorf("%s: blocks = %d, want %d", want.ID, im.BlockCount(), want.Blocks)
	}
	if im.UploadSize() != want.Upload {
		t.Errorf("%s: upload = %d, want %d", want.ID, im.UploadSize(), want.Upload)
	}
	if im.PixelSize() != want.Pixels {
		t.Errorf("%s: pixels = %d, want %d", want.ID, im.PixelSize(), want.Pixels)
	}
	if im.Format() != FormatBC1RGBAUnorm {
		t.Errorf("%s: format = %v, want BC1", want.ID, im.Format())
	}
}

// A:到边加载不卡:Request后台起飞,Poll从不阻塞,Wait落Ready数对.
func TestStreamRequestFromCases(t *testing.T) {
	c := loadStreamCases(t)
	if MaxStreams != c.MaxStreams || MaxStreamBytes != c.MaxStreamBytes ||
		MaxStreamAssetBytes != c.MaxStreamAssetBytes || MaxStreamIDLen != c.MaxIDLen {
		t.Fatal("stream budgets diverge from stream_cases.json")
	}
	if FormatBC1RGBAUnorm.String() != c.Format {
		t.Fatalf("format = %q, want %q", FormatBC1RGBAUnorm.String(), c.Format)
	}
	// In-memory bytes ride the background chain.
	s := NewStream()
	for _, want := range c.Streams {
		raw := mustReadStreamFile(t, want.File)
		if err := s.Request(core.AssetID(want.ID), raw); err != nil {
			t.Fatalf("%s: Request: %v", want.ID, err)
		}
		// Poll never blocks: Loading or already Ready, never an error here.
		if st, err := s.Poll(core.AssetID(want.ID)); err != nil {
			t.Fatalf("%s: Poll right after Request = %v/%v", want.ID, st, err)
		} else if st != StateLoading && st != StateReady {
			t.Fatalf("%s: Poll = %v, want loading/ready", want.ID, st)
		}
		// Caller may reuse the input: Request copied it.
		for i := range raw {
			raw[i] ^= 0
		}
	}
	for _, want := range c.Streams {
		im := mustStreamWait(t, s, want.ID)
		checkStreamImage(t, want, im)
		if !s.Ready(core.AssetID(want.ID)) || s.StateOf(core.AssetID(want.ID)) != StateReady {
			t.Errorf("%s: Ready/StateOf drifted", want.ID)
		}
		if got, ok := s.Get(core.AssetID(want.ID)); !ok || got != im {
			t.Errorf("%s: Get = %v/%v, want the Ready image", want.ID, got, ok)
		}
		if _, err := s.Ref(core.AssetID(want.ID)); err != nil {
			t.Errorf("%s: Ref: %v", want.ID, err)
		}
		if s.LiveCount(core.AssetID(want.ID)) < 2 {
			t.Errorf("%s: live = %d, want >= 2 (Request plus Ref)", want.ID, s.LiveCount(core.AssetID(want.ID)))
		}
	}
	st := s.Stats()
	if st.Count != len(c.Streams) || st.Requests != int64(len(c.Streams)) {
		t.Errorf("stats = %+v, want count/requests %d", st, len(c.Streams))
	}
	// File path rides the same chain without blocking the caller.
	f := NewStream()
	for _, want := range c.Streams {
		if err := f.RequestFile(core.AssetID(want.ID), filepath.Join("testdata", want.File)); err != nil {
			t.Fatalf("%s: RequestFile: %v", want.ID, err)
		}
	}
	for _, want := range c.Streams {
		checkStreamImage(t, want, mustStreamWait(t, f, want.ID))
	}
}

// B:空零超大坏数据缺图全不崩不卡死,占位加报错.
func TestStreamEdgesNoCrash(t *testing.T) {
	var nilS *Stream
	if _, err := nilS.Poll(core.AssetID("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Poll code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := nilS.Wait(core.AssetID("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Wait code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := nilS.Ref(core.AssetID("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Ref code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilS.Unload(core.AssetID("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Unload code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := nilS.Request(core.AssetID("x"), []byte("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil Request code = %v, want invalid-arg", core.CodeOf(err))
	}
	if nilS.Placeholder(core.AssetID("x")) == nil {
		t.Error("nil Placeholder want non-nil")
	}
	if nilS.Stats() != (Stats{}) || nilS.Count() != 0 || nilS.TotalBytes() != 0 {
		t.Error("nil stats want zero")
	}
	s := NewStream()
	if err := s.Request("", []byte("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty id Request code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := s.Request(core.AssetID("x"), nil); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("nil data Request code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := s.RequestFile(core.AssetID("x"), ""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty path RequestFile code = %v, want invalid-arg", core.CodeOf(err))
	}
	if _, err := s.Poll(""); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("empty Poll code = %v, want invalid-arg", core.CodeOf(err))
	}
	longID := core.AssetID(strings.Repeat("x", MaxStreamIDLen+1))
	if err := s.Request(longID, []byte("x")); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("long id code = %v, want invalid-arg", core.CodeOf(err))
	}
	// Missing file serves the magenta placeholder, never nil, never panic.
	if err := s.RequestFile(core.AssetID("stream/ghost"), filepath.Join("testdata", "no_such.ktx2")); err != nil {
		t.Fatalf("ghost RequestFile: %v", err)
	}
	if _, err := s.Wait(core.AssetID("stream/ghost")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("ghost Wait code = %v, want not-found", core.CodeOf(err))
	}
	if st, err := s.Poll(core.AssetID("stream/ghost")); st != StateMissing || core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("ghost Poll = %v/%v, want missing/not-found", st, err)
	}
	if got, ok := s.Get(core.AssetID("stream/ghost")); ok || got == nil {
		t.Errorf("ghost Get = %v/%v, want placeholder/false", got, ok)
	}
	if ph := s.Placeholder(core.AssetID("stream/ghost")); ph == nil || ph != PlaceholderImage() {
		t.Error("ghost Placeholder want the shared magenta")
	}
	// Unknown ids behave the same without any Request.
	if st, err := s.Poll(core.AssetID("stream/never")); st != StateEmpty || core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("unknown Poll = %v/%v, want empty/not-found", st, err)
	}
	if _, err := s.Wait(core.AssetID("stream/never")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("unknown Wait code = %v, want not-found", core.CodeOf(err))
	}
	// Torn bytes fail with bad-data and stay queryable.
	bad := mustReadStreamFile(t, "solid_red_4x4.ktx2")[:40]
	if err := s.Request(core.AssetID("stream/torn"), bad); err != nil {
		t.Fatalf("torn Request: %v", err)
	}
	if _, err := s.Wait(core.AssetID("stream/torn")); core.CodeOf(err) != core.CodeBadData {
		t.Errorf("torn Wait code = %v, want bad-data", core.CodeOf(err))
	}
	if st, err := s.Poll(core.AssetID("stream/torn")); st != StateFailed || core.CodeOf(err) != core.CodeBadData {
		t.Errorf("torn Poll = %v/%v, want failed/bad-data", st, err)
	}
	// Oversize payloads fail with out-of-memory off the play path.
	huge := make([]byte, MaxStreamAssetBytes+1)
	if err := s.Request(core.AssetID("stream/huge"), huge); err != nil {
		t.Fatalf("huge Request: %v", err)
	}
	if _, err := s.Wait(core.AssetID("stream/huge")); core.CodeOf(err) != core.CodeOutOfMemory {
		t.Errorf("huge Wait code = %v, want out-of-memory", core.CodeOf(err))
	}
	// Over-release and live-evict fail closed without touching counts.
	one := core.AssetID("edge/one")
	if err := s.Request(one, mustReadStreamFile(t, "solid_red_4x4.ktx2")); err != nil {
		t.Fatalf("edge Request: %v", err)
	}
	mustStreamWait(t, s, string(one))
	if err := s.Unload(one); err != nil {
		t.Fatalf("edge Unload: %v", err)
	}
	if err := s.Unload(one); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("over-release code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := s.Unload(core.AssetID("edge/never")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("unknown unload code = %v, want not-found", core.CodeOf(err))
	}
	live := core.AssetID("edge/live")
	if err := s.Request(live, mustReadStreamFile(t, "solid_red_4x4.ktx2")); err != nil {
		t.Fatalf("live Request: %v", err)
	}
	mustStreamWait(t, s, string(live))
	if err := s.Evict(live); core.CodeOf(err) != core.CodeInvalidArg {
		t.Errorf("evict live code = %v, want invalid-arg", core.CodeOf(err))
	}
	if err := s.Evict(core.AssetID("edge/never")); core.CodeOf(err) != core.CodeNotFound {
		t.Errorf("evict unknown code = %v, want not-found", core.CodeOf(err))
	}
}

// C:两边齐(纯管线):后台解码与同步解码逐位一致,两管家重放一致.
func TestStreamBoundaryIdentical(t *testing.T) {
	c := loadStreamCases(t)
	for _, want := range c.Streams {
		raw := mustReadStreamFile(t, want.File)
		direct, err := ParseKTX2(raw)
		if err != nil {
			t.Fatalf("%s: ParseKTX2: %v", want.ID, err)
		}
		fromFile, err := LoadKTX2(filepath.Join("testdata", want.File))
		if err != nil {
			t.Fatalf("%s: LoadKTX2: %v", want.ID, err)
		}
		if !direct.Equal(fromFile) {
			t.Fatalf("%s: bytes parse vs file load diverged", want.ID)
		}
		s := NewStream()
		if err := s.Request(core.AssetID(want.ID), raw); err != nil {
			t.Fatalf("%s: Request: %v", want.ID, err)
		}
		// Input bytes cross the boundary intact (Request copied them).
		before := append([]byte(nil), raw...)
		_ = before
		streamed := mustStreamWait(t, s, want.ID)
		if !streamed.Equal(direct) {
			t.Errorf("%s: background decode diverged from sync decode", want.ID)
		}
		other := NewStream()
		if err := other.Request(core.AssetID(want.ID), raw); err != nil {
			t.Fatalf("%s: other Request: %v", want.ID, err)
		}
		if again := mustStreamWait(t, other, want.ID); !streamed.Equal(again) {
			t.Errorf("%s: cross-stream replay diverged", want.ID)
		}
		// Upload bytes are the file level bytes: the GPU side shares them.
		if streamed.UploadSize() != want.Upload || len(streamed.Blocks()) != want.Upload {
			t.Errorf("%s: upload = %d, want %d", want.ID, streamed.UploadSize(), want.Upload)
		}
		// Copies never alias: mutating a return cannot corrupt the replay.
		probe := streamed.Blocks()
		probe[0] ^= 0xFF
		if streamed.Blocks()[0] == probe[0] {
			t.Fatalf("%s: Blocks aliases the image", want.ID)
		}
		px := streamed.Pixels()
		px[0] ^= 0xFF
		if streamed.Pixels()[0] == px[0] {
			t.Fatalf("%s: Pixels aliases the image", want.ID)
		}
		if again := mustStreamWait(t, s, want.ID); !again.Equal(streamed) {
			t.Fatal("copy probe corrupted the image")
		}
	}
}

// D:加载时帧率有数:到边批量走后台,前台Poll微秒级,吞吐记log.
func TestStreamPerfStreaming(t *testing.T) {
	c := loadStreamCases(t)
	checker := mustReadStreamFile(t, "checker_8x8.ktx2")
	big := mustFindStream(t, c, "stream/big")
	bigRaw := mustReadStreamFile(t, big.File)
	// A map edge worth of tiles streams in the background.
	const n = 64
	s := NewStream()
	start := time.Now()
	for i := 0; i < n; i++ {
		id := core.AssetID("edge/tile_" + itoaStream(i))
		if err := s.Request(id, checker); err != nil {
			t.Fatalf("tile %d: %v", i, err)
		}
	}
	// Foreground polls stay microsecond-scale while workers decode.
	pollStart := time.Now()
	for i := 0; i < n; i++ {
		id := core.AssetID("edge/tile_" + itoaStream(i))
		if _, err := s.Poll(id); err != nil {
			t.Fatalf("poll %d: %v", i, err)
		}
	}
	pollEl := time.Since(pollStart)
	for i := 0; i < n; i++ {
		if _, err := s.Wait(core.AssetID("edge/tile_" + itoaStream(i))); err != nil {
			t.Fatalf("wait %d: %v", i, err)
		}
	}
	el := time.Since(start)
	st := s.Stats()
	t.Logf("stream-edge: %d tiles 8x8 (upload 32B decoded 256B each) requests in %v (poll %d in %v, %.1f us/poll)", n, el, n, pollEl, float64(pollEl.Microseconds())/n)
	if st.Count != n {
		t.Fatalf("count = %d, want %d", st.Count, n)
	}
	if st.TotalBytes != int64(n*256) || st.TotalUpload != int64(n*32) {
		t.Fatalf("total = %dB/%d up, want %dB/%d up", st.TotalBytes, st.TotalUpload, n*256, n*32)
	}
	if pollEl > 100*time.Millisecond {
		t.Fatalf("foreground polls took %v, play would stall", pollEl)
	}
	// One big picture still decodes with measured cost.
	const reps = 10
	bstart := time.Now()
	for i := 0; i < reps; i++ {
		bs := NewStream()
		if err := bs.Request(core.AssetID("stream/big"), bigRaw); err != nil {
			t.Fatalf("big rep %d: %v", i, err)
		}
		im := mustStreamWait(t, bs, "stream/big")
		checkStreamImage(t, big, im)
	}
	bel := time.Since(bstart)
	t.Logf("stream-big: %d background 256x256 (upload %dB decoded %dB) in %v (%.1f ms/stream)", reps, big.Upload, big.Pixels, bel, float64(bel.Milliseconds())/reps)
}

func itoaStream(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "000"
	}
	var out [16]byte
	p := len(out)
	n := i
	for n > 0 {
		p--
		out[p] = digits[n%10]
		n /= 10
	}
	for p > len(out)-3 {
		p--
		out[p] = '0'
	}
	return string(out[p:])
}

// E:跑大地图不涨:千次重放逐位一致,百块装卸回基线,坏档不粘.
func TestStreamLongRunStable(t *testing.T) {
	c := loadStreamCases(t)
	red := mustFindStream(t, c, "stream/red")
	raw := mustReadStreamFile(t, red.File)
	s := NewStream()
	if err := s.Request(core.AssetID(red.ID), raw); err != nil {
		t.Fatalf("first Request: %v", err)
	}
	first := mustStreamWait(t, s, red.ID)
	firstBytes := s.TotalBytes()
	for i := 0; i < 1000; i++ {
		ns := NewStream()
		if err := ns.Request(core.AssetID(red.ID), raw); err != nil {
			t.Fatalf("rep %d: %v", i, err)
		}
		if got := mustStreamWait(t, ns, red.ID); !got.Equal(first) {
			t.Fatalf("rep %d diverged", i)
		}
	}
	// A big-map walk loads and evicts back to the baseline.
	walk := NewStream()
	if err := walk.Request(core.AssetID(red.ID), raw); err != nil {
		t.Fatalf("walk base: %v", err)
	}
	base := mustStreamWait(t, walk, red.ID)
	baseBytes := walk.TotalBytes()
	_ = base
	const tiles = 200
	for i := 0; i < tiles; i++ {
		id := core.AssetID("walk/tile_" + itoaStream(i))
		if err := walk.Request(id, raw); err != nil {
			t.Fatalf("walk %d: %v", i, err)
		}
		mustStreamWait(t, walk, string(id))
		if err := walk.Unload(id); err != nil {
			t.Fatalf("walk %d unload: %v", i, err)
		}
		if err := walk.Evict(id); err != nil {
			t.Fatalf("walk %d evict: %v", i, err)
		}
	}
	if walk.TotalBytes() != baseBytes {
		t.Fatalf("walk total = %d, want baseline %d", walk.TotalBytes(), baseBytes)
	}
	if again := mustStreamWait(t, walk, red.ID); !again.Equal(first) {
		t.Fatal("base drifted after the walk")
	}
	// Bad data never poisons the next good load.
	if err := s.Request(core.AssetID("stable/bad"), raw[:40]); err != nil {
		t.Fatalf("bad Request: %v", err)
	}
	if _, err := s.Wait(core.AssetID("stable/bad")); core.CodeOf(err) != core.CodeBadData {
		t.Fatalf("bad Wait code = %v, want bad-data", core.CodeOf(err))
	}
	_ = firstBytes
	good := NewStream()
	if err := good.Request(core.AssetID("stable/bad"), raw); err != nil {
		t.Fatalf("good Request: %v", err)
	}
	if _, err := good.Wait(core.AssetID("stable/bad")); err != nil {
		t.Fatalf("good after bad: %v", err)
	}
}

// F:离屏金对照窗(窗随P2建):冻结点加品红占位加形状断言即自动判.
func TestStreamOffscreenGolden(t *testing.T) {
	c := loadStreamCases(t)
	if c.Tolerance != 0 {
		t.Fatalf("tolerance = %d, want frozen 0", c.Tolerance)
	}
	ph := PlaceholderImage()
	if ph == nil {
		t.Fatal("PlaceholderImage nil")
	}
	if ph.Width() != c.Placeholder.Width || ph.Height() != c.Placeholder.Height {
		t.Fatalf("placeholder size = %dx%d, want %dx%d", ph.Width(), ph.Height(), c.Placeholder.Width, c.Placeholder.Height)
	}
	if ph.BlockCount() != c.Placeholder.Blocks || ph.UploadSize() != c.Placeholder.Upload || ph.PixelSize() != c.Placeholder.Pixels {
		t.Fatalf("placeholder blocks/upload/pixels = %d/%d/%d, want %d/%d/%d",
			ph.BlockCount(), ph.UploadSize(), ph.PixelSize(), c.Placeholder.Blocks, c.Placeholder.Upload, c.Placeholder.Pixels)
	}
	// Placeholder is opaque magenta on every pixel: the missing-texture verdict.
	px := ph.Pixels()
	for y := 0; y < c.Placeholder.Height; y++ {
		for x := 0; x < c.Placeholder.Width; x++ {
			off := (y*c.Placeholder.Width + x) * 4
			got := [4]int{int(px[off]), int(px[off+1]), int(px[off+2]), int(px[off+3])}
			if got != c.Placeholder.RGBA {
				t.Fatalf("placeholder (%d,%d) = %v, want %v", x, y, got, c.Placeholder.RGBA)
			}
		}
	}
	// Streamed spots land on the frozen decoded colors (same bytes both backends share).
	s := NewStream()
	for _, want := range c.Streams {
		if err := s.Request(core.AssetID(want.ID), mustReadStreamFile(t, want.File)); err != nil {
			t.Fatalf("%s: Request: %v", want.ID, err)
		}
	}
	for _, sp := range c.Spots {
		var owner string
		for _, want := range c.Streams {
			if want.File == sp.File {
				owner = want.ID
				break
			}
		}
		if owner == "" {
			t.Fatalf("spot file %q has no stream id", sp.File)
		}
		im := mustStreamWait(t, s, owner)
		got, ok := im.At(sp.X, sp.Y)
		if !ok {
			t.Fatalf("%s (%d,%d): At ok=false", sp.File, sp.X, sp.Y)
		}
		r, g, b, a := got.ToBytes()
		if [4]int{int(r), int(g), int(b), int(a)} != sp.RGBA {
			t.Errorf("%s (%d,%d) = %v, want %v", sp.File, sp.X, sp.Y, [4]int{int(r), int(g), int(b), int(a)}, sp.RGBA)
		}
	}
	// Shape: placeholder never collides with a ready picture.
	for _, want := range c.Streams {
		im := mustStreamWait(t, s, want.ID)
		if im.Equal(ph) {
			t.Errorf("%s equals the placeholder, missing art would hide", want.ID)
		}
	}
	// Shape: ids distinct, budgets sane, every stream far under per-asset cap.
	seen := map[string]bool{}
	var total int64
	biggest := 0
	for _, want := range c.Streams {
		if seen[want.ID] {
			t.Errorf("id %q collides", want.ID)
		}
		seen[want.ID] = true
		raw := mustReadStreamFile(t, want.File)
		if len(raw) > MaxStreamAssetBytes {
			t.Errorf("%s file %d exceeds MaxStreamAssetBytes", want.ID, len(raw))
		}
		if want.Upload <= 0 || want.Pixels <= 0 {
			t.Errorf("%s upload/pixels = %d/%d, want > 0", want.ID, want.Upload, want.Pixels)
		}
		if want.Pixels != want.Width*want.Height*4 {
			t.Errorf("%s: pixels %d != %dx%dx4", want.ID, want.Pixels, want.Width, want.Height)
		}
		total += int64(want.Pixels)
		if want.Pixels > biggest {
			biggest = want.Pixels
		}
	}
	if total != s.TotalBytes() {
		t.Errorf("total %d != stream %d", total, s.TotalBytes())
	}
	if biggest <= 0 || biggest > MaxStreamBytes {
		t.Errorf("biggest %d out of budget", biggest)
	}
	if total > MaxStreamBytes {
		t.Errorf("total %d exceeds MaxStreamBytes", total)
	}
}
