//go:build linux && !nogpu

package main

import (
	"log"
	"strings"
)

// scenarioSpec maps one window scenario to SKIA_2D_CAPABILITY_MATRIX rows.
// MatrixIDs are the authoritative capability IDs from docs/SKIA_2D_CAPABILITY_MATRIX.md
// (Skia 2D semantic parity). C01–C20 group those IDs for real X11 present verification.
// Harness code may reuse mem_anim X11 patterns; scenario content must track the matrix.
type scenarioSpec struct {
	ID     string
	Name   string
	NameCN string
	// MatrixIDs: comma-separated IDs from SKIA_2D_CAPABILITY_MATRIX.md
	MatrixIDs   string
	AllowLowFPS bool
	DamageMode  bool
	// DrawKind selects probe drawer in probes.go
	DrawKind string
	// Expect: Chinese on-screen guide of what the operator should see
	Expect string
}

func allScenarios() map[string]scenarioSpec {
	return map[string]scenarioSpec{
		// --- M0/M1 surface + paint + path (Skia Canvas/Paint/Path) ---
		"C01": {
			ID: "C01", Name: "SurfacePresentClear", NameCN: "窗口呈现/清屏",
			MatrixIDs: "S.03,S.04,S.05", DrawKind: "clear",
			Expect: "整窗清屏色相缓慢变化，中部圆横向移动（Skia clear + window present）",
		},
		"C02": {
			ID: "C02", Name: "TransformStack", NameCN: "变换栈",
			MatrixIDs: "T.01,T.02,P.01", DrawKind: "xform",
			Expect: "中心旋转缩放色块 + 左上独立旋转圆（concat/save/restore）",
		},
		"C03": {
			ID: "C03", Name: "PathFillStroke", NameCN: "路径填充+描边",
			MatrixIDs: "H.01,G.01,G.02,G.04,P.02,P.03,P.05,P.06", DrawKind: "path",
			Expect: "波浪描边 + Cap 三线(Butt/Round/Square) + 星形填充",
		},
		"C04": {
			ID: "C04", Name: "HairlineDash", NameCN: "Hairline+虚线",
			MatrixIDs: "P.04,E.01,P.08", DrawKind: "dash",
			Expect: "Hairline 虚线贝塞尔 + 脉动虚线圆（dash path effect）",
		},
		"C05": {
			ID: "C05", Name: "ClipRectRRect", NameCN: "矩形/圆角裁剪",
			MatrixIDs: "C.01,C.02,C.05,G.03", DrawKind: "clip",
			Expect: "条纹底上圆角裁剪窗与矩形裁剪窗内有运动圆",
		},
		// --- M2 UI high-frequency ---
		"C06": {
			ID: "C06", Name: "GradientPattern", NameCN: "渐变+图案",
			MatrixIDs: "D.01,D.02,D.03,D.05", DrawKind: "grad",
			Expect: "线性/径向/扫描渐变色块 + 图像 pattern 条带",
		},
		"C07": {
			ID: "C07", Name: "BlendModes", NameCN: "可分离混合",
			MatrixIDs: "B.03,B.05,B.01", DrawKind: "blend",
			Expect: "棋盘底上 Multiply橙 / Screen蓝 / Overlay黄 叠加圆",
		},
		"C08": {
			ID: "C08", Name: "LayerOpacity", NameCN: "半透明图层",
			MatrixIDs: "L.01,L.02,L.03", DrawKind: "layer",
			Expect: "双圆背景 + 半透明 saveLayer 卡片（离屏层再 DrawImage）",
		},
		"C09": {
			ID: "C09", Name: "ImageWritePixels", NameCN: "贴图+写像素",
			MatrixIDs: "I.01,I.02,I.03,S.07", DrawKind: "image",
			Expect: "棋盘贴图缩放旋转 + 右下 WritePixels 动态色块",
		},
		"C10": {
			ID: "C10", Name: "TextCJKDecor", NameCN: "中英文+装饰",
			MatrixIDs: "X.01,X.02,X.06,X.08", DrawKind: "text",
			Expect: "中英混排文本 + 下划线装饰，字号/颜色可读",
		},
		// --- M3 extended ---
		"C11": {
			ID: "C11", Name: "FilterBlurShadow", NameCN: "模糊/阴影滤镜",
			MatrixIDs: "F.01,F.02,F.04", DrawKind: "filter",
			Expect: "离屏小 RT：模糊圆 / 投影色块 / 灰度（ImageFilter）",
		},
		"C12": {
			ID: "C12", Name: "MeshVertices", NameCN: "顶点网格",
			MatrixIDs: "V.01,V.03", DrawKind: "mesh",
			Expect: "彩色 Gouraud 网格起伏变形（drawVertices/drawMesh）",
		},
		"C13": {
			ID: "C13", Name: "EvenOddPath", NameCN: "EvenOdd 填充",
			MatrixIDs: "H.03,H.01", DrawKind: "evenodd",
			Expect: "EvenOdd 圆环孔洞 vs NonZero 实心对比",
		},
		"C14": {
			ID: "C14", Name: "MaskLayer", NameCN: "蒙版图层",
			MatrixIDs: "L.06", DrawKind: "mask",
			Expect: "圆形 alpha 蒙版内可见彩色内容，蒙版外被裁掉",
		},
		"C15": {
			ID: "C15", Name: "BackdropBlur", NameCN: "背景采样层",
			MatrixIDs: "L.05,F.01", DrawKind: "backdrop",
			Expect: "动态底图上半透明 Backdrop 卡片（采样父内容）",
		},
		"C16": {
			ID: "C16", Name: "DamagePresent", NameCN: "局部 Damage Present",
			MatrixIDs: "S.09", DrawKind: "damage", DamageMode: true,
			Expect: "仅局部 dirty 区更新动画，全屏不应整帧清闪",
		},
		"C17": {
			ID: "C17", Name: "AdvBlendPanel", NameCN: "高级混合",
			MatrixIDs: "B.03,B.04,B.07", DrawKind: "advblend",
			Expect: "多混合模式色圆网格（SoftLight/Diff/HSL 相关）",
		},
		"C18": {
			ID: "C18", Name: "TextLCDShape", NameCN: "LCD 子像素文本",
			MatrixIDs: "X.04,X.05,X.02", DrawKind: "textlcd",
			Expect: "GlyphMask/LCD-RGB/Aliased 多行对照 + 中英",
		},
		"C19": {
			ID: "C19", Name: "RRectXYRadii", NameCN: "独立圆角半径",
			MatrixIDs: "G.06,G.03", DrawKind: "rrect",
			Expect: "多组 XY 独立圆角矩形并排，圆角半径可见差异",
		},
		"C20": {
			ID: "C20", Name: "CompositeUI", NameCN: "多能力 UI 合成",
			MatrixIDs: "S.03,T.01,P.01,G.01,C.01,D.01,L.03,I.01,X.02,V.01", DrawKind: "composite",
			Expect: "渐变底+变换块+文本+网格+贴图同屏，稳定 present",
		},
		// --- P2 Wave-A: remaining high-value MatrixIDs ---
		"C21": {
			ID: "C21", Name: "PorterDuffBoard", NameCN: "PorterDuff 混合板",
			MatrixIDs: "B.02", DrawKind: "porterduff",
			Expect: "色带底上 Clear/Copy/Plus/DstOut/Xor/Modulate 等 PD 色块矩阵，模式差异可见",
		},
		"C22": {
			ID: "C22", Name: "ClipPathDiff", NameCN: "路径/Difference 裁剪",
			MatrixIDs: "C.03,C.06,C.04", DrawKind: "clipdiff",
			Expect: "条纹底：圆形 path clip 内动画 + 矩形区 Difference 镂空孔洞",
		},
		"C23": {
			ID: "C23", Name: "GradientTileLocal", NameCN: "渐变 tile/局部矩阵",
			MatrixIDs: "D.04,D.06", DrawKind: "gradtile",
			Expect: "Repeat/Reflect 线性渐变条 + 旋转缩放 pattern 局部矩阵",
		},
		"C24": {
			ID: "C24", Name: "ImageAdvanced", NameCN: "图高级采样",
			MatrixIDs: "I.04,I.05,I.06,I.07", DrawKind: "imageadv",
			Expect: "mip 缩小 / 半透明 / 旋转贴图 / 九宫格拉伸同屏",
		},
		"C25": {
			ID: "C25", Name: "TextShapeEmoji", NameCN: "文本 shaping/混排",
			MatrixIDs: "X.03,X.09,X.10,X.11", DrawKind: "textadv",
			Expect: "Latin+CJK MultiFace 混排、重复串 atlas 复用；emoji 若无色字体会 tofu 仍应稳定 present",
		},
		// --- P3 Wave-B: path / transform / layer-filter / quality ---
		"C26": {
			ID: "C26", Name: "PathAdvanced", NameCN: "路径进阶",
			MatrixIDs: "H.02,G.05,H.04,H.05,E.02,E.03", DrawKind: "pathadv",
			Expect: "圆弧/椭圆弧、boolean 差集形、trim 弧长描边、corner/discrete 路径效果",
		},
		"C27": {
			ID: "C27", Name: "TransformAdvanced", NameCN: "变换进阶",
			MatrixIDs: "T.03,T.04,P.07", DrawKind: "xfmadv",
			Expect: "非均匀缩放描边、四边形透视贴图、高低 miter limit 尖角对比",
		},
		"C28": {
			ID: "C28", Name: "LayerFilterGraph", NameCN: "层混合+滤镜链",
			MatrixIDs: "L.04,F.03", DrawKind: "layerfilt",
			Expect: "半透明 layer 上 blur+色矩阵滤镜链，卡片随动画移动",
		},
		"C29": {
			ID: "C29", Name: "QualityMSAAAA", NameCN: "质量 MSAA/AA",
			MatrixIDs: "Q.01,Q.02,Q.03,Q.04,S.08", DrawKind: "quality",
			Expect: "AA 开/关斜边对比、hairline、dither 渐变带、2x 设备尺度 hairline",
		},
		// --- P5 Wave-C: atlas/picture / path fast-path / composite regression ---
		"C30": {
			ID: "C30", Name: "AtlasPicture", NameCN: "Atlas+Picture 录制",
			MatrixIDs: "V.02,R.01,S.01,S.02,S.06", DrawKind: "atlaspic",
			Expect: "DrawAtlas 多精灵 + Picture 录制回放位图合成；HUD 标注录制命令数",
		},
		"C31": {
			ID: "C31", Name: "PathRasterFast", NameCN: "路径光栅快径",
			MatrixIDs: "H.06,H.07,P.09", DrawKind: "pathfast",
			Expect: "凸多边形快径 + 非凸星形 + dither 渐变对比条",
		},
		"C32": {
			ID: "C32", Name: "CompositeRegression", NameCN: "合成压力回归",
			MatrixIDs: "S.03,T.01,P.01,G.01,C.01,D.01,B.01,L.03,I.01,X.02,V.01", DrawKind: "compreg",
			Expect: "轻量同屏：渐变底+变换+路径+裁剪+混合+贴图+文本+网格，稳定 present",
		},
	}
}

func applyScenario(id string) (scenarioSpec, bool) {
	id = strings.ToUpper(strings.TrimSpace(id))
	if id == "" {
		return scenarioSpec{}, false
	}
	s, ok := allScenarios()[id]
	if !ok {
		log.Printf("unknown GPUI_SCENARIO=%q — use C01..C32", id)
		return scenarioSpec{}, false
	}
	return s, true
}

func scenarioListOrdered() []string {
	return []string{
		"C01", "C02", "C03", "C04", "C05", "C06", "C07", "C08", "C09", "C10",
		"C11", "C12", "C13", "C14", "C15", "C16", "C17", "C18", "C19", "C20",
		"C21", "C22", "C23", "C24", "C25",
		"C26", "C27", "C28", "C29",
		"C30", "C31", "C32",
	}
}
