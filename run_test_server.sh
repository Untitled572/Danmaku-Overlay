#!/bin/bash

# 确保在出错时退出
set -e

echo "=== 配置环境变量 ==="
# 设置鉴权 Token (必须与 test.html 中的 token 匹配)
export APP_LOCAL_TOKEN="test_token"
# 数据库存储在当前文件夹
export DB_PATH="./danmaku.db"
# 其他缓存数据(covers, danmaku等)存在当前文件夹
export DATA_DIR="."

echo "=== 清理历史数据（测试环境） ==="
rm -f ./danmaku.db ./danmaku.db-shm ./danmaku.db-wal
rm -rf ./covers ./danmaku
echo "已清理旧数据库和缓存文件夹，确保本次测试全新的刮削逻辑！"

# 检查 sqlite3 是否存在
if ! command -v sqlite3 &> /dev/null; then
    echo "警告: 未安装 sqlite3，将通过启动后端再通过 API 插入 Library。"
else
    # 预设媒体库，确保后端一启动 Scanner 就会挂载到该目录
    if [ ! -f ./danmaku.db ]; then
        echo "=== 预设数据库媒体库路径 ==="
        sqlite3 ./danmaku.db "CREATE TABLE IF NOT EXISTS libraries (id INTEGER PRIMARY KEY AUTOINCREMENT, root_path TEXT);"
        sqlite3 ./danmaku.db "INSERT INTO libraries (root_path) VALUES ('/mnt/F/Anime/New');"
        echo "成功将 /mnt/F/Anime/New 写入数据库作为主媒体库。"
    fi
fi

echo "=== 启动 Go Media Core ==="
go run cmd/danmaku/main.go
