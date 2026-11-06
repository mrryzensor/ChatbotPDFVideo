@echo off
setlocal enabledelayedexpansion
title OmniSaaS Hub - Instalador y Lanzador Automatico
color 0B

echo ======================================================================
echo             BIENVENIDO A OMNISAAS HUB (IA, PDF & VIDEO)
echo ======================================================================
echo  Preparando el entorno e instalando dependencias necesarias...
echo.

cd /d "%~dp0"

:: 1. Verificar o instalar Go
where go >nul 2>&1
if %ERRORLEVEL% neq 0 (
    if not exist "C:\Program Files\Go\bin\go.exe" (
        echo [-] Go no detectado. Instalando Go automaticamente...
        winget install --id GoLang.Go -e --silent --accept-package-agreements --accept-source-agreements
        set "PATH=%PATH%;C:\Program Files\Go\bin"
    ) else (
        set "PATH=%PATH%;C:\Program Files\Go\bin"
    )
) else (
    echo [+] Go ya esta disponible.
)

:: 2. Verificar o instalar Node.js
where node >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [-] Node.js no detectado. Instalando Node.js automaticamente...
    winget install --id OpenJS.NodeJS.LTS -e --silent --accept-package-agreements --accept-source-agreements
    set "PATH=%PATH%;C:\Program Files\nodejs"
) else (
    echo [+] Node.js ya esta disponible.
)

:: 3. Verificar o instalar yt-dlp y FFmpeg
where yt-dlp >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [-] yt-dlp o FFmpeg no detectados en PATH. Comprobando instalación con winget...
    winget install --id yt-dlp.yt-dlp -e --silent --accept-package-agreements --accept-source-agreements
    winget install --id Gyan.FFmpeg -e --silent --accept-package-agreements --accept-source-agreements
)

:: 4. Preparar carpetas requeridas
if not exist "uploads" mkdir uploads
if not exist "temp_downloads" mkdir temp_downloads

:: 5. Compilar frontend si no existe dist
if not exist "frontend\dist\index.html" (
    echo.
    echo [*] Compilando el Frontend moderno en React...
    cd frontend
    call npm install
    call npm run build
    cd ..
)

:: 6. Compilar backend si no existe app.exe
if not exist "app.exe" (
    echo.
    echo [*] Compilando el servidor Go nativo...
    go build -o app.exe main.go
)

echo.
echo ======================================================================
echo    TODO LISTO! INICIANDO LA APLICACION EN http://localhost:8080
echo ======================================================================
echo.
echo [!] Tu navegador se abrira en unos segundos.
echo [!] Para detener el servidor, simplemente cierra esta ventana.
echo.

:: Abrir navegador automáticamente
start "" http://localhost:8080

:: Iniciar el servidor
app.exe

pause
