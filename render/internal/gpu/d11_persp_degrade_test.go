//go:build !nogpu

package gpu

import (
	"testing"
)

// 深审 L3-A2（D11）钉：透视打包/降级必须发生在「顶点指纹+上传」之前。
// 旧状 buildImageResources 先指纹+上传 allVertData，packPerspectiveQuads 才在
// 透视管线建不成时把降级仿射字节写回 i*6 槽位——写的是指纹之后的账：缓存的
// imageVertLastHash 对不上最终字节，GPU 顶点缓冲里留的是同槽位旧内容
// （首帧=清零字节画漏，后续帧=上一帧残顶点）。
// 判据（同包白盒）：降级发生后（cmd.Perspective 被清掉），imageVertLastHash
// 必须等于降级字节就位后的整段 staging 指纹。

func TestD11PerspDegradationVerticesUploaded(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	// 测试缝：强制透视管线不可用，走「降级仿射不丢绘制」分支（生产行为=文档契约）。
	t.Setenv("GPUI_TEST_PERSP_PIPELINE_FAIL", "1")

	s := NewGPURenderSession(device, queue, 1)
	defer s.Destroy()

	cmd := ImageDrawCommand{
		Opacity: 1, ViewportWidth: 32, ViewportHeight: 32,
		DstX: 4, DstY: 4, DstW: 24, DstH: 24,
		TLX: 4, TLY: 4, TRX: 28, TRY: 4, BRX: 28, BRY: 28, BLX: 4, BLY: 28,
		U0: 0, V0: 0, U1: 1, V1: 1,
		Perspective: true,
		Weights:     [4]float32{1, 1, 1, 1},
	}
	cmds := []ImageDrawCommand{cmd}
	if _, err := s.buildImageResources(cmds, 32, 32, []bool{false}); err != nil {
		t.Fatalf("buildImageResources: %v", err)
	}
	if cmds[0].Perspective {
		t.Fatal("degradation leg not exercised: Perspective should be cleared")
	}

	off := 6 * imageVertexStride
	staging := s.imageVertexStaging[:off]
	var nonzero bool
	for _, b := range staging {
		if b != 0 {
			nonzero = true
			break
		}
	}
	if !nonzero {
		t.Fatal("degraded affine bytes missing from staging slot")
	}
	if got, want := s.imageVertLastHash, indexBytesFingerprint(staging); got != want {
		t.Fatalf("uploaded vertex fingerprint=%d want %d（降级字节写在了指纹/上传之后：GPU 缓冲留旧账 → 残顶点/画漏，L3-A2）", got, want)
	}
}
