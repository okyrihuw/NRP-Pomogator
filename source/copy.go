//go:build windows

package main

import (
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
//  ЕДИНСТВЕННЫЙ РЕЖИМ РАБОТЫ (v1.7)
//
//  Бинд НЕ открывает чат, НЕ печатает и НЕ нажимает Enter. Он только собирает
//  готовую команду и кладёт её в буфер обмена. Дальше всё делает человек:
//  сам открывает чат (T / F8 / что настроено) и физически нажимает Ctrl+V.
//
//  Для игры это самая настоящая вставка из буфера — отличить её от живой
//  клавиатуры невозможно, поэтому «защиты» MTA тут не при чём.
// ---------------------------------------------------------------------------

var (
	lastCopiedMu sync.Mutex
	lastCopied   string
)

func setLastCopied(s string) {
	lastCopiedMu.Lock()
	lastCopied = s
	lastCopiedMu.Unlock()
}

func lastCopiedText() string {
	lastCopiedMu.Lock()
	defer lastCopiedMu.Unlock()
	return lastCopied
}

// copyTextWithRetry — буфер обмена может быть на секунду занят другой
// программой, поэтому несколько попыток с короткой паузой.
func copyTextWithRetry(text string, owner uintptr, noHistory bool) bool {
	for attempt := 0; attempt < 14; attempt++ {
		if setClipboardTextEx(text, owner, noHistory) {
			return true
		}
		sleep(20 + attempt*8)
	}
	return false
}

// runBindCopy — выполнение бинда в режиме copy
func runBindCopy(cfg *Config, b *Bind) {
	hwnd := getForegroundWindow()
	title := ""
	if hwnd != 0 {
		title = getWindowTitle(hwnd)
	}
	if filter := strings.TrimSpace(cfg.Settings.OnlyWhenTitle); filter != "" && hwnd != 0 {
		if !strings.Contains(strings.ToLower(title), strings.ToLower(filter)) {
			logf("бинд %s пропущен: активное окно %q не содержит %q", b.Hotkey.Pretty, title, filter)
			return
		}
	}

	text := clipboardText(cfg.substituteVariables(b.Send))
	if strings.TrimSpace(text) == "" {
		logf("бинд %s: нечего копировать (пустой текст)", b.Hotkey.Pretty)
		return
	}

	owner := uintptr(0)
	if ov != nil {
		owner = ov.hwnd
	}

	started := time.Now()
	if !copyTextWithRetry(text, owner, cfg.Settings.CopyNoHistory) {
		logf("!! бинд %s: не удалось положить в буфер обмена (буфер занят другой программой)", b.Hotkey.Pretty)
		toastShow("НЕ СКОПИРОВАЛОСЬ", "Буфер занят другой программой — нажми бинд ещё раз", 2400)
		trayBalloon(appName, "Не удалось скопировать команду: буфер обмена занят. Попробуй ещё раз.", true)
		return
	}
	setLastCopied(text)
	trayUpdateTip()

	if cfg.Settings.CopyBeep {
		pMessageBeep.Call(0x40) // MB_ICONASTERISK — короткий системный «дзынь»
	}
	if cfg.Settings.CopyToast {
		if toast != nil {
			toastShow(clipboardPreview(text, 64), "Открой чат и нажми Ctrl+V", cfg.Settings.CopyToastMs)
		} else {
			// тихий режим: окон поверх игры нет — сообщаем через трей
			trayBalloon("Скопировано в буфер", clipboardPreview(text, 64)+"\nОткрой чат и нажми Ctrl+V", false)
		}
	}

	logf("бинд %s -> скопировано в буфер: %q | окно %q | символов %d, заняло %dмс",
		b.Hotkey.Pretty, clipboardPreview(text, 120), truncate(title, 40),
		len([]rune(text)), time.Since(started).Milliseconds())
}
