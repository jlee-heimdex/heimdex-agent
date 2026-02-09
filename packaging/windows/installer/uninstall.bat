@echo off
setlocal

set AGENT_NAME=Heimdex Agent
set BINARY_NAME=heimdex-agent.exe
set INSTALL_DIR=%LOCALAPPDATA%\Heimdex
set STARTUP_DIR=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup
set DATA_DIR=%USERPROFILE%\.heimdex

echo Uninstalling %AGENT_NAME%...

taskkill /f /im "%BINARY_NAME%" 2>nul

if exist "%STARTUP_DIR%\%AGENT_NAME%.lnk" (
    del "%STARTUP_DIR%\%AGENT_NAME%.lnk"
    echo Removed startup shortcut
)

if exist "%INSTALL_DIR%" (
    rmdir /s /q "%INSTALL_DIR%"
    echo Removed application
)

echo.
set /p REMOVE_DATA="Remove data directory (%DATA_DIR%)? [y/N] "
if /i "%REMOVE_DATA%"=="y" (
    if exist "%DATA_DIR%" (
        rmdir /s /q "%DATA_DIR%"
        echo Removed data directory
    )
) else (
    echo Data directory preserved
)

echo.
echo Uninstallation complete!

endlocal
pause
