package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/theme"
)

func TestNoticeQueue_StaticCallEatsTheme(t *testing.T) {
	base := kit.DefaultScopeCtx()
	skin := base.Theme
	skin.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := base.WithTheme(skin)
	host := kit.BuildNoticeQueue(base, kit.DefaultNoticeQueueProps())
	host.Update(skinCtx, kit.DefaultNoticeQueueProps())
	got := kit.ResolveNoticeQueue(skinCtx.Theme)
	if got.NoticeBg != skin.ColorBgElevated {
		t.Fatal("notice bg must follow reskinned theme without code change")
	}
	if host.HolderContent("message") == "" || host.HolderContent("notification") == "" {
		t.Fatal("holderRender must resolve through Ctx for both kinds")
	}
	ok := kit.NoticeQueueMessageIconBg(skin, kit.NoticeQueueMessageSuccess)
	if ok != skin.ColorSuccess {
		t.Fatal("success tip icon must use success token")
	}
	errBg := kit.NoticeQueueNotificationIconBg(skin, kit.NoticeQueueNotificationError)
	if errBg != skin.ColorError {
		t.Fatal("error card icon must use error token")
	}
	focus := kit.ResolveNoticeQueue(skin).Focus
	if focus.Width != skin.LineWidthFocus || focus.Color != skin.ColorPrimaryBorder {
		t.Fatal("focus ring must follow seed width+border")
	}
}
