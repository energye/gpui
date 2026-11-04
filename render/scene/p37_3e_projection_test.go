//go:build !nogpu

package scene_test

// P3.7 批 3e-1：scene 的 4×4 投影口（通用层能力，链序仍住在 render）。
//
// 判据正本 `evidence/P3.7/batch3e-design.md` §2/§4：
//  1. 不调投影动词的存量场景**零新 tag**、Hash 逐位不变（delta 编码）；
//  2. Set/Push/Pop 编码—解码回环一致（流里存 float32 ⇒ 判据只用二进制精确可表示的数）；
//  3. GPU 回放 ≡ 直接用 render.Context 画同一份投影（回放只是转交，链序只有一处定义）；
//  4. CPU 瓦片腿：仿射可表达的投影折进烘焙链（画得对），真投影一律**拒画 + 具名计数**
//     ——绝不静默压扁（与批 1d 文字族同纪律）；
//  5. 合并口（Append / AppendWithTranslation）不吞投影流；
//  6. 边界：2.5D 与 ui 侧对 render/scene 零引用 —— Godot 的 2D 命令面没有投影参数
//     （core/math/transform_2d.h:58、rendering_server.h:836-855），透视走「算好再喂」。

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/scene"
)

const projProbeSize = 120

// projFlatScene 画一块屋：红矩形，恒等投影下即基线画面。
func projFlatScene() *scene.Scene {
	s := scene.NewScene()
	s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.Red),
		scene.NewRectShape(10, 10, 60, 40))
	return s
}

// projSceneWith 先装投影再画：投影是粘滞状态，只管它之后录的内容
// （与 render.Context 同语义；回放端投影 tag 与绘制 tag 同流同序）。
func projSceneWith(m render.Matrix4) *scene.Scene {
	s := scene.NewScene()
	s.SetProjection(m)
	s.Fill(scene.FillNonZero, scene.IdentityAffine(), scene.SolidBrush(render.Red),
		scene.NewRectShape(10, 10, 60, 40))
	return s
}

// dyadicHomography 元素全是 2 的负幂 ⇒ float32 流进流出零舍入。
func dyadicHomography() render.Matrix4 {
	return render.Matrix4FromElements([16]float64{
		1, 0, 0.125, 0,
		0, 1, 0.25, 0,
		0, 0, 1, 0,
		0, 0.03125, 0, 1,
	})
}

func countProjectionTags(enc *scene.Encoding) int {
	dec := scene.NewDecoder(enc)
	if dec == nil {
		return 0
	}
	n := 0
	for dec.Next() {
		if dec.Tag() == scene.TagProjection {
			n++
		}
	}
	return n
}

// 1 + 2：恒等零发射、Hash 不漂、模式与矩阵回环、栈配对、坏输入 fail-closed。
func TestP373ESceneProjectionEncoding(t *testing.T) {
	homography := dyadicHomography()

	base := projFlatScene()
	withID := projFlatScene()
	withID.SetProjection(render.Matrix4Identity())
	withID.SetProjection(render.Matrix4{}) // 零值 = 恒等（契约同 Context.SetProjection）

	if base.Encoding().Hash() != withID.Encoding().Hash() {
		t.Fatal("恒等投影改变了 Hash：delta 发射失效，存量场景的缓存键会漂")
	}
	if n := countProjectionTags(base.Encoding()); n != 0 {
		t.Fatalf("没装投影却发了 %d 枚 TagProjection", n)
	}
	if n := countProjectionTags(withID.Encoding()); n != 0 {
		t.Fatalf("恒等/零值必须零发射，发了 %d 枚", n)
	}
	// 只换投影、几何一模一样 ⇒ Hash 必须不同（LayerCache 按 Hash 取缓存，投影不进哈希
	// 就会让装了投影与没装投影的两张场景串同一份缓存像素）。
	flatForHash := projFlatScene()
	projForHash := projSceneWith(dyadicHomography())
	if flatForHash.Encoding().Hash() == projForHash.Encoding().Hash() {
		t.Fatal("投影没进 Hash：LayerCache 会串键返回错像素")
	}
	if got := base.Projection(); !got.IsIdentity() {
		t.Fatalf("未装投影时 Projection() 应为恒等：%+v", got)
	}

	s := projSceneWith(homography)
	if n := countProjectionTags(s.Encoding()); n != 1 {
		t.Fatalf("Set 一次应发一枚 TagProjection，实得 %d", n)
	}
	dec := scene.NewDecoder(s.Encoding())
	var modes []render.ProjectionMode
	var last render.Matrix4
	for dec.Next() {
		if dec.Tag() != scene.TagProjection {
			continue
		}
		mode, m := dec.Projection()
		modes = append(modes, mode)
		last = m
	}
	if len(modes) != 1 || modes[0] != render.ProjectionSet || last != homography {
		t.Fatalf("编码—解码回环不一致：%v，矩阵 %+v，want %+v", modes, last, homography)
	}
	if got := s.Projection(); got != homography {
		t.Fatalf("Projection() 应读回装上的那份：%+v", got)
	}

	// Push/Pop 是栈操作：每次都发；空栈 Pop 是 no-op（契约同 Context）。
	p := projFlatScene()
	p.PushProjection(homography)
	p.PushProjection(render.Matrix4Identity())
	p.PopProjection()
	p.PopProjection()
	if n := countProjectionTags(p.Encoding()); n != 4 {
		t.Fatalf("两次 Push 两次 Pop 应发 4 枚，实得 %d", n)
	}
	p.PopProjection() // 栈空 ⇒ no-op
	if n := countProjectionTags(p.Encoding()); n != 4 {
		t.Fatalf("空栈 Pop 不该再发 tag，现在 %d 枚", n)
	}
	if got := p.Projection(); !got.IsIdentity() {
		t.Fatalf("配对 Pop 之后应回到恒等：%+v", got)
	}

	// 非有限矩阵：忽略，绝不装进链。
	bad := projFlatScene()
	before := bad.Projection()
	bad.SetProjection(render.Matrix4FromElements([16]float64{
		1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, math.Inf(1), 0, 1,
	}))
	if bad.Projection() != before {
		t.Fatal("非有限投影被装上了")
	}
	if n := countProjectionTags(bad.Encoding()); n != 0 {
		t.Fatalf("被忽略的投影不该发 tag：%d", n)
	}
}

// 3：GPU 回放腿 ≡ 直接用 Context 画（投影只在 render 里算一次）。
func TestP373ESceneProjectionReplayMatchesContext(t *testing.T) {
	homography := dyadicHomography()

	direct := contextPaint(func(dc *render.Context) {
		dc.SetProjection(homography)
		dc.SetRGB(1, 0, 0)
		dc.DrawRectangle(10, 10, 60, 40)
		_ = dc.Fill()
	})
	replay := contextPaint(func(dc *render.Context) {
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(projSceneWith(homography)); err != nil {
			t.Fatalf("RenderScene: %v", err)
		}
	})
	if diff := paintDiff(direct, replay); diff != 0 {
		t.Fatalf("scene 回放与直接画差 %d 像素：投影在 scene 侧被另算了一遍", diff)
	}

	flat := contextPaint(func(dc *render.Context) {
		if err := scene.NewGPUSceneRenderer(dc).RenderScene(projFlatScene()); err != nil {
			t.Fatalf("flat RenderScene: %v", err)
		}
	})
	if paintDiff(flat, replay) == 0 {
		t.Fatal("装了投影与没装投影画面一模一样：投影口是空壳？")
	}
}

func contextPaint(fn func(dc *render.Context)) *render.Pixmap {
	dc := render.NewContext(projProbeSize, projProbeSize)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	fn(dc)
	if pm := dc.PixmapForTest(); pm != nil {
		out := render.NewPixmap(projProbeSize, projProbeSize)
		for y := 0; y < projProbeSize; y++ {
			for x := 0; x < projProbeSize; x++ {
				out.SetPixel(x, y, pm.GetPixel(x, y))
			}
		}
		return out
	}
	return nil
}

func paintDiff(a, b *render.Pixmap) int {
	if a == nil || b == nil {
		return -1
	}
	n := 0
	for y := 0; y < projProbeSize; y++ {
		for x := 0; x < projProbeSize; x++ {
			pa, pb := a.GetPixel(x, y), b.GetPixel(x, y)
			if pa.R != pb.R || pa.G != pb.G || pa.B != pb.B || pa.A != pb.A {
				n++
			}
		}
	}
	return n
}

// 4 号判据（CPU 瓦片腿的表态：仿射提升照画、真投影拒画并计数）在包内测
// `p37_3e_cpu_test.go` 里——Renderer.Render 一有加速器就走 GPU 优先腿，
// 外部测试够不到瓦片执行器（先例 paint_correctness_test.go）。

// 5：合并口不吞投影流。
func TestP373ESceneProjectionSurvivesMerge(t *testing.T) {
	homography := dyadicHomography()

	host := projFlatScene()
	guest := projSceneWith(homography)
	hostTags := len(host.Encoding().Tags())
	guestTags := len(guest.Encoding().Tags())
	host.Append(guest)
	if got := len(host.Encoding().Tags()); got != hostTags+guestTags {
		t.Fatalf("Append 后 tag 总数 %d，want %d", got, hostTags+guestTags)
	}
	if n := countProjectionTags(host.Encoding()); n != 1 {
		t.Fatalf("Append 吞了投影流：tag 数 %d", n)
	}
	found := false
	dec := scene.NewDecoder(host.Encoding())
	for dec.Next() {
		if dec.Tag() == scene.TagProjection {
			if _, m := dec.Projection(); m == homography {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("合并后读不回那份投影矩阵")
	}

	// 带位移的合并（RepaintBoundary 走这条路）同样不许丢。
	host2 := projFlatScene()
	host2.AppendWithTranslation(projSceneWith(homography), 5, 5)
	if n := countProjectionTags(host2.Encoding()); n == 0 {
		t.Fatal("AppendWithTranslation 吞了投影流（default panic 没兜住 ⇒ 检查 case 是否漏登记）")
	}

	// 投影是粘滞状态：合并**不**注入恒等复位（子内容/后段本来就画在同一层投影下，
	// 照抄流才是对的；曾经加过复位守卫，实测把父投影整段取消，画面差 2350 像素）。
	host3 := projSceneWith(homography)
	host3.Append(projFlatScene())
	dec3 := scene.NewDecoder(host3.Encoding())
	var seen []render.ProjectionMode
	for dec3.Next() {
		if dec3.Tag() == scene.TagProjection {
			mode, _ := dec3.Projection()
			seen = append(seen, mode)
		}
	}
	if len(seen) != 1 || seen[0] != render.ProjectionSet {
		t.Fatalf("合并流里的投影 tag 应为照抄的 1 枚 Set：%v", seen)
	}
}

// 6：边界钉——2.5D 与 ui 侧对 render/scene 零引用（scene 的 4×4 口不属于生产 2.5D 路）。
func TestP373ESceneProjectionStaysOutOfEngine(t *testing.T) {
	root := repoRootForTest(t)
	const importPath = `"github.com/energye/gpui/render/scene"`
	for _, dir := range []string{"engine", "ui"} {
		hits := []string{}
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(raw), importPath) {
				hits = append(hits, strings.TrimPrefix(path, root+"/"))
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
		if len(hits) > 0 {
			t.Errorf("%s/ 引用了 render/scene（4×4 投影口只服务通用保留式画布，2.5D 走「算好再喂」）：%v", dir, hits)
		}
	}
}

func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	for i := 0; i < 12; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("找不到仓库根（go.mod）")
	return ""
}
