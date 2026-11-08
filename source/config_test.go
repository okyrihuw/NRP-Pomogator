package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfigParses(t *testing.T) {
	cfg := parseConfig(defaultConfigINI)
	if len(cfg.Warnings) > 0 {
		t.Fatalf("замечания в конфиге по умолчанию: %v", cfg.Warnings)
	}
	if len(cfg.Binds) != 21 {
		t.Fatalf("биндов %d, ожидался 21", len(cfg.Binds))
	}
	if cfg.Settings.Nick == "" || cfg.Settings.MyID == "" {
		t.Error("ник и my_id должны быть заполнены")
	}
	if cfg.Settings.Nick != "Ник нейм" {
		t.Errorf("в конфиге по умолчанию должна быть заглушка «Ник нейм», а не %q", cfg.Settings.Nick)
	}
	if !cfg.Settings.CopyBeep {
		t.Error("звук по умолчанию должен быть включён")
	}
	if !cfg.Settings.CopyToast {
		t.Error("плашка «скопировано» по умолчанию должна быть включена")
	}
	if cfg.Settings.ToggleHint.Pretty != "Alt + P" {
		t.Errorf("подсказки должны висеть на Alt+P, а не на %q", cfg.Settings.ToggleHint.Pretty)
	}
	if !cfg.Settings.CopyNoHistory {
		t.Error("copy_no_history по умолчанию должен быть включён")
	}
}

func bindByKey(cfg *Config, key string) *Bind {
	for i := range cfg.Binds {
		if cfg.Binds[i].Hotkey.Pretty == key {
			return &cfg.Binds[i]
		}
	}
	return nil
}

// Команды сохранили слэш и по-прежнему лежат в нужных биндах
func TestDefaultCommands(t *testing.T) {
	cfg := parseConfig(defaultConfigINI)
	checks := []struct{ key, send string }{
		{"Ctrl + 1", "/adminmode"},
		{"Ctrl + 2", "/inv"},
		{"Ctrl + 3", "/reviving {my_id}"},
		{"Ctrl + 6", "/get{space}"},
		{"Ctrl + 7", "/vget{space}"},
		{"Ctrl + 8", "/setfactionlevel {my_id} 14"},
	}
	for _, c := range checks {
		b := bindByKey(cfg, c.key)
		if b == nil {
			t.Errorf("бинд %s не найден", c.key)
			continue
		}
		if b.Send != c.send {
			t.Errorf("%s: send=%q, хотим %q", c.key, b.Send, c.send)
		}
	}
}

// {my_id} и %Пол% подставляются
func TestSubstitutions(t *testing.T) {
	cfg := parseConfig(defaultConfigINI)
	b := bindByKey(cfg, "Ctrl + 8")
	if b == nil {
		t.Fatal("нет бинда Ctrl + 8")
	}
	if got := cfg.substituteVariables(b.Send); !strings.Contains(got, cfg.Settings.MyID) {
		t.Errorf("{my_id} не подставился: %q", got)
	}

	cfg = parseConfig("[vars]\nПол = а\n[bind]\nkey = numpad1\nsend = полезен{Пол} вам\n")
	if got := cfg.substituteVariables(cfg.Binds[0].Send); got != "полезена вам" {
		t.Errorf("Пол=а должен давать 'полезена вам', получили %q", got)
	}
}

// Лишние настройки из старых версий должны давать понятное замечание
func TestUnknownSettingsWarn(t *testing.T) {
	cfg := parseConfig("[settings]\nchat_key = t\n")
	if len(cfg.Warnings) == 0 {
		t.Fatal("неизвестная настройка должна давать замечание")
	}
}

func TestBindFields(t *testing.T) {
	cfg := parseConfig("[bind]\nkey = ctrl+alt+n\ngroup = Тест\nhint = подсказка\nsend = /test\n")
	if len(cfg.Binds) != 1 {
		t.Fatalf("биндов %d", len(cfg.Binds))
	}
	b := cfg.Binds[0]
	if b.Group != "Тест" || b.Hint != "подсказка" || b.Send != "/test" {
		t.Errorf("бинд разобран неверно: %+v", b)
	}
	if b.Hotkey.Pretty != "Ctrl + Alt + N" {
		t.Errorf("комбинация = %q", b.Hotkey.Pretty)
	}
}

// Плашку и звук можно выключить по отдельности, в том числе русскими словами.
func TestToastAndBeepSwitches(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "binds.ini")
	body := `[settings]
nick = Ник нейм
my_id = 00000
copy_toast = false
copy_beep = нет
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path, false)
	if err != nil {
		t.Fatalf("конфиг должен читаться: %v", err)
	}
	if cfg.Settings.CopyToast {
		t.Error("copy_toast = false должен выключать плашку")
	}
	if cfg.Settings.CopyBeep {
		t.Error("copy_beep = нет должен выключать звук")
	}
}
