#!/bin/bash
# 设置开发模式环境变量
export CCNEXUS_DEV_MODE=1

echo "Starting ccNexus in development mode..."
echo "Using separate database: ~/.ccNexus-dev/"
echo ""

wails dev
