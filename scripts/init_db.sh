#!/bin/bash
# Railway 内部数据库初始化脚本

set -e

echo "=== LeapNode 数据库初始化 ==="

# 检查环境变量
if [ -z "$LEAPNODE_PG_DSN" ]; then
    echo "错误: LEAPNODE_PG_DSN 未设置"
    exit 1
fi

# 从 DSN 提取连接参数
PGHOST=$(echo $LEAPNODE_PG_DSN | grep -o 'host=[^ ]*' | cut -d= -f2)
PGUSER=$(echo $LEAPNODE_PG_DSN | grep -o 'user=[^ ]*' | cut -d= -f2)
PGPASSWORD=$(echo $LEAPNODE_PG_DSN | grep -o 'password=[^ ]*' | cut -d= -f2)
PGDATABASE=$(echo $LEAPNODE_PG_DSN | grep -o 'dbname=[^ ]*' | cut -d= -f2)
PGPORT=$(echo $LEAPNODE_PG_DSN | grep -o 'port=[^ ]*' | cut -d= -f2)

export PGHOST PGUSER PGPASSWORD PGDATABASE PGPORT

echo "连接到: $PGHOST:$PGPORT/$PGDATABASE"

# 执行 DDL
psql -f /app/migrations/001_initial_schema.sql

echo "✅ 数据库初始化完成"
