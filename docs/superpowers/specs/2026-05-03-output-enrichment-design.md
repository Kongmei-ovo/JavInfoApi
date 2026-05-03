# 输出增强 + 搜索优化设计

> 参考项目: https://github.com/javstash/R18dev_SQL

---

## 概述

基于 R18dev_SQL 项目的处理逻辑，对 JavInfoApi 进行以下增强：

1. 搜索增强：番号正则提取 + dvd_code→content_id 兜底
2. 新增实体：导演、男演员、作者
3. 文本处理：反审查还原（decensor）
4. 名称补充：Wikidata 查询补全演员名称
5. 图片处理：按 service_code 拼接完整图片 URL

---

## 一、番号正则提取 + 搜索兜底

### 正则

参考 R18dev_SQL 的 `SUPER_DUPER_JAV_CODE_REGEX`：

```go
var dvdCodeRegex = regexp.MustCompile(`(?i).*?([A-Z]+|[3DSVR]+|[T28]+|[T38]+)-?(\d+[Z]?[E]?)(?:-pt)?(\d{1,2})?.*`)
```

从 title_ja / title_en 中提取番号前缀和数字，组合为 `PREFIX-NUMBER` 格式。

### 搜索流程（searchVideos / batchLookupVideos）

```
用户输入 dvd_code
  ↓
1. 标准化: 大写 + 确保有 "-"
  ↓
2. 查 derived_video.dvd_id (ILIKE)
  ↓ 找到 → 返回
3. 提取前缀+数字，转小写去 "-"
  ↓
4. 查 derived_video.content_id (精确匹配)
  ↓ 找到 → 返回
5. 无结果
```

### 输出补全（getVideo / batchGetVideos 等）

视频返回时，如果 `dvd_id` 为空，从 `title_ja` 或 `title_en` 正则提取番号填充。

**实现位置**: `handler_video.go` 的 `getVideo`、`loadRelatedData`、`loadRelatedDataBatch` 完成后，统一调用 `enrichVideo(&video)` 做后处理。

---

## 二、新增实体：导演、男演员、作者

### 数据库表

| 实体 | 主表 | 关联表 | 字段 |
|------|------|--------|------|
| 导演 | `derived_director` (27,787) | `derived_video_director` (797,470) | id, name_kanji, name_kana, name_romaji |
| 男演员 | `derived_actor` (104,971) | `derived_video_actor` (1,164,547) | id, name_kanji, name_kana |
| 作者 | `derived_author` (8,098) | `derived_video_author` (135,035) | id, name_kanji, name_kana |

### 新增结构体（models.go）

```go
type Director struct {
    ID         int     `json:"id"`
    NameRomaji *string `json:"name_romaji,omitempty"`
    NameKanji  *string `json:"name_kanji,omitempty"`
    NameKana   *string `json:"name_kana,omitempty"`
}

type Actor struct {
    ID        int     `json:"id"`
    NameKanji *string `json:"name_kanji,omitempty"`
    NameKana  *string `json:"name_kana,omitempty"`
}

type Author struct {
    ID        int     `json:"id"`
    NameKanji *string `json:"name_kanji,omitempty"`
    NameKana  *string `json:"name_kana,omitempty"`
}
```

### Video 结构体新增字段

```go
Directors []Director `json:"directors,omitempty"`
Actors    []Actor    `json:"actors,omitempty"`
Authors   []Author   `json:"authors,omitempty"`
```

### 查询逻辑

与现有 actress/categories 加载模式一致，并发 goroutine + sync.WaitGroup：

```sql
-- 导演
SELECT d.id, d.name_romaji, d.name_kanji, d.name_kana
FROM derived_director d
JOIN derived_video_director vd ON d.id = vd.director_id
WHERE vd.content_id = $1

-- 男演员 (按 ordinality 排序)
SELECT a.id, a.name_kanji, a.name_kana
FROM derived_actor a
JOIN derived_video_actor va ON a.id = va.actor_id
WHERE va.content_id = $1
ORDER BY va.ordinality

-- 作者
SELECT a.id, a.name_kanji, a.name_kana
FROM derived_author a
JOIN derived_video_author va ON a.id = va.author_id
WHERE va.content_id = $1
```

**影响范围**:
- `loadRelatedData` — 新增 3 个 goroutine
- `loadRelatedDataBatch` — 新增 3 个批量加载块
- `scanVideo` / `scanVideoRow` — 不变（关联数据单独加载）

### 辅助接口

新增列表接口：
- `GET /api/v1/directors` — 导演列表（q + 分页）
- `GET /api/v1/actors` — 男演员列表（q + 分页）
- `GET /api/v1/authors` — 作者列表（q + 分页）

---

## 三、反审查还原（decensor）

### 原理

参考 R18dev_SQL 的 `decensor()` 函数，用字符串替换还原被 R18 审查打码的英文词汇。

### 实现

1. 将 `decensor.csv` 嵌入 Go 二进制（`embed` 包），启动时加载为 `[][2]string` 映射表
2. 新增 `decensor.go` 文件，提供 `decensor(s string) string` 函数
3. 对所有英文文本字段应用 decensor

### 应用字段

| 字段 | 位置 |
|------|------|
| title_en | Video 输出 |
| comment_en | Video 输出 |
| name_en | Category, Maker, Label, Series 输出 |
| name_romaji | Actress, Director 输出 |

### csv 格式

```
censored_term,replacement_term
A***e,Abuse
R***e,Rape
...
```

共约 101 组映射。文件来源：R18dev_SQL 项目的 `decensor.csv`。

---

## 四、Wikidata 补全演员名称

### 原理

参考 R18dev_SQL 的 `wikidata()` 函数，通过 Wikidata SPARQL 查询补充演员信息。

R18dev_SQL 用此方法补英文名。本项目改为：**补全缺少 romaji 的演员日文名**。

### 查询方式

```sparql
SELECT DISTINCT ?itemLabel WHERE {
  SERVICE wikibase:label { bd:serviceParam wikibase:language "ja,en". }
  {
    SELECT DISTINCT ?item WHERE {
      ?item p:P9781 ?statement0.
      ?statement0 (ps:P9781) "{actress_id}".
    }
    LIMIT 1
  }
}
```

- P9781 = DMM actress ID 属性
- `wikibase:language "ja,en"` 优先获取日文标签，fallback 到英文

### 缓存策略

- 启动时预加载 `derived_actress` 中 `name_romaji IS NULL` 的演员 ID 列表
- 用 `sync.Map` 缓存查询结果，key=actress_id，value=name string
- 设置 TTL（如 24 小时），过期重新查询
- 懒加载：首次查询时触发，不预查全量

### 名称回退链

```
name_romaji (DB)
  ↓ null
Wikidata 日文标签 (ja)
  ↓ null
Wikidata 英文标签 (en)
  ↓ null
name_kanji (DB，兜底)
```

### 实现位置

- 新增 `wikidata.go` 文件
- 在 `loadRelatedData` / `loadRelatedDataBatch` 中，actress 查询完成后，对 `name_romaji == nil` 的演员异步补充

### 限制

- 只处理 actress（女优），不处理 actor（男优）和 author（作者）
- Wikidata 不一定有所有演员的数据，查不到就用 name_kanji 兜底
- HTTP 调用有延迟，用 goroutine 并发 + 超时控制（单次 5s）

---

## 五、图片 URL 拼接

### 原理

参考 R18dev_SQL 的图片处理逻辑。数据库中 `jacket_full_url` 存的是相对路径片段，需要根据 `service_code` 拼接完整 URL。

### 拼接规则

| service_code | URL 模板 | 备注 |
|-------------|---------|------|
| digital | `https://awsimgsrc.dmm.com/dig/{jacket_full_url}.jpg` | |
| mono | `https://awsimgsrc.dmm.com/dig/{去掉 adult/}.jpg` | 去掉路径中的 `adult/` |
| 其他 | `https://pics.dmm.co.jp/{jacket_full_url}.jpg` | |

### 实现

- 新增 `buildImageURL(jacketFullURL *string, serviceCode string) *string` 函数
- 在 `enrichVideo()` 后处理中调用
- Video 结构体新增 `ImageURL *string` 字段（JSON: `image_url,omitempty`）
- 原始 `jacket_full_url` / `jacket_thumb_url` 保留不动

---

## 六、后处理管线（enrichVideo）

所有输出增强统一在 `enrichVideo(video *Video)` 函数中执行：

```go
func enrichVideo(video *Video) {
    // 1. 番号补全: dvd_id 为空时从 title 提取
    if video.DvdID == nil {
        video.DvdID = extractDvdCode(video.TitleJa, video.TitleEn)
    }
    // 2. 图片 URL 拼接
    video.ImageURL = buildImageURL(video.JacketFullURL, video.ServiceCode)
    // 3. Decensor 英文字段
    video.TitleEn = decensorPtr(video.TitleEn)
    video.CommentEn = decensorPtr(video.CommentEn)
    // 4. Decensor 关联实体的英文名
    for i := range video.Categories {
        video.Categories[i].NameEn = decensor(video.Categories[i].NameEn)
    }
    // ... maker, label, series 同理
}
```

**调用点**:
- `getVideo` — 单视频详情
- `batchGetVideos` — 批量查询
- `listVideos` / `searchVideos` — 列表（轻量，只做 dvd_id 补全 + 图片拼接）
- `getActressVideos` — 演员作品列表
- `batchActressVideos` — 批量演员作品

---

## 七、涉及文件变更

| 文件 | 变更内容 |
|------|---------|
| `models.go` | 新增 Director, Actor, Author 结构体；Video 新增字段 |
| `decensor.go` | **新文件** — embed decensor.csv + decensor 函数 |
| `wikidata.go` | **新文件** — Wikidata SPARQL 查询 + 缓存 |
| `enrichment.go` | **新文件** — extractDvdCode, buildImageURL, enrichVideo |
| `handler_video.go` | loadRelatedData/loadRelatedDataBatch 新增 3 类关联；调用 enrichVideo |
| `handler_aux.go` | 新增 listDirectors, listActors, listAuthors |
| `handler_actress.go` | getActressVideos/batchActressVideos 调用 enrichVideo |
| `main.go` | 注册新路由；embed 文件初始化 |
| `data/decensor.csv` | **新文件** — decensor 映射表 |
| `handler_video.go` | searchVideos 增强 dvd_id 搜索兜底逻辑 |
| `handler_batch.go` | batchLookupVideos 增强 dvd_code→content_id 兜底 |

---

## 八、不做的事情

- 不修改数据库（不写入提取的番号、不存翻译）
- 不做 URL 解析输入（用户不需要从 URL 提取 content_id）
- 不做 studio/label 合并（API 已分开返回）
- 不做外部 URL 拼接（r18.dev / dmm 链接，业务层处理）
- 不做 LANG 切换（API 同时返回双语）
