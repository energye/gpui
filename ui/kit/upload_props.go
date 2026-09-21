package kit

import (
	"strings"
)

// UploadFileStatus names the per-file upload phase.
type UploadFileStatus string

const (
	UploadStatusNone      UploadFileStatus = ""
	UploadStatusUploading UploadFileStatus = "uploading"
	UploadStatusDone      UploadFileStatus = "done"
	UploadStatusError     UploadFileStatus = "error"
	UploadStatusRemoved   UploadFileStatus = "removed"
)

// UploadListType selects the list chrome.
type UploadListType string

const (
	UploadListText          UploadListType = "text"
	UploadListPicture       UploadListType = "picture"
	UploadListPictureCard   UploadListType = "picture-card"
	UploadListPictureCircle UploadListType = "picture-circle"
)

// UploadType selects the trigger shape.
type UploadType string

const (
	UploadTypeSelect UploadType = "select"
	UploadTypeDrag   UploadType = "drag"
)

// UploadBeforeAction is the beforeUpload verdict.
type UploadBeforeAction string

const (
	// UploadBeforeProceed uploads normally.
	UploadBeforeProceed UploadBeforeAction = "proceed"
	// UploadBeforeSkip keeps the file in the list without uploading.
	UploadBeforeSkip UploadBeforeAction = "skip"
	// UploadBeforeIgnore drops the file entirely (LIST_IGNORE).
	UploadBeforeIgnore UploadBeforeAction = "ignore"
)

// UploadFile is one list entry.
type UploadFile struct {
	UID      string
	Name     string
	Size     int64
	Type     string
	Path     string
	URL      string
	ThumbURL string
	Status   UploadFileStatus
	Percent  float64
	Response string
	Error    string
}

// UploadLocalFile is one picked/dropped/pasted file before listing.
type UploadLocalFile struct {
	Name string
	Path string
	Type string
	Size int64
}

// UploadChangeParam is the onChange payload.
type UploadChangeParam struct {
	File     UploadFile
	FileList []UploadFile
	Percent  float64
}

// UploadRequestOptions is the customRequest contract. The business
// implementation drives the upload and reports back through the
// three callbacks; the default request (nil customRequest) advances
// on Tick instead.
type UploadRequestOptions struct {
	File       UploadFile
	OnProgress func(percent float64)
	OnSuccess  func(body string)
	OnError    func(message string)
}

// UploadFilePicker injects the system file dialog. Tests use a fake.
type UploadFilePicker func() ([]UploadLocalFile, bool)

// UploadProps configures one upload host. Multiple gates the host file
// dialog (single vs multi-select); programmatic SelectFiles still lists
// the batch and MaxCount trims. ShowUploadSet is reserved for the P1
// showUploadList object form; ShowUploadList toggles the list for P0.
type UploadProps struct {
	Type            UploadType
	ListType        UploadListType
	Disabled        bool
	Multiple        bool
	Pastable        bool
	Controlled      bool
	ShowUploadList  bool
	ShowUploadSet   bool
	MaxCount        int
	Accept          string
	TriggerLabel    string
	DragText        string
	DragHint        string
	AriaLabel       string
	DefaultFileList []UploadFile
	BeforeUpload    func(file UploadLocalFile, batch []UploadLocalFile) UploadBeforeAction
	CustomRequest   func(opts UploadRequestOptions)
	OnChange        func(p UploadChangeParam)
	OnRemove        func(file UploadFile) bool
	OnPreview       func(file UploadFile)
	OnDrop          func(files []UploadLocalFile)
	PreviewFile     func(local UploadLocalFile) (string, bool)
	Picker          UploadFilePicker
	PasteProvider   func() []UploadLocalFile
}

// DefaultUploadProps returns antd 6.5.1 aligned defaults.
func DefaultUploadProps() UploadProps {
	return UploadProps{
		Type:           UploadTypeSelect,
		ListType:       UploadListText,
		ShowUploadList: true,
		ShowUploadSet:  false,
		TriggerLabel:   "Click to Upload",
		DragText:       "Click or drag file to this area to upload",
		DragHint:       "Support for a single or bulk upload.",
	}
}

// DefaultUploadDraggerProps returns the drag-type sugar defaults.
func DefaultUploadDraggerProps() UploadProps {
	p := DefaultUploadProps()
	p.Type = UploadTypeDrag
	return p
}

// MatchUploadAccept reports whether name/mime passes the accept filter.
// Empty accept passes everything. Tokens are comma-separated:
// ".png" matches extension (case-insensitive), "image/*" matches MIME
// prefix, anything else matches MIME exactly.
func MatchUploadAccept(accept, name, mime string) bool {
	accept = strings.TrimSpace(accept)
	if accept == "" {
		return true
	}
	ext := ""
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		ext = strings.ToLower(name[i:])
	}
	mime = strings.ToLower(strings.TrimSpace(mime))
	for _, tok := range strings.Split(accept, ",") {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "" {
			continue
		}
		if strings.HasPrefix(tok, ".") {
			if ext != "" && ext == tok {
				return true
			}
			continue
		}
		if strings.HasSuffix(tok, "/*") {
			if mime != "" && strings.HasPrefix(mime, tok[:len(tok)-1]) {
				return true
			}
			continue
		}
		if mime != "" && mime == tok {
			return true
		}
	}
	return false
}

// IsUploadImage reports the default thumbnail rule by extension.
func IsUploadImage(name, mime string) bool {
	if strings.HasPrefix(strings.ToLower(mime), "image/") {
		return true
	}
	lower := strings.ToLower(name)
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".svg", ".ico"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
