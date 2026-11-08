//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// ---------------------------------------------------------------------------
//  Значок в трее и меню по правому клику
// ---------------------------------------------------------------------------

const (
	trayIconID  = 1
	trayIconRes = 101 // ID иконки в pomogator.rc

	wmTray = 0x8000 + 2 // WM_APP + 2 — сообщения от значка в трее

	trayMenuToggle  = 1
	trayMenuReload  = 2
	trayMenuOpenCfg = 3
	trayMenuOpenDir = 4
	trayMenuExit    = 5
	trayMenuLog     = 6
)

var (
	trayHwnd uintptr

	trayProcCallback  = syscall.NewCallback(trayWndProc)
	taskbarCreatedMsg uintptr
	trayIconAdded     bool
)

type notifyIconDataW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UTimeoutVersion  uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         [16]byte
	HBalloonIcon     uintptr
}

type pointT struct{ X, Y int32 }

// setUTF16 копирует строку в буфер фиксированной длины с завершающим нулём
func setUTF16(dst []uint16, s string) {
	src := utf16FromString(s)
	n := copy(dst, src)
	if n < len(dst) {
		dst[n] = 0
	}
	dst[len(dst)-1] = 0
}

func trayIconHandle() uintptr {
	hInst, _, _ := pGetModuleHandleW.Call(0)
	// MAKEINTRESOURCE(101): старшее слово нулевое — значит это ID ресурса
	if r, _, _ := pLoadIconW.Call(hInst, uintptr(trayIconRes)); r != 0 {
		return r
	}
	// запасной вариант — стандартная иконка Windows
	if r, _, _ := pLoadIconW.Call(0, uintptr(idcArrow)); r != 0 {
		return r
	}
	return 0
}

func trayTip() string {
	tip := "Помогатор — биндер для админки"
	if curCfg != nil && curCfg.Settings.ToggleOK {
		tip += "\nПодсказки: " + curCfg.Settings.ToggleHint.Pretty
	}
	if s := lastCopiedText(); s != "" {
		tip += "\nВ буфере: " + clipboardPreview(s, 32)
	}
	tip += "\nМеню: правый клик по значку"
	return tip
}

func createTrayWindow() error {
	hInst, _, _ := pGetModuleHandleW.Call(0)
	className := utf16Ptr("PomogatorTrayWindow")

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		Style:         0x0003,
		LpfnWndProc:   trayProcCallback,
		HInstance:     hInst,
		LpszClassName: className,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return err
	}
	hwnd, _, err := pCreateWindowExW.Call(
		uintptr(wsExToolWnd),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr("Помогатор — трей"))),
		uintptr(wsPopup),
		0, 0, 0, 0, 0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return err
	}
	trayHwnd = hwnd
	return nil
}

func trayAdd() bool {
	if trayHwnd == 0 {
		return false
	}
	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = trayHwnd
	nid.UID = trayIconID
	nid.UFlags = nifMessage | nifIcon | nifTip
	nid.UCallbackMessage = wmTray
	nid.HIcon = trayIconHandle()
	setUTF16(nid.SzTip[:], trayTip())

	r, _, err := pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))
	if r == 0 {
		logf("не удалось поставить значок в трей: %v", err)
		return false
	}
	trayIconAdded = true
	logf("значок в трее добавлен")
	return true
}

func trayRemove() {
	if !trayIconAdded || trayHwnd == 0 {
		return
	}
	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = trayHwnd
	nid.UID = trayIconID
	pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	trayIconAdded = false
}

func trayUpdateTip() {
	if !trayIconAdded || trayHwnd == 0 {
		return
	}
	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = trayHwnd
	nid.UID = trayIconID
	nid.UFlags = nifTip
	setUTF16(nid.SzTip[:], trayTip())
	pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

// trayBalloon — всплывающая подсказка от значка
func trayBalloon(title, text string, warning bool) {
	if !trayIconAdded || trayHwnd == 0 {
		return
	}
	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = trayHwnd
	nid.UID = trayIconID
	nid.UFlags = nifInfo
	if warning {
		nid.DwInfoFlags = niifWarning
	} else {
		nid.DwInfoFlags = niifInfo
	}
	nid.UTimeoutVersion = 8000
	setUTF16(nid.SzInfoTitle[:], title)
	setUTF16(nid.SzInfo[:], text)
	pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func appendMenuItem(hmenu uintptr, flags uint32, id int32, text string) {
	var tp uintptr
	if text != "" {
		tp = uintptr(unsafe.Pointer(utf16Ptr(text)))
	}
	pAppendMenuW.Call(hmenu, uintptr(flags), uintptr(id), tp)
}

func trayShowMenu() {
	if trayHwnd == 0 {
		return
	}
	// без этого меню не закроется при клике мимо него
	pSetForegroundWindow.Call(trayHwnd)

	hmenu, _, _ := pCreatePopupMenu.Call()
	if hmenu == 0 {
		return
	}
	toggleText := "Показать подсказки"
	if ov != nil && ov.visible {
		toggleText = "Скрыть подсказки"
	}
	appendMenuItem(hmenu, mfString, trayMenuToggle, toggleText)
	appendMenuItem(hmenu, mfSeparator, 0, "")
	appendMenuItem(hmenu, mfString, trayMenuReload, "Обновить")
	appendMenuItem(hmenu, mfString, trayMenuOpenCfg, "Открыть binds.ini")
	appendMenuItem(hmenu, mfString, trayMenuOpenDir, "Открыть папку программы")
	appendMenuItem(hmenu, mfString, trayMenuLog, "Показать журнал (pomogator.log)")
	appendMenuItem(hmenu, mfSeparator, 0, "")
	appendMenuItem(hmenu, mfString, trayMenuExit, "Выход")

	var pt pointT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	cmd, _, _ := pTrackPopupMenu.Call(hmenu,
		uintptr(tpmRightButton|tpmReturnCmd),
		uintptr(pt.X), uintptr(pt.Y), 0, trayHwnd, 0)
	pDestroyMenu.Call(hmenu)
	pPostMessageW.Call(trayHwnd, wmNull, 0, 0)

	switch int32(cmd) {
	case trayMenuToggle:
		ov.toggle()
		trayUpdateTip()
	case trayMenuReload:
		logf("меню трея: перечитать конфиг")
		reloadConfig()
	case trayMenuOpenCfg:
		openConfigFile()
	case trayMenuOpenDir:
		openProgramFolder()
	case trayMenuLog:
		openLogFile()
	case trayMenuExit:
		logf("меню трея: выход")
		quitApp()
	}
}

func trayWndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	if taskbarCreatedMsg != 0 && uintptr(msg) == taskbarCreatedMsg {
		// перезапустился Проводник — значок нужно поставить заново
		logf("Проводник перезапустился, возвращаю значок в трей")
		trayIconAdded = false
		trayAdd()
		return 0
	}
	if msg == wmTray {
		switch uint32(lparam) {
		case wmLButtonUp, wmLButtonDblClk:
			ov.toggle()
			trayUpdateTip()
		case wmRButtonUp:
			trayShowMenu()
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

// trayHintRow — строка про трей в окошке с подсказками
func trayHintRow() ovRow {
	return ovRow{
		kind:  lkNote,
		key:   "Значок в трее",
		lines: []string{"правый клик — конфиг, журнал, выход"},
	}
}
