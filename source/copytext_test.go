package main

import "testing"

func TestClipboardTextKeepsSlashAndTrailingSpace(t *testing.T) {
	got := clipboardText("/get {space}")
	if got != "/get " {
		t.Fatalf("получилось %q, ожидалось %q", got, "/get ")
	}
	got = clipboardText("/fixveh{space}")
	if got != "/fixveh " {
		t.Fatalf("получилось %q, ожидалось %q", got, "/fixveh ")
	}
}

func TestClipboardTextTokens(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/adminmode", "/adminmode"},
		{"Строка один{enter}Строка два", "Строка один\r\nСтрока два"},
		{"a{tab}b", "a\tb"},
		{"без вставок", "без вставок"},
	}
	for _, c := range cases {
		if got := clipboardText(c.in); got != c.want {
			t.Errorf("clipboardText(%q) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}

func TestClipboardTextKeepsCyrillicAndPercent(t *testing.T) {
	got := clipboardText("/me Чем я могу быть полезен%Пол% вам?")
	want := "/me Чем я могу быть полезен%Пол% вам?"
	if got != want {
		t.Fatalf("получилось %q, ожидалось %q", got, want)
	}
}

func TestClipboardPreviewShort(t *testing.T) {
	if got := clipboardPreview("/adminmode", 64); got != "/adminmode" {
		t.Fatalf("preview = %q", got)
	}
}
