package kit

import (
	"strconv"
	"sync"

	"github.com/energye/gpui/ui/kit/internal/scope"
)

// defaultUploadTickRate advances default-request uploads in percent
// per virtual second. Progress never blocks the caller: Tick drives it.
const defaultUploadTickRate = 50.0

// UploadInstance owns the file list plus upload sessions for one host.
type UploadInstance struct {
	props   UploadProps
	ctx     scope.Ctx
	mu      sync.Mutex
	files   []UploadFile
	seq     uint64
	pending map[string]bool

	ctl     bool
	mounted bool
}

func newUploadInstance(ctx scope.Ctx, props UploadProps) *UploadInstance {
	in := &UploadInstance{
		props:   props,
		ctx:     ctx.Normalize(),
		pending: make(map[string]bool),
		ctl:     props.Controlled,
	}
	for _, f := range props.DefaultFileList {
		cp := f
		if cp.UID == "" {
			in.seq++
			cp.UID = "uid-" + strconv.FormatUint(in.seq, 10)
		}
		in.files = append(in.files, cp)
	}
	return in
}

// Mount marks the host live.
func (in *UploadInstance) Mount(ctx scope.Ctx) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.mounted = true
}

// Update swaps props/ctx snapshots. Controlled list wins.
func (in *UploadInstance) Update(ctx scope.Ctx, next UploadProps) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.props = next
	in.ctl = next.Controlled
}

// Unmount marks the host dead.
func (in *UploadInstance) Unmount() {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.mounted = false
}

// SetState runs f under the lock (sole state mutation gate).
func (in *UploadInstance) SetState(f func(*UploadInstance)) {
	if in == nil || f == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	f(in)
}

// Disabled reports the OR of subtree and widget disable.
func (in *UploadInstance) Disabled() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return scope.DisabledOr(in.ctx.Disabled, in.props.Disabled)
}

// FileList returns a copy of the list in order.
func (in *UploadInstance) FileList() []UploadFile {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	return append([]UploadFile(nil), in.files...)
}

// Find returns a copy of one entry by uid.
func (in *UploadInstance) Find(uid string) (UploadFile, bool) {
	if in == nil {
		return UploadFile{}, false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	for _, f := range in.files {
		if f.UID == uid {
			return f, true
		}
	}
	return UploadFile{}, false
}

// SetFileList drives controlled list from outside without callbacks.
func (in *UploadInstance) SetFileList(files []UploadFile) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctl = true
	in.files = append([]UploadFile(nil), files...)
	in.pending = make(map[string]bool)
	for _, f := range in.files {
		if f.Status == UploadStatusUploading {
			in.pending[f.UID] = true
		}
	}
}

// SetControlled flips the controlled flag.
func (in *UploadInstance) SetControlled(controlled bool) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctl = controlled
	in.props.Controlled = controlled
}

// fireChangeLocked notifies onChange without holding the host lock.
// Caller must hold in.mu on entry; it is still held on exit.
func (in *UploadInstance) fireChangeLocked(file UploadFile, percent float64) {
	cb := in.props.OnChange
	list := append([]UploadFile(nil), in.files...)
	p := UploadChangeParam{File: file, FileList: list, Percent: percent}
	in.mu.Unlock()
	if cb != nil {
		cb(p)
	}
	in.mu.Lock()
}

func (in *UploadInstance) indexLocked(uid string) int {
	for i, f := range in.files {
		if f.UID == uid {
			return i
		}
	}
	return -1
}

// trimMaxCountLocked keeps the latest maxCount entries (1 = replace)
// and drops pending sessions for trimmed files.
func (in *UploadInstance) trimMaxCountLocked() {
	max := in.props.MaxCount
	if max <= 0 || len(in.files) <= max {
		return
	}
	for _, f := range in.files[:len(in.files)-max] {
		delete(in.pending, f.UID)
	}
	in.files = append([]UploadFile(nil), in.files[len(in.files)-max:]...)
}

// startUploadLocked begins one session: customRequest now, default on Tick.
// Caller must hold in.mu on entry; it is still held on exit. The business
// callback runs without the lock so it can report back reentrantly.
func (in *UploadInstance) startUploadLocked(f UploadFile) {
	if in.props.CustomRequest != nil {
		uid := f.UID
		cr := in.props.CustomRequest
		file := f
		in.mu.Unlock()
		cr(UploadRequestOptions{
			File: file,
			OnProgress: func(p float64) {
				in.ReportProgress(uid, p)
			},
			OnSuccess: func(body string) {
				in.ReportSuccess(uid, body)
			},
			OnError: func(msg string) {
				in.ReportError(uid, msg)
			},
		})
		in.mu.Lock()
		return
	}
	in.pending[f.UID] = true
}

// addFilesLocked lists locals after accept/beforeUpload/maxCount.
func (in *UploadInstance) addFilesLocked(locals []UploadLocalFile) []UploadFile {
	var added []UploadFile
	for _, local := range locals {
		if !MatchUploadAccept(in.props.Accept, local.Name, local.Type) {
			continue
		}
		action := UploadBeforeProceed
		if in.props.BeforeUpload != nil {
			bu := in.props.BeforeUpload
			in.mu.Unlock()
			action = bu(local, locals)
			in.mu.Lock()
		}
		if action == UploadBeforeIgnore {
			continue
		}
		in.seq++
		uid := "uid-" + strconv.FormatUint(in.seq, 10)
		f := UploadFile{
			UID:    uid,
			Name:   local.Name,
			Size:   local.Size,
			Type:   local.Type,
			Path:   local.Path,
			Status: UploadStatusUploading,
		}
		if action == UploadBeforeSkip {
			f.Status = UploadStatusNone
			f.Percent = 0
		}
		in.files = append(in.files, f)
		in.trimMaxCountLocked()
		// The entry may have been trimmed; only start what survived.
		if in.indexLocked(uid) >= 0 {
			added = append(added, f)
		}
	}
	// Start uploads after the whole batch lists (stable order).
	for _, f := range added {
		if idx := in.indexLocked(f.UID); idx >= 0 && in.files[idx].Status == UploadStatusUploading {
			in.startUploadLocked(in.files[idx])
		}
	}
	return added
}

// notifyAddedLocked fires onChange for each surviving entry.
// Caller must hold in.mu on entry; it is still held on exit.
func (in *UploadInstance) notifyAddedLocked(added []UploadFile) {
	for _, f := range added {
		if idx := in.indexLocked(f.UID); idx >= 0 {
			in.fireChangeLocked(in.files[idx], in.files[idx].Percent)
		}
	}
}

// SelectFiles lists picked files (host picker or programmatic).
func (in *UploadInstance) SelectFiles(locals []UploadLocalFile) []UploadFile {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return nil
	}
	added := in.addFilesLocked(locals)
	in.notifyAddedLocked(added)
	in.mu.Unlock()
	return added
}

// DropFiles lists dropped files and fires onDrop.
func (in *UploadInstance) DropFiles(locals []UploadLocalFile) []UploadFile {
	if in == nil {
		return nil
	}
	var onDrop func([]UploadLocalFile)
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) {
		in.mu.Unlock()
		return nil
	}
	onDrop = in.props.OnDrop
	added := in.addFilesLocked(locals)
	in.notifyAddedLocked(added)
	in.mu.Unlock()
	if onDrop != nil {
		onDrop(locals)
	}
	return added
}

// PasteFiles lists pasted files when pastable.
func (in *UploadInstance) PasteFiles(locals []UploadLocalFile) []UploadFile {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	if scope.DisabledOr(in.ctx.Disabled, in.props.Disabled) || !in.props.Pastable {
		in.mu.Unlock()
		return nil
	}
	added := in.addFilesLocked(locals)
	in.notifyAddedLocked(added)
	in.mu.Unlock()
	return added
}

// PickFromHost pulls files through the injected picker.
func (in *UploadInstance) PickFromHost() []UploadFile {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	picker := in.props.Picker
	disabled := scope.DisabledOr(in.ctx.Disabled, in.props.Disabled)
	in.mu.Unlock()
	if disabled || picker == nil {
		return nil
	}
	locals, ok := picker()
	if !ok {
		return nil
	}
	return in.SelectFiles(locals)
}

// PasteFromHost pulls files through the paste provider.
func (in *UploadInstance) PasteFromHost() []UploadFile {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	provider := in.props.PasteProvider
	in.mu.Unlock()
	if provider == nil {
		return nil
	}
	return in.PasteFiles(provider())
}

// Tick advances default-request uploads in small slices so a single
// Tick call reports intermediate progress; business customRequest files
// only move through their own callbacks.
func (in *UploadInstance) Tick(dt float64) {
	if in == nil || dt <= 0 {
		return
	}
	const slice = 0.5
	remaining := dt
	for remaining > 0 {
		step := slice
		if step > remaining {
			step = remaining
		}
		if !in.tickStep(step) {
			return
		}
		remaining -= step
	}
}

func (in *UploadInstance) tickStep(dt float64) bool {
	in.mu.Lock()
	// Sweep stale sessions, then advance the earliest uploading file
	// in list order so progress is deterministic.
	for id := range in.pending {
		if in.indexLocked(id) < 0 {
			delete(in.pending, id)
		}
	}
	var uid string
	for _, f := range in.files {
		if f.Status == UploadStatusUploading && in.pending[f.UID] {
			uid = f.UID
			break
		}
	}
	if uid == "" {
		in.mu.Unlock()
		return false
	}
	idx := in.indexLocked(uid)
	in.files[idx].Percent += dt * defaultUploadTickRate
	if in.files[idx].Percent >= 100 {
		in.files[idx].Percent = 100
		in.files[idx].Status = UploadStatusDone
		delete(in.pending, uid)
		done := in.files[idx]
		in.fireChangeLocked(done, 100)
		in.mu.Unlock()
		return true
	}
	prog := in.files[idx]
	in.fireChangeLocked(prog, prog.Percent)
	in.mu.Unlock()
	return true
}

// ReportProgress applies a customRequest progress event. Files outside
// the controlled list are ignored (antd FAQ).
func (in *UploadInstance) ReportProgress(uid string, percent float64) {
	if in == nil {
		return
	}
	in.mu.Lock()
	idx := in.indexLocked(uid)
	if idx < 0 {
		in.mu.Unlock()
		return
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	in.files[idx].Percent = percent
	if in.files[idx].Status == UploadStatusNone {
		in.files[idx].Status = UploadStatusUploading
	}
	f := in.files[idx]
	in.fireChangeLocked(f, percent)
	in.mu.Unlock()
}

// ReportSuccess applies a customRequest success event.
func (in *UploadInstance) ReportSuccess(uid, body string) {
	if in == nil {
		return
	}
	in.mu.Lock()
	idx := in.indexLocked(uid)
	if idx < 0 {
		in.mu.Unlock()
		return
	}
	delete(in.pending, uid)
	in.files[idx].Status = UploadStatusDone
	in.files[idx].Percent = 100
	in.files[idx].Response = body
	f := in.files[idx]
	in.fireChangeLocked(f, 100)
	in.mu.Unlock()
}

// ReportError applies a customRequest error event; percent freezes.
func (in *UploadInstance) ReportError(uid, message string) {
	if in == nil {
		return
	}
	in.mu.Lock()
	idx := in.indexLocked(uid)
	if idx < 0 {
		in.mu.Unlock()
		return
	}
	delete(in.pending, uid)
	in.files[idx].Status = UploadStatusError
	in.files[idx].Error = message
	f := in.files[idx]
	in.fireChangeLocked(f, f.Percent)
	in.mu.Unlock()
}

// Remove deletes one entry unless onRemove vetoes (false).
func (in *UploadInstance) Remove(uid string) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	idx := in.indexLocked(uid)
	if idx < 0 {
		in.mu.Unlock()
		return false
	}
	if in.props.OnRemove != nil {
		cb := in.props.OnRemove
		f := in.files[idx]
		in.mu.Unlock()
		if !cb(f) {
			return false
		}
		in.mu.Lock()
		idx = in.indexLocked(uid)
		if idx < 0 {
			in.mu.Unlock()
			return false
		}
	}
	removed := in.files[idx]
	removed.Status = UploadStatusRemoved
	in.files = append(in.files[:idx], in.files[idx+1:]...)
	delete(in.pending, uid)
	in.fireChangeLocked(removed, removed.Percent)
	in.mu.Unlock()
	return true
}

// Preview runs the previewFile pipeline then onPreview.
func (in *UploadInstance) Preview(uid string) (string, bool) {
	if in == nil {
		return "", false
	}
	in.mu.Lock()
	idx := in.indexLocked(uid)
	if idx < 0 {
		in.mu.Unlock()
		return "", false
	}
	f := in.files[idx]
	pf := in.props.PreviewFile
	onPreview := in.props.OnPreview
	in.mu.Unlock()
	url := f.URL
	if url == "" {
		url = f.ThumbURL
	}
	if pf != nil {
		if dataURL, ok := pf(UploadLocalFile{Name: f.Name, Path: f.Path, Type: f.Type, Size: f.Size}); ok {
			url = dataURL
		}
	}
	if onPreview != nil {
		onPreview(f)
	}
	return url, true
}

// UploadingCount reports in-flight files.
func (in *UploadInstance) UploadingCount() int {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	n := 0
	for _, f := range in.files {
		if f.Status == UploadStatusUploading {
			n++
		}
	}
	return n
}

// HolderContent renders static-call holder copy through Ctx.
func (in *UploadInstance) HolderContent() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	ctx := in.ctx
	in.mu.Unlock()
	if ctx.HolderRender == nil {
		return "upload holder"
	}
	return ctx.HolderRender("upload")
}
