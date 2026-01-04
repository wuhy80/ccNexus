@echo off
REM 设置开发模式环境变量 - 禁用代理服务器但使用正式数据库
set CCNEXUS_NO_PROXY=1

echo Starting ccNexus in development mode...
echo Using production database: ~/.ccNexus/
echo Proxy server disabled for UI testing
echo.

wails dev
