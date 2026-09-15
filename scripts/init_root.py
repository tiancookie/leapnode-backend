import os
import psycopg2

dsn = os.environ.get('DATABASE_URL')
if not dsn:
    # 从 Railway 环境读取
    dsn = "postgresql://postgres:***@postgres.railway.internal:5432/railway"

conn = psycopg2.connect(dsn)
cur = conn.cursor()

# 升级 leapnode_admin 为 root
cur.execute("UPDATE users SET role = 100 WHERE username = 'leapnode_admin'")
rows = cur.rowcount
conn.commit()

print(f"✅ Updated {rows} rows")

# 确认
cur.execute("SELECT id, username, role FROM users WHERE username = 'leapnode_admin'")
user = cur.fetchone()
if user:
    print(f"Confirmed: ID={user[0]} Username={user[1]} Role={user[2]}")
else:
    print("⚠️ User not found")

conn.close()
