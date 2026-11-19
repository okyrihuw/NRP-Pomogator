package main

import "sync/atomic"

// ---------------------------------------------------------------------------
//  Пауза.
//
//  Пока пауза включена, бинды не срабатывают: нажатие проходит мимо, как
//  обычное нажатие клавиши. Сама программа при этом работает — значок на
//  месте, окошко с подсказками открывается, меню доступно.
//
//  Выключить паузу — правый клик по значку → «Продолжить».
// ---------------------------------------------------------------------------

var bindsPausedFlag atomic.Bool

// bindsPaused сообщает, стоят ли бинды на паузе.
func bindsPaused() bool {
	return bindsPausedFlag.Load()
}

// setBindsPaused включает или выключает паузу.
func setBindsPaused(v bool) {
	bindsPausedFlag.Store(v)
}

// toggleBindsPause переключает паузу и возвращает новое состояние.
func toggleBindsPause() bool {
	next := !bindsPaused()
	bindsPausedFlag.Store(next)
	return next
}
