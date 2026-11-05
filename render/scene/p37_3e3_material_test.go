//go:build !nogpu

package scene

// P3.7 批 3e-3 包内测：item 级材质编码（TagMaterial）+ 合并镜像 + CPU 瓦片腿表态。
//
// 判据正本 `evidence/P3.7/batch3e3-design.md` §1/§3。Godot 对标
// `canvas_item_set_material`（item 级 = 流内粘滞状态，item 的命令在流内连续）。
//
// 回放腿「材质有没有交到每一笔手上」钉在包外测 `p37_3e3_material_gpu_test.go`
// （Renderer.Render 一有加速器就走 GPU 优先腿，外部够不到瓦片执行器——分工同 3e-1/3e-2）。

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/render"
)

func tagsEqual(a, b []Tag) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// p37E3Effect 编一枚最小合法材质（fs_main 参数是 (uv,tint,hom_w) 前缀的一位）。
func p37E3Effect(t *testing.T, red string) *render.ShaderEffect {
	t.Helper()
	e, err := render.NewShaderEffect(fmt.Sprintf(`@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(%s, 0.0, 0.0, 1.0);
}`, red))
	if err != nil {
		t.Fatalf("材质编译失败: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

// p37E3FlatScene 是一块红矩形：装材质与否几何完全同形，Hash 差只能来自材质。
func p37E3FlatScene() *Scene {
	s := NewScene()
	s.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{R: 1, A: 1}), NewRectShape(4, 4, 24, 16))
	return s
}

func countMaterialTags(enc *Encoding) int {
	n := 0
	dec := NewDecoder(enc)
	for dec.Next() {
		if dec.Tag() == TagMaterial {
			n++
		}
	}
	return n
}

// readMaterialSwitches 按流序回读材质切换（payload 直接取 materialEntries 镜像流）。
// 手写「只消费材质 tag」的解码走查是不成立的：Decoder 每读一枚 tag 都要按该 tag
// 的载荷宽度前进 drawIdx，回放腿（GPU/CPU）才是在线序上正确消费的——材质载荷的
// 逐位正确性由包外测的像素/计数钉负责，这里只断言编码侧写进去的镜像流。
func readMaterialSwitches(t *testing.T, s *Scene) []*render.ShaderEffect {
	t.Helper()
	reg := s.Materials()
	var got []*render.ShaderEffect
	for i, idx := range s.Encoding().materialEntries {
		if idx == ^uint32(0) {
			got = append(got, nil)
			continue
		}
		if idx >= uint32(len(reg)) {
			t.Fatalf("材质索引 %d 越出注册表（%d 枚）", idx, len(reg))
		}
		got = append(got, reg[idx])
		if i < len(s.Encoding().materialEffects) && s.Encoding().materialEffects[i] != reg[idx] {
			t.Fatalf("镜像与注册表指不同枚：%d → effects %+v entries %+v", i, s.Encoding().materialEffects, s.Encoding().materialEntries)
		}
	}
	return got
}

// materialIndexes 按流序回读材质载荷镜像列表（编码侧写进去的那份）。
func materialIndexes(t *testing.T, enc *Encoding) []uint32 {
	t.Helper()
	return append([]uint32(nil), enc.materialEntries...)
}

// materialPayloadFromStream 完整走查载荷流，读出回放腿真正消费的材质索引（drawData）。
// 只看编码镜像是不够的：回放按每枚 tag 的载荷宽度前进 drawIdx，合并处的抬位落在
// drawData 上，镜像只是副本。未登记的 tag 直接判失败，防止走查悄悄错位。
func materialPayloadFromStream(t *testing.T, enc *Encoding) []uint32 {
	t.Helper()
	dec := NewDecoder(enc)
	var got []uint32
	for dec.Next() {
		switch dec.Tag() {
		case TagMaterial:
			idx, _ := dec.Material()
			got = append(got, idx)
		case TagFill:
			dec.Fill()
		case TagStroke:
			dec.Stroke()
		case TagFillRoundRect:
			dec.FillRoundRect()
		case TagImage:
			dec.Image()
		case TagImageQuad:
			dec.ImageQuad()
		case TagPushLayer:
			dec.PushLayer()
		case TagSetAntiAlias:
			dec.AntiAlias()
		case TagProjection:
			dec.Projection()
		case TagTransform:
			dec.Transform()
		case TagText:
			dec.Text()
		case TagBeginPath, TagMoveTo, TagLineTo, TagQuadTo, TagCubicTo,
			TagClosePath, TagEndPath, TagPopLayer, TagBeginClip, TagEndClip, TagBrush:
			// 不占 drawData 槽（几何在 pathData、变换在 transforms、刷子在 brushes 独立流）。
		default:
			t.Fatalf("tag %v 没登记载荷宽度：这条走查会错位，请补 case", dec.Tag())
		}
	}
	return got
}

// 钉 1：恒等零发射——不调 SetMaterial（或 nil→nil）的存量场景，tag 流与 Hash 逐位不变。
func TestP37E3ZeroEmissionWithoutMaterial(t *testing.T) {
	base := p37E3FlatScene()
	withNil := p37E3FlatScene()
	withNil.SetMaterial(nil) // nil→nil 同一位 ⇒ no-op

	if n := countMaterialTags(base.Encoding()); n != 0 {
		t.Fatalf("没装材质的场景发了 %d 枚 TagMaterial", n)
	}
	if n := countMaterialTags(withNil.Encoding()); n != 0 {
		t.Fatalf("nil→nil 不该发射，发了 %d 枚", n)
	}
	if !tagsEqual(base.Encoding().Tags(), withNil.Encoding().Tags()) {
		t.Fatal("nil→nil 改了 tag 流：存量场景的编码字节不再逐位稳定")
	}
	if base.Encoding().Hash() != withNil.Encoding().Hash() {
		t.Fatal("nil→nil 改了 Hash：delta 发射失效，存量 LayerCache 键会漂")
	}
	if len(base.Materials()) != 0 {
		t.Fatalf("没装材质却进了注册表：%d 枚", len(base.Materials()))
	}
}

// 钉 2：编码回环 + delta 去重 + 清除载荷 + 注册表指针去重 + 镜像不变量。
func TestP37E3EncodingRoundTrip(t *testing.T) {
	eA := p37E3Effect(t, "0.9")
	eB := p37E3Effect(t, "0.1")
	if eA.ID() == eB.ID() {
		t.Fatal("两枚未变异的材质 ID 撞了：LayerCache 键会串")
	}

	s := NewScene()
	s.SetMaterial(eA)
	s.SetMaterial(eB)
	s.SetMaterial(eB) // delta 去重：同 effect 不重发
	if n := countMaterialTags(s.Encoding()); n != 2 {
		t.Fatalf("材质切换应 2 枚，实得 %d", n)
	}
	switches := readMaterialSwitches(t, s)
	if len(switches) != 2 || switches[0] != eA || switches[1] != eB {
		t.Fatalf("索引回环不齐：%+v want [eA eB]", switches)
	}
	reg := s.Materials()
	if len(reg) != 2 || reg[0] != eA || reg[1] != eB {
		t.Fatalf("注册表指针去重失败：%d 枚 %+v", len(reg), reg)
	}

	// 清除载荷：SetMaterial(nil) 发 0xFFFFFFFF，解码成 nil 位。
	s.SetMaterial(nil)
	switches = readMaterialSwitches(t, s)
	if last := switches[len(switches)-1]; last != nil {
		t.Fatal("SetMaterial(nil) 没编出清除载荷")
	}
	// 清除后再装同一枚：粘滞位真变了，必须重发。
	before := countMaterialTags(s.Encoding())
	s.SetMaterial(eB)
	if got := countMaterialTags(s.Encoding()); got != before+1 {
		t.Fatalf("清除后重装 eB 应重发一枚：%d → %d", before, got)
	}
	if len(s.Materials()) != 2 {
		t.Fatalf("重装同一枚又新增注册表项：%d（指针去重失效）", len(s.Materials()))
	}

	// 镜像不变量：materialEntries 与 materialEffects 一格一对（Hash 按位配对）。
	enc := s.Encoding()
	if len(enc.materialEntries) != len(enc.materialEffects) {
		t.Fatalf("材质镜像流长度不齐：entries=%d effects=%d",
			len(enc.materialEntries), len(enc.materialEffects))
	}
}

// 钉 2b：Hash 吞 id^(gen<<1)——材质进键、状态不变不漂、uniform 改动后必变（E5b 同族防线）。
func TestP37E3HashMixesMaterialIdentityAndGen(t *testing.T) {
	eA := p37E3Effect(t, "0.9")
	eB := p37E3Effect(t, "0.1")

	flat := p37E3FlatScene()
	ma := p37E3FlatScene()
	ma.SetMaterial(eA)
	mb := p37E3FlatScene()
	mb.SetMaterial(eB)
	if flat.Encoding().Hash() == ma.Encoding().Hash() {
		t.Fatal("材质没进 Hash：LayerCache 会把装材质与不装材质的场景串成同一份缓存像素")
	}
	if ma.Encoding().Hash() == mb.Encoding().Hash() {
		t.Fatal("两枚不同材质撞键：不可变 ID 没进 Hash")
	}

	// 只重读不改状态 ⇒ Hash 不许漂。
	h := ma.Encoding().Hash()
	_ = ma.Encoding().Hash()
	if ma.Encoding().Hash() != h {
		t.Fatalf("无状态变化 Hash 漂了：%d → %d", h, ma.Encoding().Hash())
	}
	// uniform 改动（gen bump）⇒ Hash 必变：录制之后改材质，缓存必须失效。
	if eA.Gen() != 0 {
		t.Fatalf("新建材质的初始代数应为 0，实得 %d", eA.Gen())
	}
	if err := eA.SetUniformBytes([]byte{9, 9, 9, 9}); err != nil {
		t.Fatal(err)
	}
	if eA.Gen() == 0 {
		t.Fatal("SetUniformBytes 没 bump 代数")
	}
	if ma.Encoding().Hash() == h {
		t.Fatal("uniform 改动没进 Hash：LayerCache 会返回陈旧像素（E5b 同族）")
	}
	// SetImageAt 同样要 bump 代数。
	genBefore := eA.Gen()
	if err := eA.SetImageAt(0, nil); err == nil && eA.Gen() == genBefore {
		t.Fatal("SetImageAt 没 bump 代数")
	}
}

// 钉 4：合并两路——注册表并集、TagMaterial 照抄不丢、索引抬位只作用子流、
// 材质镜像必须并进 Hash。Append 走 AppendWithImages，AppendWithTranslation
// 有自己的 switch（漏 case = default panic 绊线，直接炸在那一次调用上）。
func TestP37E3MergeCarriesMaterials(t *testing.T) {
	eHost := p37E3Effect(t, "0.7")
	eChild := p37E3Effect(t, "0.9")
	mutation := 0

	for _, tc := range []struct {
		name string
		run  func(host, child *Scene)
	}{
		{"append_with_images", func(host, child *Scene) { host.Append(child) }},
		{"append_with_translation", func(host, child *Scene) { host.AppendWithTranslation(child, 4, 4) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			host := NewScene()
			host.SetMaterial(eHost)
			host.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{G: 1, A: 1}), NewRectShape(0, 0, 8, 8))
			child := NewScene()
			child.SetMaterial(eChild)
			child.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{B: 1, A: 1}), NewRectShape(8, 8, 8, 8))

			hostTags := len(host.Encoding().Tags())
			childTags := len(child.Encoding().Tags())
			tc.run(host, child)
			if got := len(host.Encoding().Tags()); got != hostTags+childTags {
				t.Fatalf("合并后 tag 数 %d，want %d（照抄丢了 tag）", got, hostTags+childTags)
			}
			if reg := host.Materials(); len(reg) != 2 || reg[0] != eHost || reg[1] != eChild {
				t.Fatalf("材质注册表并集不齐：%+v", reg)
			}
			// host 自己那枚不抬，child 那枚抬一位——两遍都要对：编码镜像与回放消费的载荷流。
			wantIdx := []uint32{0, 1}
			if idx := materialIndexes(t, host.Encoding()); !uint32Equal(idx, wantIdx) {
				t.Fatalf("合并后的材质镜像应 %v，实得 %v", wantIdx, idx)
			}
			if idx := materialPayloadFromStream(t, host.Encoding()); !uint32Equal(idx, wantIdx) {
				t.Fatalf("回放真正消费的材质载荷索引应 %v，实得 %v（drawData 没抬位）", wantIdx, idx)
			}
			enc := host.Encoding()
			if len(enc.materialEntries) != len(enc.materialEffects) {
				t.Fatalf("合并的镜像流长度不齐：entries=%d effects=%d",
					len(enc.materialEntries), len(enc.materialEffects))
			}
			// 子材质的 id^gen 必须并进 host 的 Hash，否则合并场景的缓存永不失效。
			mutation++
			h := enc.Hash()
			if err := eChild.SetUniformBytes([]byte{byte(mutation), 2, 3, 4}); err != nil {
				t.Fatal(err)
			}
			if enc.Hash() == h {
				t.Fatal("合并进来的材质没进 Hash：materialEffects 镜像在合并处丢了，LayerCache 会返陈旧像素")
			}
		})
	}
}

// 合并的清除载荷必须原样透传（抬索引不许把 0xFFFFFFFF 抬成一个大号索引）。
func TestP37E3MergePassesClearPayload(t *testing.T) {
	e := p37E3Effect(t, "0.9")
	child := NewScene()
	child.SetMaterial(e)
	child.SetMaterial(nil) // 清除
	child.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{B: 1, A: 1}), NewRectShape(8, 8, 8, 8))

	host := NewScene()
	host.SetMaterial(p37E3Effect(t, "0.1")) // 占一位 ⇒ materialOffset=1
	host.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{G: 1, A: 1}), NewRectShape(0, 0, 4, 4))
	host.Append(child)

	idx := materialIndexes(t, host.Encoding())
	if len(idx) != 3 {
		t.Fatalf("合并后应有 3 枚切换，实得 %v", idx)
	}
	if idx[1] != 1 {
		t.Fatalf("child 的材质索引没抬：%d want 1", idx[1])
	}
	if idx[2] != ^uint32(0) {
		t.Fatalf("清除载荷被抬成 %d：透传守卫丢了，回放会把清除读成一枚越界材质", idx[2])
	}
	// 回放消费的载荷流同规格：抬位只作用于真索引，清除位原样过。
	if got := materialPayloadFromStream(t, host.Encoding()); !uint32Equal(got, idx) {
		t.Fatalf("编码镜像与回放消费的载荷流不齐：镜像 %v 载荷 %v", idx, got)
	}
}

func uint32Equal(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// 钉 5：CPU 瓦片腿遇材质整笔拒画 + 具名计数（绝不裸画）；无材质对照组必画。
func TestP37E3CPUTileMaterialRefusesAndCounts(t *testing.T) {
	img := cpuQuadImage()
	e := p37E3Effect(t, "0.9")
	corners := cpuParallelogramCorners()

	build := func(withMaterial bool) *Scene {
		s := NewScene()
		if withMaterial {
			s.SetMaterial(e)
		}
		s.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{R: 1, A: 1}), NewRectShape(4, 4, 24, 16))
		s.Stroke(DefaultStrokeStyle(), IdentityAffine(), SolidBrush(render.RGBA{R: 1, A: 1}), NewRectShape(2, 2, 30, 20))
		s.DrawImage(img, IdentityAffine())
		s.DrawImageQuad(img, corners, render.QuadDrawOptions{Opacity: 1})
		return s
	}

	r := NewRenderer(64, 64, WithWorkers(1))
	if painted := cpuCanvasPainted(cpuCanvasInto(t, r, build(true))); painted != 0 {
		t.Fatalf("材质下的 CPU 瓦片腿画出了 %d 格：裸画（P3.2 无 CPU 求值器，禁止静默画）", painted)
	}
	if got := r.MaterialSkips(); got == 0 {
		t.Fatal("CPU 腿拒画没计数：表态丢了（AGENTS 禁止静默降级）")
	} else {
		t.Logf("material refusals = %d", got)
	}

	// 对照组：同一份几何、材质为空 ⇒ 必须画得出，且一格不计。
	rc := NewRenderer(64, 64, WithWorkers(1))
	if painted := cpuCanvasPainted(cpuCanvasInto(t, rc, build(false))); painted == 0 {
		t.Fatal("对照组（无材质）什么都没画：这条鉴别不成立")
	}
	if got := rc.MaterialSkips(); got != 0 {
		t.Fatalf("无材质场景被计数了：%d", got)
	}

	rc.ResetMaterialSkips()
	if got := rc.MaterialSkips(); got != 0 {
		t.Fatalf("ResetMaterialSkips 没清零：%d", got)
	}
	// 同一份场景两轮计数必须相等（可复位复量）。
	rr := NewRenderer(64, 64, WithWorkers(1))
	cpuCanvasInto(t, rr, build(true))
	first := rr.MaterialSkips()
	if first == 0 {
		t.Fatal("复用的 Renderer 第一次没计数")
	}
	rr.ResetMaterialSkips()
	cpuCanvasInto(t, rr, build(true))
	if got := rr.MaterialSkips(); got != first {
		t.Fatalf("同一份场景两轮计数不齐：%d want %d", got, first)
	}
}

// 粘滞跨层：材质装进 PushLayer/PopLayer 之间不许在层边界复位（与 TagSetAntiAlias 同族；
// 真源 batch3e3-design.md §1「跨层粘滞，layer 边界不复位，文档写死」）。
func TestP37E3MaterialStickyAcrossLayers(t *testing.T) {
	e := p37E3Effect(t, "0.9")
	s := NewScene()
	s.SetMaterial(e)
	s.PushLayer(BlendNormal, 1, nil)
	s.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{R: 1, A: 1}), NewRectShape(4, 4, 8, 8))
	s.PopLayer()
	s.Fill(FillNonZero, IdentityAffine(), SolidBrush(render.RGBA{G: 1, A: 1}), NewRectShape(20, 20, 8, 8))

	if n := countMaterialTags(s.Encoding()); n != 1 {
		t.Fatalf("跨层粘滞应当只发一枚 TagMaterial，实得 %d", n)
	}
	r := NewRenderer(64, 64, WithWorkers(1))
	if painted := cpuCanvasPainted(cpuCanvasInto(t, r, s)); painted != 0 {
		t.Fatalf("跨层材质下的 CPU 腿画出了 %d 格：粘滞位在某处被吞了", painted)
	}
	if got := r.MaterialSkips(); got < 2 {
		t.Fatalf("层内与层后各一笔都该拒画计数，实得 %d：粘滞在某处断了", got)
	}
}

func cpuParallelogramCorners() [4]render.Point {
	return [4]render.Point{
		{X: 10, Y: 8}, {X: 34, Y: 8}, {X: 28, Y: 32}, {X: 4, Y: 32},
	}
}
