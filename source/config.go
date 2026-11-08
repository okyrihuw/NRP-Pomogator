package main

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"strings"
)

//go:embed default_binds.ini
var defaultConfigINI string

// ---------------------------------------------------------------------------
//  Конфиг v1.7
//
//  Всё, что связано с автоматической печатью в игру (клавиша чата, режимы
//  ввода, удержания клавиш, задержки, раскладки), из программы убрано.
//  Осталось только то, что реально нужно: бинды → буфер обмена.
// ---------------------------------------------------------------------------

type Settings struct {
	Nick string
	Rank string
	MyID string

	// подтверждение копирования
	CopyToast     bool
	CopyToastMs   int
	CopyBeep      bool
	CopyNoHistory bool // не засорять историю Win+V

	// окошко подсказок
	HintFont   string
	HintSize   int
	HintX      int
	HintY      int
	HintAlpha  int
	HintBG     uint32
	TitleColor uint32
	GroupColor uint32
	KeyColor   uint32
	TextColor  uint32

	HintOnStart       bool
	HintStartMs       int
	TrayNotifyOnStart bool
	OnlyWhenTitle     string

	ToggleHint Hotkey // Alt+P — показать/скрыть подсказки
	ToggleOK   bool
}

func defaultSettings() Settings {
	return Settings{
		Nick:              "Админ",
		Rank:              "",
		MyID:              "",
		CopyToast:         true,
		CopyToastMs:       1600,
		CopyBeep:          true,
		CopyNoHistory:     true,
		HintFont:          "Consolas",
		HintSize:          14,
		HintX:             10,
		HintY:             300,
		HintAlpha:         225,
		HintBG:            0x0A0A0A,
		TitleColor:        0xFFD75F,
		GroupColor:        0xFF4D4D,
		KeyColor:          0x7FD1FF,
		TextColor:         0xF2F2F2,
		HintOnStart:       true,
		HintStartMs:       5000,
		TrayNotifyOnStart: true,
		ToggleOK:          true,
	}
}

// Bind — один бинд: клавиша + текст, который кладётся в буфер
type Bind struct {
	KeyRaw string
	Group  string
	Hint   string
	Send   string

	Hotkey Hotkey
	Line   int
	Idx    int
}

type Config struct {
	Settings Settings
	Vars     map[string]string
	Binds    []Bind
	Path     string
	Warnings []string
}

// ---------------------------------------------------------------------------
//  Разбор INI
// ---------------------------------------------------------------------------

func loadConfig(path string, autoCreate bool) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && autoCreate {
			if werr := os.WriteFile(path, []byte(defaultConfigINI), 0o644); werr != nil {
				return nil, werr
			}
			data = []byte(defaultConfigINI)
		} else {
			return nil, err
		}
	}
	cfg := parseConfig(string(data))
	cfg.Path = path
	return cfg, nil
}

func parseConfig(text string) *Config {
	cfg := &Config{
		Settings: defaultSettings(),
		Vars:     map[string]string{},
	}
	section := ""

	var cur *Bind
	flush := func() {
		if cur == nil {
			return
		}
		cur.Idx = len(cfg.Binds)
		if err := finishBind(cur); err != nil {
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("строка %d: %v (бинд пропущен)", cur.Line, err))
		} else {
			cfg.Binds = append(cfg.Binds, *cur)
		}
		cur = nil
	}

	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		raw := strings.TrimSpace(sc.Text())
		if raw == "" || strings.HasPrefix(raw, ";") || strings.HasPrefix(raw, "#") {
			continue
		}
		if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
			flush()
			switch strings.ToLower(strings.Trim(raw, "[] ")) {
			case "settings", "настройки":
				section = "settings"
			case "vars", "переменные":
				section = "vars"
			case "bind", "бинд":
				section = "bind"
				cur = &Bind{Line: lineNo}
			default:
				section = ""
				cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("строка %d: неизвестный раздел %s", lineNo, raw))
			}
			continue
		}
		key, val, ok := splitKeyVal(raw)
		if !ok {
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("строка %d: не понял строку %q", lineNo, truncate(raw, 60)))
			continue
		}
		switch section {
		case "settings":
			if err := cfg.applySetting(key, val); err != nil {
				cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("строка %d: %v", lineNo, err))
			}
		case "vars":
			cfg.Vars[key] = val
		case "bind":
			if cur == nil {
				cur = &Bind{Line: lineNo}
			}
			if err := applyBindField(cur, key, val); err != nil {
				cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("строка %d: %v", lineNo, err))
			}
		default:
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("строка %d: настройка %q вне раздела [settings]/[vars]/[bind]", lineNo, key))
		}
	}
	flush()
	cfg.finishSettings()
	return cfg
}

func (c *Config) finishSettings() {
	if c.Settings.HintSize < 7 {
		c.Settings.HintSize = 7
	}
	if c.Settings.HintSize > 40 {
		c.Settings.HintSize = 40
	}
	if c.Settings.HintAlpha < 40 {
		c.Settings.HintAlpha = 40
	}
	if c.Settings.HintAlpha > 255 {
		c.Settings.HintAlpha = 255
	}
	if c.Settings.CopyToastMs < 400 {
		c.Settings.CopyToastMs = 400
	}

	// встроенные переменные: {my_id}, {nick}, {rank}
	builtin := map[string]string{
		"my_id": c.Settings.MyID, "MyID": c.Settings.MyID, "id": c.Settings.MyID,
		"nick": c.Settings.Nick, "rank": c.Settings.Rank,
		"ник": c.Settings.Nick, "звание": c.Settings.Rank,
	}
	for k, v := range builtin {
		if _, user := c.Vars[k]; !user && v != "" {
			c.Vars[k] = v
		}
	}

	if c.Settings.ToggleHint.VK == 0 {
		c.Settings.ToggleOK = false
	}
}

func (c *Config) applySetting(key, val string) error {
	s := &c.Settings
	var err error
	switch strings.ToLower(key) {
	case "nick", "никнейм", "nickname":
		s.Nick = val
	case "rank", "звание":
		s.Rank = val
	case "my_id", "myid", "мой_id":
		s.MyID = val

	// --- подтверждение копирования ---
	case "copy_toast", "toast", "плашка", "подтверждение_копирования":
		s.CopyToast = parseBool(val, s.CopyToast)
	case "copy_toast_ms":
		s.CopyToastMs, err = intField("copy_toast_ms", val, s.CopyToastMs)
		if err != nil {
			return err
		}
	case "copy_beep", "beep", "звук", "звук_при_копировании":
		s.CopyBeep = parseBool(val, s.CopyBeep)
	case "copy_no_history", "не_писать_в_историю":
		s.CopyNoHistory = parseBool(val, s.CopyNoHistory)

	// --- окошко подсказок ---
	case "hint_font", "шрифт":
		s.HintFont = val
	case "hint_size", "размер_подсказки":
		s.HintSize, err = intField("hint_size", val, s.HintSize)
		if err != nil {
			return err
		}
	case "hint_x":
		s.HintX, err = intField("hint_x", val, s.HintX)
		if err != nil {
			return err
		}
	case "hint_y":
		s.HintY, err = intField("hint_y", val, s.HintY)
		if err != nil {
			return err
		}
	case "hint_alpha":
		s.HintAlpha, err = intField("hint_alpha", val, s.HintAlpha)
		if err != nil {
			return err
		}
	case "hint_bg":
		v, err := parseHexColor(val)
		if err != nil {
			return fmt.Errorf("hint_bg: %v", err)
		}
		s.HintBG = v
	case "title_color":
		v, err := parseHexColor(val)
		if err != nil {
			return fmt.Errorf("title_color: %v", err)
		}
		s.TitleColor = v
	case "group_color":
		v, err := parseHexColor(val)
		if err != nil {
			return fmt.Errorf("group_color: %v", err)
		}
		s.GroupColor = v
	case "key_color":
		v, err := parseHexColor(val)
		if err != nil {
			return fmt.Errorf("key_color: %v", err)
		}
		s.KeyColor = v
	case "text_color":
		v, err := parseHexColor(val)
		if err != nil {
			return fmt.Errorf("text_color: %v", err)
		}
		s.TextColor = v
	case "hint_show_on_start":
		s.HintOnStart = parseBool(val, s.HintOnStart)
	case "hint_start_hide_ms":
		s.HintStartMs, err = intField("hint_start_hide_ms", val, s.HintStartMs)
		if err != nil {
			return err
		}
	case "tray_notify_on_start", "подсказка_в_трее":
		s.TrayNotifyOnStart = parseBool(val, s.TrayNotifyOnStart)
	case "only_when_window":
		s.OnlyWhenTitle = val
	case "toggle_hint":
		if strings.TrimSpace(val) == "" {
			s.ToggleOK = false
			return nil
		}
		hk, err := parseHotkey(val)
		if err != nil {
			s.ToggleOK = false
			return fmt.Errorf("toggle_hint: %v", err)
		}
		s.ToggleHint = hk
	default:
		return fmt.Errorf("неизвестная настройка %q (в v1.7 лишних настроек нет, проверь название)", key)
	}
	return nil
}

func applyBindField(b *Bind, key, val string) error {
	switch strings.ToLower(key) {
	case "key", "клавиша":
		b.KeyRaw = val
	case "group", "раздел":
		b.Group = val
	case "hint", "подсказка":
		b.Hint = val
	case "send", "текст":
		b.Send = val
	default:
		return fmt.Errorf("неизвестное поле бинда %q (в v1.7 есть только key/group/hint/send)", key)
	}
	return nil
}

func finishBind(b *Bind) error {
	if b.KeyRaw == "" {
		return fmt.Errorf("нет поля key")
	}
	hk, err := parseHotkey(b.KeyRaw)
	if err != nil {
		return err
	}
	b.Hotkey = hk
	return nil
}

// ---------------------------------------------------------------------------
//  Мелкие помощники
// ---------------------------------------------------------------------------

func splitKeyVal(line string) (string, string, bool) {
	i := strings.Index(line, "=")
	if i < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:i])
	if key == "" {
		return "", "", false
	}
	val := stripTrailingComment(line[i+1:])
	return key, val, true
}

func stripTrailingComment(v string) string {
	depth := 0
	for i, r := range v {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case ';', '#':
			// комментарий, если он стоит в начале значения или после пробела
			if depth == 0 && (i == 0 || v[i-1] == ' ' || v[i-1] == '\t') {
				return strings.TrimSpace(v[:i])
			}
		}
	}
	return strings.TrimSpace(v)
}

// intField — как atoiDef, но сообщает об ошибке (чтобы человек видел замечание)
func intField(what, s string, def int) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def, fmt.Errorf("%s: ожидалось число, а не %q", what, s)
	}
	return n, nil
}

func parseBool(s string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "да", "on", "вкл", "y":
		return true
	case "0", "false", "no", "нет", "off", "выкл", "n":
		return false
	}
	return def
}

func parseHexColor(s string) (uint32, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	s = strings.TrimPrefix(s, "0x")
	if len(s) != 6 {
		return 0, fmt.Errorf("цвет должен быть в виде RRGGBB, а не %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("цвет %q не разобран", s)
	}
	return uint32(v), nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// substituteVariables подставляет %Имя% и {Имя} из раздела [vars]
func (c *Config) substituteVariables(s string) string {
	if len(c.Vars) == 0 {
		return s
	}
	for k, v := range c.Vars {
		s = strings.ReplaceAll(s, "%"+k+"%", v)
		s = strings.ReplaceAll(s, "{"+k+"}", v)
	}
	return s
}
