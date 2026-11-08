//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	appName    = "Помогатор"
	appVersion = "1.0"
)

var (
	logMu   sync.Mutex
	logFile *os.File

	cfgPath string
	curCfg  *Config

	bindMu sync.Mutex

	actions = map[int32]*action{}

	wndProcCallback = syscall.NewCallback(wndProc)
)

func logf(format string, a ...any) {
	logMu.Lock()
	defer logMu.Unlock()
	if logFile == nil {
		return
	}
	fmt.Fprintf(logFile, "[%s] %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
	logFile.Sync()
}

func exePath() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	return p
}

func setupLogging(dir string) {
	f, err := os.Create(filepath.Join(dir, "pomogator.log"))
	if err != nil {
		f, err = os.Create(filepath.Join(os.TempDir(), "pomogator.log"))
		if err != nil {
			return
		}
	}
	logFile = f
}

func isElevated() bool {
	var token uintptr
	proc, _ := syscall.GetCurrentProcess()
	r, _, _ := pOpenProcessToken.Call(uintptr(proc), 0x0008 /*TOKEN_QUERY*/, uintptr(unsafe.Pointer(&token)))
	if r == 0 {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(token))

	var elev tokenElevation
	var n uint32
	r, _, _ = pGetTokenInformation.Call(token, 20, /*TokenElevation*/
		uintptr(unsafe.Pointer(&elev)), unsafe.Sizeof(elev), uintptr(unsafe.Pointer(&n)))
	return r != 0 && elev.TokenIsElevated != 0
}

func singleInstance() bool {
	name := utf16Ptr("Local\\PomogatorBinder_SingleInstance")
	h, _, err := pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return true
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 183 /*ERROR_ALREADY_EXISTS*/ {
		return false
	}
	return true
}

// ---------------------------------------------------------------------------
//  Служебные сообщения окна
// ---------------------------------------------------------------------------

const (
	wmAppReload   = 0x8000 + 1
	wmAppTrayMenu = 0x8000 + 5 // показать меню трея (для снимка/тестов)
	wmAppQuit     = 0x8000 + 7 // корректный выход в главном потоке
)

func clearHotkeys() {
	for id := range actions {
		unregisterHotKey(ov.hwnd, id)
	}
	actions = map[int32]*action{}
}

// registerAllHotkeys — запасной режим, если хук не установился
func registerAllHotkeys(cfg *Config) []string {
	clearHotkeys()

	var warns []string
	used := map[uint32]string{}

	add := func(id int32, hk Hotkey, kind int, b *Bind) {
		k := uint32(hk.Mods)<<16 | uint32(hk.VK)
		if prev, dup := used[k]; dup {
			warns = append(warns, fmt.Sprintf("%s — клавиша уже занята («%s»)", hk.Pretty, prev))
			return
		}
		if err := registerHotKey(ov.hwnd, id, hk.registerID(), hk.VK); err != nil {
			warns = append(warns, fmt.Sprintf("%s — не удалось занять (её держит другая программа: %v)", hk.Pretty, err))
			return
		}
		used[k] = hk.Pretty
		actions[id] = &action{kind: kind, bind: b, name: hk.Pretty}
	}

	if cfg.Settings.ToggleOK {
		add(9001, cfg.Settings.ToggleHint, actToggle, nil)
	}
	for i := range cfg.Binds {
		b := &cfg.Binds[i]
		add(int32(i+1), b.Hotkey, actBind, b)
	}
	return warns
}

// setupTriggers включает перехват клавиш: хук, при неудаче — горячие клавиши Windows
func setupTriggers(cfg *Config) []string {
	clearHotkeys()

	if err := installHook(); err != nil {
		logf("хук клавиатуры не установился (%v) — перехожу на горячие клавиши Windows", err)
		warns := registerAllHotkeys(cfg)
		logf("режим перехвата: горячие клавиши Windows (RegisterHotKey), занято %d", len(actions))
		return warns
	}

	warns := buildTriggers(cfg)
	resetTriggerState()
	logf("режим перехвата: хук клавиатуры, комбинаций %d", len(triggers))
	for _, t := range triggers {
		logf("   %-18s %s", t.name, t.desc)
	}
	return warns
}

func handleHotkey(id int32) {
	act, ok := actions[id]
	if !ok {
		return
	}
	performAction(act.kind, act.bind, act.name)
}

func runTrigger(idx int) {
	if idx < 0 || idx >= len(triggers) {
		return
	}
	t := triggers[idx]
	performAction(t.kind, t.bind, t.name)
}

func performAction(kind int, b *Bind, name string) {
	switch kind {
	case actBind:
		if b == nil {
			return
		}
		go func() {
			bindMu.Lock()
			defer bindMu.Unlock()
			defer func() {
				if r := recover(); r != nil {
					logf("ошибка при выполнении бинда: %v", r)
				}
			}()
			runBindCopy(curCfg, b)
		}()
	case actToggle:
		logf("служебная клавиша %s (окошко было видно: %v)", name, ov.visible)
		ov.toggle()
		trayUpdateTip()
	}
}

// ---------------------------------------------------------------------------
//  Файлы / выход
// ---------------------------------------------------------------------------

func shellOpen(target, dir string) {
	pShellExecuteW.Call(0,
		uintptr(unsafe.Pointer(utf16Ptr("open"))),
		uintptr(unsafe.Pointer(utf16Ptr(target))),
		0,
		uintptr(unsafe.Pointer(utf16Ptr(dir))),
		1)
}

func openConfigFile() {
	dir := filepath.Dir(cfgPath)
	shellOpen(filepath.Join(dir, "binds.ini"), dir)
}

func openProgramFolder() {
	dir := filepath.Dir(cfgPath)
	shellOpen(dir, dir)
}

func openLogFile() {
	dir := filepath.Dir(cfgPath)
	shellOpen(filepath.Join(dir, "pomogator.log"), dir)
}

func reloadConfig() {
	newCfg, err := loadConfig(cfgPath, true)
	if err != nil {
		notifyBox("Не удалось прочитать binds.ini:\n\n"+err.Error(), false)
		return
	}
	curCfg = newCfg
	warns := setupTriggers(newCfg)
	ov.rebuild(newCfg)
	if ov.visible {
		ov.show()
	}
	logf("конфиг перечитан: биндов %d, предупреждений %d", len(newCfg.Binds), len(warns)+len(newCfg.Warnings))
	showWarnings(warns, newCfg.Warnings)
}

// quitApp можно звать из любого потока
func quitApp() {
	if ov != nil && ov.hwnd != 0 {
		postMessage(ov.hwnd, wmAppQuit, 0, 0)
		return
	}
	quitAppNow()
}

func quitAppNow() {
	removeHook()
	trayRemove()
	for id := range actions {
		unregisterHotKey(ov.hwnd, id)
	}
	if toast != nil && toast.hwnd != 0 {
		pKillTimer.Call(toast.hwnd, 1)
		pDestroyWindow.Call(toast.hwnd)
		toast.hwnd = 0
	}
	if ov.hwnd != 0 {
		pKillTimer.Call(ov.hwnd, 1)
		pKillTimer.Call(ov.hwnd, 2)
		// DestroyWindow -> WM_DESTROY -> PostQuitMessage
		pDestroyWindow.Call(ov.hwnd)
	}
	if trayHwnd != 0 {
		pDestroyWindow.Call(trayHwnd)
		trayHwnd = 0
	}
	logf("выход")
}

// notifyBox — окно с сообщением в отдельном потоке, чтобы не блокировать цикл сообщений
func notifyBox(text string, warning bool) {
	var flags uint32 = mbOK | mbIconWarning | mbTopMost
	if !warning {
		flags = mbOK | mbIconError | mbTopMost
	}
	msg := text
	go func() { messageBox(appName, msg, flags) }()
}

func showWarnings(hotkeyWarns, cfgWarns []string) {
	if len(hotkeyWarns)+len(cfgWarns) == 0 {
		return
	}
	var b strings.Builder
	if len(cfgWarns) > 0 {
		b.WriteString("Замечания к binds.ini:\r\n")
		for _, w := range cfgWarns {
			b.WriteString("  • " + w + "\r\n")
		}
	}
	if len(hotkeyWarns) > 0 {
		b.WriteString("\r\nКлавиши, которые не удалось занять:\r\n")
		for _, w := range hotkeyWarns {
			b.WriteString("  • " + w + "\r\n")
		}
	}
	b.WriteString("\r\nПоправь binds.ini и выбери в трее (правый клик по значку)\r\n")
	b.WriteString("«Перечитать binds.ini» — конфиг перечитается без перезапуска.")
	logf("замечания:\n%s", b.String())
	notifyBox(b.String(), true)
}

// ---------------------------------------------------------------------------
//  Оконная процедура и главный цикл
// ---------------------------------------------------------------------------

func wndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmHotkey:
		handleHotkey(int32(wparam))
		return 0
	case wmTimer:
		switch wparam {
		case 1:
			pKillTimer.Call(hwnd, 1)
			ov.hide()
		case 2:
			// держим окошко поверх игры
			if ov.visible {
				pSetWindowPos.Call(hwnd, hwndTopmost, 0, 0, 0, 0,
					uintptr(swpNoMove|swpNoSize|swpNoActivate|swpShowWindow))
			}
		}
		return 0
	case wmPaint:
		ov.paint()
		return 0
	case wmAppReload:
		reloadConfig()
		return 0
	case wmAppRunTrigger:
		runTrigger(int(wparam))
		return 0
	case wmAppTrayMenu:
		trayShowMenu()
		return 0
	case wmAppQuit:
		quitAppNow()
		return 0
	case wmEraseBkgnd:
		return 1
	case wmDisplayChange:
		if ov.visible {
			ov.measure()
			ov.applyPosition()
			pInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

func main() {
	runtime.LockOSThread()

	exe := exePath()
	exeDir := filepath.Dir(exe)
	setupLogging(exeDir)
	logf("=== %s %s запускается ===", appName, appVersion)
	logf("права администратора: %v (для этого режима они не нужны)", isElevated())

	if !singleInstance() {
		messageBox(appName, "Помогатор уже запущен.\n\nВторой экземпляр не нужен: у него те же горячие клавиши.", mbOK|mbIconWarning|mbTopMost)
		return
	}

	cfgPath = filepath.Join(exeDir, "binds.ini")
	cfg, err := loadConfig(cfgPath, true)
	if err != nil {
		logf("конфиг: %v", err)
		messageBox(appName, "Не удалось открыть binds.ini:\n\n"+err.Error(), mbOK|mbIconError|mbTopMost)
		return
	}
	curCfg = cfg
	logf("конфиг: %s, биндов %d", cfgPath, len(cfg.Binds))
	logf("режим: только копирование в буфер (открываешь чат и жмёшь Ctrl+V сам)")

	initMeasureDC()
	ov = newOverlay()
	ov.cfg = cfg
	toast = newToast()

	// окно нужно до регистрации горячих клавиш
	if err := ov.createWindow(); err != nil {
		logf("окно: %v", err)
		messageBox(appName, "Не удалось создать окно подсказки:\n\n"+err.Error(), mbOK|mbIconError|mbTopMost)
		return
	}

	if err := toast.createWindow(); err != nil {
		logf("окно подтверждения копирования: %v", err)
		toast = nil
	}

	if err := createTrayWindow(); err != nil {
		logf("окно трея: %v", err)
	} else {
		taskbarCreatedMsg, _, _ = pRegisterWindowMsgW.Call(uintptr(unsafe.Pointer(utf16Ptr("TaskbarCreated"))))
		trayAdd()
	}

	warns := setupTriggers(cfg)
	ov.rebuild(cfg)

	// таймер «поверх всех окон», пока окошко видно
	pSetTimer.Call(ov.hwnd, 2, 3000, 0)

	if cfg.Settings.HintOnStart {
		ov.show()
		if cfg.Settings.HintStartMs > 0 {
			pSetTimer.Call(ov.hwnd, 1, uintptr(cfg.Settings.HintStartMs), 0)
		}
	}
	logf("размер подсказки: %dx%d, высота строки %d (масштаб %.2f)", ov.winW, ov.winH, ov.lineH, ov.scale)
	if hookActive() {
		logf("перехват клавиатуры работает через хук (комбинации видны всегда)")
	}

	if cfg.Settings.TrayNotifyOnStart {
		msg := "Правый клик по значку в трее — перечитать/открыть конфиг, выход."
		if cfg.Settings.ToggleOK {
			msg = "Подсказки: " + cfg.Settings.ToggleHint.Pretty + ". " + msg
		}
		trayBalloon(appName, msg, false)
	}
	trayUpdateTip()

	if len(warns)+len(cfg.Warnings) > 0 {
		showWarnings(warns, cfg.Warnings)
	}

	// автотесты (нужны только разработчику)
	switch os.Getenv("POMOGATOR_TEST") {
	case "shot":
		go func() {
			sleep(2000)
			postMessage(ov.hwnd, wmAppTrayMenu, 0, 0)
			sleep(1500)
			dir := filepath.Join(filepath.Dir(exePath()), "diagnostika")
			os.MkdirAll(dir, 0o755)
			captureScreenPNG(filepath.Join(dir, "tray_menu.png"))
			logf("снимок меню трея сделан")
			sleep(500)
			quitApp()
		}()
	case "copy":
		go func() {
			sleep(1200)
			var target *Bind
			for i := range curCfg.Binds {
				if curCfg.Binds[i].Hotkey.Pretty == "Ctrl + 7" {
					target = &curCfg.Binds[i]
					break
				}
			}
			if target == nil && len(curCfg.Binds) > 0 {
				target = &curCfg.Binds[0]
			}
			if target != nil {
				runBindCopy(curCfg, target)
				sleep(300)
				logf("автотест копирования: в буфере сейчас %q", getClipboardText())
			} else {
				logf("автотест копирования: в конфиге нет биндов")
			}
			sleep(400)
			quitApp()
		}()
	}

	var msg msgT
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
	logf("цикл сообщений завершён")
}

// ошибки нужны только для понятных сообщений при старте
var _ = errors.New
