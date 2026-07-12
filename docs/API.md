# Danmaku Overlay API 文档

**基础地址：** `http://<host>:<port>`

**认证方式：** 设置了 `APP_LOCAL_TOKEN` 后，受保护接口需在请求头添加 `Authorization: Bearer <token>`。

---

## 1. 健康检查

```
GET /api/v1/health
```

无需认证。响应：
```json
{"status": "ok"}
```

---

## 2. 获取任务状态

```
GET /api/v1/status
```

需认证。获取扫描和刮削的进度状态。完成后保留最后一次结果供查询。

响应：
```json
{
  "scan": {
    "status": "completed",
    "current": 200,
    "total": 200,
    "percentage": 100,
    "message": "200 files, 50 new, 5.2s",
    "started_at": "2026-07-12T12:00:00Z",
    "updated_at": "2026-07-12T12:00:05Z",
    "duration": "5.2s"
  },
  "scrape": {
    "status": "running",
    "current": 3,
    "total": 10,
    "percentage": 30,
    "message": "scraping...",
    "started_at": "2026-07-12T12:00:06Z",
    "updated_at": "2026-07-12T12:01:00Z",
    "duration": ""
  }
}
```

**status 字段说明**：
| 值 | 说明 |
|---|---|
| `idle` | 空闲 |
| `running` | 运行中 |
| `completed` | 已完成（保留最后一次结果） |

---

## 3. 获取日志

```
GET /api/v1/logs?level=<级别>&limit=<数量>
```

需认证。获取系统日志，支持按级别筛选。

**参数**：
| 参数 | 说明 | 默认值 |
|---|---|---|
| level | 过滤级别：info/warn/error | 空（全部） |
| limit | 返回条数 | 100 |

响应：
```json
{
  "logs": [
    {
      "time": "2026-07-12T12:41:42+08:00",
      "level": "INFO",
      "msg": "scan started",
      "attrs": {"root": "/mnt/F/Anime/New"}
    },
    {
      "time": "2026-07-12T12:41:43+08:00",
      "level": "WARN",
      "msg": "duplicate file skipped",
      "attrs": {"path": "..."}
    }
  ],
  "total": 2
}
```

**日志文件位置**：`data/logs/danmaku-YYYY-MM-DD.log`

**配置**：
| 环境变量 | 说明 | 默认值 |
|---|---|---|
| `LOG_MAX_DAYS` | 日志保留天数 | 7 |

---

## 2. 搜索系列

```
GET /api/v1/search?q=<关键词>&airdate=<yyyy-mm>&rating_min=<评分>&tags=<标签1,标签2>
```

需认证。所有参数可选，支持多维度搜索。

**参数**：
| 参数 | 说明 | 示例 |
|---|---|---|
| q | 关键词（搜索 title, name_cn） | 进击的巨人 |
| airdate | 放送日期（精确匹配） | 2026-07 |
| rating_min | 最低评分 | 7.5 |
| tags | 标签（逗号分隔，匹配任一） | 热血,冒险 |

响应：
```json
{
  "series": [
    {
      "ID": "123",
      "BangumiID": 123,
      "Title": "番剧标题",
      "NameCN": "中文名",
      "CoverPath": "covers/bgm_123.jpg",
      "TotalEps": 24,
      "CurrentEp": 24,
      "AirDate": "2026-07",
      "Rating": 8.5,
      "Tags": "[\"热血\",\"冒险\"]",
      "Summary": "简介",
      "LastPlayedAt": "2026-07-12T12:00:00Z"
    }
  ],
  "total": 1
}
```

---

## 3. 获取剧集列表

```
GET /api/v1/episodes?series_id=<id>
```

需认证。`series_id` 可选，按系列筛选（现在是字符串格式的 BangumiID）。

响应：`Episode[]`
```json
[{
  "ID": "1230014",
  "SeriesID": "123",
  "LibraryID": 1,
  "DandanEpisodeID": 12345,
  "RelativePath": "S01/E01.mp4",
  "FileMD5": "abc123",
  "FileHash": "xxhash64",
  "DanmakuPath": "data/danmaku/abc123.json",
  "EpIndex": 1.0,
  "MatchStatus": "matched",
  "ScrapeStatus": "completed",
  "WatchProgress": 0.5
}]
```

---

## 4. 获取弹幕

```
GET /api/v1/episodes/:id/danmaku
```

需认证。`:id` 是字符串格式的 Episode ID（如 `1230014`）。如果弹幕未缓存，会触发懒加载从弹弹play下载。

响应：`DanmakuLine[]`
```json
[{"time": 1.5, "text": "哈哈哈", "color": 16777215, "type": 1}]
```

---

## 5. 匹配弹幕

```
POST /api/v1/episodes/:id/match
```

需认证。`:id` 是字符串格式的 Episode ID。触发指定剧集的弹幕匹配和下载。

响应：
```json
{
  "episode_id": "1230014",
  "dandan_episode_id": 12345,
  "danmaku_path": "data/danmaku/abc123.json"
}
```

---

## 6. 开始播放

```
GET /api/v1/play?episode_id=<id>
```

需认证。`episode_id` 是字符串格式的 Episode ID。获取指定剧集的文件路径，自动加载弹幕（如未加载）。

响应：
```json
{
  "episode_id": "1230014",
  "file_path": "/media/anime/番剧标题/S01E01.mkv",
  "danmaku_loaded": true,
  "danmaku_path": "data/danmaku/abc123.json",
  "series_title": "番剧标题",
  "series_name_cn": "中文名",
  "ep_index": 1.0,
  "watch_progress": 0.5
}
```

---

## 7. 获取播放进度

```
GET /api/v1/progress?episode_id=<id>
```

需认证。`episode_id` 可选（字符串格式）。

响应：`History[]`
```json
[{
  "ID": 1,
  "UserID": 1,
  "EpisodeID": "1230014",
  "Position": 120.5,
  "UpdatedAt": "2024-01-01T00:00:00Z"
}]
```

---

## 8. 更新播放进度

```
POST /api/v1/progress
```

需认证。请求体：
```json
{
  "episode_id": "1230014",
  "position": 120.5,
  "duration": 1440
}
```

| 字段 | 必填 | 说明 |
|---|---|---|
| episode_id | 是 | Episode ID（字符串） |
| position | 是 | 当前播放位置（秒） |
| duration | 否 | 视频总时长（秒），提供时自动计算 WatchProgress |

响应：`{"ok": true}`

---

## 9. 触发扫描

```
POST /api/v1/scan
```

需认证。只触发文件扫描，不触发刮削。

响应：`{"message": "scan triggered"}` (202 Accepted)

---

## 10. 触发刮削

```
POST /api/v1/scrape
```

需认证。只触发元数据刮削，处理所有 `scrape_status = "unscraped"` 的 episodes。

响应：`{"message": "scrape triggered"}` (202 Accepted)

---

## 11. 初始化数据库

```
POST /api/v1/library/init
```

需认证。仅在首次启动、尚无可用的数据库文件时需要调用。初始化成功后数据库路径会被持久化到 `.danmaku-dbpath` 标记文件，下次启动自动复用。

请求体：
```json
{"db_path": "/home/user/danmaku.db"}
```

响应 (201 Created)：
```json
{"db_path": "/home/user/danmaku.db", "message": "database initialized"}
```

### 11.1 查询初始化状态

```
GET /api/v1/library/init/status
```

需认证。返回数据库是否已初始化。

响应：
```json
{"initialized": true, "db_path": "data/danmaku.db", "status": "ready"}
```

未初始化时：
```json
{"initialized": false, "status": "uninitialized"}
```

### 11.2 查询迁移状态

```
GET /api/v1/library/migration/status
```

需认证。当通过 `PUT /api/v1/settings` 修改 `db_path` 后，下次重启会自动搬移数据库文件。此接口返回迁移进度。

响应（空闲时）：
```json
{"status": "idle"}
```

响应（迁移中）：
```json
{"status": "migrating", "from": "/old/path.db", "to": "/new/path.db"}
```

---

## 12. 获取设置

```
GET /api/v1/settings
```

需认证。返回所有设置的键值对。

响应：
```json
{
  "api_keys": {"bangumi_access_token": "...", "tmdb_api_key": "..."}
}
```

---

## 13. 更新设置

```
PUT /api/v1/settings
```

需认证。请求体为键值对：
```json
{
  "api_keys": {"bangumi_access_token": "...", "tmdb_api_key": "..."}
}
```

响应：`{"ok": true}`

---

### 可设置项

| 键名 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `danmaku_enabled` | bool | `false` | 弹幕开关。关闭时播放接口不加载弹幕 |
| `db_path` | string | — | 数据库文件路径，修改后重启自动迁移 |
| `scan_interval_hours` | number | `24` | 自动扫描间隔（小时） |
| `api_keys` | object | `{}` | API 密钥（如 `bangumi_access_token`, `tmdb_api_key`） |

---

## 14. 获取媒体库列表

```
GET /api/v1/library
```

需认证。

响应：`Library[]`
```json
[{"ID": 1, "RootPath": "data"}]
```

---

## 15. 创建媒体库

```
POST /api/v1/library
```

需认证。请求体：
```json
{"root_path": "/path/to/media"}
```

响应 (201 Created)：
```json
{"ID": 2, "RootPath": "/path/to/media"}
```

---

## 16. 获取媒体库文件列表

```
GET /api/v1/library/files?library_id=<id>
```

需认证。`library_id` 必填，返回指定媒体库下所有文件，并按系列和集数排序。

响应：`LibraryFile[]`
```json
[{
  "id": "1230014",
  "series_id": "123",
  "series_title": "测试番剧",
  "relative_path": "S01/E01.mp4",
  "file_md5": "abc123",
  "file_hash": "xxhash64",
  "ep_index": 1.0,
  "match_status": "matched",
  "scrape_status": "completed",
  "watch_progress": 0.75,
  "danmaku_path": "data/danmaku/abc123.json"
}]
```

---

## 17. 静态文件 - 封面

```
GET /covers/<filename>
```

无需认证。提供封面图片文件。

---

## 18. WebSocket

```
GET /ws?client=<type>&ep=<episode_id>&token=<token>
```

需认证（通过 URL 参数 `token` 传递）。升级为 WebSocket 连接。

**查询参数：**

| 参数 | 必填 | 说明 |
|---|---|---|
| `client` | 是 | 客户端类型：`overlay` / `tauri` / `ui` |
| `ep` | 是 | 剧集 ID（字符串格式） |
| `token` | 是 | 认证 token |

**消息类型：**

| 类型 | 方向 | 说明 |
|---|---|---|
| `ping` | Server → Client | 心跳 (30s 间隔) |
| `pong` | Client → Server | 心跳回复 |
| `danmaku` | Server → Client | 弹幕数据 |
| `time_sync` | 双向 | 时间同步 |
| `config_sync` | Server → Client | 配置同步 |

---

## 认证说明

所有 `/api/v1/*` 接口（除 `/api/v1/health`）和 `/ws` 均需认证。认证方式：

- 设置环境变量 `APP_LOCAL_TOKEN`（未设置时跳过认证）
- 请求头：`Authorization: Bearer <token>`
- WebSocket 通过 URL 参数 `token` 传递

---

## 数据模型

### Library
| 字段 | 类型 | 说明 |
|---|---|---|
| ID | uint | 主键 |
| RootPath | string | 媒体文件根目录 |

### Series
| 字段 | 类型 | 说明 |
|---|---|---|
| ID | string | 主键（BangumiID） |
| BangumiID | *uint | Bangumi ID |
| Title | string | 标题 |
| NameCN | *string | 中文名 |
| CoverPath | *string | 封面路径 |
| TotalEps | *uint | 总集数 |
| CurrentEp | *uint | 当前已匹配集数 |
| AirDate | *string | 播出日期（yyyy-mm 格式） |
| Rating | *float64 | 评分（来自 Bangumi） |
| Tags | *string | 标签（JSON 数组） |
| Summary | *string | 简介 |
| LastPlayedAt | time.Time | 最后播放时间 |

### Episode
| 字段 | 类型 | 说明 |
|---|---|---|
| ID | string | 主键（格式：BangumiID + epIndex + 验证位） |
| SeriesID | string | 所属系列（关联 Series.ID） |
| LibraryID | uint | 所属媒体库 |
| DandanEpisodeID | uint | 弹弹play 剧集 ID |
| RelativePath | string | 相对路径 |
| FileMD5 | string | 文件 MD5 |
| FileHash | string | 文件 xxHash |
| DanmakuPath | *string | 弹幕文件路径 |
| EpIndex | *float64 | 集数索引 |
| MatchStatus | string | unmatched / matched（兼容保留） |
| ScrapeStatus | string | unscraped / no_match / completed |
| WatchProgress | float64 | 播放进度：0=未播放, 1=已播放, 0.x=百分比 |
| LastPlayedAt | time.Time | 最后播放时间 |

### Episode ID 格式说明
- 格式：`{BangumiID}_{epIndex(3位)}_{验证位(1位)}`（下划线分隔）
- 示例：`123_001_4`（BangumiID=123, epIndex=001, 验证位=4）
- 验证位算法：`epIndex % 10`

### History
| 字段 | 类型 | 说明 |
|---|---|---|
| ID | uint | 主键 |
| UserID | uint | 用户 ID |
| EpisodeID | string | 剧集 ID（关联 Episode.ID） |
| Position | float64 | 播放位置（秒） |
| UpdatedAt | time.Time | 更新时间 |

### Setting
| 字段 | 类型 | 说明 |
|---|---|---|
| ID | uint | 主键 |
| UserID | uint | 用户 ID |
| Key | string | 设置键名 |
| Value | json.RawMessage | 设置值（任意 JSON） |

---

## WebSocket 消息格式

```json
{"type": "danmaku", "payload": {"lines": [...]}}
```

### DanmakuLine
| 字段 | 类型 | 说明 |
|---|---|---|
| time | float64 | 弹幕出现时间（秒） |
| text | string | 弹幕内容 |
| color | int | 颜色值 |
| type | int | 弹幕类型：0=滚动, 1=底部, 2=顶部 |
