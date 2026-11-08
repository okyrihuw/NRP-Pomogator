package main

import "testing"

func TestParseHotkey(t *testing.T) {
	cases := []struct {
		in         string
		mods       uint32
		vk         uint16
		wantPretty string
	}{
		{"alt+p", modAlt, 0x50, "Alt + P"},
		{"alt+з", modAlt, 0x50, "Alt + P"}, // русская раскладка, как !p в AHK
		{"!p", 0, 0, ""},                   // неверная запись — должна быть ошибкой
		{"ctrl+1", modControl, 0x31, "Ctrl + 1"},
		{"^4", 0, 0, ""},
		{"Numpad1", 0, 0x61, "Numpad 1"},
		{"alt+numpad0", modAlt, 0x60, "Alt + Numpad 0"},
		{"ctrl+7", modControl, 0x37, "Ctrl + 7"},
		{"ctrl+алт", 0, 0, ""},
		{"t", 0, 0x54, "T"},
		{"е", 0, 0x54, "T"}, // русская "е" = физическая T, как в старом скрипте
		{"ctrl+alt+q", modControl | modAlt, 0x51, "Ctrl + Alt + Q"},
		{"ctrl+alt+й", modControl | modAlt, 0x51, "Ctrl + Alt + Q"},
		{"shift+f5", modShift, 0x74, "Shift + F5"},
		{"", 0, 0, ""},
		{"ctrl+", 0, 0, ""},
	}
	for _, c := range cases {
		hk, err := parseHotkey(c.in)
		if c.wantPretty == "" {
			if err == nil {
				t.Errorf("parseHotkey(%q): ожидалась ошибка, получили %+v", c.in, hk)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseHotkey(%q): неожиданная ошибка %v", c.in, err)
			continue
		}
		if hk.Mods != c.mods || hk.VK != c.vk {
			t.Errorf("parseHotkey(%q) = mods=%#x vk=%#x, хотим mods=%#x vk=%#x", c.in, hk.Mods, hk.VK, c.mods, c.vk)
		}
		if hk.Pretty != c.wantPretty {
			t.Errorf("parseHotkey(%q).Pretty = %q, хотим %q", c.in, hk.Pretty, c.wantPretty)
		}
	}
}
