//go:build !nogpu

//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// h19CountingDevice 只实现 ensureSharedLayoutsLocked 真会调到的那一个方法
// （CreateBindGroupLayout），其余方法靠内嵌接口在误用时 panic——这条钉要断的是
// 「共享 layout 是谁家设备的」，不是整套后端行为。
type h19CountingDevice struct {
	hal.Device
	name     string
	count    *h19Counters
	released int
}

type h19Counters struct{ created, destroyed int }

func (d *h19CountingDevice) CreateBindGroupLayout(desc *hal.BindGroupLayoutDescriptor) (hal.BindGroupLayout, error) {
	d.count.created++
	return &h19FakeLayout{label: desc.Label, count: d.count}, nil
}

// abandon 会真调这几个：实现出来，其余方法留给内嵌接口在误用时炸。
func (d *h19CountingDevice) IsLost() bool { return false }

func (d *h19CountingDevice) WaitIdle() error { return nil }

func (d *h19CountingDevice) Queue() hal.Queue { return nil }

func (d *h19CountingDevice) Release() { d.released++ }

type h19FakeLayout struct {
	hal.BindGroupLayout
	label string
	count *h19Counters
}

func (l *h19FakeLayout) Destroy() { l.count.destroyed++ }

// TestP376SharedLayoutsFollowDevice 钉 H19：共享 clip/mask bind group layout
// 必须跟着当前绑定的设备走，不能「非 nil 就复用」。旧设备的句柄喂给新设备的
// CreatePipelineLayout 在 wgpu-native 里是非展开 panic（整进程 SIGABRT），拿不到
// 错误返回——所以这条必须在真下驱动调用之前，把「谁建的就谁的」这个不变量直接断出来。
//
// 负向（本轮实测）：摘掉 ensureSharedLayoutsLocked 里的设备身份守卫 ⇒ created 停在 2、
// 旧句柄被复用，本钉红；守卫还原后绿。
func TestP376SharedLayoutsFollowDevice(t *testing.T) {
	count := &h19Counters{}
	devA := &h19CountingDevice{name: "A", count: count}
	devB := &h19CountingDevice{name: "B", count: count}

	s := NewGPUShared()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.device = devA
	if err := s.ensureSharedLayoutsLocked(); err != nil {
		t.Fatalf("设备 A 建共享 layout: %v", err)
	}
	if count.created != 2 || count.destroyed != 0 {
		t.Fatalf("首建 = 建 %d 毁 %d，want 2/0", count.created, count.destroyed)
	}
	if s.sharedLayoutsDev != hal.Device(devA) {
		t.Fatalf("首建后归属 = %v，want A", s.sharedLayoutsDev)
	}
	if s.clipBindLayout == nil || s.maskBindLayout == nil {
		t.Fatal("首建后两枚共享 layout 不该为 nil")
	}

	// H19 的那一步：设备换了，且没有任何 abandon 走过。
	s.device = devB
	if err := s.ensureSharedLayoutsLocked(); err != nil {
		t.Fatalf("设备 B 建共享 layout: %v", err)
	}
	if count.created != 4 {
		t.Fatalf("换设备后没重建共享 layout（还在复用 A 的句柄 = H19 硬中止路径）：created=%d want 4", count.created)
	}
	if count.destroyed != 2 {
		t.Fatalf("A 的两枚 layout 没释放（设备换了还挂着旧句柄=泄漏）：destroyed=%d want 2", count.destroyed)
	}
	if s.sharedLayoutsDev != hal.Device(devB) {
		t.Fatalf("换设备后归属 = %v，want B", s.sharedLayoutsDev)
	}

	// 同一台设备重复调用必须幂等：不重建也不误毁，「一设备 N 表面」的缓存不能白扔。
	if err := s.ensureSharedLayoutsLocked(); err != nil {
		t.Fatalf("同设备重复调用: %v", err)
	}
	if count.created != 4 || count.destroyed != 2 {
		t.Fatalf("同设备重复调用动了句柄：created=%d destroyed=%d，want 4/2", count.created, count.destroyed)
	}

	s.destroySharedLayoutsLocked()
	if count.destroyed != 4 || s.clipBindLayout != nil || s.maskBindLayout != nil || s.sharedLayoutsDev != nil {
		t.Fatalf("销毁口没清干净：destroyed=%d clip=%v mask=%v dev=%v",
			count.destroyed, s.clipBindLayout != nil, s.maskBindLayout != nil, s.sharedLayoutsDev != nil)
	}
}

// TestP376InstallGuardOnlyAbandonsOnDeviceChange 钉漏斗：initGPU 那两条重绑路径
// 现在必须过同一道身份判断——同设备重绑不动缓存，换设备才拆。
func TestP376InstallGuardOnlyAbandonsOnDeviceChange(t *testing.T) {
	count := &h19Counters{}
	devA := &h19CountingDevice{name: "A", count: count}
	devB := &h19CountingDevice{name: "B", count: count}

	s := NewGPUShared()
	s.mu.Lock()
	defer s.mu.Unlock()

	// 还没绑过任何设备：守卫直接放行，不碰 teardown。
	s.installDeviceGuardLocked(devA)
	if s.device != nil || s.deviceGen != 0 {
		t.Fatalf("空绑定时守卫动了状态：device=%v gen=%d", s.device != nil, s.deviceGen)
	}

	s.device = devA
	s.externalDevice = true // 借来的设备：abandon 不许替我们 Release
	s.deviceGen = 1
	s.installDeviceGuardLocked(devA) // 同设备重绑
	if s.device != hal.Device(devA) || s.deviceGen != 1 {
		t.Fatalf("同设备重绑被当成换设备处理：device=%v gen=%d，want 仍是 A/1", s.device != nil, s.deviceGen)
	}

	s.installDeviceGuardLocked(devB) // 换设备：必须先拆干净
	if s.device != nil {
		t.Fatal("换设备重绑前没走 abandon：旧 device 还挂在共享持有者上")
	}
	if devA.released != 0 {
		t.Fatalf("借来的设备被我们 Release 了（归属判断错）：released=%d want 0", devA.released)
	}

	// 自建设备（externalDevice=false）换绑时必须真释放，不然一台设备就漏在进程里。
	s.device = devA
	s.externalDevice = false
	s.installDeviceGuardLocked(devB)
	if devA.released != 1 {
		t.Fatalf("自建设备换绑时没释放：released=%d want 1", devA.released)
	}
}
