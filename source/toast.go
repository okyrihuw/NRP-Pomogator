//go:build windows

package main

import (
	"sync"
	"syscall"
	"unsafe"
)

// ---------------------------------------------------------------------------
//  Маленькое подтверждение «скопировано» поверх игры.
//
//  Окно не забирает фокус (WS_EX_NOACTIVATE), всегда поверх всех, поэтому
//  его видно и в оконной, и в безрамочной игре. Само прячется по таймеру.
// ---------------------------------------------------------------------------

type toastWindow struct {
	hwnd uintptr

	fontHead  uintptr
	fontTitle uintptr
	fontBody  uintptr

	brBg     uintptr
	brAccent uintptr

	scale float64
	lineH int32

	mu     sync.Mutex
	head   string
	title  string
	line   string
	w, h   int32
	visible bool
}

var (
	toast             *toastWindow
	toastProcCallback = syscall.NewCallback(toastWndProc)
)

func newToast() *toastWindow {
	sc := float64(getDpi()) / 96.0
	if sc < 1 {
		sc = 1
	}
	t := &toastWindow{scale: sc}
	t.brBg = createSolidBrush(0x161616)
	t.brAccent = createSolidBrush(0x4CD964)
	t.fontHead = createFontW(-int32(11*sc), 700, "Segoe UI")
	t.fontTitle = createFontW(-int32(15*sc), 700, "Segoe UI")
	t.fontBody = createFontW(-int32(12*sc), 400, "Segoe UI")
	return t
}

func (t *toastWindow) createWindow() error {
	hInst, _, _ := pGetModuleHandleW.Call(0)
	className := utf16Ptr("PomogatorToastWindow")

	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		Style:         0x0003, // CS_HREDRAW | CS_VREDRAW
		LpfnWndProc:   toastProcCallback,
		HInstance:     hInst,
		LpszClassName: className,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return err
	}
	hwnd, _, err := pCreateWindowExW.Call(
		uintptr(wsExLayered|wsExTopmost|wsExToolWnd|wsExNoActive),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr("Помогатор — буфер"))),
		uintptr(wsPopup),
		0, 0, 10, 10,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return err
	}
	t.hwnd = hwnd
	pSetLayeredAttrs.Call(hwnd, 0, 242, lwaAlpha)
	return nil
}

// show показывает подтверждение на ms миллисекунд
func (t *toastWindow) show(head, title, line string, ms int) {
	if t.hwnd == 0 {
		return
	}
	if ms <= 0 {
		ms = 1600
	}
	t.mu.Lock()
	t.head, t.title, t.line = head, title, line
	t.measureLocked()
	w, h := t.w, t.h
	t.mu.Unlock()

	sw, _, _ := pGetSystemMetrics.Call(0)
	sh, _, _ := pGetSystemMetrics.Call(1)
	x := (int32(sw) - w) / 2
	y := int32(float64(sh)*0.74) - h/2
	if x < 8 {
		x = 8
	}
	if y < 8 {
		y = 8
	}

	pSetWindowPos.Call(t.hwnd, hwndTopmost, uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(swpNoActivate|swpShowWindow))
	pShowWindow.Call(t.hwnd, uintptr(swShowNoActive))
	pInvalidateRect.Call(t.hwnd, 0, 0)
	pSetTimer.Call(t.hwnd, 1, uintptr(ms), 0)
	t.visible = true
}

func (t *toastWindow) measureLocked() {
	initMeasureDC()
	hdc := measDC
	sc := t.scale
	maxText := int32(620 * sc)

	wrap := func(font uintptr, s string) (int32, int32) {
		w, h := withFont(hdc, font, func() (int32, int32) { return measureText(hdc, s) })
		if w > maxText {
			w = maxText
		}
		return w, h
	}

	hw, hh := wrap(t.fontHead, t.head)
	tw, th := wrap(t.fontTitle, t.title)
	bw, bh := wrap(t.fontBody, t.line)

	w := hw
	if tw > w {
		w = tw
	}
	if bw > w {
		w = bw
	}
	padX := int32(18 * sc)
	padY := int32(10 * sc)
	gap := int32(4 * sc)

	t.w = w + padX*2
	t.h = padY*2 + hh + th + bh + gap*2
	t.lineH = th
}

func (t *toastWindow) paint() {
	if t.hwnd == 0 {
		return
	}
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(t.hwnd, uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(t.hwnd, uintptr(unsafe.Pointer(&ps)))

	var rc rectT
	pGetClientRect.Call(t.hwnd, uintptr(unsafe.Pointer(&rc)))
	fillRect(hdc, rc, t.brBg)

	sc := t.scale
	accentW := int32(5 * sc)
	fillRect(hdc, rectT{0, 0, accentW, rc.Bottom}, t.brAccent)

	t.mu.Lock()
	head, title, line := t.head, t.title, t.line
	t.mu.Unlock()

	padX := int32(18 * sc)
	padY := int32(10 * sc)
	y := padY

	draw := func(font uintptr, s string, color uint32, hgt int32) {
		old, _, _ := pSelectObject.Call(hdc, font)
		drawText(hdc, s, rectT{padX, y, rc.Right - padX, y + hgt},
			dtLeft|dtVCenter|dtSingleLine|dtNoPrefix, color)
		pSelectObject.Call(hdc, old)
		y += hgt + int32(4*sc)
	}

	_, hh := withFont(hdc, t.fontHead, func() (int32, int32) { return measureText(hdc, head) })
	_, th := withFont(hdc, t.fontTitle, func() (int32, int32) { return measureText(hdc, title) })
	_, bh := withFont(hdc, t.fontBody, func() (int32, int32) { return measureText(hdc, line) })

	draw(t.fontHead, head, 0x4CD964, hh)
	draw(t.fontTitle, title, 0xFFFFFF, th)
	draw(t.fontBody, line, 0xB9B9B9, bh)
}

func toastWndProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case wmPaint:
		if toast != nil {
			toast.paint()
		}
		return 0
	case wmTimer:
		pKillTimer.Call(hwnd, 1)
		pShowWindow.Call(hwnd, uintptr(swHide))
		if toast != nil {
			toast.visible = false
		}
		return 0
	case wmEraseBkgnd:
		return 1
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

// toastShow — безопасная обёртка: если окно ещё не создано, просто ничего не делаем
func toastShow(title, line string, ms int) {
	if toast == nil {
		return
	}
	toast.show("СКОПИРОВАНО В БУФЕР", title, line, ms)
}
