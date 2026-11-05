// Command p37_height_zsort is P3.7 batch 6 window ②: height & z-order.
//
// 必画五臂（钉版清单 batch6-window-plan.md §1 ② 行，缺一即台账 743 仍 ☑）：
//
//	a) YSort25D z 桶分组与 ±4096 CLAMP：ZIndex 桶大者盖小者（与 y 无关），
//	   越界值经 camera.ClampZIndex 饱和；z 槽指派技巧往返（y 序 → YSortZ 槽
//	   → 再按槽排序 = 同序）；
//	b) 层内键按 is_equal_approx 判等 + DFS 破平票：y 差出容差由 y 定序；
//	   y 落进容差带由提交序号破平票，且平票序跨帧不闪（窗内逐帧重排序断言）；
//	c) Transform.Height 经世界折叠（纯求和）与 Basis25D 投影改变盖序——
//	   排序键仍是投影脚位 Y（D-D 口径：高度不进排序键公式）；LandShadow
//	   落影子、ShadowVisible 判可见；
//	d) 非有限 fail-closed：树路 InvalidArg、sprite.NewItem InvalidArg、
//	   超限 OutOfMemory、平铺 YSort 不校验（批 2 钉下的口径，如实记）、
//	   LandShadow 地下目标隐藏——拒绝态在画面底带可见；
//	e) 跨 tier 提交序=盖序不变式：路径填充（SDF tier）与精灵（image tier）
//	   交错提交，交叠处 = 最后提交者，与 tier 无关。
//
// 满窗即内容：指标行浮在画面左上（金图 mask 掉 y<32 带）；B1 带随相位横移
// （画面真在动），B2 平票对输入恒定（平票不闪的承重载体）；画面相位只随
// 已完成帧数走、过 settling 帧后定格（H15）。金图按后端分文件
// goldens/p37_height_zsort_final_{native,go}.png 且 fail-closed（缺基线判红，
// 播种那一跑也不算过）。像素断言坐标由「画面所用同一份排序/投影输出」反推
// （P3.5 教训）。
//
// 跑法：
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=8 go run ./examples/engine/p37_height_zsort -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=8 go run ./examples/engine/p37_height_zsort -auto-only  # 播种，仍不算过
//	go run ./examples/engine/p37_height_zsort                                                # 常驻人工
//	GPUI_BACKEND=go RUN_SECONDS=8 go run ./examples/engine/p37_height_zsort -auto-only       # 第二后端一轮
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/engine/camera"
	"github.com/energye/gpui/engine/core"
	"github.com/energye/gpui/engine/sprite"
	"github.com/energye/gpui/engine/world"
	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200.0, 800.0
	metricStripH = 32.0
	winTitle     = "gpui p37_height_zsort — 高度与 z 序"

	// A 臂 z 桶 CLAMP：四只精灵同脚位，ZIndex 越界饱和，桶大者盖小者。
	aFirstX = 150.0
	aStepX  = 64.0
	aW      = 96.0
	aH      = 72.0
	aFootY  = 150.0

	// B 臂：B1 y 键（随相位横移）；B2 平票（输入恒定）。
	b1NearX = 680.0 // 脚位更低（更近）的那只
	b1FarX  = 726.0
	bW      = 90.0
	bH      = 64.0
	b1NearY = 308.0
	b1FarY  = 268.0
	b2AX    = 880.0 // Order 0
	b2BX    = 926.0 // Order 1
	b2FootY = 396.0
	b2Dy    = 1e-9 // 出判等带之外的平票：|Δ| < tol(=1e-5·|y|)

	// C 臂 Height：同世界 (x,y) 三档折叠高度，Basis45 投影出脚位阶梯。
	cOriginX, cOriginY = 140.0, 560.0
	cWorldX, cWorldY   = 3.0, 2.0
	cRootHeight        = 1.0 // 平台高度（折叠：父 + 子 纯求和）
	cW                 = 88.0
	cH                 = 60.0
	cShadowW           = 120.0
	cShadowH           = 14.0
	cShadowOpacity     = 0.35

	// E 臂跨 tier：填充（SDF tier）与精灵（image tier）交错，交叠=最后提交者。
	eFirstX = 140.0
	eStepX  = 74.0
	eW      = 110.0
	eH      = 54.0
	eTopY   = 620.0

	// D 拒绝态底带。
	refusalY = 712.0
	chipW    = 16.0
	chipStep = 44.0
	chipN    = 8

	// H15：定帧停机 + 过 settling 帧定格。
	gateFrames   = 240
	gateFreezeAt = 120
	gateHz       = 60.0

	// 面板底色与影子：探针期望由同一份常数算出（不从屏幕反推结论）。
	backdropRGB = 0.055

	// 容差（写死，不设暗档）：图集腿最近邻 + 整数对齐 ⇒ 直色探针零容差起，
	// GPU 预乘/混合路径给六档；影子是 alpha 混合，同样六档。
	directTol   = 2.0
	blendTol    = 6.0
	bitExactWan = 0
)

// cChildHeights：三个孩子的本地高度——折叠（父 1.0 纯求和）后世界高 {0,1,2}。
var cChildHeights = [3]float64{-1.0, 0.0, 1.0}

// shadowTileRGB：影子块色（配 Opacity 用）——探针期望由同一份常数算出。
var shadowTileRGB = [3]uint8{40, 44, 52}

// 八块色板（4×2 图集）：探针期望色 = 这里写死的同一份常数。
var tileRGB = [8][3]uint8{
	{80, 130, 240}, // t0 blue
	{60, 200, 190}, // t1 teal
	{230, 180, 70}, // t2 amber
	{235, 90, 60},  // t3 red
	{160, 90, 235}, // t4 purple
	{240, 140, 60}, // t5 orange
	{70, 200, 120}, // t6 green
	{220, 80, 180}, // t7 magenta
}

const (
	tileSize   = 64.0
	tileShadow = 7 // 底带影子块（配 Opacity 用）
)

var testdataDir = filepath.Join("examples", "engine", "p37_height_zsort", "testdata")

// zsp 是一只被排序的精灵载体：item 喂给 camera.YSort25D，dst 是它在窗里的
// 落点（C 臂的 dst 由 Basis25D 投影输出反推，不手写屏幕常数）。
type zsp struct {
	name       string
	item       camera.YSortItem
	dstX, dstY float64
	w, h       float64
	tile       int
}

func (z zsp) atlas() render.AtlasSprite {
	return render.AtlasSprite{
		SrcX: float64(z.tile%4) * tileSize, SrcY: float64(z.tile/4) * tileSize,
		SrcW: tileSize, SrcH: tileSize,
		DstX: z.dstX, DstY: z.dstY, DstW: z.w, DstH: z.h,
		Filter: render.InterpNearest,
	}
}

func mul8(v, s uint8) uint8 { return uint8(uint16(v) * uint16(s) / 255) }

// newSheetTexture 生成图集（4×3，每块 64px）：第 0–1 行是八块色板，
// 第 2 行第 0 块是影子暗块（配 Opacity 用）。
func newSheetTexture() *render.ImageBuf {
	buf, err := render.NewImageBuf(int(tileSize)*4, int(tileSize)*3, render.FormatRGBA8)
	if err != nil {
		panic(fmt.Sprintf("p37_height_zsort: sheet: %v", err))
	}
	for ty := 0; ty < 2; ty++ {
		for tx := 0; tx < 4; tx++ {
			c := tileRGB[ty*4+tx]
			for y := 0; y < int(tileSize); y++ {
				for x := 0; x < int(tileSize); x++ {
					if err := buf.SetRGBA(tx*int(tileSize)+x, ty*int(tileSize)+y, c[0], c[1], c[2], 255); err != nil {
						panic(fmt.Sprintf("p37_height_zsort: sheet pixel: %v", err))
					}
				}
			}
		}
	}
	for y := 0; y < int(tileSize); y++ {
		for x := 0; x < int(tileSize); x++ {
			if err := buf.SetRGBA(x, 2*int(tileSize)+y, shadowTileRGB[0], shadowTileRGB[1], shadowTileRGB[2], 255); err != nil {
				panic(fmt.Sprintf("p37_height_zsort: sheet shadow pixel: %v", err))
			}
		}
	}
	return buf
}

// sortedSprites 把载体按 camera.YSort25D 排成绘制序（远先近后）。
// Order 字段就是载体下标，排序结果直接映射回 dst。
func sortedSprites(zs []zsp) ([]render.AtlasSprite, []int, error) {
	items := make([]camera.YSortItem, len(zs))
	for i := range zs {
		items[i] = zs[i].item
	}
	got, err := camera.YSort(items)
	if err != nil {
		return nil, nil, err
	}
	out := make([]render.AtlasSprite, 0, len(zs))
	order := make([]int, 0, len(zs))
	for _, it := range got {
		out = append(out, zs[it.Order].atlas())
		order = append(order, it.Order)
	}
	return out, order, nil
}

// paintBackdrop 铺满窗底色（探针期望色的底）。
func paintBackdrop(dc render.Canvas, w, h float64) {
	dc.SetRGB(backdropRGB, backdropRGB, backdropRGB+0.02)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
}

// fillRect 一笔 SDF tier 路径填充（E 臂用）。
func fillRect(dc render.Canvas, x, y, w, h float64, c [3]uint8) {
	dc.SetRGB(float64(c[0])/255, float64(c[1])/255, float64(c[2])/255)
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

// armA 载体：四只同脚位精灵，ZIndex 故意越界（±5000）。
func armA() []zsp {
	tiles := []int{0, 1, 2, 3}
	zs := []int{-5000, -10, 10, 5000}
	out := make([]zsp, 0, 4)
	for i := 0; i < 4; i++ {
		out = append(out, zsp{
			name: fmt.Sprintf("a%d", i),
			item: camera.YSortItem{
				Pos:   core.V3(aFirstX+float64(i)*aStepX, aFootY, 0),
				Order: i, ZIndex: zs[i],
			},
			dstX: aFirstX + float64(i)*aStepX, dstY: aFootY - aH,
			w: aW, h: aH, tile: tiles[i],
		})
	}
	return out
}

// armB1：y 键定序（近者脚位更低、后画、盖住），随相位横移。
func armB1(pan float64) []zsp {
	return []zsp{
		{name: "b1_near", item: camera.YSortItem{Pos: core.V3(b1NearX+pan, b1NearY, 0), Order: 0},
			dstX: b1NearX + pan, dstY: b1NearY - bH, w: bW, h: bH, tile: 6},
		{name: "b1_far", item: camera.YSortItem{Pos: core.V3(b1FarX+pan, b1FarY, 0), Order: 1},
			dstX: b1FarX + pan, dstY: b1FarY - bH, w: bW, h: bH, tile: 0},
	}
}

// armB2：平票对——两脚位差 1e-9 落进 is_equal_approx 容差带，提交序号破平票。
func armB2() []zsp {
	return []zsp{
		{name: "b2_first", item: camera.YSortItem{Pos: core.V3(b2AX, b2FootY, 0), Order: 0},
			dstX: b2AX, dstY: b2FootY - bH, w: bW, h: bH, tile: 4},
		{name: "b2_second", item: camera.YSortItem{Pos: core.V3(b2BX, b2FootY+b2Dy, 0), Order: 1},
			dstX: b2BX, dstY: b2FootY - bH, w: bW, h: bH, tile: 5},
	}
}

// cCarrier：C 臂的一只——世界折叠高度 + Basis45 投影脚位 + LandShadow 落点。
type cCarrier struct {
	zsp
	worldHeight float64
	foot        core.Vec2 // 屏幕（带 origin 偏移前）投影脚位
	shadow      core.Vec2
	shadowOK    bool
	visible     bool
}

// buildC 把 world 折叠（父 1.0 + 子 {-1,0,1} ⇒ 世界高 {0,1,2}）、Basis45
// 投影、LandShadow 全链跑一遍——画面与探针吃同一份输出。
func buildC() ([]cCarrier, camera.Basis25D) {
	basis, err := camera.NewBasis25D(camera.View45)
	if err != nil {
		panic(fmt.Sprintf("p37_height_zsort: basis: %v", err))
	}
	w := world.NewWorld()
	root, err := w.Spawn(world.NoEntity)
	if err != nil {
		panic(err)
	}
	if err := w.SetTransform(root, world.Transform{Height: cRootHeight}); err != nil {
		panic(err)
	}
	tiles := []int{6, 1, 3} // 绿（矮）/ 青（中）/ 红（高）
	out := make([]cCarrier, 0, 3)
	for i, ch := range cChildHeights {
		id, err := w.Spawn(root)
		if err != nil {
			panic(err)
		}
		if err := w.SetTransform(id, world.Transform{Height: ch}); err != nil {
			panic(err)
		}
		wt, err := w.WorldOf(id)
		if err != nil {
			panic(err)
		}
		foot, ok := basis.Project(core.V3(cWorldX, cWorldY, wt.Height))
		if !ok {
			panic("p37_height_zsort: basis project failed")
		}
		landed, landOK := camera.LandShadow(basis, core.V3(cWorldX, cWorldY, wt.Height), cWorldY)
		out = append(out, cCarrier{
			zsp: zsp{
				name: fmt.Sprintf("c%d", i),
				item: camera.YSortItem{Pos: core.V3(cOriginX+foot.X, cOriginY+foot.Y, 0), Order: i},
				dstX: cOriginX + foot.X - cW/2, dstY: cOriginY + foot.Y - cH,
				w: cW, h: cH, tile: tiles[i],
			},
			worldHeight: wt.Height,
			foot:        core.V2(cOriginX+foot.X, cOriginY+foot.Y),
			shadow:      core.V2(cOriginX+landed.X, cOriginY+landed.Y),
			shadowOK:    landOK,
			visible:     camera.ShadowVisible(core.V3(cWorldX, cWorldY, wt.Height), cWorldY, landOK),
		})
	}
	return out, basis
}

// paintC 画 C 臂：影子先铺（地面层），精灵按 YSort25D 排序后画。
func paintC(dc render.Canvas, sheet *render.ImageBuf, cs []cCarrier) ([]int, error) {
	shadows := make([]render.AtlasSprite, 0, len(cs))
	for i := range cs {
		c := &cs[i]
		if !c.shadowOK || !c.visible {
			continue
		}
		shadows = append(shadows, render.AtlasSprite{
			SrcX: 0, SrcY: 2 * tileSize, SrcW: tileSize, SrcH: tileSize,
			DstX: c.shadow.X - cShadowW/2, DstY: c.shadow.Y - cShadowH/2,
			DstW: cShadowW, DstH: cShadowH,
			Opacity: cShadowOpacity, Filter: render.InterpNearest,
		})
	}
	if _, err := dc.DrawAtlasEx(sheet, shadows, render.AtlasDrawOptions{}); err != nil {
		return nil, err
	}
	zs := make([]zsp, len(cs))
	for i := range cs {
		zs[i] = cs[i].zsp
	}
	sorted, order, err := sortedSprites(zs)
	if err != nil {
		return nil, err
	}
	if _, err := dc.DrawAtlasEx(sheet, sorted, render.AtlasDrawOptions{}); err != nil {
		return nil, err
	}
	return order, nil
}

// paintE 画 E 臂：填充与精灵交错提交（提交序就是答案，不排序）。
func paintE(dc render.Canvas, sheet *render.ImageBuf) error {
	tiles := []int{2, 0, 6, 1, 3, 7} // 琥珀/蓝/绿/青/红/品红
	for k := 0; k < 6; k++ {
		x := eFirstX + float64(k)*eStepX
		if k%2 == 0 {
			fillRect(dc, x, eTopY, eW, eH, tileRGB[tiles[k]])
			continue
		}
		sp := render.AtlasSprite{
			SrcX: float64(tiles[k]%4) * tileSize, SrcY: float64(tiles[k]/4) * tileSize,
			SrcW: tileSize, SrcH: tileSize,
			DstX: x, DstY: eTopY, DstW: eW, DstH: eH,
			Filter: render.InterpNearest,
		}
		if _, err := dc.DrawAtlasEx(sheet, []render.AtlasSprite{sp}, render.AtlasDrawOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// refusalChecks 八项拒绝态实测（纯逻辑，窗底带与离屏探针吃同一份结果）。
// flatNaNUnchecked 记录的是批 2 钉下的口径：平铺 YSort 不校验非有限（树路校验）。
func refusalChecks() (map[string]bool, map[string]string) {
	ok := map[string]bool{}
	got := map[string]string{}

	_, err := camera.YSortTree([]camera.YSortNode{{Parent: -1, Pos: core.V3(1, math.NaN(), 0)}})
	ok["tree_nan"] = err != nil
	got["tree_nan"] = fmt.Sprint(err)

	_, err = camera.YSortTree([]camera.YSortNode{{Parent: -1, Pos: core.V3(1, math.Inf(1), 0)}})
	ok["tree_inf"] = err != nil
	got["tree_inf"] = fmt.Sprint(err)

	_, err = camera.YSortTree([]camera.YSortNode{{Parent: 5, Pos: core.V3(1, 1, 0)}})
	ok["tree_bad_parent"] = err != nil
	got["tree_bad_parent"] = fmt.Sprint(err)

	_, err = camera.YSortTree([]camera.YSortNode{{Parent: 0, Pos: core.V3(1, 1, 0)}})
	ok["tree_cycle"] = err != nil
	got["tree_cycle"] = fmt.Sprint(err)

	_, err = sprite.NewItem("x", sprite.LayerWorld, math.NaN())
	ok["new_item_nan"] = err != nil
	got["new_item_nan"] = fmt.Sprint(err)

	many := make([]camera.YSortItem, camera.MaxYSort25D+1)
	_, err = camera.YSortWithKey(many, camera.YSortKeyY)
	ok["too_many_oom"] = err != nil
	got["too_many_oom"] = fmt.Sprint(err)

	// 批 2 钉下的口径：平铺 YSort 对非有限不校验（比较器对 NaN 恒 false，
	// 稳定序原样返回）；树路才拒。这条记「口径如此」，不算拒绝。
	_, err = camera.YSort([]camera.YSortItem{{Pos: core.V3(1, math.NaN(), 0), Order: 0}})
	ok["flat_nan_unchecked"] = err == nil
	got["flat_nan_unchecked"] = fmt.Sprint(err)

	_, shadowOK := camera.LandShadow(mustBasis(), core.V3(1, 1, 0), 2)
	ok["shadow_below_ground_hidden"] = !shadowOK
	got["shadow_below_ground_hidden"] = fmt.Sprint(shadowOK)

	return ok, got
}

func mustBasis() camera.Basis25D {
	b, err := camera.NewBasis25D(camera.View45)
	if err != nil {
		panic(err)
	}
	return b
}

// paintRefusalBand 把八项拒绝态画成色片（绿=按预期拒绝/按口径放行，红=不符）。
func paintRefusalBand(dc render.Canvas, facts map[string]bool) {
	keys := []string{
		"tree_nan", "tree_inf", "tree_bad_parent", "tree_cycle",
		"new_item_nan", "too_many_oom", "flat_nan_unchecked", "shadow_below_ground_hidden",
	}
	for i, k := range keys {
		x := 150.0 + float64(i)*chipStep
		if facts[k] {
			fillRect(dc, x, refusalY, chipW, chipW, tileRGB[6])
			continue
		}
		fillRect(dc, x, refusalY, chipW, chipW, tileRGB[3])
	}
}

func paintAll(dc render.Canvas, sheet *render.ImageBuf, w, h, pan float64, facts map[string]bool) error {
	paintBackdrop(dc, w, h)
	// 臂 a：z 桶序（远桶先画）。
	if sorted, _, err := sortedSprites(armA()); err != nil {
		return err
	} else if _, err := dc.DrawAtlasEx(sheet, sorted, render.AtlasDrawOptions{}); err != nil {
		return err
	}
	// 臂 b1：sprite.Sort 委托口（批 2 步 2b 的生产面）定序。
	b1 := armB1(pan)
	sItems := make([]sprite.Item, len(b1))
	for i := range b1 {
		sItems[i] = sprite.Item{Name: b1[i].name, FeetY: b1[i].item.Pos.Y}
	}
	sorted, err := sprite.Sort(sItems)
	if err != nil {
		return err
	}
	for _, it := range sorted {
		for i := range b1 {
			if b1[i].name == it.Name {
				if _, err := dc.DrawAtlasEx(sheet, []render.AtlasSprite{b1[i].atlas()}, render.AtlasDrawOptions{}); err != nil {
					return err
				}
				break
			}
		}
	}
	// 臂 b2：平票对（Order 破平票）。
	if b2Sorted, _, err := sortedSprites(armB2()); err != nil {
		return err
	} else if _, err := dc.DrawAtlasEx(sheet, b2Sorted, render.AtlasDrawOptions{}); err != nil {
		return err
	}
	// 臂 c：Height→投影→盖序 + 影子。
	cs, _ := buildC()
	if _, err := paintC(dc, sheet, cs); err != nil {
		return err
	}
	// 臂 e：跨 tier 提交序。
	if err := paintE(dc, sheet); err != nil {
		return err
	}
	// 臂 d：拒绝态底带。
	paintRefusalBand(dc, facts)
	return nil
}

func newCtx(w, h int) *render.Context {
	return render.NewContext(w, h)
}

func pixelAt(dc *render.Context, x, y int) (uint8, uint8, uint8, uint8) {
	img := dc.Image()
	cr, cg, cb, ca := img.At(x, y).RGBA()
	return uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8), uint8(ca >> 8)
}

func flushCtx(dc *render.Context) error { return dc.FlushGPU() }

// backdrop255 是底色的 8-bit 值（探针与画面同一份常数折算）。
func backdrop255() uint8 { return uint8(math.Round(backdropRGB * 255)) }

// isEqualApprox 复算 Godot is_equal_approx 公式（同 ySort25d.go 的常数），
// 只用于探针记录键判等成立；权威判据是排序行为本身。
func isEqualApprox(a, b float64) bool {
	if a == b {
		return true
	}
	tol := 1e-5 * math.Abs(a)
	if tol < 1e-5 {
		tol = 1e-5
	}
	return math.Abs(b-a) < tol
}

func blendChannel(back, fore uint8, alpha float64) uint8 {
	return uint8(math.Round(float64(back)*(1-alpha) + float64(fore)*alpha))
}

type probeResult struct {
	name string
	ok   bool
	got  string
}

type probeReport struct {
	results  []probeResult
	extra    map[string]any
	pixelErr error
}

func (p *probeReport) add(name string, ok bool, got string) {
	p.results = append(p.results, probeResult{name, ok, got})
}

func expectColor(dc *render.Context, x, y int, tile int, tol float64) (bool, string) {
	gr, gg, gb, _ := pixelAt(dc, x, y)
	want := tileRGB[tile]
	dr := abs(int(gr) - int(want[0]))
	dg := abs(int(gg) - int(want[1]))
	db := abs(int(gb) - int(want[2]))
	ok := dr <= int(tol) && dg <= int(tol) && db <= int(tol)
	return ok, fmt.Sprintf("got=(%d,%d,%d) want=%v", gr, gg, gb, want)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// runProbes 离屏证据：五臂各画一次 + 逐像素断言（pan=0 固定几何）。
func runProbes(sheet *render.ImageBuf) probeReport {
	var rep probeReport
	w, h := float64(winW), float64(winH)
	rep.extra = map[string]any{}
	facts, factGot := refusalChecks()

	failPixel := func(name string, err error) {
		rep.pixelErr = errors.Join(rep.pixelErr, fmt.Errorf("%s: %w", name, err))
	}

	// ---- 臂 a：z 桶序 + CLAMP ----
	zs := armA()
	zclamp := map[string]int{}
	for _, v := range []int{-5000, -4097, -4096, -1, 0, 1, 4096, 4097, 5000} {
		zclamp[fmt.Sprint(v)] = camera.ClampZIndex(v)
	}
	rep.extra["zclamp_map"] = zclamp
	clampOK := zclamp["-5000"] == -4096 && zclamp["4097"] == 4096 &&
		zclamp["-4096"] == -4096 && zclamp["4096"] == 4096 && zclamp["0"] == 0
	rep.add("zclamp_saturates", clampOK, fmt.Sprint(zclamp))

	// z 槽指派往返：y 序（同脚位 → Order 破平票）→ YSortZ 槽 → 再按槽排序 = 同序。
	flat := []camera.YSortItem{{Pos: core.V3(0, 10, 0), Order: 0}, {Pos: core.V3(0, 10, 0), Order: 1},
		{Pos: core.V3(0, 10, 0), Order: 2}}
	first, err := camera.YSort(flat)
	if err != nil {
		failPixel("zslot roundtrip sort", err)
	}
	slots := make([]int, len(first))
	for rank, it := range first {
		slots[it.Order] = camera.ClampZIndex(camera.YSortZ(rank))
	}
	round := make([]camera.YSortItem, len(flat))
	for i := range round {
		round[i] = camera.YSortItem{Pos: core.V3(0, 10, 0), Order: i, ZIndex: slots[i]}
	}
	second, err := camera.YSort(round)
	if err != nil {
		failPixel("zslot roundtrip resort", err)
	}
	roundtripOK := true
	for k := range second {
		if second[k].Order != k {
			roundtripOK = false
		}
	}
	rep.extra["zslot_roundtrip"] = roundtripOK
	rep.extra["zslot_rank_slots"] = slots
	rep.add("zslot_roundtrip_same_order", roundtripOK, fmt.Sprintf("slots=%v", slots))

	dcA := newCtx(int(winW), int(winH))
	var orderA []int
	if err := flushErr(dcA, func() error {
		paintBackdrop(dcA, w, h)
		var e error
		sorted, ord, err2 := sortedSprites(zs)
		orderA = ord
		if err2 != nil {
			return err2
		}
		_, e = dcA.DrawAtlasEx(sheet, sorted, render.AtlasDrawOptions{})
		return e
	}); err != nil {
		failPixel("arm a flush", err)
	}
	rep.extra["a_draw_order"] = orderA
	bucketOK := len(orderA) == 4 && orderA[0] == 0 && orderA[1] == 1 && orderA[2] == 2 && orderA[3] == 3
	rep.add("z_bucket_cover_order", bucketOK, fmt.Sprintf("order=%v (z -5000→-10→10→5000)", orderA))
	if ok, got := expectColor(dcA, 160, 114, 0, directTol); !ok {
		rep.add("a_low_only_blue", false, got)
	} else {
		rep.add("a_low_only_blue", true, got)
	}
	if ok, got := expectColor(dcA, 230, 114, 1, directTol); !ok {
		rep.add("a_seam_low_teal_covers", false, got)
	} else {
		rep.add("a_seam_low_teal_covers", true, got)
	}
	if ok, got := expectColor(dcA, 294, 114, 2, directTol); !ok {
		rep.add("a_seam_mid_amber_covers", false, got)
	} else {
		rep.add("a_seam_mid_amber_covers", true, got)
	}
	if ok, got := expectColor(dcA, 358, 114, 3, directTol); !ok {
		rep.add("a_seam_top_red_covers", false, got)
	} else {
		rep.add("a_seam_top_red_covers", true, got)
	}
	_ = dcA.Close()

	// ---- 臂 b1：y 键定序 + sprite.Sort 委托对账 ----
	b1 := armB1(0)
	camItems := make([]camera.YSortItem, len(b1))
	for i := range b1 {
		camItems[i] = b1[i].item
	}
	camGot, err := camera.YSort(camItems)
	if err != nil {
		failPixel("b1 camera sort", err)
	}
	camOrder := []string{}
	for _, it := range camGot {
		camOrder = append(camOrder, b1[it.Order].name)
	}
	sItems := []sprite.Item{{Name: b1[0].name, FeetY: b1[0].item.Pos.Y}, {Name: b1[1].name, FeetY: b1[1].item.Pos.Y}}
	spGot, err := sprite.Sort(sItems)
	if err != nil {
		failPixel("b1 sprite sort", err)
	}
	spOrder := []string{}
	for _, it := range spGot {
		spOrder = append(spOrder, it.Name)
	}
	delegateParity := len(camOrder) == 2 && camOrder[0] == spOrder[0] && camOrder[1] == spOrder[1]
	rep.extra["b1_camera_order"] = camOrder
	rep.extra["b1_sprite_order"] = spOrder
	rep.add("delegate_parity_camera_vs_sprite", delegateParity,
		fmt.Sprintf("camera=%v sprite=%v", camOrder, spOrder))

	dcB := newCtx(int(winW), int(winH))
	if err := flushErr(dcB, func() error {
		paintBackdrop(dcB, w, h)
		return paintBandB1(dcB, sheet, b1)
	}); err != nil {
		failPixel("arm b1 flush", err)
	}
	if ok, got := expectColor(dcB, 740, 260, 6, directTol); !ok {
		rep.add("b1_near_covers_by_y", false, got)
	} else {
		rep.add("b1_near_covers_by_y", true, got)
	}
	if ok, got := expectColor(dcB, 800, 250, 0, directTol); !ok {
		rep.add("b1_far_right_visible", false, got)
	} else {
		rep.add("b1_far_right_visible", true, got)
	}
	_ = dcB.Close()

	// ---- 臂 b2：平票（is_equal_approx 判等 → Order 破平票）+ 200 轮不闪 ----
	b2 := armB2()
	ka, kb := b2[0].item.Pos.Y, b2[1].item.Pos.Y
	tieEqual := isEqualApprox(ka, kb)
	rep.extra["tie_keys"] = []float64{ka, kb}
	rep.extra["tie_equal_approx"] = tieEqual
	flips := 0
	var firstTie []int
	for i := 0; i < 200; i++ {
		got, err := camera.YSort([]camera.YSortItem{b2[0].item, b2[1].item})
		if err != nil {
			failPixel("b2 tie sort", err)
			break
		}
		cur := []int{got[0].Order, got[1].Order}
		if firstTie == nil {
			firstTie = cur
		} else if cur[0] != firstTie[0] || cur[1] != firstTie[1] {
			flips++
		}
	}
	rep.extra["tie_frames_offscreen"] = 200
	rep.extra["tie_flips_offscreen"] = flips
	rep.add("tie_stable_200_frames_offscreen", flips == 0, fmt.Sprintf("first=%v flips=%d", firstTie, flips))
	rep.add("tie_keys_equal_approx", tieEqual, fmt.Sprintf("ka=%v kb=%v", ka, kb))

	dcB2 := newCtx(int(winW), int(winH))
	if err := flushErr(dcB2, func() error {
		paintBackdrop(dcB2, w, h)
		sorted, _, err2 := sortedSprites(armB2())
		if err2 != nil {
			return err2
		}
		_, e := dcB2.DrawAtlasEx(sheet, sorted, render.AtlasDrawOptions{})
		return e
	}); err != nil {
		failPixel("arm b2 flush", err)
	}
	if ok, got := expectColor(dcB2, 940, 364, 5, directTol); !ok {
		rep.add("b2_order_breaks_tie", false, got)
	} else {
		rep.add("b2_order_breaks_tie", true, got)
	}
	if ok, got := expectColor(dcB2, 900, 364, 4, directTol); !ok {
		rep.add("b2_first_left_visible", false, got)
	} else {
		rep.add("b2_first_left_visible", true, got)
	}
	_ = dcB2.Close()

	// ---- 臂 c：Height 折叠→投影→盖序 + 影子 ----
	cs, _ := buildC()
	heights := []float64{}
	foots := []float64{}
	for i := range cs {
		heights = append(heights, cs[i].worldHeight)
		foots = append(foots, cs[i].foot.Y)
	}
	mono := len(foots) == 3 && foots[0] < foots[1] && foots[1] < foots[2]
	rep.extra["c_world_heights"] = heights
	rep.extra["c_foot_y_screen"] = foots
	rep.add("height_folds_and_projects_monotonic", mono,
		fmt.Sprintf("heights=%v foot_y=%v", heights, foots))

	dcC := newCtx(int(winW), int(winH))
	if err := flushErr(dcC, func() error {
		paintBackdrop(dcC, w, h)
		_, err2 := paintC(dcC, sheet, cs)
		return err2
	}); err != nil {
		failPixel("arm c flush", err)
	}
	if ok, got := expectColor(dcC, 236, 460, 6, directTol); !ok {
		rep.add("c_low_top_visible", false, got)
	} else {
		rep.add("c_low_top_visible", true, got)
	}
	if ok, got := expectColor(dcC, 236, 490, 1, directTol); !ok {
		rep.add("c_mid_covers_low", false, got)
	} else {
		rep.add("c_mid_covers_low", true, got)
	}
	if ok, got := expectColor(dcC, 236, 520, 3, directTol); !ok {
		rep.add("c_tall_covers_mid", false, got)
	} else {
		rep.add("c_tall_covers_mid", true, got)
	}
	shadowWant := [3]uint8{
		blendChannel(backdrop255(), shadowTileRGB[0], cShadowOpacity),
		blendChannel(backdrop255(), shadowTileRGB[1], cShadowOpacity),
		blendChannel(backdrop255(), shadowTileRGB[2], cShadowOpacity),
	}
	shadowOK := true
	shadowGot := ""
	for i, py := range []int{int(math.Round(foots[0])), int(math.Round(foots[2]))} {
		gr, gg, gb, _ := pixelAt(dcC, 180, py)
		dr := abs(int(gr) - int(shadowWant[0]))
		dg := abs(int(gg) - int(shadowWant[1]))
		db := abs(int(gb) - int(shadowWant[2]))
		if dr > int(blendTol) || dg > int(blendTol) || db > int(blendTol) {
			shadowOK = false
			shadowGot = fmt.Sprintf("shadow%d got=(%d,%d,%d) want=%v", i, gr, gg, gb, shadowWant)
		}
	}
	landedPts := []map[string]any{}
	for i := range cs {
		landedPts = append(landedPts, map[string]any{
			"x": cs[i].shadow.X, "y": cs[i].shadow.Y,
			"landed": cs[i].shadowOK, "visible": cs[i].visible,
		})
	}
	rep.extra["shadow_landed"] = landedPts
	rep.add("shadow_lands_on_ground_line", shadowOK && cs[0].shadowOK && cs[0].visible, shadowGot)
	_ = dcC.Close()

	// ---- 臂 e：跨 tier 提交序=盖序 ----
	dcE := newCtx(int(winW), int(winH))
	if err := flushErr(dcE, func() error {
		paintBackdrop(dcE, w, h)
		return paintE(dcE, sheet)
	}); err != nil {
		failPixel("arm e flush", err)
	}
	if ok, got := expectColor(dcE, 230, 647, 0, directTol); !ok {
		rep.add("tier_sprite_over_rect", false, got)
	} else {
		rep.add("tier_sprite_over_rect", true, got)
	}
	if ok, got := expectColor(dcE, 530, 647, 7, directTol); !ok {
		rep.add("tier_last_submission_wins", false, got)
	} else {
		rep.add("tier_last_submission_wins", true, got)
	}
	_ = dcE.Close()

	// ---- 臂 d：拒绝态 ----
	rep.extra["nonfinite_rejects"] = factGot
	allRefuse := true
	for _, v := range facts {
		if !v {
			allRefuse = false
		}
	}
	rep.add("refusal_family_all_expected", allRefuse, fmt.Sprint(factGot))

	return rep
}

func flushErr(dc *render.Context, paint func() error) error {
	if err := paint(); err != nil {
		return err
	}
	return dc.FlushGPU()
}

// paintBandB1 画 B1 带（sprite.Sort 排序后的绘制序）。
func paintBandB1(dc render.Canvas, sheet *render.ImageBuf, b1 []zsp) error {
	sItems := make([]sprite.Item, len(b1))
	for i := range b1 {
		sItems[i] = sprite.Item{Name: b1[i].name, FeetY: b1[i].item.Pos.Y}
	}
	sorted, err := sprite.Sort(sItems)
	if err != nil {
		return err
	}
	for _, it := range sorted {
		for i := range b1 {
			if b1[i].name == it.Name {
				if _, err := dc.DrawAtlasEx(sheet, []render.AtlasSprite{b1[i].atlas()}, render.AtlasDrawOptions{}); err != nil {
					return err
				}
				break
			}
		}
	}
	return nil
}

// semanticProbes 终帧快照上的语义像素断言（坐标由同一份常数/冻结相位反推）。
func semanticProbes(pan float64) []wrkit.PixelProbe {
	shadowWant := [3]float64{
		float64(blendChannel(backdrop255(), shadowTileRGB[0], cShadowOpacity)) / 255,
		float64(blendChannel(backdrop255(), shadowTileRGB[1], cShadowOpacity)) / 255,
		float64(blendChannel(backdrop255(), shadowTileRGB[2], cShadowOpacity)) / 255,
	}
	tile := func(t int) (r, g, b float64) {
		return float64(tileRGB[t][0]) / 255, float64(tileRGB[t][1]) / 255, float64(tileRGB[t][2]) / 255
	}
	probes := make([]wrkit.PixelProbe, 0, 14)
	add := func(name string, x, y float64, t int, tol uint8) {
		r, g, b := tile(t)
		probes = append(probes, wrkit.PixelProbe{Name: name, X: x, Y: y, R: r, G: g, B: b, Tol: tol})
	}
	add("a_low_only_blue", 160, 114, 0, directTol)
	add("a_seam_low_teal_covers", 230, 114, 1, directTol)
	add("a_seam_mid_amber_covers", 294, 114, 2, directTol)
	add("a_seam_top_red_covers", 358, 114, 3, directTol)
	add("b1_near_covers_by_y", 740+pan, 260, 6, directTol)
	add("b1_far_right_visible", 800+pan, 250, 0, directTol)
	add("b2_order_breaks_tie", 940, 364, 5, directTol)
	add("b2_first_left_visible", 900, 364, 4, directTol)
	add("c_low_top_visible", 236, 460, 6, directTol)
	add("c_mid_covers_low", 236, 490, 1, directTol)
	add("c_tall_covers_mid", 236, 520, 3, directTol)
	probes = append(probes, wrkit.PixelProbe{Name: "c_shadow_low_sliver", X: 180, Y: math.Round(560 - 45.25484197), R: shadowWant[0], G: shadowWant[1], B: shadowWant[2], Tol: uint8(blendTol)})
	probes = append(probes, wrkit.PixelProbe{Name: "c_shadow_tall_sliver", X: 182, Y: 560, R: shadowWant[0], G: shadowWant[1], B: shadowWant[2], Tol: uint8(blendTol)})
	add("tier_sprite_over_rect", 230, 647, 0, directTol)
	add("tier_last_submission_wins", 530, 647, 7, directTol)
	return probes
}

func backendTag() string {
	b, err := render.ResolveBackendFor(render.BackendNative)
	if err != nil {
		return "native"
	}
	return b.String()
}

type tick struct{ on func(dt float64) bool }

func (t *tick) Tick(dt float64) bool { return t.on(dt) }

func main() {
	out := flag.String("out", "", "extra JSON path (optional)")
	autoOnly := flag.Bool("auto-only", false, "gate mode: fixed-frame run, JSON on stdout, exit 1 on fail")
	manualSeconds := flag.Int("manual-seconds", 0, "resident phase timeout in seconds (0 = until close)")
	flag.Parse()
	_ = os.MkdirAll(testdataDir, 0o755)

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly && !secsSet {
		secs = 8
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	}
	gateMode := *autoOnly
	wrkit.EnsureUIFace()

	sheet := newSheetTexture()
	t0 := time.Now()

	// 离屏证据先出（开窗后再拿 GPU 深度会失真）。
	rep := runProbes(sheet)
	probesOK := true
	for _, r := range rep.results {
		if !r.ok {
			probesOK = false
			fmt.Fprintf(os.Stderr, "p37_height_zsort: 探针 FAIL %s → %s\n", r.name, r.got)
		}
	}
	var summary manualSummary
	var paintErr string
	var tieFrames int64
	tieStable := true
	var firstTieOrder []int

	win, err := platform.Open(platform.Options{Width: int(winW), Height: int(winH), Title: winTitle, Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: backdropRGB, G: backdropRGB, B: backdropRGB + 0.02, A: 1}

	content := rendering.NewRenderBox()
	content.FixedWidth, content.FixedHeight = winW, winH
	frameNo := int64(0)
	pan := 0.0
	facts, _ := refusalChecks()
	content.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc := pc.DC
		// H15：相位只随已完成帧数走，过 settling 帧定格（末帧与墙钟无关）。
		idx := frameNo
		if gateMode && idx > gateFreezeAt {
			idx = gateFreezeAt
		}
		pan = math.Sin(float64(idx)/gateHz*1.1) * 18
		w, h := size.Width, size.Height
		if e := paintAll(dc, sheet, w, h, pan, facts); e != nil {
			paintErr = e.Error()
		}
		frameNo++
	}
	root.Place(content, 0, 0)
	metric := wrkit.Label("presents=0", 13, 0.92, 0.94, 0.98)
	root.Place(metric, 8, 8)

	setTitle := func() {
		ctl := win.Controls()
		if ctl == nil {
			return
		}
		ctl.SetTitle(fmt.Sprintf("%s — ptr=%d key=%d rs=%d t=%.0fs", winTitle,
			summary.Pointer, summary.Key, summary.Resize, time.Since(t0).Seconds()))
	}

	gateMaxFrames := int64(0)
	if gateMode {
		gateMaxFrames = gateFrames
	}
	snapPath := filepath.Join(testdataDir, "p37_height_zsort_final.png")
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{
		ClearR: backdropRGB, ClearG: backdropRGB, ClearB: backdropRGB + 0.02, ClearA: 1,
		RunFor:       time.Duration(secs) * time.Second,
		MaxFrames:    gateMaxFrames,
		WarmUp:       true,
		SnapshotPath: snapPath,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "p37_height_zsort: close (%s)\n", win.Backend())
			case platform.EventResize:
				summary.Resize++
				setTitle()
				if ev.Width > 0 && ev.Height > 0 {
					root.FixedWidth, root.FixedHeight = float64(ev.Width), float64(ev.Height)
					content.FixedWidth, content.FixedHeight = float64(ev.Width), float64(ev.Height)
					root.MarkNeedsPaint()
				}
			case platform.EventPointer:
				summary.Pointer++
				if summary.Pointer%24 == 1 {
					setTitle()
				}
			case platform.EventKey:
				if ev.Pressed {
					summary.Key++
					setTitle()
				}
			default:
				fmt.Fprintf(os.Stderr, "p37_height_zsort: event %s\n", ev.Type)
			}
		},
	})
	var proc scheduler.ProcessTracker
	proc.Start()
	app.Scheduler().Tickers().Add(&tick{on: func(dt float64) bool {
		// 平票不闪的窗内证据：每帧对 B2 平票对重新排序，序必须恒同。
		b2 := armB2()
		got, err := camera.YSort([]camera.YSortItem{b2[0].item, b2[1].item})
		if err == nil {
			cur := []int{got[0].Order, got[1].Order}
			if firstTieOrder == nil {
				firstTieOrder = cur
			} else if cur[0] != firstTieOrder[0] || cur[1] != firstTieOrder[1] {
				tieStable = false
			}
			tieFrames++
		}
		metric.SetText(fmt.Sprintf("presents=%d tie_frames=%d tie_stable=%t",
			app.PresentCount(), tieFrames, tieStable))
		metric.MarkNeedsPaint()
		setTitle()
		content.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()
		return true
	}})
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: app.Run: %v\n", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	if gateMaxFrames > 0 && app.PresentCount() < gateMaxFrames {
		fmt.Fprintf(os.Stderr, "p37_height_zsort: FAIL 定帧停机没达成 presents=%d want>=%d（末帧随墙钟漂）\n",
			app.PresentCount(), gateMaxFrames)
		probesOK = false
	}

	// 金图按后端分文件，fail-closed：缺基线判红，播种那一跑也不算过。
	tag := backendTag()
	basePath := filepath.Join(testdataDir, "goldens", fmt.Sprintf("p37_height_zsort_final_%s.png", tag))
	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGateW(snapPath, basePath, winW,
		[]wrkit.GoldenRect{{X: 0, Y: metricStripH, W: winW, H: winH - metricStripH}})
	goldenOK := goldenErr == nil && !seeded && goldenDiff == 0
	if goldenErr != nil {
		fmt.Fprintf(os.Stderr, "p37_height_zsort: golden: %v\n", goldenErr)
	}
	if seeded {
		fmt.Fprintf(os.Stderr, "p37_height_zsort: golden 已播种（%s），这一跑不算 PASS —— 复核后提交基线再复跑\n", basePath)
	}

	pixelOK := paintErr == ""
	if gateMode {
		// 冻结相位的 pan 与画面同源（sin(idx/gateHz·1.1)·18，idx=gateFreezeAt）。
		frozenPan := math.Sin(float64(gateFreezeAt)/gateHz*1.1) * 18
		if pixelErr := wrkit.CheckPixelProbes(snapPath, winW, semanticProbes(frozenPan)); pixelErr != nil {
			pixelOK = false
			fmt.Fprintf(os.Stderr, "p37_height_zsort: 像素断言 FAIL: %v\n", pixelErr)
		}
	} else {
		fmt.Fprintln(os.Stderr, "p37_height_zsort: 常驻相末帧未定格（H15 只在门禁相成立），像素与金图只报数不判绿")
	}
	if paintErr != "" {
		fmt.Fprintf(os.Stderr, "p37_height_zsort: 绘制 FAIL: %s\n", paintErr)
	}

	snap := app.Metrics().Snapshot()
	wrkit.MergeBoundaryCache(app, &snap)
	extra := map[string]any{
		"probes_ok": probesOK, "pixel_ok": pixelOK, "paint_err": paintErr,
		"golden_diff_pct": goldenDiff, "golden_total_px": goldenPx,
		"golden_seeded": seeded, "golden_backend_tag": tag,
		"window_tie_frames_checked": tieFrames,
		"window_tie_stable":         tieStable,
		"window_tie_first_order":    firstTieOrder,
		"gate_frames":               gateFrames, "gate_freeze_at": gateFreezeAt,
		"golden_tolerance_pct": 0,
	}
	for k, v := range rep.extra {
		extra[k] = v
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "p37-6-2", Scenario: "p37_height_zsort", Backend: tag,
		Snap: snap, PresentCount: app.PresentCount(), ElapsedSec: time.Since(t0).Seconds(),
		SurfaceAreaPx: winW * winH, Warmup: true, Extra: extra,
	})
	raw, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(raw))
	if *out != "" {
		_ = os.WriteFile(*out, raw, 0o644)
	}

	if *autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1}); err != nil {
			fmt.Fprintln(os.Stderr, "FAIL:", err)
			os.Exit(1)
		}
		if !probesOK {
			fmt.Fprintln(os.Stderr, "FAIL: 逻辑探针未全绿")
			os.Exit(1)
		}
		if tieFrames < 60 || !tieStable {
			fmt.Fprintf(os.Stderr, "FAIL: 平票不闪证据不足（frames=%d stable=%t）\n", tieFrames, tieStable)
			os.Exit(1)
		}
		if !pixelOK {
			fmt.Fprintln(os.Stderr, "FAIL: 语义像素断言未过")
			os.Exit(1)
		}
		if !goldenOK {
			fmt.Fprintf(os.Stderr, "FAIL: golden diff=%.4f%% over %d px seeded=%v err=%v\n", goldenDiff, goldenPx, seeded, goldenErr)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "p37_height_zsort: OK backend=%s presents=%d probes=%d/%d golden=%.4f%% (%d px) tie_frames=%d\n",
			tag, app.PresentCount(), len(rep.results), len(rep.results), goldenDiff, goldenPx, tieFrames)
		return
	}
	fmt.Fprintf(os.Stderr, "p37_height_zsort: resident done backend=%s presents=%d probes_ok=%v pixel_ok=%v golden=%.4f%%\n",
		tag, app.PresentCount(), probesOK, pixelOK, goldenDiff)
}

type manualSummary struct {
	Pointer, Key, Resize int
}
