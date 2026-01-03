@echo off
REM 设置开发模式环境变量
set CCNEXUS_DEV_MODE=1

echo Starting ccNexus in development mode...
echo Using separate database: ~/.ccNexus-dev/
echo.

wails dev
