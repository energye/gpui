//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerUpload() {
	// Upload — docs/antd/upload.md §6.8 P0
	// https://ant.design/components/upload
	// demos: basic / avatar / defaultFileList / picture-card / picture-circle /
	//        fileList / drag / paste
	//
	// P1 not shown: directory, manual upload, png-only, picture-style,
	// drag-sorting, crop, OSS, customize-progress, style-class.

	face, th := c.face, c.theme
	status := c.status

	wire := func(u *kit.Upload, tag string) *kit.Upload {
		u.SetFace(face)
		if th != nil {
			u.SetTheme(th)
		}
		// Demo customRequest: finish immediately with fake URL.
		u.SetCustomRequest(func(opts kit.UploadRequestOptions) {
			opts.OnProgress(60)
			opts.OnSuccess(map[string]string{"url": "demo://" + opts.File.Name})
		})
		u.SetOnChange(func(p kit.UploadChangeParam) {
			if status != nil {
				*status = fmt.Sprintf("upload %s → %s status=%s list=%d",
					tag, p.File.Name, p.File.Status, len(p.FileList))
			}
			if u.Controlled {
				u.SetFileList(p.FileList)
			}
		})
		u.SetOnPreview(func(f kit.UploadFile) {
			if status != nil {
				*status = fmt.Sprintf("preview %s url=%s", f.Name, f.URL)
			}
		})
		// Demo picker: cycle deterministic demo files.
		n := 0
		u.SetPicker(&galleryUploadPicker{next: func() []kit.UploadLocalFile {
			n++
			return []kit.UploadLocalFile{{
				Name: fmt.Sprintf("%s-%d.png", tag, n),
				Path: fmt.Sprintf("/demo/%s-%d.png", tag, n),
				Type: "image/png",
			}}
		}})
		return u
	}

	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		return f
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewUpload("Click to Upload"), "basic")
	secBasic := demoSection(face, th, "点击上传",
		"最基础的用法：触发器 + 文件列表 + onChange / customRequest。",
		spaceWrap(12, basic.Node()))

	// ---------- avatar.tsx ----------
	avatar := wire(kit.NewUpload(), "avatar")
	avatar.SetListType(kit.UploadListPictureCard)
	avatar.SetShowUploadList(false)
	avatar.SetTriggerLabel("Upload")
	avatar.SetBeforeUpload(func(f kit.UploadLocalFile, _ []kit.UploadLocalFile) kit.UploadBeforeAction {
		// Accept images only (demo gate).
		if f.Type == "image/png" || f.Type == "image/jpeg" {
			return kit.UploadProceed
		}
		if status != nil {
			*status = "avatar: only PNG/JPG"
		}
		return kit.UploadReject
	})
	avatarCircle := wire(kit.NewUpload(), "avatar-circle")
	avatarCircle.SetListType(kit.UploadListPictureCircle)
	avatarCircle.SetShowUploadList(false)
	avatarCircle.SetTriggerLabel("Upload")
	secAvatar := demoSection(face, th, "用户头像",
		"picture-card / picture-circle + showUploadList=false + beforeUpload 校验。",
		spaceWrap(16, avatar.Node(), avatarCircle.Node()))

	// ---------- defaultFileList.tsx ----------
	defl := wire(kit.NewUpload("Upload"), "default")
	defl.SetDefaultFileList([]kit.UploadFile{
		{UID: "1", Name: "xxx.png", Status: kit.UploadStatusUploading, Percent: 33, URL: "http://www.baidu.com/xxx.png"},
		{UID: "2", Name: "yyy.png", Status: kit.UploadStatusDone, URL: "http://www.baidu.com/yyy.png"},
		{UID: "3", Name: "zzz.png", Status: kit.UploadStatusError, Error: "Server Error 500", URL: "http://www.baidu.com/zzz.png"},
	})
	secDef := demoSection(face, th, "已上传的文件列表",
		"defaultFileList 展示 uploading / done / error 与 percent。",
		spaceWrap(12, defl.Node()))

	// ---------- picture-card.tsx ----------
	pc := wire(kit.NewUpload(), "picture-card")
	pc.SetListType(kit.UploadListPictureCard)
	pc.SetFileList([]kit.UploadFile{
		{UID: "-1", Name: "image.png", Status: kit.UploadStatusDone},
		{UID: "-2", Name: "image.png", Status: kit.UploadStatusDone},
		{UID: "-xxx", Name: "image.png", Status: kit.UploadStatusUploading, Percent: 50},
		{UID: "-5", Name: "image.png", Status: kit.UploadStatusError},
	})
	secPC := demoSection(face, th, "照片墙",
		"listType=picture-card：卡片格 + 上传触发卡片。",
		spaceWrap(12, pc.Node()))

	// ---------- picture-circle.tsx ----------
	pcirc := wire(kit.NewUpload(), "picture-circle")
	pcirc.SetListType(kit.UploadListPictureCircle)
	pcirc.SetFileList([]kit.UploadFile{
		{UID: "c1", Name: "a.png", Status: kit.UploadStatusDone},
		{UID: "c2", Name: "b.png", Status: kit.UploadStatusDone},
	})
	secPCirc := demoSection(face, th, "圆形照片墙",
		"listType=picture-circle。",
		spaceWrap(12, pcirc.Node()))

	// ---------- fileList.tsx controlled ----------
	ctrl := wire(kit.NewUpload("Upload"), "controlled")
	ctrl.SetControlled(true)
	ctrl.SetMultiple(true)
	ctrl.SetFileList([]kit.UploadFile{
		{UID: "-1", Name: "xxx.png", Status: kit.UploadStatusDone, URL: "http://www.baidu.com/xxx.png"},
	})
	ctrl.SetOnChange(func(p kit.UploadChangeParam) {
		list := p.FileList
		if len(list) > 2 {
			list = list[len(list)-2:]
		}
		ctrl.SetFileList(list)
		if status != nil {
			*status = fmt.Sprintf("controlled list=%d last=%s", len(list), p.File.Name)
		}
	})
	secCtrl := demoSection(face, th, "完全控制的上传列表",
		"Controlled + onChange 中切片最多保留 2 个文件。",
		spaceWrap(12, ctrl.Node()))

	// ---------- drag.tsx ----------
	drag := wire(kit.NewUploadDragger(), "drag")
	drag.SetMultiple(true)
	// Also allow programmatic drop button for headless demo without OS DnD.
	dropBtn := kit.NewButton("模拟拖入 2 个文件")
	dropBtn.SetType(kit.ButtonDefault)
	dropBtn.SetFace(face)
	dropBtn.SetOnClick(func() {
		drag.DropFiles([]kit.UploadLocalFile{
			{Name: "drop-a.txt", Path: "/demo/drop-a.txt"},
			{Name: "drop-b.txt", Path: "/demo/drop-b.txt"},
		})
	})
	*c.buttons = append(*c.buttons, dropBtn)
	secDrag := demoSection(face, th, "拖拽上传",
		"type=drag（Upload.Dragger）。桌面宿主可 DropFiles；示例按钮模拟拖入。",
		col(drag.Node(), dropBtn.Node()))

	// ---------- paste.tsx ----------
	paste := wire(kit.NewUpload("Paste or click to upload"), "paste")
	paste.SetPastable(true)
	paste.SetPasteProvider(func() []kit.UploadLocalFile {
		return []kit.UploadLocalFile{{Name: "pasted.png", Path: "/demo/pasted.png", Type: "image/png"}}
	})
	pasteBtn := kit.NewButton("模拟粘贴")
	pasteBtn.SetType(kit.ButtonDefault)
	pasteBtn.SetFace(face)
	pasteBtn.SetOnClick(func() {
		if paste.PasteProvider != nil {
			paste.PasteFiles(paste.PasteProvider())
		}
	})
	*c.buttons = append(*c.buttons, pasteBtn)
	secPaste := demoSection(face, th, "粘贴上传",
		"pastable + PasteFiles / PasteProvider（桌面剪贴板文件由宿主注入）。",
		spaceWrap(12, paste.Node(), pasteBtn.Node()))

	// ---------- disabled / maxCount extras for P0 states ----------
	dis := wire(kit.NewUpload("Disabled"), "disabled")
	dis.SetDisabled(true)
	max1 := wire(kit.NewUpload(), "max1")
	max1.SetListType(kit.UploadListPictureCard)
	max1.SetMaxCount(1)
	max1.SetFileList([]kit.UploadFile{{UID: "m1", Name: "only.png", Status: kit.UploadStatusDone}})
	secExtra := demoSection(face, th, "禁用与数量限制",
		"disabled；picture-card + maxCount=1 触顶隐藏触发器。",
		spaceWrap(16, dis.Node(), max1.Node()))

	// Lifecycle (#9)
	life := wire(kit.NewUpload("Lifecycle"), "lifecycle")
	life.SetMultiple(true)
	life.SetMaxCount(3)
	life.SetDefaultFileList([]kit.UploadFile{
		{UID: "l1", Name: "life.png", Status: kit.UploadStatusDone},
	})
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (fileList/listType) then chromeChange；ensureBuilt 懒构建。",
		spaceWrap(12, life.Node()))

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeUpload, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeUpload); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinU := wire(kit.NewUpload("Skin upload"), "skin")
	if f, ok := skinU.Node().(*primitive.Flex); ok {
		f.SkinType = kit.TypeUpload
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root Flex SkinType/TypeID=kit.Upload 已注册；Theme.Skin Override 可命中（default walk）。",
		spaceWrap(12, skinU.Node()))

	c.add("upload", "Upload", "Data Entry · Upload",
		demoPage(face, "Upload",
			"文件选择上传与拖拽上传。P0：fileList/onChange/customRequest/beforeUpload/listType/drag/paste/maxCount/accept/disabled/status/percent。",
			secBasic, secAvatar, secDef, secPC, secPCirc, secCtrl, secDrag, secPaste, secExtra, secLife, secSkin))
}

// galleryUploadPicker is a deterministic CapFile stand-in for gallery demos.
type galleryUploadPicker struct {
	next func() []kit.UploadLocalFile
}

func (p *galleryUploadPicker) PickFiles(title string, filters []string, multiple bool) ([]kit.UploadLocalFile, bool) {
	if p == nil || p.next == nil {
		return nil, false
	}
	return p.next(), true
}
