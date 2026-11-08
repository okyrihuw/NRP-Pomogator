@echo off
rem ---------------------------------------------------------------------------
rem  Сборка Помогатора под Windows x64
rem  Требуется Go 1.22+ в PATH: https://go.dev/dl/
rem ---------------------------------------------------------------------------
setlocal
cd /d "%~dp0"

where go >nul 2>nul
if errorlevel 1 (
    echo.
    echo [ОШИБКА] Go не найден в PATH.
    echo Установи Go с https://go.dev/dl/ и перезапусти командную строку.
    echo.
    pause
    exit /b 1
)

cd source
echo Сборка...
go build -trimpath -ldflags "-s -w -H windowsgui" -o "..\Pomogator.exe" .
if errorlevel 1 (
    echo.
    echo [ОШИБКА] Сборка не удалась.
    pause
    exit /b 1
)
cd ..

echo.
echo Готово: %cd%\Pomogator.exe
echo Рядом положи binds.ini (он уже в папке) и запускай.
pause
