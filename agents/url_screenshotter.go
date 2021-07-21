package agents

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/VasilyKaiser/aquatone/core"
	"github.com/chromedp/chromedp"
)

type URLScreenshotter struct {
	session         *core.Session
	tempUserDirPath string
}

func NewURLScreenshotter() *URLScreenshotter {
	return &URLScreenshotter{}
}

func (a *URLScreenshotter) ID() string {
	return "agent:url_screenshotter"
}

func (a *URLScreenshotter) Register(s *core.Session) error {
	s.EventBus.SubscribeAsync(core.URLResponsive, a.OnURLResponsive, false)
	s.EventBus.SubscribeAsync(core.SessionEnd, a.OnSessionEnd, false)
	a.session = s
	a.createTempUserDir()
	// a.locateChrome()

	return nil
}

func (a *URLScreenshotter) OnURLResponsive(url string) {
	a.session.Out.Debug("[%s] Received new responsive URL %s\n", a.ID(), url)
	page := a.session.GetPage(url)
	if page == nil {
		a.session.Out.Error("Unable to find page for URL: %s\n", url)
		return
	}

	a.session.WaitGroup.Add()
	go func(page *core.Page) {
		defer a.session.WaitGroup.Done()
		a.screenshotPage(page)
	}(page)
}

func (a *URLScreenshotter) OnSessionEnd() {
	a.session.Out.Debug("[%s] Received SessionEnd event\n", a.ID())
	os.RemoveAll(a.tempUserDirPath)
	a.session.Out.Debug("[%s] Deleted temporary user directory at: %s\n", a.ID(), a.tempUserDirPath)
}

func (a *URLScreenshotter) createTempUserDir() {
	dir, err := ioutil.TempDir("", "aquatone-chrome")
	if err != nil {
		a.session.Out.Fatal("Unable to create temporary user directory for Chrome/Chromium browser\n")
		os.Exit(1)
	}
	a.session.Out.Debug("[%s] Created temporary user directory at: %s\n", a.ID(), dir)
	a.tempUserDirPath = dir
}

func (a *URLScreenshotter) screenshotPage(aquaPage *core.Page) {
	filePath := fmt.Sprintf("screenshots/%s.png", aquaPage.BaseFilename())
	resolution := strings.Split(*a.session.Options.Resolution, ",")

	width, _ := strconv.Atoi(resolution[0])
	height, _ := strconv.Atoi(resolution[1])

	opts := []chromedp.ExecAllocatorOption{
		chromedp.UserAgent(RandomUserAgent()),
		chromedp.WindowSize(width, height),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.IgnoreCertErrors,
	}

	ctx, cancelExec := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelExec()
	ctx, cancel := chromedp.NewContext(ctx)
	defer cancel()

	var buf []byte

	// capture entire browser viewport, returning png with quality=90
	if err := chromedp.Run(ctx, takeScreenshot(aquaPage.URL, &buf, *a.session.Options.ScreenshotTimeout)); err != nil {
		a.session.Out.Debug("[%s] Error: %v\n", a.ID(), err)
		a.session.Stats.IncrementScreenshotFailed()
		a.session.Out.Error("%s: screenshot failed: %s\n", aquaPage.URL, err)
		return
	}
	if err := ioutil.WriteFile(a.session.GetFilePath(filePath), buf, 0o644); err != nil {
		a.session.Out.Debug("[%s] Error: %v\n", a.ID(), err)
		a.session.Stats.IncrementScreenshotFailed()
		a.session.Out.Error("%s: screenshot failed: %s\n", aquaPage.URL, err)
		return
	}

	a.session.Stats.IncrementScreenshotSuccessful()
	a.session.Out.Info("%s: %s\n", aquaPage.URL, Green("screenshot successful"))
	aquaPage.ScreenshotPath = filePath
	aquaPage.HasScreenshot = true
}

func takeScreenshot(urlstr string, res *[]byte, timeout int) chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.Navigate(urlstr),
		chromedp.Sleep(time.Duration(timeout) * time.Millisecond),
		chromedp.CaptureScreenshot(res),
	}
}
