package main

import "testing"

func TestPauseStartsOffAndToggles(t *testing.T) {
	setBindsPaused(false)
	if bindsPaused() {
		t.Fatal("после запуска бинды должны работать, а не стоять на паузе")
	}
	if !toggleBindsPause() {
		t.Error("первое нажатие «Пауза» должно ставить на паузу")
	}
	if !bindsPaused() {
		t.Error("пауза не включилась")
	}
	if toggleBindsPause() {
		t.Error("второе нажатие «Продолжить» должно снимать паузу")
	}
	if bindsPaused() {
		t.Error("пауза не снялась")
	}
	setBindsPaused(false)
}
