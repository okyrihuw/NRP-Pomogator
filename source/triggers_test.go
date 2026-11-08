package main

import (
	"testing"
)

// Главная проверка: правильные ли комбинации попадают в перехват
func TestBuildTriggers(t *testing.T) {
	cfg := parseConfig(defaultConfigINI)
	warns := buildTriggers(cfg)
	if len(warns) > 0 {
		t.Fatalf("неожиданные замечания: %v", warns)
	}

	tr, ok := triggersBy[comboKey(modControl, 0x35)]
	if !ok {
		t.Fatal("комбинация Ctrl+5 не попала в список перехвата")
	}
	if tr.kind != actBind || tr.bind == nil {
		t.Fatalf("Ctrl+5: неверное действие %+v", tr)
	}
	if !tr.swallow {
		t.Error("комбинация должна «проглатываться» (не уходить в игру)")
	}
	if tr.name != "Ctrl + 5" {
		t.Errorf("имя комбинации = %q", tr.name)
	}

	if _, ok := triggersBy[comboKey(modAlt, 0x59)]; !ok {
		t.Error("Alt+Y не попал в список перехвата")
	}
	// Alt+з — это физическая P (как !p в AHK)
	if _, ok := triggersBy[comboKey(modAlt, 0x50)]; !ok {
		t.Error("Alt+P (подсказки) не попал в список перехвата")
	}
	// Numpad 1 — без модификаторов
	if _, ok := triggersBy[comboKey(0, 0x61)]; !ok {
		t.Error("Numpad 1 не попал в список перехвата")
	}

	// все бинды из конфига на месте (21 бинд + Alt+P)
	if len(triggers) != len(cfg.Binds)+1 {
		t.Errorf("комбинаций %d, ожидалось %d", len(triggers), len(cfg.Binds)+1)
	}
}

func TestDuplicateComboWarns(t *testing.T) {
	cfg := parseConfig(`
[settings]
toggle_hint =
[bind]
key = ctrl+1
send = /a
[bind]
key = ctrl+1
send = /b
`)
	warns := buildTriggers(cfg)
	if len(warns) != 1 {
		t.Fatalf("ожидалось одно замечание о дубликате, получили %v", warns)
	}
	if len(triggers) != 1 {
		t.Errorf("в перехвате %d комбинаций, ожидалась 1", len(triggers))
	}
}
