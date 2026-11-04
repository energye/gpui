package world

// P3.7 批 2 步 2d：实体世界高度（Transform.Height）的钉。
// 口径：高度只喂投影（仿射链不含它），遮挡排序键仍取脚位 Y（Godot 无 2D 高度，
// Sprite2D.offset 也不进 y-sort 键 —— 见 batch2-assessment.md §6 高度行）；
// 父链把高度按普通加法折叠；文件面走 additive 字段（Prefab/Scene 1.0 → 1.1）。

import (
	"math"
	"strings"
	"testing"

	"github.com/energye/gpui/engine/camera"
	"github.com/energye/gpui/engine/core"
)

func TestP37HeightRejectedWhenNonFinite(t *testing.T) {
	w := NewWorld()
	id, err := w.Spawn(NoEntity)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	base := Transform{Pos: core.V2(3, 4), Scale: core.V2(1, 1)}
	if err := w.SetTransform(id, base); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for name, bad := range map[string]float64{"NaN": math.NaN(), "+Inf": math.Inf(1), "-Inf": math.Inf(-1)} {
		badT := base
		badT.Height = bad
		if err := w.SetTransform(id, badT); err == nil {
			t.Fatalf("%s height 必须拒收", name)
		} else if code := core.CodeOf(err); code != core.CodeInvalidArg {
			t.Fatalf("%s height err=%v，want invalid-arg", name, err)
		}
		if back, err := w.Local(id); err != nil || back != base {
			t.Fatalf("%s 被拒后本地量被改写：%+v err=%v", name, back, err)
		}
	}
}

func TestP37HeightNeverEntersAffineChain(t *testing.T) {
	flat := Transform{Pos: core.V2(7, 9), Rot: 0.3, Scale: core.V2(2, 0.5)}
	lifted := flat
	lifted.Height = 123.5
	if LocalMatrix(flat) != LocalMatrix(lifted) {
		t.Fatal("Height 混进了 LocalMatrix（2×3 链必须逐位不变）")
	}
	w := NewWorld()
	root, err := w.Spawn(NoEntity)
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	if err := w.SetTransform(root, flat); err != nil {
		t.Fatalf("root: %v", err)
	}
	mRoot, err := w.WorldMatrix(root)
	if err != nil {
		t.Fatalf("world matrix: %v", err)
	}
	if err := w.SetTransform(root, lifted); err != nil {
		t.Fatalf("root lifted: %v", err)
	}
	mLift, err := w.WorldMatrix(root)
	if err != nil {
		t.Fatalf("world matrix lifted: %v", err)
	}
	if mRoot != mLift {
		t.Fatal("Height 改动了 WorldMatrix")
	}
}

func TestP37WorldOfSumsHeights(t *testing.T) {
	w := NewWorld()
	platform, err := w.Spawn(NoEntity)
	if err != nil {
		t.Fatalf("spawn platform: %v", err)
	}
	hero, err := w.Spawn(platform)
	if err != nil {
		t.Fatalf("spawn hero: %v", err)
	}
	if err := w.SetTransform(platform, Transform{Pos: core.V2(0, 100), Scale: core.V2(1, 1), Height: 30}); err != nil {
		t.Fatalf("platform: %v", err)
	}
	if err := w.SetTransform(hero, Transform{Pos: core.V2(5, 0), Scale: core.V2(1, 1), Height: 12}); err != nil {
		t.Fatalf("hero: %v", err)
	}
	got, err := w.WorldOf(hero)
	if err != nil {
		t.Fatalf("worldof: %v", err)
	}
	if got.Height != 42 {
		t.Fatalf("父链高度必须相加：%v want 42", got.Height)
	}
	// 仿射侧逐位不变：同链条、全零高度，Pos/Rot/Scale 折叠必须一模一样。
	zero := NewWorld()
	zp, _ := zero.Spawn(NoEntity)
	zh, _ := zero.Spawn(zp)
	_ = zero.SetTransform(zp, Transform{Pos: core.V2(0, 100), Scale: core.V2(1, 1)})
	_ = zero.SetTransform(zh, Transform{Pos: core.V2(5, 0), Scale: core.V2(1, 1)})
	gotZero, err := zero.WorldOf(zh)
	if err != nil {
		t.Fatalf("worldof zero: %v", err)
	}
	if got.Pos != gotZero.Pos || got.Rot != gotZero.Rot || got.Scale != gotZero.Scale {
		t.Fatal("高度折叠改动了位置/旋转/缩放")
	}
}

func TestP37HeightSurvivesFileRoundTrip(t *testing.T) {
	lifted := Transform{Pos: core.V2(10, 20), Rot: 0.25, Scale: core.V2(2, 2), Height: 7.5}
	pf, err := NewPrefab("hero", lifted)
	if err != nil {
		t.Fatalf("NewPrefab: %v", err)
	}
	raw, err := pf.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if !strings.Contains(string(raw), `"height":7.5`) {
		t.Fatalf("prefab 文件没带 height：%s", raw)
	}
	back, err := ParsePrefab(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if back.Local() != lifted || !back.Equal(pf) {
		t.Fatalf("prefab 往返丢了高度：%+v", back.Local())
	}
	sf, err := NewSceneFile("level")
	if err != nil {
		t.Fatalf("NewSceneFile: %v", err)
	}
	if err := sf.AddEntity("hero", "", lifted); err != nil {
		t.Fatalf("add entity: %v", err)
	}
	sraw, err := sf.Encode()
	if err != nil {
		t.Fatalf("scene encode: %v", err)
	}
	sback, err := ParseScene(sraw)
	if err != nil {
		t.Fatalf("scene parse: %v", err)
	}
	if !sback.Equal(sf) {
		t.Fatal("scene 往返不等（高度被吞？）")
	}
	ents := sback.Entities()
	if len(ents) != 1 || ents[0].Local.Height != 7.5 {
		t.Fatalf("scene 实体高度没回来：%+v", ents)
	}
	// 1.0 旧文件（无 height 键）必须仍能读，且高度按 0 处理。
	old := []byte(`{"version":"1.0","name":"hero","local":{"pos":[10,20],"rot":0.25,"scale":[2,2]},"comps":[]}`)
	pfOld, err := ParsePrefab(old)
	if err != nil {
		t.Fatalf("1.0 prefab must still load: %v", err)
	}
	if pfOld.Local().Height != 0 {
		t.Fatalf("缺 height 键必须读成 0，得到 %v", pfOld.Local().Height)
	}
	// 坏高度进文件 = BadData。实测这一拒收来自 encoding/json 本身
	// （1e999 解不进 float64），transformFromJSON 里的 finite 校验是第二道
	// 保险（负向拆掉它仍红在此处 ⇒ 记为不可达分支，见 batch2-design.md §11）。
	bad := []byte(`{"version":"1.1","name":"hero","local":{"pos":[0,0],"rot":0,"scale":[1,1],"height":1e999},"comps":[]}`)
	if _, err := ParsePrefab(bad); err == nil || core.CodeOf(err) != core.CodeBadData {
		t.Fatalf("Inf height 文件必须 BadData，得到 %v", err)
	}
}

func TestP37HeightStaysOutOfSortKey(t *testing.T) {
	// 高个子站在近处（脚位 Y 小）⇒ 仍按脚位排前面；把高度塞进键就会反过来。
	feetLow, feetHigh := 10.0, 20.0
	a := Transform{Pos: core.V2(0, feetLow), Scale: core.V2(1, 1), Height: 200}
	b := Transform{Pos: core.V2(0, feetHigh), Scale: core.V2(1, 1)}
	items := []camera.YSortItem{
		{Pos: core.V3(a.Pos.X, a.Pos.Y, 0), Order: 0},
		{Pos: core.V3(b.Pos.X, b.Pos.Y, 0), Order: 1},
	}
	sorted, err := camera.YSort(items)
	if err != nil {
		t.Fatalf("ysort: %v", err)
	}
	if sorted[0].Order != 0 || sorted[1].Order != 1 {
		t.Fatalf("排序键没取脚位：%+v（Height 不许进键）", sorted)
	}
	if camera.YSortKeyY(core.V3(a.Pos.X, a.Pos.Y, 0)) != feetLow {
		t.Fatal("标准键必须只看 Y")
	}
}
