package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
//  Комбинации, за которыми следит Помогатор.
//  Эта часть не зависит от Windows и покрыта тестами (triggers_test.go).
// ---------------------------------------------------------------------------

const (
	actBind = iota
	actToggle
)

type action struct {
	kind int
	bind *Bind
	name string
}

// trigger — одна отслеживаемая комбинация
type trigger struct {
	idx     int
	mods    uint32
	vk      uint16
	kind    int
	bind    *Bind
	name    string // "Ctrl + 5"
	desc    string // что делает (для лога)
	swallow bool   // не пускать комбинацию в игру
}

var (
	triggers   []*trigger
	triggersBy = map[uint32]*trigger{}
)

func comboKey(mods uint32, vk uint16) uint32 { return mods<<16 | uint32(vk) }

func modBit(vk uint16) uint32 {
	switch vk {
	case 0xA2, 0xA3, 0x11: // Ctrl
		return modControl
	case 0xA4, 0xA5, 0x12: // Alt
		return modAlt
	case 0xA0, 0xA1, 0x10: // Shift
		return modShift
	case 0x5B, 0x5C: // Win
		return modWin
	}
	return 0
}

// prettyCombo — "Ctrl + 5" для логов
func prettyCombo(mods uint32, vk uint16) string {
	var parts []string
	if mods&modControl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&modAlt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&modShift != 0 {
		parts = append(parts, "Shift")
	}
	if mods&modWin != 0 {
		parts = append(parts, "Win")
	}
	parts = append(parts, prettyKeyName(vk))
	return strings.Join(parts, " + ")
}

// buildTriggers собирает список комбинаций, за которыми следит хук
func buildTriggers(cfg *Config) []string {
	triggers = nil
	triggersBy = map[uint32]*trigger{}
	var warns []string

	add := func(hk Hotkey, kind int, b *Bind, swallow bool, desc string) {
		if hk.VK == 0 {
			return // клавиша не назначена
		}
		k := comboKey(hk.Mods, hk.VK)
		if prev, dup := triggersBy[k]; dup {
			warns = append(warns, fmt.Sprintf("%s — комбинация уже занята («%s»)", hk.Pretty, prev.desc))
			return
		}
		t := &trigger{
			idx: len(triggers), mods: hk.Mods, vk: hk.VK,
			kind: kind, bind: b, name: hk.Pretty, desc: desc, swallow: swallow,
		}
		triggers = append(triggers, t)
		triggersBy[k] = t
	}

	if cfg.Settings.ToggleOK {
		add(cfg.Settings.ToggleHint, actToggle, nil, true, "показать/скрыть подсказки")
	}
	for i := range cfg.Binds {
		b := &cfg.Binds[i]
		desc := b.Hint
		if desc == "" {
			desc = truncate(cfg.substituteVariables(b.Send), 60)
		}
		add(b.Hotkey, actBind, b, true, desc)
	}
	return warns
}
