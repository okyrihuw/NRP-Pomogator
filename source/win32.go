//go:build windows

package main

import (
	"syscall"
	"time"
	"unsafe"
)

// ---------------------------------------------------------------------------
//  Тонкие обёртки над Win32 (только то, что нужно Помогатору)
// ---------------------------------------------------------------------------

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	pRegisterClassExW   = user32.NewProc("RegisterClassExW")
	pCreateWindowExW    = user32.NewProc("CreateWindowExW")
	pDefWindowProcW     = user32.NewProc("DefWindowProcW")
	pGetMessageW        = user32.NewProc("GetMessageW")
	pTranslateMessage   = user32.NewProc("TranslateMessage")
	pDispatchMessageW   = user32.NewProc("DispatchMessageW")
	pPostQuitMessage    = user32.NewProc("PostQuitMessage")
	pRegisterHotKey     = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	pGetForegroundWnd   = user32.NewProc("GetForegroundWindow")
	pGetWindowTextW     = user32.NewProc("GetWindowTextW")
	pGetWindowTextLenW  = user32.NewProc("GetWindowTextLengthW")
	pSetWindowPos       = user32.NewProc("SetWindowPos")
	pShowWindow         = user32.NewProc("ShowWindow")
	pSetLayeredAttrs    = user32.NewProc("SetLayeredWindowAttributes")
	pGetClientRect      = user32.NewProc("GetClientRect")
	pBeginPaint         = user32.NewProc("BeginPaint")
	pEndPaint           = user32.NewProc("EndPaint")
	pInvalidateRect     = user32.NewProc("InvalidateRect")
	pSetTimer           = user32.NewProc("SetTimer")
	pKillTimer          = user32.NewProc("KillTimer")
	pMessageBoxW        = user32.NewProc("MessageBoxW")
	pMapVirtualKeyW     = user32.NewProc("MapVirtualKeyW")
	pGetAsyncKeyState   = user32.NewProc("GetAsyncKeyState")
	pDrawTextW          = user32.NewProc("DrawTextW")
	pGetDC              = user32.NewProc("GetDC")
	pReleaseDC          = user32.NewProc("ReleaseDC")
	pFillRect           = user32.NewProc("FillRect")
	pLoadCursorW        = user32.NewProc("LoadCursorW")
	pSendInput          = user32.NewProc("SendInput")
	pSendMessageTimeout = user32.NewProc("SendMessageTimeoutW")
	pPostMessageW       = user32.NewProc("PostMessageW")
	pOpenClipboard      = user32.NewProc("OpenClipboard")
	pEmptyClipboard     = user32.NewProc("EmptyClipboard")
	pSetClipboardData   = user32.NewProc("SetClipboardData")
	pGetClipboardData   = user32.NewProc("GetClipboardData")
	pCloseClipboard     = user32.NewProc("CloseClipboard")
	pRegisterClipFmtW   = user32.NewProc("RegisterClipboardFormatW")
	pGetSystemMetrics   = user32.NewProc("GetSystemMetrics")
	pVkKeyScanExW       = user32.NewProc("VkKeyScanExW")
	pGetKeyboardLayout  = user32.NewProc("GetKeyboardLayout")
	pGetWindowThreadPID = user32.NewProc("GetWindowThreadProcessId")
	pFindWindowW        = user32.NewProc("FindWindowW")
	pEnumWindows        = user32.NewProc("EnumWindows")
	pGetClassNameW      = user32.NewProc("GetClassNameW")
	pGetLastError       = kernel32.NewProc("GetLastError")
	pDestroyWindow      = user32.NewProc("DestroyWindow")
	pMessageBeep        = user32.NewProc("MessageBeep")

	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pDeleteDC           = gdi32.NewProc("DeleteDC")

	pCreateFontW      = gdi32.NewProc("CreateFontW")
	pCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	pSelectObject     = gdi32.NewProc("SelectObject")
	pDeleteObject     = gdi32.NewProc("DeleteObject")
	pSetTextColor     = gdi32.NewProc("SetTextColor")
	pSetBkMode        = gdi32.NewProc("SetBkMode")
	pTextExtent       = gdi32.NewProc("GetTextExtentPoint32W")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pCreateMutexW     = kernel32.NewProc("CreateMutexW")
	pGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")

	pOpenProcessToken    = advapi32.NewProc("OpenProcessToken")
	pGetTokenInformation = advapi32.NewProc("GetTokenInformation")

	pShellExecuteW = shell32.NewProc("ShellExecuteW")

	// трей и меню
	pShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	pLoadIconW           = user32.NewProc("LoadIconW")
	pCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	pAppendMenuW         = user32.NewProc("AppendMenuW")
	pTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	pDestroyMenu         = user32.NewProc("DestroyMenu")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pRegisterWindowMsgW  = user32.NewProc("RegisterWindowMessageW")
)

// ---- структуры -------------------------------------------------------------

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type msgT struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type rectT struct{ Left, Top, Right, Bottom int32 }

type paintStruct struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     rectT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type sizeT struct{ Cx, Cy int32 }

type keybInput struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

// INPUT должен быть ровно 40 байт на x64
type inputT struct {
	Type uint32
	_    uint32
	Data [32]byte
}

type tokenElevation struct {
	TokenIsElevated uint32
}

// ---- константы -------------------------------------------------------------

const (
	cfgHotkeyID     = 1
	wmHotkey        = 0x0312
	wmPaint         = 0x000F
	wmEraseBkgnd    = 0x0014
	wmTimer         = 0x0113
	wmDestroy       = 0x0002
	wmDisplayChange = 0x007E

	wsPopup      = 0x80000000
	wsExLayered  = 0x00080000
	wsExTopmost  = 0x00000008
	wsExToolWnd  = 0x00000080
	wsExNoActive = 0x08000000

	swHide         = 0
	swShowNoActive = 4

	hwndTopmost = ^uintptr(0)

	swpNoActivate = 0x0010
	swpShowWindow = 0x0040
	swpNoSize     = 0x0001
	swpNoMove     = 0x0002

	lwaAlpha = 0x0002

	dtLeft       = 0x00000000
	dtCenter     = 0x00000001
	dtRight      = 0x00000002
	dtVCenter    = 0x00000004
	dtSingleLine = 0x00000020
	dtNoClip     = 0x00000100
	dtNoPrefix   = 0x00000800

	transparentBk = 1

	keyevfExtended = 0x0001
	keyevfKeyUp    = 0x0002
	keyevfUnicode  = 0x0004
	keyevfScanCode = 0x0008

	smtoAbortIfHung = 0x0002
	wmInputLangReq  = 0x0050

	cfgUnicodeText = 13
	gmemMoveable   = 0x0002

	mbOK            = 0x00000000
	mbIconWarning   = 0x00000030
	mbIconError     = 0x00000010
	mbTopMost       = 0x00040000
	mbSetForeground = 0x00010000

	idcArrow = 32512

	// --- значок в трее ------------------------------------------------------
	nimAdd          = 0
	nimModify       = 1
	nimDelete       = 2
	nifMessage      = 0x00000001
	nifIcon         = 0x00000002
	nifTip          = 0x00000004
	nifInfo         = 0x00000010
	niifInfo        = 0x00000001
	niifWarning     = 0x00000002
	mfString        = 0x00000000
	mfSeparator     = 0x00000800
	mfGrayed        = 0x00000001
	mfChecked       = 0x00000008
	tpmRightButton  = 0x0002
	tpmReturnCmd    = 0x0100
	wmNull          = 0x0000
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
)

// ---- helpers ---------------------------------------------------------------

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

func getForegroundWindow() uintptr {
	r, _, _ := pGetForegroundWnd.Call()
	return r
}

func getWindowTitle(hwnd uintptr) string {
	if hwnd == 0 {
		return ""
	}
	n, _, _ := pGetWindowTextLenW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+2)
	pGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), n+1)
	return syscall.UTF16ToString(buf)
}

func registerHotKey(hwnd uintptr, id int32, mods uint32, vk uint16) error {
	r, _, err := pRegisterHotKey.Call(hwnd, uintptr(id), uintptr(mods), uintptr(vk))
	if r == 0 {
		return err
	}
	return nil
}

func unregisterHotKey(hwnd uintptr, id int32) {
	pUnregisterHotKey.Call(hwnd, uintptr(id))
}

// sendWindowMessage с таймаутом (чтобы не зависнуть на чужом окне)
func sendWindowMessageTimeout(hwnd uintptr, msg uint32, wparam, lparam uintptr, ms uint32) bool {
	var res uintptr
	r, _, _ := pSendMessageTimeout.Call(hwnd, uintptr(msg), wparam, lparam,
		uintptr(smtoAbortIfHung), uintptr(ms), uintptr(unsafe.Pointer(&res)))
	return r != 0
}

func postMessage(hwnd uintptr, msg uint32, wparam, lparam uintptr) {
	pPostMessageW.Call(hwnd, uintptr(msg), wparam, lparam)
}

func messageBox(title, text string, flags uint32) {
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16Ptr(text))),
		uintptr(unsafe.Pointer(utf16Ptr(title))), uintptr(flags))
}

// ---- SendInput -------------------------------------------------------------

func kbInput(vk, scan uint16, flags uint32) inputT {
	in := inputT{Type: 1}
	ki := (*keybInput)(unsafe.Pointer(&in.Data[0]))
	ki.Vk = vk
	ki.Scan = scan
	ki.Flags = flags
	return in
}

// reportInputFailure — ввод не ушёл целиком (частая причина — разный уровень прав)
var inputFailuresLogged int

func reportInputFailure(got, want int, err error) {
	if inputFailuresLogged >= 5 {
		return
	}
	inputFailuresLogged++
	code, _, _ := pGetLastError.Call()
	logf("!! SendInput отправил только %d из %d событий (ошибка %v, код %d)", got, want, err, code)
	logf("   похоже, Windows блокирует ввод в это окно: обычно так бывает, когда игра запущена")
	logf("   от имени администратора, а Помогатор — нет (или наоборот). Запусти Помогатора")
	logf("   правой кнопкой -> «Запуск от имени администратора».")
}

// ---- буфер обмена ----------------------------------------------------------

// setClipboardText — прежняя подпись (используется режимом вставки type_mode=paste)
func setClipboardText(s string) bool {
	return setClipboardTextEx(s, 0, false)
}

// setClipboardTextEx кладёт Unicode-текст в буфер обмена.
// owner — окно-владелец (0 — текущая задача), noHistory — пометить запись так,
// чтобы она не попадала в историю Win+V и не уходила в облако.
func setClipboardTextEx(s string, owner uintptr, noHistory bool) bool {
	if r, _, _ := pOpenClipboard.Call(owner); r == 0 {
		return false
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()

	utf, err := syscall.UTF16FromString(s)
	if err != nil {
		return false
	}
	n := len(utf) * 2
	h, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(n))
	if h == 0 {
		return false
	}
	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return false
	}
	dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), n)
	for i := 0; i < n; i++ {
		dst[i] = byte(utf[i/2] >> (8 * (i % 2)))
	}
	pGlobalUnlock.Call(h)
	r, _, _ := pSetClipboardData.Call(cfgUnicodeText, h)
	if r == 0 {
		return false
	}

	if noHistory {
		// Официальные форматы Windows 10/11: DWORD 0 = «не сохранять».
		// Если система их не знает, RegisterClipboardFormat просто вернёт 0.
		setClipboardDword("CanIncludeInClipboardHistory", 0)
		setClipboardDword("CanUploadToCloudClipboard", 0)
	}
	return true
}

func setClipboardDword(format string, value uint32) bool {
	id, _, _ := pRegisterClipFmtW.Call(uintptr(unsafe.Pointer(utf16Ptr(format))))
	if id == 0 {
		return false
	}
	h, _, _ := pGlobalAlloc.Call(gmemMoveable, 4)
	if h == 0 {
		return false
	}
	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return false
	}
	*(*uint32)(unsafe.Pointer(ptr)) = value
	pGlobalUnlock.Call(h)
	r, _, _ := pSetClipboardData.Call(id, h)
	return r != 0
}

// getClipboardText читает текст из буфера (нужно для автотеста и журнала)
func getClipboardText() string {
	if r, _, _ := pOpenClipboard.Call(0); r == 0 {
		return ""
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(cfgUnicodeText)
	if h == 0 {
		return ""
	}
	ptr, _, _ := pGlobalLock.Call(h)
	if ptr == 0 {
		return ""
	}
	defer pGlobalUnlock.Call(h)

	var buf []uint16
	for p := unsafe.Pointer(ptr); ; p = unsafe.Add(p, 2) {
		c := *(*uint16)(p)
		if c == 0 {
			break
		}
		buf = append(buf, c)
		if len(buf) > 1<<20 {
			break
		}
	}
	return syscall.UTF16ToString(buf)
}

// ---- мелочи -----------------------------------------------------------------

func sleep(ms int) {
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}

func isKeyDown(vk uint16) bool {
	r, _, _ := pGetAsyncKeyState.Call(uintptr(vk))
	return int16(r) < 0
}

// ---- GDI -------------------------------------------------------------------

func createFontW(height, weight int32, name string) uintptr {
	h, _, _ := pCreateFontW.Call(
		uintptr(height), 0, 0, 0,
		uintptr(weight),
		0, // italic
		0, // underline
		0, // strikeout
		1, // DEFAULT_CHARSET (кириллица ок)
		0, // out precision
		0, // clip precision
		5, // CLEARTYPE_QUALITY
		0, // pitch and family
		uintptr(unsafe.Pointer(utf16Ptr(name))),
	)
	return h
}

func createSolidBrush(rgb uint32) uintptr {
	// rgb — обычный RGB (как в HTML), GDI хочет BGR
	ref := ((rgb & 0xFF) << 16) | (rgb & 0xFF00) | ((rgb >> 16) & 0xFF)
	h, _, _ := pCreateSolidBrush.Call(uintptr(ref))
	return h
}

// measDC — memory-DC для измерения текста (экранный DC ненадёжен)
var (
	measDC     uintptr
	measBitmap uintptr
)

func initMeasureDC() {
	if measDC != 0 {
		return
	}
	dc, _, _ := pCreateCompatibleDC.Call(0)
	if dc == 0 {
		return
	}
	bmp, _, _ := pCreateCompatBitmap.Call(dc, 8, 8)
	if bmp != 0 {
		pSelectObject.Call(dc, bmp)
	}
	measDC, measBitmap = dc, bmp
}

func measureText(hdc uintptr, s string) (int32, int32) {
	if s == "" {
		return 0, 0
	}
	buf := utf16FromString(s)
	var sz sizeT
	pTextExtent.Call(hdc, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)-1), uintptr(unsafe.Pointer(&sz)))
	return sz.Cx, sz.Cy
}

func drawText(hdc uintptr, s string, rc rectT, flags uint32, color uint32) {
	if s == "" {
		return
	}
	ref := ((color & 0xFF) << 16) | (color & 0xFF00) | ((color >> 16) & 0xFF)
	pSetTextColor.Call(hdc, uintptr(ref))
	pSetBkMode.Call(hdc, transparentBk)
	buf := utf16FromString(s)
	pDrawTextW.Call(hdc, uintptr(unsafe.Pointer(&buf[0])), ^uintptr(0), uintptr(unsafe.Pointer(&rc)), uintptr(flags|dtNoPrefix))
}

func fillRect(hdc uintptr, r rectT, brush uintptr) {
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), brush)
}

func utf16FromString(s string) []uint16 {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		u, _ = syscall.UTF16FromString("?")
	}
	return u
}

func getDpi() uint32 {
	var dpi uint32 = 96
	// GetDpiForSystem есть с Windows 10 1607; если нет — вернём 96
	if p := user32.NewProc("GetDpiForSystem"); p.Find() == nil {
		if r, _, _ := p.Call(); r > 0 {
			dpi = uint32(r)
		}
	}
	return dpi
}
