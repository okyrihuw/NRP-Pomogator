package main

import "strings"

// ---------------------------------------------------------------------------
//  Текст для буфера обмена
//
//  В режиме copy бинд НЕ печатает и НЕ нажимает клавиши — он только кладёт
//  готовую команду в буфер обмена. Поэтому:
//    • начальный "/" сохраняется как есть (команду можно вставить и в чат,
//      и в репорт);
//    • служебные вставки {space}, {tab}, {enter} превращаются в настоящие
//      символы (так "/get {space}" даёт "/get " с пробелом на конце —
//      удобно, когда ID вписываешь руками);
//    • переменные {my_id}, %Пол% и т.п. подставляет config.substituteVariables
//      ещё до вызова этой функции.
//
//  Эта часть не зависит от Windows и покрыта тестами (copytext_test.go).
// ---------------------------------------------------------------------------

func clipboardText(s string) string {
	var b strings.Builder
	for _, t := range splitTokens(s) {
		if t.vk == 0 {
			b.WriteString(t.text)
			continue
		}
		switch t.vk {
		case vkSpace:
			b.WriteByte(' ')
		case vkTab:
			b.WriteByte('\t')
		case vkReturn:
			b.WriteString("\r\n")
		}
	}
	out := b.String()
	// на конце оставляем ровно один пробел: "/get {space}" -> "/get "
	if strings.HasSuffix(out, " ") || strings.HasSuffix(out, "\t") {
		out = strings.TrimRight(out, " \t") + " "
	}
	return out
}

// clipboardPreview — одна короткая строка для подтверждения/трея
func clipboardPreview(s string, n int) string {
	s = strings.ReplaceAll(s, "\r\n", " ↵ ")
	s = strings.ReplaceAll(s, "\n", " ↵ ")
	return truncate(strings.TrimSpace(s), n)
}
