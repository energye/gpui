package merge

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// SceneLite is the subset of scene JSON needed for captions.
type SceneLite struct {
	ID    string           `json:"id"`
	Size  [2]int           `json:"size"`
	Scale float64          `json:"scale,omitempty"`
	Note  string           `json:"note,omitempty"`
	Ops   []map[string]any `json:"ops"`
}

// LoadSceneLite reads a scene JSON file.
func LoadSceneLite(path string) (*SceneLite, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s SceneLite
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// DescribeSceneCN builds a human-readable Chinese expected-effect description
// from scene drawing ops (background, shapes, text, clip/layer...).
func DescribeSceneCN(s *SceneLite) string {
	if s == nil {
		return "（无场景数据）"
	}
	var b strings.Builder
	w, h := s.Size[0], s.Size[1]
	if w > 0 && h > 0 {
		fmt.Fprintf(&b, "画布：%d×%d", w, h)
		if s.Scale > 0 && s.Scale != 1 {
			fmt.Fprintf(&b, "（缩放×%.2f）", s.Scale)
		}
		b.WriteString("。\n")
	}

	bg := detectBackground(s.Ops)
	fmt.Fprintf(&b, "背景：%s。\n", bg)

	// Collect readable steps (limit to keep footer usable).
	var steps []string
	clipDepth, layerDepth := 0, 0
	for _, op := range s.Ops {
		name, _ := op["op"].(string)
		switch name {
		case "clear":
			// already in background
		case "fill_rect":
			steps = append(steps, fmt.Sprintf("实心矩形 %s %s", rectStr(op), colorStr(op)))
		case "fill_rrect":
			steps = append(steps, fmt.Sprintf("圆角矩形 %s 半径%s %s", rectStr(op), numStr(op, "radius"), colorStr(op)))
		case "fill_circle":
			steps = append(steps, fmt.Sprintf("实心圆 心(%.0f,%.0f) 半径%s %s", num(op, "cx"), num(op, "cy"), numStr(op, "radius"), colorStr(op)))
		case "fill_path":
			steps = append(steps, fmt.Sprintf("填充路径(%d点) %s", pointsN(op), colorStr(op)))
		case "stroke_line":
			steps = append(steps, fmt.Sprintf("线段 (%.0f,%.0f)-(%.0f,%.0f) 线宽%s %s%s",
				num(op, "x"), num(op, "y"), num(op, "x2"), num(op, "y2"), numStr(op, "width"), colorStr(op), dashStr(op)))
		case "stroke_rect":
			steps = append(steps, fmt.Sprintf("描边矩形 %s 线宽%s %s%s", rectStr(op), numStr(op, "width"), colorStr(op), dashStr(op)))
		case "stroke_rrect":
			steps = append(steps, fmt.Sprintf("描边圆角矩形 %s 线宽%s %s", rectStr(op), numStr(op, "width"), colorStr(op)))
		case "stroke_circle":
			steps = append(steps, fmt.Sprintf("描边圆 心(%.0f,%.0f) 线宽%s %s", num(op, "cx"), num(op, "cy"), numStr(op, "width"), colorStr(op)))
		case "stroke_path":
			steps = append(steps, fmt.Sprintf("描边路径(%d点) 线宽%s %s%s", pointsN(op), numStr(op, "width"), colorStr(op), dashStr(op)))
		case "fill_text":
			steps = append(steps, fmt.Sprintf("文字「%s」位置(%.0f,%.0f) %s", textStr(op), num(op, "x"), num(op, "y"), colorStr(op)))
		case "stroke_text":
			steps = append(steps, fmt.Sprintf("描边文字「%s」位置(%.0f,%.0f) %s", textStr(op), num(op, "x"), num(op, "y"), colorStr(op)))
		case "draw_image_solid":
			steps = append(steps, fmt.Sprintf("纯色图块 %dx%d 于(%.0f,%.0f) %s",
				int(num(op, "img_w")), int(num(op, "img_h")), num(op, "x"), num(op, "y"), colorStr(op)))
		case "clip_rect":
			clipDepth++
			steps = append(steps, fmt.Sprintf("裁剪矩形 %s（之后内容仅在此区域内可见）", rectStr(op)))
		case "clip_rrect":
			clipDepth++
			steps = append(steps, fmt.Sprintf("裁剪圆角矩形 %s", rectStr(op)))
		case "clip_path":
			clipDepth++
			steps = append(steps, "裁剪路径")
		case "reset_clip":
			if clipDepth > 0 {
				clipDepth--
			}
			steps = append(steps, "取消裁剪（恢复可见区域）")
		case "layer_begin":
			layerDepth++
			steps = append(steps, fmt.Sprintf("开始图层 透明度%s 混合%s", numStr(op, "opacity"), blendStr(op)))
		case "layer_end":
			if layerDepth > 0 {
				layerDepth--
			}
			steps = append(steps, "结束图层并合成")
		case "set_blend":
			steps = append(steps, "混合模式="+blendStr(op))
		case "set_mask_alpha":
			steps = append(steps, "启用遮罩（后续绘制受遮罩调制）")
		case "clear_mask":
			steps = append(steps, "清除遮罩")
		case "translate":
			steps = append(steps, fmt.Sprintf("平移(%.0f,%.0f)", num(op, "x"), num(op, "y")))
		case "scale":
			steps = append(steps, fmt.Sprintf("缩放(%.2f,%.2f)", num(op, "x"), numDef(op, "y", num(op, "x"))))
		case "rotate":
			steps = append(steps, fmt.Sprintf("旋转%.1f°", num(op, "angle")*180/3.14159265))
		case "push":
			steps = append(steps, "保存变换")
		case "pop":
			steps = append(steps, "恢复变换")
		}
	}

	// Summarize if too many
	const maxSteps = 18
	b.WriteString("应看到：\n")
	if len(steps) == 0 {
		b.WriteString("- （场景无有效绘制步骤）\n")
	} else if len(steps) <= maxSteps {
		for i, s := range steps {
			fmt.Fprintf(&b, "%d) %s\n", i+1, s)
		}
	} else {
		// keep first and last chunks
		keepHead, keepTail := 12, 4
		for i := 0; i < keepHead; i++ {
			fmt.Fprintf(&b, "%d) %s\n", i+1, steps[i])
		}
		fmt.Fprintf(&b, "… 中间省略 %d 步 …\n", len(steps)-keepHead-keepTail)
		for i := len(steps) - keepTail; i < len(steps); i++ {
			fmt.Fprintf(&b, "%d) %s\n", i+1, steps[i])
		}
	}

	// Quick visual checklist
	b.WriteString("辨认要点：左=CanvasKit标准，右=gpui待测；二者应结构一致（颜色、位置、裁剪与文字大致吻合，允许抗锯齿细微差）。")
	return strings.TrimSpace(b.String())
}

func detectBackground(ops []map[string]any) string {
	// Prefer last clear, else first large fill_rect covering origin.
	for i := len(ops) - 1; i >= 0; i-- {
		if ops[i]["op"] == "clear" {
			return colorStr(ops[i]) + "清屏"
		}
	}
	for _, op := range ops {
		if op["op"] == "fill_rect" {
			return colorStr(op) + "铺底矩形"
		}
	}
	return "未明确（默认白底）"
}

func colorStr(op map[string]any) string {
	rgba, _ := op["rgba"].([]any)
	if len(rgba) < 3 {
		return "颜色未知"
	}
	r, g, b := asF(rgba[0]), asF(rgba[1]), asF(rgba[2])
	a := 1.0
	if len(rgba) >= 4 {
		a = asF(rgba[3])
	}
	name := colorName(r, g, b, a)
	if a < 0.999 {
		return fmt.Sprintf("%s(RGBA %.2f,%.2f,%.2f,%.2f)", name, r, g, b, a)
	}
	return fmt.Sprintf("%s(RGB %.2f,%.2f,%.2f)", name, r, g, b)
}

func colorName(r, g, b, a float64) string {
	if a < 0.08 {
		return "近透明"
	}
	// coarse buckets for quick recognition
	if r > 0.85 && g > 0.85 && b > 0.85 {
		return "白色/浅色"
	}
	if r < 0.2 && g < 0.2 && b < 0.2 {
		return "黑色/深色"
	}
	if r > g && r > b && r-g > 0.15 {
		return "偏红"
	}
	if g > r && g > b && g-r > 0.15 {
		return "偏绿"
	}
	if b > r && b > g && b-r > 0.15 {
		return "偏蓝"
	}
	if r > 0.5 && g > 0.5 && b < 0.4 {
		return "偏黄"
	}
	if r > 0.4 && g < 0.45 && b > 0.4 {
		return "偏紫"
	}
	if r > 0.4 && g > 0.35 && b > 0.3 && max3(r, g, b)-min3(r, g, b) < 0.2 {
		return "灰色"
	}
	return "彩色"
}

func max3(a, b, c float64) float64 {
	if a < b {
		a = b
	}
	if a < c {
		a = c
	}
	return a
}
func min3(a, b, c float64) float64 {
	if a > b {
		a = b
	}
	if a > c {
		a = c
	}
	return a
}

func rectStr(op map[string]any) string {
	rect, _ := op["rect"].([]any)
	if len(rect) < 4 {
		return "[?]"
	}
	return fmt.Sprintf("[x=%.0f y=%.0f w=%.0f h=%.0f]", asF(rect[0]), asF(rect[1]), asF(rect[2]), asF(rect[3]))
}

func num(op map[string]any, key string) float64 {
	return asF(op[key])
}
func numDef(op map[string]any, key string, def float64) float64 {
	if op[key] == nil {
		return def
	}
	return asF(op[key])
}
func numStr(op map[string]any, key string) string {
	return fmt.Sprintf("%.2g", num(op, key))
}
func textStr(op map[string]any) string {
	s, _ := op["text"].(string)
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) > 24 {
		rs := []rune(s)
		return string(rs[:24]) + "…"
	}
	return s
}
func pointsN(op map[string]any) int {
	pts, _ := op["points"].([]any)
	return len(pts)
}
func dashStr(op map[string]any) string {
	d, _ := op["dash"].([]any)
	if len(d) == 0 {
		return ""
	}
	return " 虚线"
}
func blendStr(op map[string]any) string {
	s, _ := op["blend"].(string)
	if s == "" {
		return "normal"
	}
	return s
}
func asF(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	default:
		return 0
	}
}
