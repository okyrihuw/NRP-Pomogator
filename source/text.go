package main

import "strings"

// ---------------------------------------------------------------------------
//  Разбор текста на литералы и служебные вставки {space} {enter} {tab}
//  (кросс-платформенная часть — покрыта тестами)
// ---------------------------------------------------------------------------

type token struct {
	text string
	vk   uint16
}

// splitTokens разбивает строку на текст и {space}/{enter}/{tab}/... токены
func splitTokens(s string) []token {
	var out []token
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			out = append(out, token{text: lit.String()})
			lit.Reset()
		}
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '{' {
			end := -1
			for j := i + 1; j < len(runes) && j < i+20; j++ {
				if runes[j] == '}' {
					end = j
					break
				}
			}
			if end > 0 {
				name := strings.ToLower(strings.TrimSpace(string(runes[i+1 : end])))
				if vk, ok := vkByName[name]; ok {
					flush()
					out = append(out, token{vk: vk})
					i = end
					continue
				}
			}
		}
		lit.WriteRune(runes[i])
	}
	flush()
	return out
}
