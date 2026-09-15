#!/bin/bash
# 在 Railway 环境内执行：升级 leapnode_admin 为 root (role=100)
# 用法: railway run --service leapnode-backend bash scripts/promote_root.sh
# 用 PG* 环境变量连接（psql 原生支持），避免 GORM DSN 的 TimeZone 参数
export PGPASSWORD="$PGPASSWORD"
psql -h "$PGHOST" -p "${PGPORT:-5432}" -U "${PGUSER:-postgres}" -d "${PGDATABASE:-railway}" \
  -c "UPDATE users SET role = 100 WHERE username = 'leapnode_admin';" \
  -c "SELECT id, username, role, quota FROM users WHERE username = 'leapnode_admin';"
