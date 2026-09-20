package kit

// NoticeQueueMessageType selects the message semantic icon.
type NoticeQueueMessageType string

const (
	NoticeQueueMessageInfo    NoticeQueueMessageType = "info"
	NoticeQueueMessageSuccess NoticeQueueMessageType = "success"
	NoticeQueueMessageError   NoticeQueueMessageType = "error"
	NoticeQueueMessageWarning NoticeQueueMessageType = "warning"
	NoticeQueueMessageLoading NoticeQueueMessageType = "loading"
)

// NoticeQueueNotificationType selects the notification semantic icon.
// Open carries no preset icon; the rest bring a semantic icon.
type NoticeQueueNotificationType string

const (
	NoticeQueueNotificationOpen    NoticeQueueNotificationType = "open"
	NoticeQueueNotificationInfo    NoticeQueueNotificationType = "info"
	NoticeQueueNotificationSuccess NoticeQueueNotificationType = "success"
	NoticeQueueNotificationWarning NoticeQueueNotificationType = "warning"
	NoticeQueueNotificationError   NoticeQueueNotificationType = "error"
)

// NoticeQueuePlacement selects the notification corner pool.
type NoticeQueuePlacement string

const (
	NoticeQueuePlacementTop         NoticeQueuePlacement = "top"
	NoticeQueuePlacementTopLeft     NoticeQueuePlacement = "topLeft"
	NoticeQueuePlacementTopRight    NoticeQueuePlacement = "topRight"
	NoticeQueuePlacementBottom      NoticeQueuePlacement = "bottom"
	NoticeQueuePlacementBottomLeft  NoticeQueuePlacement = "bottomLeft"
	NoticeQueuePlacementBottomRight NoticeQueuePlacement = "bottomRight"
)

// NoticeQueueAction is one notification button.
type NoticeQueueAction struct {
	Label   string
	Primary bool
	OnClick func()
}

// NoticeQueueMessageConfig is one lightweight top-center tip.
type NoticeQueueMessageConfig struct {
	Content         string
	Type            NoticeQueueMessageType
	Duration        float64
	DurationSet     bool
	Key             string
	PauseOnHover    bool
	PauseOnHoverSet bool
	IconName        string
	OnClick         func()
	OnClose         func()
	StyleBgSet      bool
	StyleBgR        float64
	StyleBgG        float64
	StyleBgB        float64
	StyleRadiusSet  bool
	StyleRadius     float64
}

// NoticeQueueNotificationConfig is one corner card.
type NoticeQueueNotificationConfig struct {
	Title           string
	Description     string
	Type            NoticeQueueNotificationType
	Duration        float64
	DurationSet     bool
	Key             string
	Placement       NoticeQueuePlacement
	PlacementSet    bool
	Closable        bool
	ClosableSet     bool
	ShowProgress    bool
	PauseOnHover    bool
	PauseOnHoverSet bool
	IconName        string
	Actions         []NoticeQueueAction
	Role            string
	OnClick         func()
	OnClose         func()
	StyleBgSet      bool
	StyleBgR        float64
	StyleBgG        float64
	StyleBgB        float64
	StyleRadiusSet  bool
	StyleRadius     float64
}

// NoticeQueueProps configures the shared message/notification host.
type NoticeQueueProps struct {
	ZIndexBase            int
	MessageTop            float64
	MessageTopSet         bool
	MessageDuration       float64
	MessageDurationSet    bool
	NotificationDuration  float64
	NotificationDurSet    bool
	NotificationPlacement NoticeQueuePlacement
	NotificationPlaceSet  bool
	NotificationTop       float64
	NotificationTopSet    bool
	NotificationBottom    float64
	NotificationBotSet    bool
	MaxCount              int
	StackEnabled          bool
	StackThreshold        int
	StackThresholdSet     bool
	PauseOnHover          bool
	PauseOnHoverSet       bool
}

// DefaultNoticeQueueProps returns antd 6.5.1 aligned defaults:
// message top 8 duration 3s, notification duration 4.5s placement
// topRight edge 24, stack off threshold 3, pause on hover on.
func DefaultNoticeQueueProps() NoticeQueueProps {
	return NoticeQueueProps{
		ZIndexBase:            1000,
		MessageTop:            8,
		MessageTopSet:         false,
		MessageDuration:       3,
		MessageDurationSet:    false,
		NotificationDuration:  4.5,
		NotificationDurSet:    false,
		NotificationPlacement: NoticeQueuePlacementTopRight,
		NotificationPlaceSet:  false,
		NotificationTop:       24,
		NotificationTopSet:    false,
		NotificationBottom:    24,
		NotificationBotSet:    false,
		MaxCount:              0,
		StackEnabled:          false,
		StackThreshold:        3,
		StackThresholdSet:     false,
		PauseOnHover:          true,
		PauseOnHoverSet:       false,
	}
}

// DefaultNoticeQueueMessageConfig returns info type with unset duration.
func DefaultNoticeQueueMessageConfig() NoticeQueueMessageConfig {
	return NoticeQueueMessageConfig{Type: NoticeQueueMessageInfo}
}

// DefaultNoticeQueueNotificationConfig returns open type, closable,
// alert role with unset placement and duration.
func DefaultNoticeQueueNotificationConfig() NoticeQueueNotificationConfig {
	return NoticeQueueNotificationConfig{
		Type:     NoticeQueueNotificationOpen,
		Closable: true,
		Role:     "alert",
	}
}
