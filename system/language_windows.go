package system

var procGetUserDefaultUILanguage = kernel32.NewProc("GetUserDefaultUILanguage")

// systemLanguage maps the Windows UI language to the two UI languages shipped
// with DeepFocus. Everything non-Chinese falls back to English.
func systemLanguage() string {
	langID, _, _ := procGetUserDefaultUILanguage.Call()
	if langID&0x3ff == 0x04 {
		return "zh"
	}
	return "en"
}
