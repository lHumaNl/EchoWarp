package tui

// KickOverlayState groups the fields that drive the kick confirmation overlay.
type KickOverlayState struct {
	ClientID      string
	ClientNick    string
	Reasons       []string
	RecentStart   int
	CustomStart   int
	SelectedIndex int
	CustomText    string
	CustomEditing bool
	FocusButton   int // 0=list, 1=[Kick], 2=[Cancel]
}

// BanOverlayState groups the fields that drive the ban confirmation overlay.
type BanOverlayState struct {
	ClientID        string
	ClientNick      string
	ClientIP        string
	ClientHWID      string
	CriteriaIP      bool
	CriteriaNick    bool
	CriteriaHWID    bool
	FocusSection    int // 0=criteria, 1=reasons, 2=buttons
	CriteriaIndex   int // highlighted criterion (0=IP, 1=Nick, 2=HWID)
	Reasons         []string
	RecentStart     int
	CustomStart     int
	SelectedReason  int
	CustomText      string
	CustomEditing   bool
	ButtonFocus     int // 0=[Confirm], 1=[Cancel]
	ValidationError string
}
