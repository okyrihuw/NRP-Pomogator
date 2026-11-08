//go:build windows

package main

import (
	"image"
	"image/png"
	"os"
	"unsafe"
)

// ---------------------------------------------------------------------------
//  Снимок экрана — нужен только автотесту (POMOGATOR_TEST=shot),
//  чтобы проверить, что окно и меню рисуются.
// ---------------------------------------------------------------------------

const srcCopy = 0x00CC0020

var (
	pBitBlt    = gdi32.NewProc("BitBlt")
	pGetDIBits = gdi32.NewProc("GetDIBits")
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

func screenMetric(i int) int {
	r, _, _ := pGetSystemMetrics.Call(uintptr(i))
	return int(r)
}

func captureScreenPNG(path string) bool {
	w, h := screenMetric(0), screenMetric(1)
	if w <= 0 || h <= 0 {
		logf("снимок: не удалось узнать размер экрана (%dx%d)", w, h)
		return false
	}
	hdc, _, _ := pGetDC.Call(0)
	if hdc == 0 {
		logf("снимок: GetDC вернул 0")
		return false
	}
	defer pReleaseDC.Call(0, hdc)
	mem, _, _ := pCreateCompatibleDC.Call(hdc)
	if mem == 0 {
		logf("снимок: CreateCompatibleDC вернул 0")
		return false
	}
	bmp, _, _ := pCreateCompatBitmap.Call(hdc, uintptr(w), uintptr(h))
	if bmp == 0 {
		logf("снимок: CreateCompatibleBitmap вернул 0")
		pDeleteDC.Call(mem)
		return false
	}
	prev, _, _ := pSelectObject.Call(mem, bmp)
	pBitBlt.Call(mem, 0, 0, uintptr(w), uintptr(h), hdc, 0, 0, srcCopy)

	bi := bitmapInfoHeader{Size: 40, Width: int32(w), Height: int32(-h), Planes: 1, BitCount: 32}
	buf := make([]byte, w*h*4)
	pGetDIBits.Call(mem, bmp, 0, uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), 0)

	pSelectObject.Call(mem, prev)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(mem)

	for i := 0; i+3 < len(buf); i += 4 { // BGRA -> RGBA
		buf[i], buf[i+2] = buf[i+2], buf[i]
		buf[i+3] = 255
	}
	img := &image.RGBA{Pix: buf, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	f, err := os.Create(path)
	if err != nil {
		logf("снимок: не удалось создать файл: %v", err)
		return false
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		logf("снимок: ошибка записи PNG: %v", err)
		return false
	}
	return true
}
