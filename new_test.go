package browsers

import "testing"

func TestBrowserController_LaunchBrowser(t *testing.T) {
	controller := NewBrowserController()

	for i := 0; i < 20; i++ {
		controller.LaunchBrowser(BrowserOptions{
			Path:        "C:\\Users\\luoxk\\AppData\\Local\\Chromium\\Application\\chrome.exe",
			Fingerprint: "",
			Proxy:       "",
			UserDir:     "",
			Headless:    false,
			HookFunc:    nil,
		})
	}
	controller.CloseAllBrowsers()

}
