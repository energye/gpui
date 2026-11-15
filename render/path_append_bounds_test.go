package render

import "testing"

// 诊断钉（O13 发现第四件根因）：SetPath = Clear + Append。Clear 重置
// 增量 bounds（boundsValid=false），而 Append 只拷 verbs/coords 不同步
// bounds —— Append 后 Bounds() 返回空 rect，GPU stencil 链的 cover quad
// 从 Bounds() 计算 → 面积 0 → 整笔不落地。CPU 光栅链不读增量 bounds
// （逐边扫描），所以 CPU 腿一直绿。
func TestAppendMaintainsBounds(t *testing.T) {
	src := NewPath()
	src.MoveTo(20, 20)
	src.LineTo(140, 20)
	src.LineTo(140, 90)
	src.LineTo(20, 90)
	src.Close()
	src.MoveTo(50, 45)
	src.LineTo(50, 85)
	src.LineTo(110, 85)
	src.LineTo(110, 45)
	src.Close()

	// 源路径自身 bounds 必须有效（MoveTo/LineTo 逐点 expandBounds）。
	b := src.Bounds()
	if b.Min.X != 20 || b.Min.Y != 20 || b.Max.X != 140 || b.Max.Y != 90 {
		t.Fatalf("src bounds wrong: %v", b)
	}

	// SetPath 的实际序列：Clear + Append。
	dst := NewPath()
	dst.Clear()
	dst.Append(src)
	b2 := dst.Bounds()
	if b2.Min.X != 20 || b2.Min.Y != 20 || b2.Max.X != 140 || b2.Max.Y != 90 {
		t.Fatalf("Append 后 bounds 丢失（GPU cover quad 因此为空）: %v", b2)
	}

	// Append 进已有内容的路径：bounds 应取并集。
	dst2 := NewPath()
	dst2.MoveTo(200, 200)
	dst2.LineTo(260, 200)
	dst2.LineTo(260, 240)
	dst2.Close()
	dst2.Append(src)
	b3 := dst2.Bounds()
	if b3.Min.X != 20 || b3.Min.Y != 20 || b3.Max.X != 260 || b3.Max.Y != 240 {
		t.Fatalf("Append 并集 bounds 错: got=%v want=(20,20)-(260,240)", b3)
	}

	// 追加空路径不得破坏现有 bounds。
	dst2.Append(nil)
	dst2.Append(NewPath())
	b4 := dst2.Bounds()
	if b4.Min.X != 20 || b4.Min.Y != 20 || b4.Max.X != 260 || b4.Max.Y != 240 {
		t.Fatalf("Append 空路径改变 bounds: %v", b4)
	}
}
