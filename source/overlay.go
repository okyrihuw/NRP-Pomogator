//go:build windows

package main

import (
	"strings"
	"unsafe"
)

// ---------------------------------------------------------------------------
//  Окошко с подсказками (аналог Gui +AlwaysOnTop -Caption из AHK)
// ---------------------------------------------------------------------------

const (
	lkTitle = iota
	lkSub
	lkGroup
	lkItem
	lkNote
)

type ovRow struct {
	kind  int
	key   string
	keyW  int32
	lines []string
	w     int32 // ширина блока с текстом
	h     int32
}

type overlay struct {
	hwnd    uintptr
	visible bool
	cfg     *Config
	rows    []ovRow

	scale float64

	fontBody  uintptr
	fontBold  uintptr
	fontGroup uintptr
	fontTitle uintptr

	brBg   uintptr
	brRule uintptr

	padX, padY int32
	keyColW    int32
	gap        int32
	lineH      int32

	winW, winH int32
}

var ov *overlay

func newOverlay() *overlay {
	sc := float64(getDpi()) / 96.0
	if sc < 1 {
		sc = 1
	}
	o := &overlay{scale: sc}
	o.brBg = createSolidBrush(0x0A0A0A)
	o.brRule = createSolidBrush(0x555555)
	return o
}

func (o *overlay) font(size int32, weight int32) uintptr {
	name := o.cfg.Settings.HintFont
	if name == "" {
		name = "Consolas"
	}
	return createFontW(-size, weight, name)
}

// rebuild пересобирает содержимое окошка из конфига
func (o *overlay) rebuild(cfg *Config) {
	o.cfg = cfg

	// пересоздаём кисти/шрифты под новые настройки
	if o.brBg != 0 {
		pDeleteObject.Call(o.brBg)
	}
	if o.brRule != 0 {
		pDeleteObject.Call(o.brRule)
	}
	o.brBg = createSolidBrush(cfg.Settings.HintBG)
	o.brRule = createSolidBrush(cfg.Settings.GroupColor)

	var content []ovRow
	title := "ПОМОГАТОР"
	var subParts []string
	if cfg.Settings.Nick != "" {
		subParts = append(subParts, cfg.Settings.Nick)
	}
	if cfg.Settings.Rank != "" {
		subParts = append(subParts, cfg.Settings.Rank)
	}
	content = append(content, ovRow{kind: lkTitle, lines: []string{title}})
	if len(subParts) > 0 {
		content = append(content, ovRow{kind: lkSub, lines: []string{strings.Join(subParts, "  ·  ")}})
	}
	content = append(content, ovRow{
		kind: lkNote, key: "Ctrl + V",
		lines: []string{"бинды кладут команду в буфер — открой чат и вставь"},
	})

	curGroup := "\x00"
	for i := range cfg.Binds {
		b := &cfg.Binds[i]
		g := b.Group
		if g == "" {
			g = "ПРОЧЕЕ"
		}
		if !strings.EqualFold(g, curGroup) {
			content = append(content, ovRow{kind: lkGroup, lines: []string{strings.ToUpper(g)}})
			curGroup = g
		}
		hint := b.Hint
		if hint == "" {
			hint = cfg.substituteVariables(b.Send)
		} else {
			hint = cfg.substituteVariables(hint)
		}
		content = append(content, ovRow{kind: lkItem, key: b.Hotkey.Pretty, lines: []string{hint}})
	}

	// служебные клавиши
	var svc []ovRow
	if cfg.Settings.ToggleOK {
		svc = append(svc, ovRow{kind: lkNote, key: cfg.Settings.ToggleHint.Pretty, lines: []string{"показать / скрыть это окошко"}})
	}
	svc = append(svc, trayHintRow())
	content = append(content, ovRow{kind: lkGroup, lines: []string{"СЛУЖЕБНЫЕ КЛАВИШИ"}})
	content = append(content, svc...)

	o.rows = content
	o.measure()
	o.applyPosition()
	if o.hwnd != 0 {
		pSetLayeredAttrs.Call(o.hwnd, 0, uintptr(cfg.Settings.HintAlpha), uintptr(lwaAlpha))
		pInvalidateRect.Call(o.hwnd, 0, 0)
	}
}

// measure считает размеры всех строк и итоговый размер окошка
func (o *overlay) measure() {
	cfg := o.cfg
	if cfg == nil {
		return
	}
	sc := o.scale
	base := int32(float64(cfg.Settings.HintSize) * sc)

	// удаляем старые шрифты
	for _, f := range []uintptr{o.fontBody, o.fontBold, o.fontGroup, o.fontTitle} {
		if f != 0 {
			pDeleteObject.Call(f)
		}
	}
	o.fontBody = o.font(base, 400)
	o.fontBold = o.font(base, 700)
	o.fontGroup = o.font(base+int32(1*sc)+1, 700)
	o.fontTitle = o.font(base+int32(4*sc)+2, 700)

	initMeasureDC()
	hdc := measDC

	o.padX = int32(14 * sc)
	o.padY = int32(10 * sc)
	o.gap = int32(14 * sc)
	maxTextW := int32(680 * sc)

	_, cy := withFont(hdc, o.fontBody, func() (int32, int32) { return measureText(hdc, "Ай") })
	o.lineH = cy + int32(6*sc)

	var maxKeyW, maxTextW2 int32
	var maxTitleW, maxGroupW int32
	totalH := o.padY * 2

	for i := range o.rows {
		r := &o.rows[i]
		switch r.kind {
		case lkTitle:
			w, _ := withFont(hdc, o.fontTitle, func() (int32, int32) { return measureText(hdc, r.lines[0]) })
			h := int32(float64(base+int32(4*sc)+2) * 1.6)
			r.w, r.h = w, h
			totalH += h + int32(2*sc)
			if w > maxTitleW {
				maxTitleW = w
			}
		case lkSub:
			w, _ := withFont(hdc, o.fontBody, func() (int32, int32) { return measureText(hdc, r.lines[0]) })
			r.w, r.h = w, o.lineH
			totalH += r.h
			if w > maxTitleW {
				maxTitleW = w
			}
		case lkGroup:
			w, _ := withFont(hdc, o.fontGroup, func() (int32, int32) { return measureText(hdc, r.lines[0]) })
			r.w, r.h = w, int32(float64(base+int32(1*sc)+1)*1.5)
			totalH += r.h + int32(4*sc)
			if w > maxGroupW {
				maxGroupW = w
			}
		default: // lkItem, lkNote
			kw, _ := withFont(hdc, o.fontBold, func() (int32, int32) { return measureText(hdc, r.key) })
			r.keyW = kw
			if kw > maxKeyW {
				maxKeyW = kw
			}
			r.lines = wrapText(hdc, o.fontBody, r.lines[0], maxTextW)
			var tw int32
			for _, ln := range r.lines {
				w, _ := withFont(hdc, o.fontBody, func() (int32, int32) { return measureText(hdc, ln) })
				if w > tw {
					tw = w
				}
			}
			r.w = tw
			if tw > maxTextW2 {
				maxTextW2 = tw
			}
			r.h = int32(len(r.lines)) * o.lineH
			totalH += r.h + int32(3*sc)
		}
	}

	o.keyColW = maxKeyW
	o.winW = o.padX*2 + o.keyColW + o.gap + maxTextW2
	if min := o.padX*2 + maxTitleW; o.winW < min {
		o.winW = min
	}
	if min := o.padX*2 + maxGroupW + int32(40*sc); o.winW < min {
		o.winW = min
	}
	if o.winW > int32(1100*sc) {
		o.winW = int32(1100 * sc)
	}
	o.winH = totalH
}

func (o *overlay) applyPosition() {
	if o.hwnd == 0 || o.cfg == nil {
		return
	}
	sw, _, _ := pGetSystemMetrics.Call(0) // SM_CXSCREEN
	sh, _, _ := pGetSystemMetrics.Call(1) // SM_CYSCREEN
	x := int32(o.cfg.Settings.HintX)
	y := int32(o.cfg.Settings.HintY)
	if int32(sw) > 0 && x+o.winW > int32(sw) {
		x = int32(sw) - o.winW - 8
	}
	if int32(sh) > 0 && y+o.winH > int32(sh) {
		y = int32(sh) - o.winH - 8
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	pSetWindowPos.Call(o.hwnd, hwndTopmost, uintptr(x), uintptr(y),
		uintptr(o.winW), uintptr(o.winH), uintptr(swpNoActivate))
}

// withFont выбирает шрифт в DC на время измерения
func withFont(hdc, font uintptr, fn func() (int32, int32)) (int32, int32) {
	old, _, _ := pSelectObject.Call(hdc, font)
	w, h := fn()
	pSelectObject.Call(hdc, old)
	return w, h
}

func wrapText(hdc, font uintptr, text string, maxW int32) []string {
	if text == "" {
		return []string{""}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var out []string
	cur := ""
	for _, w := range words {
		try := w
		if cur != "" {
			try = cur + " " + w
		}
		tw, _ := withFont(hdc, font, func() (int32, int32) { return measureText(hdc, try) })
		if tw > maxW && cur != "" {
			out = append(out, cur)
			cur = w
			continue
		}
		cur = try
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// paint рисует окошко
func (o *overlay) paint() {
	if o.hwnd == 0 || o.cfg == nil {
		return
	}
	var ps paintStruct
	hdc, _, _ := pBeginPaint.Call(o.hwnd, uintptr(unsafe.Pointer(&ps)))
	defer pEndPaint.Call(o.hwnd, uintptr(unsafe.Pointer(&ps)))

	var rc rectT
	pGetClientRect.Call(o.hwnd, uintptr(unsafe.Pointer(&rc)))

	if rc.Right != o.winW || rc.Bottom != o.winH {
		o.winW, o.winH = rc.Right, rc.Bottom
	}

	fillRect(hdc, rc, o.brBg)

	sc := o.scale
	y := o.padY
	textColor := o.cfg.Settings.TextColor
	keyColor := o.cfg.Settings.KeyColor
	groupColor := o.cfg.Settings.GroupColor
	titleColor := o.cfg.Settings.TitleColor
	dim := blend(textColor, o.cfg.Settings.HintBG, 0.35)

	for i := range o.rows {
		r := &o.rows[i]
		switch r.kind {
		case lkTitle:
			old, _, _ := pSelectObject.Call(hdc, o.fontTitle)
			drawText(hdc, r.lines[0], rectT{o.padX, y, rc.Right - o.padX, y + r.h}, dtCenter|dtVCenter|dtSingleLine, titleColor)
			pSelectObject.Call(hdc, old)
			// линия под заголовком
			ly := y + r.h - int32(3*sc)
			fillRect(hdc, rectT{o.padX, ly, rc.Right - o.padX, ly + int32(2*sc)}, o.brRule)
			y += r.h + int32(2*sc)
		case lkSub:
			old, _, _ := pSelectObject.Call(hdc, o.fontBody)
			drawText(hdc, r.lines[0], rectT{o.padX, y, rc.Right - o.padX, y + r.h}, dtCenter|dtVCenter|dtSingleLine, dim)
			pSelectObject.Call(hdc, old)
			y += r.h
		case lkGroup:
			old, _, _ := pSelectObject.Call(hdc, o.fontGroup)
			drawText(hdc, r.lines[0], rectT{o.padX, y, o.padX + r.w + int32(20*sc), y + r.h}, dtLeft|dtVCenter|dtSingleLine, groupColor)
			pSelectObject.Call(hdc, old)
			// линия справа от названия раздела
			ly := y + r.h/2
			x1 := o.padX + r.w + int32(10*sc)
			if x1 < rc.Right-o.padX {
				fillRect(hdc, rectT{x1, ly, rc.Right - o.padX, ly + int32(1*sc) + 1}, o.brRule)
			}
			y += r.h + int32(4*sc)
		default:
			ky := y
			old, _, _ := pSelectObject.Call(hdc, o.fontBold)
			kc := keyColor
			if r.kind == lkNote {
				kc = groupColor
			}
			drawText(hdc, r.key, rectT{o.padX, ky, o.padX + o.keyColW, ky + o.lineH}, dtLeft|dtVCenter|dtSingleLine, kc)
			pSelectObject.Call(hdc, old)

			old, _, _ = pSelectObject.Call(hdc, o.fontBody)
			tc := textColor
			if r.kind == lkNote {
				tc = dim
			}
			x := o.padX + o.keyColW + o.gap
			for _, ln := range r.lines {
				drawText(hdc, ln, rectT{x, ky, rc.Right - o.padX, ky + o.lineH}, dtLeft|dtVCenter|dtSingleLine, tc)
				ky += o.lineH
			}
			pSelectObject.Call(hdc, old)
			y += r.h + int32(3*sc)
		}
	}
}

// blend смешивает два цвета RGB (0..1, 0 — чистый a, 1 — чистый b)
func blend(a, b uint32, t float64) uint32 {
	mix := func(x, y uint32) uint32 {
		return uint32(float64(x)*(1-t) + float64(y)*t)
	}
	return mix((a>>16)&0xFF, (b>>16)&0xFF)<<16 |
		mix((a>>8)&0xFF, (b>>8)&0xFF)<<8 |
		mix(a&0xFF, b&0xFF)
}

func (o *overlay) createWindow() error {
	hInst, _, _ := pGetModuleHandleW.Call(0)
	className := utf16Ptr("PomogatorHintWindow")

	cur, _, _ := pLoadCursorW.Call(0, uintptr(idcArrow))
	wc := wndClassExW{
		CbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		Style:         0x0003, // CS_HREDRAW | CS_VREDRAW
		LpfnWndProc:   wndProcCallback,
		HInstance:     hInst,
		HCursor:       cur,
		LpszClassName: className,
	}
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return err
	}

	hwnd, _, err := pCreateWindowExW.Call(
		uintptr(wsExLayered|wsExTopmost|wsExToolWnd|wsExNoActive),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr("Помогатор"))),
		uintptr(wsPopup),
		0, 0, 300, 300,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return err
	}
	o.hwnd = hwnd
	alpha := uint8(255)
	if o.cfg != nil {
		alpha = uint8(o.cfg.Settings.HintAlpha)
	}
	pSetLayeredAttrs.Call(hwnd, 0, uintptr(alpha), uintptr(lwaAlpha))
	return nil
}

func (o *overlay) show() {
	if o.hwnd == 0 {
		return
	}
	o.applyPosition()
	pShowWindow.Call(o.hwnd, uintptr(swShowNoActive))
	pInvalidateRect.Call(o.hwnd, 0, 0)
	o.visible = true
}

func (o *overlay) hide() {
	if o.hwnd == 0 {
		return
	}
	pShowWindow.Call(o.hwnd, uintptr(swHide))
	o.visible = false
}

func (o *overlay) toggle() {
	if o.visible {
		o.hide()
	} else {
		o.show()
	}
}
