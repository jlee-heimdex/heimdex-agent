@echo off
setlocal

set AGENT_NAME=Heimdex Agent
set BINARY_NAME=heimdex-agent.exe
set INSTALL_DIR=%LOCALAPPDATA%\Heimdex
set STARTUP_DIR=%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup

echo Installing %AGENT_NAME%...

if not exist "%BINARY_NAME%" (
    echo Error: %BINARY_NAME% not found in current directory
    exit /b 1
)

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
copy /Y "%BINARY_NAME%" "%INSTALL_DIR%\"

echo Set oWS = WScript.CreateObject("WScript.Shell") > "%TEMP%\CreateShortcut.vbs"
echo sLinkFile = "%STARTUP_DIR%\%AGENT_NAME%.lnk" >> "%TEMP%\CreateShortcut.vbs"
echo Set oLink = oWS.CreateShortcut(sLinkFile) >> "%TEMP%\CreateShortcut.vbs"
echo oLink.TargetPath = "%INSTALL_DIR%\%BINARY_NAME%" >> "%TEMP%\CreateShortcut.vbs"
echo oLink.WorkingDirectory = "%INSTALL_DIR%" >> "%TEMP%\CreateShortcut.vbs"
echo oLink.Save >> "%TEMP%\CreateShortcut.vbs"
cscript /nologo "%TEMP%\CreateShortcut.vbs"
del "%TEMP%\CreateShortcut.vbs"

echo.
echo Installation complete!
echo.
echo The agent will start automatically on next login.
echo To start now, run:
echo   "%INSTALL_DIR%\%BINARY_NAME%"
echo.
echo Data is stored in: %USERPROFILE%\.heimdex

start "" "%INSTALL_DIR%\%BINARY_NAME%"

endlocal
