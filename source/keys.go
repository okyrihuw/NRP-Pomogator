package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
//  Разбор горячих клавиш: "ctrl+alt+з" -> модификаторы + виртуальный код (VK)
//  Поддерживаются русские буквы (как в AHK: !з == Alt+P)
// ---------------------------------------------------------------------------

const (
	modAlt      uint32 = 0x0001
	modControl  uint32 = 0x0002
	modShift    uint32 = 0x0004
	modWin      uint32 = 0x0008
	modNoRepeat uint32 = 0x4000
)

const (
	vkBackspace uint16 = 0x08
	vkTab       uint16 = 0x09
	vkReturn    uint16 = 0x0D
	vkEscape    uint16 = 0x1B
	vkSpace     uint16 = 0x20
)

// Служебные клавиши
var specialVK = map[string]uint16{
	"space": 0x20, "spacebar": 0x20,
	"enter": 0x0D, "return": 0x0D, "numpadenter": 0x0D, "numenter": 0x0D,
	"tab": 0x09, "esc": 0x1B, "escape": 0x1B,
	"backspace": 0x08, "bksp": 0x08, "bs": 0x08,
	"insert": 0x2D, "ins": 0x2D, "delete": 0x2E, "del": 0x2E,
	"home": 0x24, "end": 0x23,
	"pageup": 0x21, "pgup": 0x21, "pagedown": 0x22, "pgdn": 0x22,
	"up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27,
	"pause": 0x13, "break": 0x13,
	"capslock": 0x14, "numlock": 0x90, "scrolllock": 0x91,
	"printscreen": 0x2C, "prtsc": 0x2C,
	"apps": 0x5D, "menu": 0x5D,
	"numpadmul": 0x6A, "numpad*": 0x6A, "numpadadd": 0x6B,
	"numpadsub": 0x6D, "numpad-": 0x6D, "numpaddec": 0x6E, "numpad.": 0x6E,
	"numpaddiv": 0x6F, "numpad/": 0x6F,
	"-": 0xBD, "=": 0xBB, "[": 0xDB, "]": 0xDD, "\\": 0xDC,
	";": 0xBA, "'": 0xDE, ",": 0xBC, ".": 0xBE, "/": 0xBF, "`": 0xC0, "~": 0xC0,
	// знаки на русской раскладке
	"х": 0xDB, "ъ": 0xDD, "ж": 0xBA, "э": 0xDE, "б": 0xBC, "ю": 0xBE, "ё": 0xC0, "№": 0xC0,
}

// Русские буквы -> физические клавиши (ЙЦУКЕН)
var ruToVK = map[string]uint16{
	"й": 0x51, "ц": 0x57, "у": 0x45, "к": 0x52, "е": 0x54, "н": 0x59, "г": 0x55,
	"ш": 0x49, "щ": 0x4F, "з": 0x50,
	"ф": 0x41, "ы": 0x53, "в": 0x44, "а": 0x46, "п": 0x47, "р": 0x48, "о": 0x4A, "л": 0x4B, "д": 0x4C,
	"я": 0x5A, "ч": 0x58, "с": 0x43, "м": 0x56, "и": 0x42, "т": 0x4E, "ь": 0x4D,
}

var vkByName = map[string]uint16{}
var nameByVK = map[uint16]string{}

func init() {
	// 0-9
	for i := 0; i < 10; i++ {
		k := string(rune('0' + i))
		vkByName[k] = uint16(0x30 + i)
		nameByVK[uint16(0x30+i)] = k
	}
	// a-z
	for i := 0; i < 26; i++ {
		ch := string(rune('a' + i))
		vk := uint16(0x41 + i)
		vkByName[ch] = vk
		nameByVK[vk] = strings.ToUpper(ch)
	}
	// F1-F24
	for i := 1; i <= 24; i++ {
		name := fmt.Sprintf("f%d", i)
		vk := uint16(0x6F + i)
		vkByName[name] = vk
		nameByVK[vk] = strings.ToUpper(name)
	}
	// numpad 0-9
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("numpad%d", i)
		vk := uint16(0x60 + i)
		vkByName[name] = vk
		nameByVK[vk] = fmt.Sprintf("Numpad %d", i)
	}
	for name, vk := range specialVK {
		if _, busy := vkByName[name]; !busy {
			vkByName[name] = vk
		}
		if _, busy := nameByVK[vk]; !busy {
			nameByVK[vk] = strings.ToUpper(name)
		}
	}
	// поправки "человеческих" названий (чтобы подписи не зависели от порядка map)
	nameByVK[0x20] = "Space"
	nameByVK[0x0D] = "Enter"
	nameByVK[0x09] = "Tab"
	nameByVK[0x1B] = "Esc"
	nameByVK[0xBD] = "-"
	nameByVK[0xBB] = "="
	nameByVK[0xDB] = "["
	nameByVK[0xDD] = "]"
	nameByVK[0xDC] = "\\"
	nameByVK[0xBA] = ";"
	nameByVK[0xDE] = "'"
	nameByVK[0xBC] = ","
	nameByVK[0xBE] = "."
	nameByVK[0xBF] = "/"
	nameByVK[0xC0] = "`"
	nameByVK[0x08] = "Backspace"
	nameByVK[0x2D] = "Insert"
	nameByVK[0x2E] = "Delete"
	nameByVK[0x24] = "Home"
	nameByVK[0x23] = "End"
	nameByVK[0x21] = "PageUp"
	nameByVK[0x22] = "PageDown"
	nameByVK[0x26] = "Up"
	nameByVK[0x28] = "Down"
	nameByVK[0x25] = "Left"
	nameByVK[0x27] = "Right"
	// русские буквы становятся доступны по имени (й -> Q)
	for ru, vk := range ruToVK {
		if _, busy := vkByName[ru]; !busy {
			vkByName[ru] = vk
		}
	}
}

// Hotkey — разобранная комбинация
type Hotkey struct {
	Mods   uint32
	VK     uint16
	Pretty string
	Raw    string
}

func (h Hotkey) registerID() uint32 { return h.Mods | modNoRepeat }

// parseHotkey разбирает строку вида "ctrl+alt+1", "Numpad1", "alt+з"
func parseHotkey(s string) (Hotkey, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Hotkey{}, fmt.Errorf("пустая комбинация")
	}
	parts := strings.Split(strings.ToLower(raw), "+")
	var mods uint32
	var modNames []string
	var vk uint16
	var vkFound bool

	for i, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			return Hotkey{}, fmt.Errorf("не понятна комбинация %q (проверь плюсы)", raw)
		}
		isLast := i == len(parts)-1
		if !isLast {
			switch p {
			case "ctrl", "control", "c", "с":
				mods |= modControl
				modNames = append(modNames, "Ctrl")
			case "alt", "a":
				mods |= modAlt
				modNames = append(modNames, "Alt")
			case "shift", "s":
				mods |= modShift
				modNames = append(modNames, "Shift")
			case "win", "super", "lwin":
				mods |= modWin
				modNames = append(modNames, "Win")
			default:
				return Hotkey{}, fmt.Errorf("неизвестный модификатор %q", p)
			}
			continue
		}
		v, ok := vkByName[p]
		if !ok {
			return Hotkey{}, fmt.Errorf("неизвестная клавиша %q", p)
		}
		vk, vkFound = v, true
	}
	if !vkFound {
		return Hotkey{}, fmt.Errorf("не указана клавиша в %q", raw)
	}
	pretty := strings.Join(modNames, " + ")
	if pretty != "" {
		pretty += " + "
	}
	pretty += prettyKeyName(vk)
	return Hotkey{Mods: mods, VK: vk, Pretty: pretty, Raw: raw}, nil
}

func prettyKeyName(vk uint16) string {
	if n, ok := nameByVK[vk]; ok {
		return n
	}
	return fmt.Sprintf("0x%02X", vk)
}
