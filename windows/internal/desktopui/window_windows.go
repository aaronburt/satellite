package desktopui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"satellite/internal/toast"

	webview "github.com/jchv/go-webview2"
	"github.com/jchv/go-webview2/webviewloader"
	"golang.org/x/sys/windows"
)

var (
	user32              = windows.NewLazySystemDLL("user32.dll")
	showWindow          = user32.NewProc("ShowWindow")
	isIconic            = user32.NewProc("IsIconic")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")
	allowForeground     = user32.NewProc("AllowSetForegroundWindow")
)

func Run(input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(input)
	var initial string
	if err := decoder.Decode(&initial); err != nil {
		return err
	}
	if err := validateURL(initial); err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	version, err := webviewloader.GetInstalledVersion()
	if err != nil || version == "" {
		return fmt.Errorf("Microsoft Edge WebView2 Runtime is required")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	view := webview.NewWithOptions(webview.WebViewOptions{
		DataPath:  filepath.Join(cache, "Satellite", "WebView2"),
		AutoFocus: true,
		WindowOptions: webview.WindowOptions{
			Title: "Satellite", Width: 1200, Height: 820, Center: true, IconId: 1,
		},
	})
	if view == nil {
		return fmt.Errorf("could not create WebView2 window")
	}
	defer view.Destroy()
	view.SetSize(760, 560, webview.HintMin)
	if err := view.Bind("satelliteOpenExternal", func(target string) error {
		return toast.OpenBrowser(target)
	}); err != nil {
		return err
	}
	view.Init(externalLinksScript)
	view.Navigate(initial)
	showWindow.Call(uintptr(view.Window()), 5)
	setForegroundWindow.Call(uintptr(view.Window()))
	if _, err := fmt.Fprintln(output, "ready"); err != nil {
		return err
	}
	go func() {
		current := initial
		for {
			var target string
			if err := decoder.Decode(&target); err != nil {
				view.Dispatch(view.Terminate)
				return
			}
			if validateURL(target) != nil {
				continue
			}
			changed := current != target
			current = target
			view.Dispatch(func() {
				if changed {
					view.Navigate(target)
				}
				hwnd := uintptr(view.Window())
				if minimized, _, _ := isIconic.Call(hwnd); minimized != 0 {
					showWindow.Call(hwnd, 9)
				}
				setForegroundWindow.Call(hwnd)
			})
		}
	}()
	view.Run()
	return nil
}

const externalLinksScript = `
(() => {
  const openExternal = value => {
    const target = new URL(value, location.href);
    if (target.protocol !== 'http:' && target.protocol !== 'https:') return false;
    if (target.origin === location.origin) return false;
    window.satelliteOpenExternal(target.href).catch(() => {});
    return true;
  };
  document.addEventListener('click', event => {
    const link = event.target.closest && event.target.closest('a[href]');
    if (link && openExternal(link.href)) event.preventDefault();
  }, true);
  document.addEventListener('auxclick', event => {
    const link = event.target.closest && event.target.closest('a[href]');
    if (link && openExternal(link.href)) event.preventDefault();
  }, true);
  window.open = value => {
    if (value && !openExternal(value)) location.assign(new URL(value, location.href));
    return null;
  };
})();`
