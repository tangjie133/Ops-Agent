package tui

// KeyAction 是主界面或设置页对一个按键的决定。
type KeyAction string

const (
	KeyMoveDown      KeyAction = "move_down"
	KeyMoveUp        KeyAction = "move_up"
	KeyOpenSettings  KeyAction = "open_settings"
	KeyQuit          KeyAction = "quit"
	KeyInsert        KeyAction = "insert"
	KeyCloseSettings KeyAction = "close_settings"
	KeyIgnore        KeyAction = "ignore"
)

// RouteMainKey 在输入框为空时把 j/k/s/esc 交给任务和设置，有文字时交给输入框。
func RouteMainKey(inputEmpty bool, key string) KeyAction {
	if !inputEmpty {
		return KeyInsert
	}
	switch key {
	case "j", "down":
		return KeyMoveDown
	case "k", "up":
		return KeyMoveUp
	case "s":
		return KeyOpenSettings
	case "esc":
		return KeyQuit
	default:
		return KeyIgnore
	}
}

// RouteSettingsKey 让设置页的 Esc 只关闭设置页。
func RouteSettingsKey(key string) KeyAction {
	if key == "esc" {
		return KeyCloseSettings
	}
	return KeyIgnore
}
