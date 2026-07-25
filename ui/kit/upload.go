package kit

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Upload defaults — components/upload/style prepareComponentToken.
// docs/antd/upload.md §6.2 / §6.10
const (
	DefaultUploadPictureCardSize = 102.0 // controlHeightLG(40) * 2.55
	DefaultUploadThumbnailSize   = 48.0  // fontSizeHeading3(24) * 2
	DefaultUploadFocusOutset     = 1.5
	DefaultUploadProgressStroke  = 2.0
	DefaultUploadTriggerLabel    = "Click to Upload"
	DefaultUploadDragText        = "Click or drag file to this area to upload"
	DefaultUploadDragHint        = "Support for a single or bulk upload."
	DefaultUploadCardLabel       = "Upload"
	DefaultUploadItemHeightText  = 22.0 // ≈ lineHeight * fontSize
	DefaultUploadDemoTickStep    = 18.0 // percent per tick for default request
	DefaultUploadDragMinHeight   = 180.0
	DefaultUploadDragIconSize    = 48.0
)

// UploadType is antd type: select | drag.
type UploadType int

const (
	UploadSelect UploadType = iota // default
	UploadDrag
)

// UploadListType is antd listType.
type UploadListType int

const (
	UploadListText UploadListType = iota // default
	UploadListPicture
	UploadListPictureCard
	UploadListPictureCircle
)

// UploadFileStatus is antd UploadFile.status.
type UploadFileStatus string

const (
	UploadStatusEmpty     UploadFileStatus = ""
	UploadStatusUploading UploadFileStatus = "uploading"
	UploadStatusDone      UploadFileStatus = "done"
	UploadStatusError     UploadFileStatus = "error"
	UploadStatusRemoved   UploadFileStatus = "removed"
)

// UploadBeforeAction is the result of beforeUpload.
type UploadBeforeAction int

const (
	// UploadProceed continues to customRequest / default upload (status=uploading).
	UploadProceed UploadBeforeAction = iota
	// UploadSkipUpload adds the file to the list without starting upload
	// (antd beforeUpload return false).
	UploadSkipUpload
	// UploadReject drops the file (not added to list).
	UploadReject
)

// UploadLocalFile is a host-selected file (path/name/mime/size).
// Desktop substitute for browser File / RcFile.
type UploadLocalFile struct {
	Name string
	Path string
	Type string // MIME or extension hint
	Size int64
}

// UploadFile is antd UploadFile (P0 fields).
type UploadFile struct {
	UID      string
	Name     string
	Size     int64
	Type     string
	Path     string
	URL      string
	Status   UploadFileStatus
	Percent  float64 // 0..100
	ThumbURL string
	Response any
	Error    string
	// Origin carries the local pick used to start upload.
	Origin UploadLocalFile
}

// UploadProgressEvent is onChange event.percent payload.
type UploadProgressEvent struct {
	Percent float64
}

// UploadChangeParam is antd onChange info.
type UploadChangeParam struct {
	File     UploadFile
	FileList []UploadFile
	Event    *UploadProgressEvent
}

// UploadRequestOptions is customRequest options (antd RequestOptions subset).
type UploadRequestOptions struct {
	File       UploadFile
	OnProgress func(percent float64)
	OnSuccess  func(response any)
	OnError    func(err error)
}

// UploadFilePicker is the host CapFile dialog (inject in tests).
type UploadFilePicker interface {
	// PickFiles opens a file dialog. filters are accept tokens (".png", "image/*").
	// ok=false means cancel.
	PickFiles(title string, filters []string, multiple bool) (files []UploadLocalFile, ok bool)
}

// Upload is Ant Design Upload (select / drag / list).
//
//	Flex Root (column; picture-card/circle: wrap row for trigger+items)
//	  ├─ Trigger Pressable
//	  └─ List (optional)
//
// Product contract: docs/antd/upload.md §6 (P0 DoD).
// Desktop main path: customRequest (default simulated upload via Ticker).
type Upload struct {
	Root    *primitive.Flex
	trigger *primitive.Pressable
	list    *primitive.Flex

	// Config
	Type           UploadType
	ListType       UploadListType
	Disabled       bool
	Multiple       bool
	Pastable       bool
	Controlled     bool
	ShowUploadList bool
	MaxCount       int // 0 = unlimited; 1 = replace
	Accept         string
	TriggerLabel   string
	DragText       string
	DragHint       string
	AriaLabel      string
	TriggerNode    core.Node // custom children; nil → default chrome

	// Data
	fileList []UploadFile

	// Hooks
	BeforeUpload  func(file UploadLocalFile, batch []UploadLocalFile) UploadBeforeAction
	CustomRequest func(UploadRequestOptions)
	OnChange      func(UploadChangeParam)
	OnRemove      func(UploadFile) bool
	OnPreview     func(UploadFile)
	OnDrop        func([]UploadLocalFile)

	// Host
	Picker        UploadFilePicker
	PasteProvider func() []UploadLocalFile

	Face  text.Face
	Theme *core.Theme
	Style Style

	// Internal
	uidSeq     uint64
	boundTree  *core.Tree
	simUploads map[string]*uploadSim
}

type uploadSim struct {
	uid     string
	percent float64
	done    bool
	err     bool
}

// NewUpload creates a select-type Upload. Optional triggerLabel overrides default.
func NewUpload(triggerLabel ...string) *Upload {
	u := &Upload{
		Type:           UploadSelect,
		ListType:       UploadListText,
		ShowUploadList: true,
		TriggerLabel:   DefaultUploadTriggerLabel,
		DragText:       DefaultUploadDragText,
		DragHint:       DefaultUploadDragHint,
		simUploads:     map[string]*uploadSim{},
	}
	if len(triggerLabel) > 0 && triggerLabel[0] != "" {
		u.TriggerLabel = triggerLabel[0]
	}
	u.rebuild()
	return u
}

// NewUploadDragger creates type=drag Upload (antd Upload.Dragger).
func NewUploadDragger(hint ...string) *Upload {
	u := NewUpload()
	u.Type = UploadDrag
	if len(hint) > 0 && hint[0] != "" {
		u.DragHint = hint[0]
	}
	u.rebuild()
	return u
}

// Node returns the root.
func (u *Upload) Node() core.Node {
	if u == nil {
		return nil
	}
	if u.Root == nil {
		u.rebuild()
	}
	return u.Root
}

// TriggerPressable returns the trigger pressable (tests / focus).
func (u *Upload) TriggerPressable() *primitive.Pressable {
	if u.trigger == nil {
		u.rebuild()
	}
	return u.trigger
}

// ListRoot returns the list flex (may be nil when showUploadList=false).
func (u *Upload) ListRoot() *primitive.Flex { return u.list }

// FileList returns a copy of the current list.
func (u *Upload) FileList() []UploadFile {
	if u == nil {
		return nil
	}
	out := make([]UploadFile, len(u.fileList))
	copy(out, u.fileList)
	return out
}

// SetType sets select | drag.
func (u *Upload) SetType(t UploadType) {
	if u.Type == t {
		return
	}
	u.Type = t
	u.rebuild()
}

// SetListType sets text | picture | picture-card | picture-circle.
func (u *Upload) SetListType(t UploadListType) {
	if u.ListType == t {
		return
	}
	u.ListType = t
	u.rebuild()
}

// SetDisabled disables interaction.
func (u *Upload) SetDisabled(d bool) {
	u.Disabled = d
	if u.trigger != nil {
		u.trigger.SetDisabled(d)
	}
	u.applyChrome()
	u.rebuildListOnly()
}

// SetMultiple allows multi-file pick.
func (u *Upload) SetMultiple(v bool) { u.Multiple = v }

// SetPastable enables Ctrl/Cmd+V paste path when focused.
func (u *Upload) SetPastable(v bool) { u.Pastable = v }

// SetControlled marks parent-owned fileList (onChange does not mutate local list).
func (u *Upload) SetControlled(v bool) { u.Controlled = v }

// SetShowUploadList toggles the file list UI.
func (u *Upload) SetShowUploadList(v bool) {
	if u.ShowUploadList == v {
		return
	}
	u.ShowUploadList = v
	u.rebuild()
}

// SetMaxCount limits file count (0 unlimited; 1 replace).
func (u *Upload) SetMaxCount(n int) {
	if n < 0 {
		n = 0
	}
	u.MaxCount = n
	u.rebuild()
}

// SetAccept sets accept filter (".png,.jpg,image/*").
func (u *Upload) SetAccept(s string) { u.Accept = s }

// SetTriggerLabel sets default button / card label.
func (u *Upload) SetTriggerLabel(s string) {
	u.TriggerLabel = s
	u.rebuild()
}

// SetDragText sets drag primary text.
func (u *Upload) SetDragText(s string) {
	u.DragText = s
	u.rebuild()
}

// SetDragHint sets drag secondary hint.
func (u *Upload) SetDragHint(s string) {
	u.DragHint = s
	u.rebuild()
}

// SetTriggerNode sets custom trigger content (antd children).
func (u *Upload) SetTriggerNode(n core.Node) {
	u.TriggerNode = n
	u.rebuild()
}

// SetFileList sets the file list (controlled or force).
func (u *Upload) SetFileList(list []UploadFile) {
	u.fileList = cloneUploadFiles(list)
	u.rebuildListOnly()
	u.rebuild() // maxCount may hide trigger
}

// SetDefaultFileList sets initial list when empty and not controlled.
func (u *Upload) SetDefaultFileList(list []UploadFile) {
	if u.Controlled && len(u.fileList) > 0 {
		return
	}
	if len(u.fileList) > 0 {
		return
	}
	u.fileList = cloneUploadFiles(list)
	u.rebuildListOnly()
	u.rebuild()
}

// SetBeforeUpload sets beforeUpload hook.
func (u *Upload) SetBeforeUpload(fn func(UploadLocalFile, []UploadLocalFile) UploadBeforeAction) {
	u.BeforeUpload = fn
}

// SetCustomRequest sets customRequest (nil restores default simulated upload).
func (u *Upload) SetCustomRequest(fn func(UploadRequestOptions)) {
	u.CustomRequest = fn
}

// SetOnChange sets onChange.
func (u *Upload) SetOnChange(fn func(UploadChangeParam)) { u.OnChange = fn }

// SetOnRemove sets onRemove; return false to prevent removal.
func (u *Upload) SetOnRemove(fn func(UploadFile) bool) { u.OnRemove = fn }

// SetOnPreview sets onPreview.
func (u *Upload) SetOnPreview(fn func(UploadFile)) { u.OnPreview = fn }

// SetOnDrop sets onDrop.
func (u *Upload) SetOnDrop(fn func([]UploadLocalFile)) { u.OnDrop = fn }

// SetPicker injects CapFile host dialog.
func (u *Upload) SetPicker(p UploadFilePicker) { u.Picker = p }

// SetPasteProvider injects clipboard-file reader for pastable demos/tests.
func (u *Upload) SetPasteProvider(fn func() []UploadLocalFile) { u.PasteProvider = fn }

// SetAriaLabel sets accessible name for the trigger.
func (u *Upload) SetAriaLabel(s string) {
	u.AriaLabel = s
	u.applyA11y()
}

// SetFace sets text face.
func (u *Upload) SetFace(face text.Face) {
	u.Face = face
	u.rebuild()
}

// SetTheme sets product theme.
func (u *Upload) SetTheme(th *core.Theme) {
	u.Theme = th
	u.rebuild()
}

// SetStyle sets optional style overrides.
func (u *Upload) SetStyle(st Style) {
	u.Style = st
	u.applyChrome()
}

// AttachTicker binds default upload simulation / loading animation.
func (u *Upload) AttachTicker(t *core.Tree) {
	if u == nil || t == nil {
		return
	}
	u.boundTree = t
	t.BindTicker(u, u.needsTicker())
}

// Tick advances simulated uploads. Implements core.Ticker.
func (u *Upload) Tick(dt float64) bool {
	if u == nil {
		return false
	}
	if len(u.simUploads) == 0 {
		return false
	}
	// Advance each sim; fire progress / done.
	var finished []string
	for uid, sim := range u.simUploads {
		if sim == nil || sim.done {
			finished = append(finished, uid)
			continue
		}
		sim.percent += DefaultUploadDemoTickStep
		if sim.percent >= 100 {
			sim.percent = 100
			sim.done = true
			u.finishSim(uid, sim)
			finished = append(finished, uid)
			continue
		}
		u.progressSim(uid, sim.percent)
	}
	for _, uid := range finished {
		delete(u.simUploads, uid)
	}
	return len(u.simUploads) > 0
}

func (u *Upload) needsTicker() bool { return len(u.simUploads) > 0 }

// SelectFiles injects host-selected files (tests / CapFile after pick).
func (u *Upload) SelectFiles(files []UploadLocalFile) {
	if u == nil || u.Disabled || len(files) == 0 {
		return
	}
	u.ingest(files)
}

// DropFiles handles drag-drop files (UPL-S9 / UPL-10).
func (u *Upload) DropFiles(files []UploadLocalFile) {
	if u == nil || u.Disabled || len(files) == 0 {
		return
	}
	if u.OnDrop != nil {
		u.OnDrop(files)
	}
	u.ingest(files)
}

// PasteFiles handles paste upload (UPL-19).
func (u *Upload) PasteFiles(files []UploadLocalFile) {
	if u == nil || u.Disabled || !u.Pastable || len(files) == 0 {
		return
	}
	u.ingest(files)
}

// RemoveFile removes by uid (respects onRemove).
func (u *Upload) RemoveFile(uid string) {
	if u == nil || u.Disabled {
		return
	}
	idx := u.indexOfUID(uid)
	if idx < 0 {
		return
	}
	file := u.fileList[idx]
	if u.OnRemove != nil && !u.OnRemove(file) {
		return
	}
	file.Status = UploadStatusRemoved
	next := make([]UploadFile, 0, len(u.fileList)-1)
	for i, f := range u.fileList {
		if i != idx {
			next = append(next, f)
		}
	}
	u.commitChange(file, next, nil)
	delete(u.simUploads, uid)
}

// ---------------------------------------------------------------------------
// Internals: ingest / change / request
// ---------------------------------------------------------------------------

func (u *Upload) ingest(files []UploadLocalFile) {
	// accept filter
	filtered := make([]UploadLocalFile, 0, len(files))
	for _, f := range files {
		if u.accepts(f) {
			filtered = append(filtered, f)
		}
	}
	if len(filtered) == 0 {
		return
	}
	// multiple: when false keep first
	if !u.Multiple && len(filtered) > 1 {
		filtered = filtered[:1]
	}

	// maxCount
	if u.MaxCount == 1 {
		// replace: only last of batch
		if len(filtered) > 1 {
			filtered = filtered[len(filtered)-1:]
		}
	} else if u.MaxCount > 1 {
		room := u.MaxCount - len(u.fileList)
		if room <= 0 {
			return
		}
		if len(filtered) > room {
			filtered = filtered[:room]
		}
	}

	// beforeUpload + build new list
	next := cloneUploadFiles(u.fileList)
	if u.MaxCount == 1 {
		next = next[:0]
	}

	type staged struct {
		file   UploadFile
		upload bool
	}
	var stagedFiles []staged

	for _, loc := range filtered {
		action := UploadProceed
		if u.BeforeUpload != nil {
			action = u.BeforeUpload(loc, filtered)
		}
		if action == UploadReject {
			continue
		}
		uf := u.localToFile(loc)
		if action == UploadSkipUpload {
			// antd: list may contain file without auto upload (status empty)
			uf.Status = UploadStatusEmpty
			stagedFiles = append(stagedFiles, staged{file: uf, upload: false})
		} else {
			uf.Status = UploadStatusUploading
			uf.Percent = 0
			stagedFiles = append(stagedFiles, staged{file: uf, upload: true})
		}
		// maxCount=1 already cleared; else append
		next = updateUploadList(uf, next)
	}
	if len(stagedFiles) == 0 {
		return
	}

	// Fire onChange once per file (antd compatibility: one event per file).
	for _, st := range stagedFiles {
		u.commitChange(st.file, next, nil)
		if st.upload {
			u.startRequest(st.file)
		}
	}
}

func (u *Upload) startRequest(file UploadFile) {
	opts := UploadRequestOptions{
		File: file,
		OnProgress: func(percent float64) {
			u.applyProgress(file.UID, percent)
		},
		OnSuccess: func(response any) {
			u.applySuccess(file.UID, response)
		},
		OnError: func(err error) {
			msg := "upload error"
			if err != nil {
				msg = err.Error()
			}
			u.applyError(file.UID, msg)
		},
	}
	if u.CustomRequest != nil {
		u.CustomRequest(opts)
		return
	}
	// Default simulated upload via Ticker.
	u.simUploads[file.UID] = &uploadSim{uid: file.UID, percent: 0}
	if u.boundTree != nil {
		u.boundTree.AddTicker(u)
	}
}

func (u *Upload) progressSim(uid string, percent float64) {
	u.applyProgress(uid, percent)
}

func (u *Upload) finishSim(uid string, sim *uploadSim) {
	if sim != nil && sim.err {
		u.applyError(uid, "upload failed")
		return
	}
	u.applySuccess(uid, map[string]string{"url": "demo://" + uid})
}

func (u *Upload) applyProgress(uid string, percent float64) {
	idx := u.indexOfUID(uid)
	if idx < 0 {
		return
	}
	f := u.fileList[idx]
	f.Status = UploadStatusUploading
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	f.Percent = percent
	next := updateUploadList(f, u.fileList)
	ev := &UploadProgressEvent{Percent: percent}
	u.commitChange(f, next, ev)
}

func (u *Upload) applySuccess(uid string, response any) {
	idx := u.indexOfUID(uid)
	if idx < 0 {
		return
	}
	f := u.fileList[idx]
	f.Status = UploadStatusDone
	f.Percent = 100
	f.Response = response
	if f.URL == "" {
		if m, ok := response.(map[string]string); ok {
			f.URL = m["url"]
		}
	}
	next := updateUploadList(f, u.fileList)
	u.commitChange(f, next, nil)
}

func (u *Upload) applyError(uid string, msg string) {
	idx := u.indexOfUID(uid)
	if idx < 0 {
		return
	}
	f := u.fileList[idx]
	f.Status = UploadStatusError
	f.Error = msg
	next := updateUploadList(f, u.fileList)
	u.commitChange(f, next, nil)
}

func (u *Upload) commitChange(file UploadFile, next []UploadFile, ev *UploadProgressEvent) {
	// Always keep an internal merged list so progress/error can resolve by uid
	// (antd rc-upload does the same). Controlled demos re-assert via SetFileList
	// in OnChange (may slice / rewrite); that replaces this list.
	u.fileList = cloneUploadFiles(next)
	if u.OnChange != nil {
		u.OnChange(UploadChangeParam{
			File:     file,
			FileList: cloneUploadFiles(next),
			Event:    ev,
		})
	}
	u.rebuildListOnly()
}

func (u *Upload) shouldHideTrigger() bool {
	if u.MaxCount <= 0 {
		return false
	}
	// antd hides card trigger when fileList.length >= maxCount
	if u.ListType == UploadListPictureCard || u.ListType == UploadListPictureCircle {
		return len(u.fileList) >= u.MaxCount
	}
	return false
}

func (u *Upload) localToFile(loc UploadLocalFile) UploadFile {
	uid := u.nextUID()
	name := loc.Name
	if name == "" {
		name = filepath.Base(loc.Path)
	}
	if name == "" {
		name = "file"
	}
	return UploadFile{
		UID:    uid,
		Name:   name,
		Size:   loc.Size,
		Type:   loc.Type,
		Path:   loc.Path,
		Origin: loc,
	}
}

func (u *Upload) nextUID() string {
	n := atomic.AddUint64(&u.uidSeq, 1)
	return fmt.Sprintf("upload-%d", n)
}

func (u *Upload) indexOfUID(uid string) int {
	for i, f := range u.fileList {
		if f.UID == uid {
			return i
		}
	}
	return -1
}

func (u *Upload) accepts(f UploadLocalFile) bool {
	acc := strings.TrimSpace(u.Accept)
	if acc == "" {
		return true
	}
	parts := strings.Split(acc, ",")
	name := strings.ToLower(f.Name)
	if name == "" {
		name = strings.ToLower(filepath.Base(f.Path))
	}
	mime := strings.ToLower(f.Type)
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, ".") {
			if strings.HasSuffix(name, p) {
				return true
			}
			continue
		}
		if strings.HasSuffix(p, "/*") {
			prefix := strings.TrimSuffix(p, "/*")
			if strings.HasPrefix(mime, prefix+"/") {
				return true
			}
			// extension fallback for image/*
			if prefix == "image" {
				for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg"} {
					if strings.HasSuffix(name, ext) {
						return true
					}
				}
			}
			continue
		}
		// exact mime
		if mime == p {
			return true
		}
	}
	return false
}

func (u *Upload) acceptFilters() []string {
	if strings.TrimSpace(u.Accept) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(u.Accept, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func cloneUploadFiles(in []UploadFile) []UploadFile {
	if len(in) == 0 {
		return nil
	}
	out := make([]UploadFile, len(in))
	copy(out, in)
	return out
}

func updateUploadList(file UploadFile, list []UploadFile) []UploadFile {
	out := make([]UploadFile, 0, len(list)+1)
	found := false
	for _, f := range list {
		if f.UID == file.UID {
			out = append(out, file)
			found = true
		} else {
			out = append(out, f)
		}
	}
	if !found {
		out = append(out, file)
	}
	return out
}

// ---------------------------------------------------------------------------
// Theme / chrome
// ---------------------------------------------------------------------------

func (u *Upload) theme() *core.Theme {
	var n core.Node
	if u.Root != nil {
		n = u.Root
	}
	return themeOf(u.Theme, n)
}

func (u *Upload) pictureCardSize() float64 {
	th := u.theme()
	hLG := th.SizeOr(core.TokenControlHeightLG, 40)
	return hLG * 2.55 // antd prepareComponentToken
}

func (u *Upload) thumbnailSize() float64 {
	// fontSizeHeading3 ≈ 24; token not always present → default 48
	return DefaultUploadThumbnailSize
}

func (u *Upload) applyA11y() {
	if u.trigger == nil {
		return
	}
	name := u.AriaLabel
	if name == "" {
		name = u.TriggerLabel
	}
	if name == "" {
		name = "Upload"
	}
	u.trigger.Base().Label = name
	u.trigger.Base().Role = "button"
}

func (u *Upload) applyChrome() {
	if u.trigger == nil {
		return
	}
	u.trigger.SetDisabled(u.Disabled)
	u.applyA11y()
	// Drag chrome border hover is applied inside rebuild trigger body.
	if u.Root != nil {
		u.Root.MarkNeedsPaint()
	}
}

// ---------------------------------------------------------------------------
// rebuild
// ---------------------------------------------------------------------------

func (u *Upload) rebuild() {
	th := u.theme()
	cardMode := u.ListType == UploadListPictureCard || u.ListType == UploadListPictureCircle

	// Root axis: card modes place trigger + items in a wrapping row (antd).
	axis := core.AxisVertical
	if cardMode {
		axis = core.AxisHorizontal
	}
	if u.Root == nil {
		u.Root = primitive.NewFlex(axis)
		u.Root.Init(u.Root)
	} else {
		u.Root.Axis = axis
		u.Root.ClearChildren()
	}
	u.Root.Gap = th.SizeOr(core.TokenMarginXS, 4)
	u.Root.CrossAlign = core.CrossStart
	u.Root.MainAlign = core.MainStart
	u.Root.Wrap = cardMode
	u.Root.ExpandMax = u.Type == UploadDrag
	u.Root.Base().Role = "group"
	if u.AriaLabel != "" {
		u.Root.Base().Label = u.AriaLabel
	}

	hideTrig := u.shouldHideTrigger()
	if !hideTrig {
		u.buildTrigger(th, cardMode)
		if u.trigger != nil {
			u.Root.AddChild(u.trigger)
		}
	} else {
		u.trigger = nil
	}

	if u.ShowUploadList {
		u.buildList(th, cardMode)
		if u.list != nil {
			u.Root.AddChild(u.list)
		}
	} else {
		u.list = nil
	}
	u.Root.MarkNeedsLayout()
	u.Root.MarkNeedsPaint()
}

func (u *Upload) rebuildListOnly() {
	if u.Root == nil {
		u.rebuild()
		return
	}
	// Full rebuild is simpler and keeps maxCount trigger visibility correct.
	u.rebuild()
}

func (u *Upload) buildTrigger(th *core.Theme, cardMode bool) {
	var body core.Node
	if u.TriggerNode != nil {
		body = u.TriggerNode
	} else if u.Type == UploadDrag {
		body = u.buildDragBody(th)
	} else if cardMode {
		body = u.buildCardTriggerBody(th)
	} else {
		body = u.buildButtonTriggerBody(th)
	}

	if u.trigger == nil {
		u.trigger = primitive.NewPressable(body)
	} else {
		u.trigger.ClearChildren()
		if body != nil {
			u.trigger.AddChild(body)
		}
	}
	u.trigger.EnableRipple = false
	u.trigger.ShowFocusRing = true
	u.trigger.FocusRingOutset = DefaultUploadFocusOutset
	u.trigger.FocusRingRadius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	u.trigger.SetDisabled(u.Disabled)
	u.trigger.Click = func() { u.onTriggerClick() }
	u.trigger.Base().Role = "button"
	u.applyA11y()
}

func (u *Upload) onTriggerClick() {
	if u.Disabled {
		return
	}
	// Prefer host picker.
	if u.Picker != nil {
		files, ok := u.Picker.PickFiles("Upload", u.acceptFilters(), u.Multiple)
		if !ok {
			return
		}
		u.SelectFiles(files)
		return
	}
	// Headless / no CapFile: no automatic fake pick (antd requires user/host).
	// Gallery demos inject Picker or call SelectFiles.
}

func (u *Upload) buildButtonTriggerBody(th *core.Theme) core.Node {
	// Compose Button chrome without nesting a second Pressable: paint equivalent
	// outlined default button (icon + label) as the trigger child.
	label := u.TriggerLabel
	if label == "" {
		label = DefaultUploadTriggerLabel
	}
	h := th.SizeOr(core.TokenControlHeight, 32)
	padInline := th.SizeOr(core.TokenButtonPaddingInline, 15)
	font := th.SizeOr(core.TokenFontSize, 14)
	radius := th.SizeOr(core.TokenBorderRadius, 6)
	lineW := th.SizeOr(core.TokenLineWidth, 1)

	row := primitive.Row()
	row.Gap = 8
	row.CrossAlign = core.CrossCenter
	row.MainAlign = core.MainCenter

	ic := primitive.NewIcon("plus")
	ic.Size = font
	ic.Color = th.Color(core.TokenColorText)
	if u.Disabled {
		ic.Color = th.Color(core.TokenColorDisabledText)
	}
	row.AddChild(ic)

	t := primitive.NewText(label)
	t.FontSize = font
	t.Color = th.Color(core.TokenColorText)
	if u.Disabled {
		t.Color = th.Color(core.TokenColorDisabledText)
	}
	if u.Face != nil {
		t.Face = u.Face
	}
	row.AddChild(t)

	dec := primitive.NewDecorated(row)
	dec.Height = h
	dec.Padding = primitive.EdgeInsets{Left: padInline, Right: padInline, Top: 0, Bottom: 0}
	dec.Radius = radius
	dec.BorderWidth = lineW
	dec.Background = th.Color(core.TokenColorBgContainer)
	dec.BorderColor = th.Color(core.TokenColorBorder)
	if u.Disabled {
		dec.Background = th.Color(core.TokenColorDisabledBg)
		dec.BorderColor = th.Color(core.TokenColorBorder)
	}
	dec.CenterContent = true
	return dec
}

func (u *Upload) buildCardTriggerBody(th *core.Theme) core.Node {
	sz := u.pictureCardSize()
	radius := th.SizeOr(core.TokenBorderRadiusLG, 8)
	if u.ListType == UploadListPictureCircle {
		radius = sz / 2
	}
	lineW := th.SizeOr(core.TokenLineWidth, 1)
	bg := th.Color(core.TokenColorFillSecondary) // ≈ colorFillAlter
	bd := th.Color(core.TokenColorBorder)
	if u.Disabled {
		bd = th.Color(core.TokenColorDisabledText)
	}

	col := primitive.Column()
	col.MainAlign = core.MainCenter
	col.CrossAlign = core.CrossCenter
	col.Gap = th.SizeOr(core.TokenMarginXS, 4)

	ic := primitive.NewIcon("plus")
	ic.Size = 20
	ic.Color = th.Color(core.TokenColorTextSecondary)
	if u.Disabled {
		ic.Color = th.Color(core.TokenColorDisabledText)
	}
	col.AddChild(ic)

	lab := primitive.NewText(DefaultUploadCardLabel)
	if u.TriggerLabel != "" && u.TriggerLabel != DefaultUploadTriggerLabel {
		lab.Value = u.TriggerLabel
	}
	lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
	lab.Color = th.Color(core.TokenColorTextSecondary)
	if u.Disabled {
		lab.Color = th.Color(core.TokenColorDisabledText)
	}
	if u.Face != nil {
		lab.Face = u.Face
	}
	col.AddChild(lab)

	dec := primitive.NewDecorated(col)
	dec.Width = sz
	dec.Height = sz
	dec.Radius = radius
	dec.BorderWidth = lineW
	dec.BorderDash = []float64{4, 3}
	dec.Background = bg
	dec.BorderColor = bd
	dec.StretchChild = true
	dec.CenterContent = true
	return dec
}

func (u *Upload) buildDragBody(th *core.Theme) core.Node {
	lineW := th.SizeOr(core.TokenLineWidth, 1)
	radius := th.SizeOr(core.TokenBorderRadiusLG, 8)
	pad := th.SizeOr(core.TokenPadding, 16)
	bg := th.Color(core.TokenColorFillSecondary)
	bd := th.Color(core.TokenColorBorder)
	if u.Disabled {
		bd = th.Color(core.TokenColorDisabledText)
	}

	col := primitive.Column()
	col.MainAlign = core.MainCenter
	col.CrossAlign = core.CrossCenter
	col.Gap = th.SizeOr(core.TokenMarginSM, 8)

	// Inbox icon fallback: use "plus" large as drag icon (antd InboxOutlined).
	ic := primitive.NewIcon("plus")
	ic.Size = DefaultUploadDragIconSize
	ic.Color = th.Color(core.TokenColorPrimary)
	if u.Disabled {
		ic.Color = th.Color(core.TokenColorDisabledText)
	}
	col.AddChild(ic)

	textMain := u.DragText
	if textMain == "" {
		textMain = DefaultUploadDragText
	}
	t1 := primitive.NewText(textMain)
	t1.FontSize = th.SizeOr(core.TokenFontSizeLG, 16)
	t1.Color = th.Color(core.TokenColorText)
	if u.Disabled {
		t1.Color = th.Color(core.TokenColorDisabledText)
	}
	if u.Face != nil {
		t1.Face = u.Face
	}
	col.AddChild(t1)

	hint := u.DragHint
	if hint == "" {
		hint = DefaultUploadDragHint
	}
	t2 := primitive.NewText(hint)
	t2.FontSize = th.SizeOr(core.TokenFontSize, 14)
	t2.Color = th.Color(core.TokenColorTextSecondary)
	if u.Disabled {
		t2.Color = th.Color(core.TokenColorDisabledText)
	}
	if u.Face != nil {
		t2.Face = u.Face
	}
	col.AddChild(t2)

	dec := primitive.NewDecorated(col)
	dec.Padding = primitive.All(pad)
	dec.Radius = radius
	dec.BorderWidth = lineW
	dec.BorderDash = []float64{4, 3}
	dec.Background = bg
	dec.BorderColor = bd
	dec.MinHeight = DefaultUploadDragMinHeight
	dec.ExpandWidth = true
	dec.StretchChild = true
	dec.CenterContent = true
	return dec
}

func (u *Upload) buildList(th *core.Theme, cardMode bool) {
	axis := core.AxisVertical
	if cardMode {
		axis = core.AxisHorizontal
	}
	if u.list == nil {
		u.list = primitive.NewFlex(axis)
		u.list.Init(u.list)
	} else {
		u.list.Axis = axis
		u.list.ClearChildren()
	}
	u.list.Gap = th.SizeOr(core.TokenMarginXS, 4)
	u.list.CrossAlign = core.CrossStart
	u.list.Wrap = cardMode
	u.list.Base().Role = "list"

	for i := range u.fileList {
		item := u.buildListItem(th, u.fileList[i], cardMode)
		if item != nil {
			u.list.AddChild(item)
		}
	}
}

func (u *Upload) buildListItem(th *core.Theme, file UploadFile, cardMode bool) core.Node {
	if cardMode {
		return u.buildCardItem(th, file)
	}
	return u.buildTextPictureItem(th, file)
}

func (u *Upload) buildTextPictureItem(th *core.Theme, file UploadFile) core.Node {
	padXS := th.SizeOr(core.TokenPaddingXS, 4)
	font := th.SizeOr(core.TokenFontSize, 14)
	lineW := th.SizeOr(core.TokenLineWidth, 1)
	radius := th.SizeOr(core.TokenBorderRadiusSM, 4)

	row := primitive.Row()
	row.CrossAlign = core.CrossCenter
	row.Gap = padXS
	row.Base().Role = "listitem"
	row.Base().Label = file.Name

	// Leading icon / thumb
	if u.ListType == UploadListPicture {
		thumbSz := u.thumbnailSize()
		thumb := primitive.NewDecorated()
		thumb.Width = thumbSz
		thumb.Height = thumbSz
		thumb.Radius = th.SizeOr(core.TokenBorderRadius, 6)
		thumb.BorderWidth = lineW
		thumb.BorderColor = th.Color(core.TokenColorBorder)
		thumb.Background = th.Color(core.TokenColorFillSecondary)
		// Placeholder glyph
		ic := primitive.NewIcon("info")
		ic.Size = 18
		ic.Color = th.Color(core.TokenColorPrimary)
		if file.Status == UploadStatusError {
			ic.Color = th.Color(core.TokenColorError)
			thumb.BorderColor = th.Color(core.TokenColorError)
		}
		inner := primitive.NewDecorated(ic)
		inner.Width = thumbSz
		inner.Height = thumbSz
		inner.CenterContent = true
		inner.StretchChild = true
		inner.Background = thumb.Background
		inner.BorderWidth = lineW
		inner.BorderColor = thumb.BorderColor
		inner.Radius = thumb.Radius
		row.AddChild(inner)
	} else {
		ic := primitive.NewIcon("info")
		ic.Size = font
		ic.Color = th.Color(core.TokenColorTextSecondary)
		if file.Status == UploadStatusError {
			ic.Color = th.Color(core.TokenColorError)
		}
		row.AddChild(ic)
	}

	// Name (preview on click)
	nameColor := th.Color(core.TokenColorText)
	if file.Status == UploadStatusError {
		nameColor = th.Color(core.TokenColorError)
	}
	name := primitive.NewText(file.Name)
	name.FontSize = font
	name.Color = nameColor
	name.Ellipsis = true
	name.MaxWidth = 240
	if u.Face != nil {
		name.Face = u.Face
	}
	namePress := primitive.NewPressable(name)
	namePress.EnableRipple = false
	namePress.ShowFocusRing = false
	namePress.SetDisabled(u.Disabled)
	uid := file.UID
	namePress.Click = func() {
		if u.Disabled {
			return
		}
		f := u.fileByUID(uid)
		if u.OnPreview != nil {
			u.OnPreview(f)
		}
	}
	row.AddChild(namePress)

	// Status text
	if file.Status == UploadStatusError && file.Error != "" {
		errT := primitive.NewText(file.Error)
		errT.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
		errT.Color = th.Color(core.TokenColorError)
		if u.Face != nil {
			errT.Face = u.Face
		}
		row.AddChild(errT)
	}

	// Remove
	rmIcon := primitive.NewIcon("close")
	rmIcon.Size = font
	rmIcon.Color = th.Color(core.TokenColorTextSecondary)
	if file.Status == UploadStatusError {
		rmIcon.Color = th.Color(core.TokenColorError)
	}
	rm := primitive.NewPressable(rmIcon)
	rm.EnableRipple = false
	rm.ShowFocusRing = true
	rm.SetDisabled(u.Disabled)
	rm.Base().Role = "button"
	rm.Base().Label = "Remove " + file.Name
	rm.Click = func() { u.RemoveFile(uid) }
	row.AddChild(rm)

	// Shell with optional progress under
	shell := primitive.Column(row)
	shell.Gap = 2
	shell.CrossAlign = core.CrossStart

	if file.Status == UploadStatusUploading {
		prog := NewProgress(file.Percent)
		prog.SetShowInfo(false)
		prog.SetWidth(200)
		if u.Theme != nil {
			prog.SetTheme(u.Theme)
		}
		shell.AddChild(prog.Node())
	}

	dec := primitive.NewDecorated(shell)
	dec.Padding = primitive.EdgeInsets{Left: padXS, Right: padXS, Top: 0, Bottom: 0}
	dec.Radius = radius
	if u.ListType == UploadListPicture {
		dec.Padding = primitive.All(padXS)
		dec.BorderWidth = lineW
		dec.BorderColor = th.Color(core.TokenColorBorder)
		dec.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
		if file.Status == UploadStatusError {
			dec.BorderColor = th.Color(core.TokenColorError)
		}
		if file.Status == UploadStatusUploading {
			dec.BorderDash = []float64{4, 3}
		}
	}
	// Hover fill for text list (controlItemBgHover ≈ fill secondary)
	if file.Status != UploadStatusError {
		// static default; hover would need pressable wrap — skip pixel hover for P0 list rows
	}
	return dec
}

func (u *Upload) buildCardItem(th *core.Theme, file UploadFile) core.Node {
	sz := u.pictureCardSize()
	lineW := th.SizeOr(core.TokenLineWidth, 1)
	radius := th.SizeOr(core.TokenBorderRadiusLG, 8)
	if u.ListType == UploadListPictureCircle {
		radius = sz / 2
	}
	padXS := th.SizeOr(core.TokenPaddingXS, 4)

	// Content: name centered or status
	col := primitive.Column()
	col.MainAlign = core.MainCenter
	col.CrossAlign = core.CrossCenter
	col.Gap = padXS

	label := file.Name
	if file.Status == UploadStatusUploading {
		label = fmt.Sprintf("%.0f%%", file.Percent)
	}
	if file.Status == UploadStatusError {
		label = "Error"
	}
	t := primitive.NewText(label)
	t.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
	t.Color = th.Color(core.TokenColorTextSecondary)
	t.Ellipsis = true
	t.MaxWidth = sz - padXS*2
	if file.Status == UploadStatusError {
		t.Color = th.Color(core.TokenColorError)
	}
	if u.Face != nil {
		t.Face = u.Face
	}
	col.AddChild(t)

	// Actions row: preview + remove
	actions := primitive.Row()
	actions.Gap = 8
	actions.MainAlign = core.MainCenter
	actions.CrossAlign = core.CrossCenter

	uid := file.UID
	prevIcon := primitive.NewIcon("search")
	prevIcon.Size = 14
	prevIcon.Color = th.Color(core.TokenColorText)
	prev := primitive.NewPressable(prevIcon)
	prev.EnableRipple = false
	prev.ShowFocusRing = false
	prev.SetDisabled(u.Disabled)
	prev.Base().Label = "Preview " + file.Name
	prev.Click = func() {
		if u.OnPreview != nil {
			u.OnPreview(u.fileByUID(uid))
		}
	}
	actions.AddChild(prev)

	rmIcon := primitive.NewIcon("close")
	rmIcon.Size = 14
	rmIcon.Color = th.Color(core.TokenColorText)
	rm := primitive.NewPressable(rmIcon)
	rm.EnableRipple = false
	rm.ShowFocusRing = false
	rm.SetDisabled(u.Disabled)
	rm.Base().Label = "Remove " + file.Name
	rm.Click = func() { u.RemoveFile(uid) }
	actions.AddChild(rm)

	col.AddChild(actions)

	dec := primitive.NewDecorated(col)
	dec.Width = sz
	dec.Height = sz
	dec.Radius = radius
	dec.BorderWidth = lineW
	dec.Background = th.Color(core.TokenColorBgContainer)
	dec.BorderColor = th.Color(core.TokenColorBorder)
	dec.StretchChild = true
	dec.CenterContent = true
	if file.Status == UploadStatusError {
		dec.BorderColor = th.Color(core.TokenColorError)
	}
	if file.Status == UploadStatusUploading {
		dec.BorderDash = []float64{4, 3}
	}
	dec.Base().Role = "listitem"
	dec.Base().Label = file.Name
	return dec
}

func (u *Upload) fileByUID(uid string) UploadFile {
	if i := u.indexOfUID(uid); i >= 0 {
		return u.fileList[i]
	}
	return UploadFile{UID: uid}
}
