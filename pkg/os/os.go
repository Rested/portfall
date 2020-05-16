package os

import (
	"github.com/pkg/browser"
	"github.com/wailsapp/wails"
	"portfall/pkg/logger"
	"runtime/debug"
)

// PortfallOS manages os related functionality such as opening files or browsers
type PortfallOS struct {
	rt *wails.Runtime
	log *logger.CustomLogger
}

// OpenFile opens the system dialog to get a file and return it to the frontend
func (p *PortfallOS) OpenFile() string {
	file := p.rt.Dialog.SelectFile()
	return file
}

// OpenInBrowser opens the operating system browser at the specified url
func (p *PortfallOS) OpenInBrowser(openUrl string) {
	err := browser.OpenURL(openUrl)
	if err != nil {
		p.log.Errorf("%v", err)
	}
}

func (p *PortfallOS) GetVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		p.log.Warn("Could not get build info")
	}
	p.log.Debugf("Got version %s", bi.Main.Version)
	return bi.Main.Version
}

// WailsInit assigns the runtime to the PortfallOS struct
func (p *PortfallOS) WailsInit(runtime *wails.Runtime) error {
	p.rt = runtime
	p.log = logger.NewCustomLogger("PortfallOS", runtime)
	return nil
}
