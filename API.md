# JavInfoApi 接口文档

> JAV 元数据查询 API，基于 r18.dev 数据库（PostgreSQL）

---

## 目录

- [基础信息](#基础信息)
- [数据库结构](#数据库结构)
- [通用说明](#通用说明)
- [接口列表](#接口列表)
  - [视频相关](#视频相关)
  - [演员相关](#演员相关)
  - [辅助数据](#辅助数据)
  - [系统](#系统)
- [调用流程](#调用流程)
- [响应示例](#响应示例)

---

## 基础信息

| 项目 | 值 |
|------|-----|
| 基础URL | `http://localhost:8080` |
| 数据格式 | JSON |
| 请求方法 | GET |
| 字符编码 | UTF-8 |

### 数据库规模

| 表名 | 记录数 |
|------|--------|
| derived_video | 1,856,310 |
| derived_actress | 101,274 |
| derived_maker | 6,831 |
| derived_label | 13,032 |
| derived_series | 93,536 |
| derived_category | 984 |

---

## 数据库结构

### derived_video (视频主表)

| 字段名 | 类型 | 说明 | 示例 |
|--------|------|------|------|
| content_id | varchar | 内容唯一标识 | `100tv00031` |
| dvd_id | varchar | DVD编号（可含横杠） | `100TV-031` |
| title_en | varchar | 英文标题 | `This Is What Happened...` |
| title_ja | varchar | 日文标题 | `女子○生を靴下一丁に...` |
| comment_en | varchar | 英文描述 | |
| comment_ja | varchar | 日文描述 | |
| runtime_mins | integer | 时长（分钟） | `13` |
| release_date | date | 发售日期 | `2019-07-21` |
| sample_url | varchar | 预览视频URL | |
| maker_id | integer | 厂商ID | `6733` |
| label_id | integer | 品牌ID | `46007` |
| series_id | integer | 系列ID | `222184` |
| jacket_full_url | varchar | 封面大图URL | `digital/video/.../100tv00031pl` |
| jacket_thumb_url | varchar | 封面缩略图URL | `digital/video/.../100tv00031ps` |
| gallery_thumb_first | varchar | 画廊第一张缩略图 | |
| gallery_thumb_last | varchar | 画廊最后一张缩略图 | |
| site_id | integer | 站点ID | `1`=DMM.com, `2`=FANZA |
| service_code | varchar | 服务代码 | `digital`, `mono`, `rental` |

### derived_actress (演员表)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | integer | 演员唯一ID |
| name_romaji | varchar | 罗马音名 | `Yui Hatano` |
| name_kanji | varchar | 汉字名 | `臀原有紗` |
| name_kana | varchar | 假名 | `はたの ゆい` |
| image_url | varchar | 头像图片URL | |

### derived_maker (厂商表)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | integer | 厂商唯一ID |
| name_en | varchar | 英文名 | `SOD` |
| name_ja | varchar | 日文名 | `SOD創作` |

### derived_label (品牌表)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | integer | 品牌唯一ID |
| name_en | varchar | 英文名 | `SOD Fresh Face` |
| name_ja | varchar | 日文名 | `SOD新人AVデビュー` |

### derived_series (系列表现)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | integer | 系列唯一ID |
| name_en | varchar | 英文名 | `Married Women Fucked On The Casting Couch` |
| name_ja | varchar | 日文名 | `AV面接にavingsきた○○人妻` |

### derived_category (题材分类表)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| id | integer | 分类唯一ID |
| name_en | varchar | 英文名 | `Amateur` |
| name_ja | varchar | 日文名 | `素人` |

### derived_video_actress (视频-演员关联表)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| content_id | varchar | 视频ID |
| actress_id | integer | 演员ID |
| ordinality | integer | 演员排序顺序 |

### derived_video_category (视频-分类关联表)

| 字段名 | 类型 | 说明 |
|--------|------|------|
| content_id | varchar | 视频ID |
| category_id | integer | 分类ID |

---

## 通用说明

### 分页参数

所有列表接口支持分页：

| 参数 | 类型 | 默认值 | 最大值 | 说明 |
|------|------|--------|--------|------|
| page | int | 1 | - | 页码，从1开始 |
| page_size | int | 20 | 100 | 每页数量 |

### 通用响应格式

```json
{
  "data": [...],
  "page": 1,
  "page_size": 20,
  "total_count": 12345,
  "total_pages": 618
}
```

### 错误响应

```json
{
  "error": "video not found"
}
```

HTTP状态码：
- `200` - 成功
- `400` - 请求参数错误
- `404` - 资源不存在
- `500` - 服务器内部错误

---

## 接口列表

### 视频相关

---

#### 1. 搜索视频

**接口**: `GET /api/v1/videos/search`

**描述**: 多条件组合搜索视频

**参数**:

| 参数 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| q | string | 否 | 关键词搜索（匹配标题、描述） | `q=tokyo` |
| content_id | string | 否 | 内容ID精确匹配 | `content_id=100tv00031` |
| dvd_id | string | 否 | DVD编号（自动处理横杠） | `dvd_id=100TV-031` 或 `dvd_id=100TV031` |
| maker_id | int | 否 | 厂商ID | `maker_id=1001` |
| maker_name | string | 否 | 厂商名称（模糊匹配） | `maker_name=SOD` |
| series_id | int | 否 | 系列ID | `series_id=211325` |
| series_name | string | 否 | 系列名称（模糊匹配） | `series_name=Tokyo` |
| actress_id | int | 否 | 演员ID | `actress_id=12345` |
| actress_name | string | 否 | 演员名称（模糊匹配） | `actress_name=Yui` |
| category_id | int | 否 | 分类ID | `category_id=4024` |
| category_name | string | 否 | 分类名称（模糊匹配） | `category_name=Amateur` |
| page | int | 否 | 页码 | `page=1` |
| page_size | int | 否 | 每页数量 | `page_size=20` |

**说明**:
- `dvd_id` 搜索时会自动去除横杠和转换为小写，支持 `ABC-123`、`ABC123`、`abc123` 等格式
- `q` 参数使用 ILIKE 模糊匹配，会匹配 title_en、title_ja、comment_en
- `*_name` 参数使用 ILIKE 模糊匹配，支持名称模糊搜索
- `*_id` 和 `*_name` 同时存在时，优先使用 `*_id`
- 多个条件同时使用时为 AND 关系
- 结果按 release_date 降序排序

**示例**:
```bash
# 关键词搜索
curl "http://localhost:8080/api/v1/videos/search?q=tokyo"

# 精确查找（推荐，已建索引）
curl "http://localhost:8080/api/v1/videos/search?dvd_id=ABP-001"

# 按演员ID搜索
curl "http://localhost:8080/api/v1/videos/search?actress_id=12345"

# 按厂商名称搜索
curl "http://localhost:8080/api/v1/videos/search?maker_name=SOD"

# 按演员名称搜索
curl "http://localhost:8080/api/v1/videos/search?actress_name=Yui%20Hatano"

# 按分类名称搜索
curl "http://localhost:8080/api/v1/videos/search?category_name=Amateur"

# 组合搜索
curl "http://localhost:8080/api/v1/videos/search?maker_name=SOD&category_name=Amateur&page=1"
```

---

#### 2. 视频列表

**接口**: `GET /api/v1/videos`

**描述**: 获取视频分页列表

**参数**:

| 参数 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| page | int | 否 | 页码 | `page=2` |
| page_size | int | 否 | 每页数量 | `page_size=50` |

**说明**:
- 结果按 release_date 降序排序
- 返回数据不包含关联数据（maker、actresses等）

**示例**:
```bash
curl "http://localhost:8080/api/v1/videos?page=1&page_size=20"
```

---

#### 3. 视频详情

**接口**: `GET /api/v1/videos/:content_id`

**描述**: 获取单个视频的完整信息

**参数**:

| 参数 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| content_id | string | 是 | 视频内容ID（路径参数） | |
| service_code | string | 否 | 服务代码筛选 | `service_code=FANZA` |

**说明**:
- 返回完整视频信息，包括关联的演员、厂商、品牌、系列、题材分类
- 演员信息通过 `derived_video_actress` 关联表获取，按 ordinality 排序
- 题材分类通过 `derived_video_category` 关联表获取，按 name_en 排序

**示例**:
```bash
curl "http://localhost:8080/api/v1/videos/100tv00031"

# 指定服务代码（用于区分同一content_id在不同站点的版本）
curl "http://localhost:8080/api/v1/videos/100tv00031?service_code=digital"
```

---

### 演员相关

---

#### 4. 演员列表

**接口**: `GET /api/v1/actresses`

**描述**: 获取演员分页列表

**参数**:

| 参数 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| page | int | 否 | 页码 | `page=1` |
| page_size | int | 否 | 每页数量 | `page_size=50` |

**说明**:
- 结果按 name_romaji 升序排序

**示例**:
```bash
curl "http://localhost:8080/api/v1/actresses?page=1&page_size=50"
```

---

#### 5. 演员详情

**接口**: `GET /api/v1/actresses/:id`

**描述**: 获取单个演员的信息

**参数**:

| 参数 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| id | int | 是 | 演员ID（路径参数） | |

**示例**:
```bash
curl "http://localhost:8080/api/v1/actresses/12345"
```

---

#### 6. 演员作品列表

**接口**: `GET /api/v1/actresses/:id/videos`

**描述**: 获取某演员出演的所有视频

**参数**:

| 参数 | 类型 | 必填 | 说明 | 示例 |
|------|------|------|------|------|
| id | int | 是 | 演员ID（路径参数） | |
| page | int | 否 | 页码 | `page=1` |
| page_size | int | 否 | 每页数量 | `page_size=20` |

**说明**:
- 结果按 release_date 降序排序

**示例**:
```bash
curl "http://localhost:8080/api/v1/actresses/12345/videos"
```

---

### 辅助数据

---

#### 7. 厂商列表

**接口**: `GET /api/v1/makers`

**描述**: 获取所有制作厂商

**参数**: 无分页参数（数据量小，6,831条）

**说明**:
- 结果按 name_en 升序排序

**示例**:
```bash
curl "http://localhost:8080/api/v1/makers"
```

**用途**: 下拉选择、搜索时展示厂商名称

---

#### 8. 品牌列表

**接口**: `GET /api/v1/labels`

**描述**: 获取所有品牌系列

**参数**: 无分页参数（数据量小，13,032条）

**说明**:
- 结果按 name_en 升序排序

**示例**:
```bash
curl "http://localhost:8080/api/v1/labels"
```

---

#### 9. 系列列表

**接口**: `GET /api/v1/series`

**描述**: 获取所有影片系列

**参数**: 无分页参数（数据量小，93,536条）

**说明**:
- 结果按 name_en 升序排序

**示例**:
```bash
curl "http://localhost:8080/api/v1/series"
```

---

#### 10. 题材分类列表

**接口**: `GET /api/v1/categories`

**描述**: 获取所有题材分类

**参数**: 无分页参数（数据量小，984条）

**说明**:
- 结果按 name_en 升序排序

**示例**:
```bash
curl "http://localhost:8080/api/v1/categories"
```

---

### 系统

---

#### 11. 统计数据

**接口**: `GET /api/v1/stats`

**描述**: 获取数据库统计信息

**参数**: 无

**示例**:
```bash
curl "http://localhost:8080/api/v1/stats"
```

**响应**:
```json
{
  "videos": 1856310,
  "actresses": 101274,
  "makers": 6831,
  "series": 93536,
  "labels": 13032
}
```

---

#### 12. 健康检查

**接口**: `GET /health`

**描述**: 检查服务状态和数据库连接

**参数**: 无

**示例**:
```bash
curl "http://localhost:8080/health"
```

**响应**:
```json
{
  "status": "healthy"
}
```

---

## 调用流程

### 标准搜索流程

```
1. 调用 /api/v1/stats 确认服务正常
   ↓
2. 调用 /api/v1/makers 或 /api/v1/categories 获取可选的筛选条件
   ↓
3. 调用 /api/v1/videos/search 进行搜索
   ↓
4. 根据返回的 content_id 调用 /api/v1/videos/:content_id 获取详情
```

### 场景1: 已知 DVD ID，查找视频

```bash
# Step 1: 使用 dvd_id 搜索（推荐，已建索引）
curl "http://localhost:8080/api/v1/videos/search?dvd_id=ABC-123"

# Step 2: 获取完整信息
curl "http://localhost:8080/api/v1/videos/{返回的content_id}"
```

### 场景2: 关键词搜索

```bash
# Step 1: 关键词搜索
curl "http://localhost:8080/api/v1/videos/search?q=Tokyo%20Hot&page=1&page_size=20"

# Step 2: 获取每个视频的详细信息
for content_id in $(curl -s "http://localhost:8080/api/v1/videos/search?q=Tokyo%20Hot" | jq -r '.data[].content_id'); do
  curl "http://localhost:8080/api/v1/videos/$content_id"
done
```

### 场景3: 按演员筛选

```bash
# Step 1: 找到演员ID（可通过演员列表页搜索）
curl "http://localhost:8080/api/v1/actresses?page=1&page_size=50" | jq '.data[] | select(.name_romaji | contains("Yui"))'

# Step 2: 获取该演员的所有作品
curl "http://localhost:8080/api/v1/actresses/{演员ID}/videos"
```

### 场景4: 批量下载元数据

```bash
# 获取所有厂商信息（用于后续匹配maker_id）
curl "http://localhost:8080/api/v1/makers" > makers.json

# 获取所有题材分类
curl "http://localhost:8080/api/v1/categories" > categories.json

# 搜索特定厂商的所有视频
curl "http://localhost:8080/api/v1/videos/search?maker_id=1001&page_size=100" > maker_videos.json
```

### 场景5: 下载工具集成

```
你的下载工具需要：
1. 调用 /api/v1/videos/search?dvd_id={用户输入} 获取 content_id
2. 调用 /api/v1/videos/{content_id} 获取：
   - jacket_thumb_url (封面)
   - sample_url (预览)
   - title_en / title_ja (标题)
   - release_date (日期)
   - actresses (演员列表)
3. 使用 content_id 和 service_code 组合去磁力站点/JAVBus/JAVLibrary 搜索下载链接
```

---

## 响应示例

### 搜索视频响应

```json
{
  "data": [
    {
      "content_id": "100tv00031",
      "dvd_id": "100TV-031",
      "title_en": "This Is What Happened When I Stripped A Sch**lgirl Down To One Pair Of Socks And Inserted My Cock Into Her... 2 Mio Chihana",
      "runtime_mins": 13,
      "release_date": "2019-07-21",
      "jacket_thumb_url": "digital/video/100tv00031/100tv00031ps",
      "site_id": 2,
      "service_code": "digital"
    }
  ],
  "page": 1,
  "page_size": 20,
  "total_count": 1,
  "total_pages": 1
}
```

### 视频详情响应

```json
{
  "content_id": "100tv00031",
  "dvd_id": "100TV-031",
  "title_en": "This Is What Happened When I Stripped A Sch**lgirl Down To One Pair Of Socks And Inserted My Cock Into Her... 2 Mio Chihana",
  "title_ja": "女子○生を靴下一丁にひん剥いて挿入してみたら…2 ちーちゃん",
  "comment_en": "...",
  "comment_ja": "...",
  "runtime_mins": 13,
  "release_date": "2019-07-21",
  "sample_url": "...",
  "maker_id": 6733,
  "label_id": 46007,
  "series_id": 222184,
  "jacket_full_url": "digital/video/100tv00031/100tv00031pl",
  "jacket_thumb_url": "digital/video/100tv00031/100tv00031ps",
  "gallery_thumb_first": "digital/video/100tv00031/100tv00031-1",
  "gallery_thumb_last": "digital/video/100tv00031/100tv00031-5",
  "site_id": 2,
  "service_code": "digital",
  "maker": {
    "id": 6733,
    "name_en": "Hyakkin TV",
    "name_ja": "HyakkinTV"
  },
  "label": {
    "id": 46007,
    "name_en": "Hyakkin TV",
    "name_ja": "HyakkinTV"
  },
  "series": {
    "id": 222184,
    "name_en": "I Stripped A Y********l Down To Her Socks And Fucked Her...",
    "name_ja": "女子○生を靴下一丁にひん剥いて挿入してみたら…"
  },
  "actresses": [
    {
      "id": 12345,
      "name_romaji": "Mio Chihana",
      "name_kanji": "千華ねむ",
      "name_kana": "ちーはな みお",
      "image_url": "..."
    }
  ],
  "categories": [
    {
      "id": 4024,
      "name_en": "Amateur",
      "name_ja": "素人"
    },
    {
      "id": 4025,
      "name_en": "Featured Actress",
      "name_ja": "単体作品"
    }
  ]
}
```

### 演员详情响应

```json
{
  "id": 12345,
  "name_romaji": "Yui Hatano",
  "name_kanji": "臀原有紗",
  "name_kana": "はたの ゆい",
  "image_url": "..."
}
```

### 厂商列表响应（部分）

```json
[
  {
    "id": 1001,
    "name_en": "(TQT)",
    "name_ja": "（TQT）"
  },
  {
    "id": 5439,
    "name_en": "*Shusei",
    "name_ja": "○修正"
  }
]
```

---

## 字段说明

### 布尔字段处理

接口中所有可能为 NULL 的字段均使用指针类型或省略：
- 字符串字段：如果为 NULL，JSON 中会省略该字段
- 数字字段：如果为 NULL，JSON 中会省略该字段

### 图片URL处理

返回的 `jacket_full_url`、`jacket_thumb_url`、`gallery_thumb_first`、`gallery_thumb_last`、`image_url`、`sample_url` 等字段是相对路径，需要配合实际站点前缀使用。

### 敏感内容过滤

部分 title_en 和 comment_en 字段可能包含过滤字符（如 `Sch**l`、`F**k`），这是原始数据的特点，应用层如需展示可自行处理。

---

## 错误码

| HTTP状态码 | error 消息 | 说明 |
|-----------|-----------|------|
| 200 | - | 成功 |
| 400 | bad request | 参数错误 |
| 404 | video not found | 视频不存在 |
| 404 | actress not found | 演员不存在 |
| 500 | database error | 数据库错误 |

---

## 配置说明

通过环境变量或 `.env` 文件配置：

| 变量名 | 默认值 | 说明 |
|--------|--------|------|
| DB_HOST | localhost | 数据库地址 |
| DB_PORT | 5432 | 数据库端口 |
| DB_USER | kongmei | 数据库用户名 |
| DB_PASSWORD | (空) | 数据库密码 |
| DB_NAME | r18 | 数据库名 |
| DB_MAX_CONN | 20 | 最大连接数 |
| DB_MIN_CONN | 5 | 最小连接数 |
| SERVER_HOST | 0.0.0.0 | 服务监听地址 |
| SERVER_PORT | 8080 | 服务监听端口 |

---
