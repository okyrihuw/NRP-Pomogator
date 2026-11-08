# Как класть текст в буфер обмена так, чтобы он НЕ попадал в историю Win+V

Техническая записка. Всё проверено на Windows 10/11. Подходит для любого языка,
который умеет дёргать Win32 API (Go, Python, C#, C++, AutoHotkey, Rust и т.д.).

---

## 1. Суть в двух строках

Windows 10 (начиная с 1809) и Windows 11 при записи в буфер обмена смотрят на
**два необязательных «служебных» формата буфера**. Если они присутствуют и равны `0`,
запись **не попадает** в историю и не уходит в облако:

| Формат (имя регистрируется через `RegisterClipboardFormatW`) | Тип значения | `0` означает |
|---|---|---|
| `CanIncludeInClipboardHistory` | `DWORD` (4 байта) | не добавлять в историю Win+V |
| `CanUploadToCloudClipboard` | `DWORD` (4 байта) | не синхронизировать с облаком |

Это официальный механизм Microsoft, которым пользуются менеджеры паролей и банковские
клиенты. Он **не блокирует вставку** — обычный `Ctrl+V` работает как обычно, текст лежит
в буфере. Меняется только то, увидит ли его служба истории.

---

## 2. Порядок вызовов Win32 (критично)

Всё делается **внутри одной открытой сессии буфера**:

```
OpenClipboard(hwnd)                  // открыли
  EmptyClipboard()                   // очистили
  SetClipboardData(CF_UNICODETEXT, …)  // положили сам текст
  SetClipboardData(fmt1, dword 0)      // «не в историю»
  SetClipboardData(fmt2, dword 0)      // «не в облако»
CloseClipboard()                     // закрыли
```

Правила, без которых не работает:

1. **Флаги ставятся до `CloseClipboard`.** Служба истории читает содержимое в момент
   завершения записи буфера. Поставил после закрытия — уже поздно.
2. **`EmptyClipboard` — первым.** Он стирает *все* форматы, поэтому флаги нужно ставить
   после него, а не до.
3. **Оба флага — `DWORD` = 0, ровно 4 байта.** Это не строка и не `BOOL`-через-строку.
   Память выделяется `GlobalAlloc(GMEM_MOVEABLE, 4)`, в неё пишется `uint32 0`,
   затем `GlobalLock` → запись → `GlobalUnlock` → `SetClipboardData`.
4. **Имена форматов надо получать через `RegisterClipboardFormatW`**, а не хардкодить
   числовые ID: `CF_*`-констант для них нет, значения выдаёт система динамически.
5. Если `RegisterClipboardFormatW` вернул `0` (теоретически на очень старых сборках
   Windows) — просто пропускаем шаг, это безвредно.
6. После успешного `SetClipboardData` система **забирает владение** `HGLOBAL`.
   Освобождать его (и `GlobalFree`) в этом случае нельзя.

---

## 3. Готовый код

### Go (так сделано в Помогаторе)

```go
var (
    user32                  = syscall.NewLazyDLL("user32.dll")
    kernel32                = syscall.NewLazyDLL("kernel32.dll")
    pOpenClipboard          = user32.NewProc("OpenClipboard")
    pEmptyClipboard         = user32.NewProc("EmptyClipboard")
    pSetClipboardData       = user32.NewProc("SetClipboardData")
    pCloseClipboard         = user32.NewProc("CloseClipboard")
    pRegisterClipFmtW       = user32.NewProc("RegisterClipboardFormatW")
    pGlobalAlloc            = kernel32.NewProc("GlobalAlloc")
    pGlobalLock             = kernel32.NewProc("GlobalLock")
    pGlobalUnlock           = kernel32.NewProc("GlobalUnlock")
)

const (
    cfgUnicodeText = 13   // CF_UNICODETEXT
    gmemMoveable   = 0x0002
)

// setClipboardDword ставит служебный формат со значением DWORD.
// Вызывать ТОЛЬКО при уже открытом буфере (между OpenClipboard и CloseClipboard).
func setClipboardDword(format string, value uint32) bool {
    id, _, _ := pRegisterClipFmtW.Call(uintptr(unsafe.Pointer(utf16Ptr(format))))
    if id == 0 {
        return false
    }
    h, _, _ := pGlobalAlloc.Call(gmemMoveable, 4)
    if h == 0 {
        return false
    }
    ptr, _, _ := pGlobalLock.Call(h)
    if ptr == 0 {
        return false
    }
    *(*uint32)(unsafe.Pointer(ptr)) = value
    pGlobalUnlock.Call(h)
    r, _, _ := pSetClipboardData.Call(id, h)
    return r != 0
}

func setClipboardTextEx(s string, owner uintptr, noHistory bool) bool {
    if r, _, _ := pOpenClipboard.Call(owner); r == 0 {
        return false
    }
    defer pCloseClipboard.Call()
    pEmptyClipboard.Call()

    // --- CF_UNICODETEXT ---
    utf, _ := syscall.UTF16FromString(s)
    n := len(utf) * 2
    h, _, _ := pGlobalAlloc.Call(gmemMoveable, uintptr(n))
    if h == 0 {
        return false
    }
    ptr, _, _ := pGlobalLock.Call(h)
    if ptr == 0 {
        return false
    }
    dst := unsafe.Slice((*byte)(unsafe.Pointer(ptr)), n)
    for i := 0; i < n; i++ {
        dst[i] = byte(utf[i/2] >> (8 * (i % 2)))
    }
    pGlobalUnlock.Call(h)
    if r, _, _ := pSetClipboardData.Call(cfgUnicodeText, h); r == 0 {
        return false
    }

    // --- главное: просим не записывать в историю/облако ---
    if noHistory {
        setClipboardDword("CanIncludeInClipboardHistory", 0)
        setClipboardDword("CanUploadToCloudClipboard", 0)
    }
    return true
}
```

### Python (ctypes) — для GUI-биндера

```python
import ctypes, time
from ctypes import wintypes

user32   = ctypes.WinDLL("user32", use_last_error=True)
kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)

user32.OpenClipboard.argtypes       = [wintypes.HWND]
user32.OpenClipboard.restype        = wintypes.BOOL
user32.EmptyClipboard.restype       = wintypes.BOOL
user32.CloseClipboard.restype       = wintypes.BOOL
user32.SetClipboardData.argtypes    = [wintypes.UINT, wintypes.HANDLE]
user32.SetClipboardData.restype     = wintypes.HANDLE
user32.RegisterClipboardFormatW.argtypes = [wintypes.LPCWSTR]
user32.RegisterClipboardFormatW.restype  = wintypes.UINT
kernel32.GlobalAlloc.argtypes       = [wintypes.UINT, ctypes.c_size_t]
kernel32.GlobalAlloc.restype        = wintypes.HGLOBAL
kernel32.GlobalLock.argtypes        = [wintypes.HGLOBAL]
kernel32.GlobalLock.restype         = ctypes.c_void_p
kernel32.GlobalUnlock.argtypes      = [wintypes.HGLOBAL]

CF_UNICODETEXT = 13
GMEM_MOVEABLE  = 0x0002


def _set_dword_format(name: str, value: int) -> bool:
    """Ставит служебный формат. Вызывать при уже открытом буфере."""
    fmt = user32.RegisterClipboardFormatW(name)
    if not fmt:
        return False
    h = kernel32.GlobalAlloc(GMEM_MOVEABLE, 4)
    if not h:
        return False
    p = kernel32.GlobalLock(h)
    if not p:
        return False
    ctypes.cast(p, ctypes.POINTER(ctypes.c_uint32))[0] = value
    kernel32.GlobalUnlock(h)
    return bool(user32.SetClipboardData(fmt, h))


def copy_text(text: str, *, no_history: bool = True, retries: int = 15) -> bool:
    """Кладёт текст в буфер, не засоряя историю Win+V."""
    for i in range(retries):
        if user32.OpenClipboard(None):
            break
        time.sleep(0.02 + i * 0.01)   # буфер бывает занят другой программой
    else:
        return False

    try:
        user32.EmptyClipboard()

        data = text.encode("utf-16-le") + b"\x00\x00"
        h = kernel32.GlobalAlloc(GMEM_MOVEABLE, len(data))
        if not h:
            return False
        p = kernel32.GlobalLock(h)
        if not p:
            return False
        ctypes.memmove(p, data, len(data))
        kernel32.GlobalUnlock(h)
        if not user32.SetClipboardData(CF_UNICODETEXT, h):
            return False

        if no_history:
            _set_dword_format("CanIncludeInClipboardHistory", 0)
            _set_dword_format("CanUploadToCloudClipboard", 0)
        return True
    finally:
        user32.CloseClipboard()
```

### C# (.NET, WinForms/WPF)

```csharp
var data = new DataObject();
data.SetData(DataFormats.UnicodeText, text);

// значение должно быть 4-байтовым DWORD = 0
data.SetData("CanIncludeInClipboardHistory", new byte[4]);
data.SetData("CanUploadToCloudClipboard", new byte[4]);

Clipboard.SetDataObject(data, copy: true);
```

Если конкретная версия .NET капризничает с `byte[]` — заверни значение в `MemoryStream`
той же длины 4 байта либо используй Win32-путь из Go/Python выше (он универсальный).

### AutoHotkey v2

```ahk
CopyNoHistory(text) {
    static CF_UNICODETEXT := 13, GMEM_MOVEABLE := 2
    if !DllCall("OpenClipboard", "ptr", 0)
        return false
    try {
        DllCall("EmptyClipboard")

        ; сначала сам текст
        buf := StrPut(text, "UTF-16") * 2
        h := DllCall("GlobalAlloc", "uint", GMEM_MOVEABLE, "uptr", buf, "ptr")
        p := DllCall("GlobalLock", "ptr", h, "ptr")
        StrPut(text, p, "UTF-16")
        DllCall("GlobalUnlock", "ptr", h)
        DllCall("SetClipboardData", "uint", CF_UNICODETEXT, "ptr", h)

        ; потом «не в историю»
        for name in ["CanIncludeInClipboardHistory", "CanUploadToCloudClipboard"] {
            fmt := DllCall("RegisterClipboardFormatW", "wstr", name, "uint")
            hh  := DllCall("GlobalAlloc", "uint", GMEM_MOVEABLE, "uptr", 4, "ptr")
            pp  := DllCall("GlobalLock", "ptr", hh, "ptr")
            NumPut("uint", 0, pp)
            DllCall("GlobalUnlock", "ptr", hh)
            DllCall("SetClipboardData", "uint", fmt, "ptr", hh)
        }
        return true
    } finally {
        DllCall("CloseClipboard")
    }
}
```

Важно: нельзя писать `A_Clipboard := text` и потом «добавлять» форматы — присваивание
само открывает и закрывает буфер, служебные форматы уже не успеют.

---

## 4. Как проверить, что сработало

**Практически:** скопируй текст, нажми `Win+V`. Нового элемента вверху списка быть не
должно, а `Ctrl+V` в Блокноте вставляет текст нормально.

**Программно** (перечисление форматов в буфере):

```python
def clipboard_formats() -> dict[int, str]:
    out = {}
    if not user32.OpenClipboard(None):
        return out
    try:
        fmt = 0
        while True:
            fmt = user32.EnumClipboardFormats(fmt)
            if not fmt:
                break
            buf = ctypes.create_unicode_buffer(256)
            user32.GetClipboardFormatNameW(fmt, buf, 256)
            out[fmt] = buf.value or f"CF_{fmt}"
    finally:
        user32.CloseClipboard()
    return out

# В списке должны появиться CanIncludeInClipboardHistory и CanUploadToCloudClipboard
```

---

## 5. Ограничения и подводные камни

| Что | Детали |
|---|---|
| Версия Windows | Форматы появились в Windows 10 **1809** вместе с историей буфера. На более старых сборках они просто игнорируются (безвредно). |
| Только новые записи | Уже накопленные записи из истории это не удаляет. **API удалить одну запись нет** — только «Очистить всё» в `Win+V` или Параметры → Буфер обмена. |
| История должна быть включена | Если пользователь выключил историю, формат ни на что не влияет (и не нужен). |
| Облако — отдельно | `CanIncludeInClipboardHistory` не мешает синхронизации, для неё нужен второй флаг. Ставь оба. |
| Сторонние менеджеры буфера | Ditto, ClipClip, ClipboardFusion и подобные эти форматы обычно **игнорируют** — они читают `CF_UNICODETEXT` напрямую. Механизм действует только на встроенную службу Windows. |
| Вставка не блокируется | Текст остаётся в буфере и вставляется обычным `Ctrl+V`. Это именно то, что нужно для биндера. |
| OLE-путь | Если пишешь через `IDataObject`/`DataObject`, добавь те же два формата в объект данных до передачи в `OleSetClipboard`/`Clipboard.SetDataObject`. |

### Что НЕ работает (частые заблуждения)

- ❌ `EmptyClipboard()` в конце — просто стёр бы текст, историю не трогает.
- ❌ «Очистить буфер после вставки» — запись в истории уже создана, она останется.
- ❌ Старый формат `Clipboard Viewer Ignore` — он влияет только на древние
  clipboard-viewer'ы (`WM_DRAWCLIPBOARD`-цепочку), к Win+V отношения не имеет.
- ❌ `ExcludeClipboardContentFromMonitorProcessing` — тоже про сторонние мониторы
  буфера, а не про встроенную историю; в связке с двумя основными форматами не нужен.

---

## 6. Короткий промпт для другой нейронки

> Мне нужно класть текст в буфер обмена Windows так, чтобы он **не попадал в историю
> Win+V** и не уходил в облачный буфер, но при этом обычный `Ctrl+V` работал.
> Используй официальный механизм: при открытом буфере (`OpenClipboard` →
> `EmptyClipboard` → `SetClipboardData(CF_UNICODETEXT, …)`) дополнительно запиши два
> формата, полученных через `RegisterClipboardFormatW`: `CanIncludeInClipboardHistory`
> и `CanUploadToCloudClipboard`, каждый — как `HANDLE` от `GlobalAlloc(GMEM_MOVEABLE, 4)`
> с `DWORD 0` внутри. Все вызовы — до `CloseClipboard`, порядок именно такой.
> Дай рабочий код на <язык> без внешних зависимостей, с повторными попытками
> `OpenClipboard` (буфер бывает занят) и функцией проверки через `EnumClipboardFormats`.
