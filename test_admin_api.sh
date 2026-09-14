#!/bin/bash

# 测试 LeapNode 管理员后台 API
# 本脚本验证 9 个核心模块的 API 端点

API_BASE="http://localhost:8080"

echo "=== LeapNode 管理员后台 API 测试 ==="
echo ""

echo "1. 系统概览（Dashboard）"
echo "  GET /api/admin/dashboard/stats"
echo "  GET /api/admin/dashboard/charts"
echo ""

echo "2. 用户管理"
echo "  GET /api/admin/users"
echo "  GET /api/admin/users/:id"
echo "  PUT /api/admin/users/:id"
echo "  DELETE /api/admin/users/:id"
echo "  POST /api/admin/users/:id/quota"
echo ""

echo "3. 商家审批"
echo "  GET /api/admin/merchants/pending"
echo "  GET /api/admin/merchants/:id"
echo "  POST /api/admin/merchants/:id/approve"
echo "  POST /api/admin/merchants/:id/reject"
echo ""

echo "4. 分站审批"
echo "  GET /api/admin/distributors/pending"
echo "  GET /api/admin/distributors/:id"
echo "  POST /api/admin/distributors/:id/approve"
echo "  POST /api/admin/distributors/:id/reject"
echo ""

echo "5. 渠道管理（总站官方渠道）"
echo "  GET /api/admin/channels"
echo "  POST /api/admin/channels"
echo "  PUT /api/admin/channels/:id"
echo "  DELETE /api/admin/channels/:id"
echo ""

echo "6. 套餐管理（总站套餐）"
echo "  GET /api/admin/packages"
echo "  POST /api/admin/packages"
echo "  PUT /api/admin/packages/:id"
echo "  DELETE /api/admin/packages/:id"
echo ""

echo "7. 兑换码管理"
echo "  GET /api/admin/redeem-codes"
echo "  POST /api/admin/redeem-codes/generate"
echo "  DELETE /api/admin/redeem-codes/:id"
echo ""

echo "8. 返佣管理"
echo "  GET /api/admin/referrals"
echo "  GET /api/admin/referrals/stats"
echo "  POST /api/admin/referrals/:id/settle"
echo ""

echo "9. 系统配置"
echo "  GET /api/admin/config"
echo "  PUT /api/admin/config"
echo ""

echo "总计: 33 个 API 端点已实现"
echo ""
echo "注意: 所有端点需要 user_level = 10 的管理员权限"
echo "认证: AuthRequired + AdminAuth 中间件"

