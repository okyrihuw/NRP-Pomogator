//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"
)

// ---------------------------------------------------------------------------
//  Перехват клавиатуры низкоуровневым хуком (WH_KEYBOARD_LL)
//
//  Работает так же, как горячие клавиши в AutoHotKey:
//  • видит нажатие даже если комбинацию «занял» кто-то ещё
//    (RegisterHotKey в этом случае просто не регистрируется);
//  • комбинация не улетает в игру (можно отключить: swallow = false);
//  • не зависит от того, как игра опрашивает клавиатуру.
//
//  Сам обработчик хука должен возвращаться мгновенно, поэтому он только
//  сопоставляет комбинацию и отправляет сообщение своему окну — вся работа
//  делается потом, в главном потоке.
// ---------------------------------------------------------------------------

const (
	whKeyboardLL    = 13
	llkhfInjected   = 0x10
	wmAppRunTrigger = 0x8000 + 3

	wmKeyDown  = 0x0100
	wmKeyUp    = 0x0101
	wmSysKeyDn = 0x0104
	wmSysKeyUp = 0x0105
)

type kbdLLHookStruct struct {
	VkCode      uint32
	ScanCode    uint32
	Flags       uint32
	Time        uint32
	DwExtraInfo uintptr
}

var (
	pSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	pUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	pCallNextHookEx      = user32.NewProc("CallNextHookEx")

	hookProcCallback = syscall.NewCallback(lowLevelKeyboardProc)

	hookHandle uintptr

	// physMods — какие модификаторы зажаты ФИЗИЧЕСКИ (по событиям хука).
	// Свои собственные нажатия (которые мы шлём через SendInput) сюда не попадают,
	// иначе модификатор «залипал» бы после бинда.
	physMods    = map[uint16]bool{}
	swallowedKs = map[uint16]bool{}
)

var modifierVKList = []uint16{0xA2, 0xA3, 0xA0, 0xA1, 0xA4, 0xA5, 0x5B, 0x5C}

// modsFromSystem — опрос системы (для режима горячих клавиш, где хука нет)
func modsFromSystem() uint32 {
	var m uint32
	for _, vk := range modifierVKList {
		if isKeyDown(vk) {
			m |= modBit(vk)
		}
	}
	return m
}

// modsNow — какие модификаторы зажаты прямо сейчас
func modsNow() uint32 {
	if !hookActive() {
		return modsFromSystem()
	}
	var m uint32
	for vk, down := range physMods {
		if down {
			m |= modBit(vk)
		}
	}
	return m
}

// physicallyHeldMods — список физически зажатых модификаторов
func physicallyHeldMods() []uint16 {
	var list []uint16
	for _, vk := range modifierVKList {
		if physicallyHeld(vk) {
			list = append(list, vk)
		}
	}
	return list
}

func physicallyHeld(vk uint16) bool {
	if !hookActive() {
		return isKeyDown(vk)
	}
	return physMods[vk]
}

// resetTriggerState — сбросить служебное состояние (при смене режима/конфига)
func resetTriggerState() {
	swallowedKs = map[uint16]bool{}
	physMods = map[uint16]bool{}
}

func matchTrigger(mods uint32, vk uint16) *trigger {
	t, ok := triggersBy[comboKey(mods, vk)]
	if !ok {
		return nil
	}
	if curCfg != nil {
		f := strings.TrimSpace(curCfg.Settings.OnlyWhenTitle)
		if f != "" {
			title := strings.ToLower(getWindowTitle(getForegroundWindow()))
			if !strings.Contains(title, strings.ToLower(f)) {
				return nil
			}
		}
	}
	return t
}

// lowLevelKeyboardProc — сам хук (должен быть максимально быстрым)
func lowLevelKeyboardProc(nCode uintptr, wparam, lparam uintptr) uintptr {
	if int32(nCode) < 0 {
		r, _, _ := pCallNextHookEx.Call(0, nCode, wparam, lparam)
		return r
	}
	kb := (*kbdLLHookStruct)(unsafe.Pointer(lparam))
	vk := uint16(kb.VkCode)
	injected := kb.Flags&llkhfInjected != 0
	down := wparam == wmKeyDown || wparam == wmSysKeyDn

	// сами модификаторы как отдельные бинды не поддерживаются
	if modBit(vk) != 0 {
		if !injected { // свои нажатия не учитываем, иначе модификатор «залипнет»
			if down {
				physMods[vk] = true
			} else {
				delete(physMods, vk)
			}
		}
		r, _, _ := pCallNextHookEx.Call(0, nCode, wparam, lparam)
		return r
	}

	if down {
		if swallowedKs[vk] {
			if isKeyDown(vk) {
				return 1 // настоящий автоповтор уже проглоченной клавиши
			}
			// событие отпускания до нас не дошло — забываем метку, чтобы
			// клавиша не «залипла» навсегда
			delete(swallowedKs, vk)
		}
		if !injected {
			mods := modsNow()
			if t := matchTrigger(mods, vk); t != nil {
				postMessage(ov.hwnd, wmAppRunTrigger, uintptr(t.idx), 0)
				if t.swallow {
					swallowedKs[vk] = true
					return 1
				}
			}
		}
	} else {
		if swallowedKs[vk] {
			delete(swallowedKs, vk)
			return 1 // вместе с нажатием прячем и отпускание
		}
	}

	r, _, _ := pCallNextHookEx.Call(0, nCode, wparam, lparam)
	return r
}

func installHook() error {
	if hookHandle != 0 {
		return nil
	}
	hInst, _, _ := pGetModuleHandleW.Call(0)
	h, _, err := pSetWindowsHookExW.Call(uintptr(whKeyboardLL), hookProcCallback, hInst, 0)
	if h == 0 {
		return err
	}
	hookHandle = h
	logf("хук клавиатуры установлен (WH_KEYBOARD_LL)")
	return nil
}

func removeHook() {
	if hookHandle != 0 {
		pUnhookWindowsHookEx.Call(hookHandle)
		hookHandle = 0
		logf("хук клавиатуры снят")
	}
}

func hookActive() bool { return hookHandle != 0 }
