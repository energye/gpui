package input

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
)

func TestFromPlatform_DeviceAdded(t *testing.T) {
	ev := FromPlatform(platform.Event{
		Type:        platform.EventDeviceAdded,
		DeviceClass: platform.DeviceKeyboard,
		DeviceName:  "AT keyboard",
	}, Modifiers{})
	if ev.Kind != KindDeviceAdded {
		t.Fatalf("kind = %s, want device-added", ev.Kind)
	}
	if ev.Device.Class != DeviceKeyboard || ev.Device.Name != "AT keyboard" {
		t.Fatalf("device = %+v", ev.Device)
	}
}

func TestFromPlatform_DeviceRemoved(t *testing.T) {
	ev := FromPlatform(platform.Event{
		Type:        platform.EventDeviceRemoved,
		DeviceClass: platform.DeviceTouch,
		DeviceName:  "touchscreen",
	}, Modifiers{})
	if ev.Kind != KindDeviceRemoved {
		t.Fatalf("kind = %s, want device-removed", ev.Kind)
	}
	if ev.Device.Class != DeviceTouch || ev.Device.Name != "touchscreen" {
		t.Fatalf("device = %+v", ev.Device)
	}
}

func TestFromPlatform_DeviceClasses(t *testing.T) {
	cases := []struct {
		in   platform.DeviceClass
		want DeviceClass
	}{
		{platform.DeviceKeyboard, DeviceKeyboard},
		{platform.DeviceMouse, DeviceMouse},
		{platform.DeviceTouch, DeviceTouch},
		{platform.DevicePen, DevicePen},
		{platform.DeviceUnknown, DeviceUnknown},
		{platform.DeviceClass(99), DeviceUnknown},
	}
	for _, c := range cases {
		add := FromPlatform(platform.Event{Type: platform.EventDeviceAdded, DeviceClass: c.in, DeviceName: "n"}, Modifiers{})
		if add.Device.Class != c.want {
			t.Errorf("added class %d: got %s, want %s", int(c.in), add.Device.Class, c.want)
		}
		rm := FromPlatform(platform.Event{Type: platform.EventDeviceRemoved, DeviceClass: c.in}, Modifiers{})
		if rm.Kind != KindDeviceRemoved || rm.Device.Class != c.want {
			t.Errorf("removed class %d: got %s/%s", int(c.in), rm.Kind, rm.Device.Class)
		}
	}
}
