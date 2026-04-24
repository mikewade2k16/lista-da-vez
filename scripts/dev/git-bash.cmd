@echo off
setlocal

set "BASH_EXE=C:\Program Files\Git\bin\bash.exe"

if not exist "%BASH_EXE%" (
  echo Git Bash nao encontrado em "%BASH_EXE%".
  exit /b 1
)

"%BASH_EXE%" %*
