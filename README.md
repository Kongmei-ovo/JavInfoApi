# JavInfoApi

高性能JAV元数据API，基于Go + PostgreSQL

## 快速开始

```bash
# 复制配置
cp .env.example .env
# 编辑.env填入数据库信息

# 下载依赖
go mod tidy

# 运行
go run main.go
```

## API接口

### 视频搜索 `GET /api/v1/videos/search`

| 参数 | 类型 | 说明 |
|------|------|------|
| q | string | 关键词搜索(标题) |
| content_id | string | 内容ID精确匹配 |
| dvd_id | string | DVD ID匹配(自动处理-和0) |
| maker_id | int | 厂商ID |
| series_id | int | 系列ID |
| actress_id | int | 演员ID |
| category_id | int | 分类ID |
| page | int | 页码(默认1) |
| page_size | int | 每页数量(默认20,最大100) |

**示例:**
```bash
# 搜索关键词
curl "http://localhost:8080/api/v1/videos/search?q= Tokyo"

# 精确查找
curl "http://localhost:8080/api/v1/videos/search?content_id=ABP-001"

# 按dvd_id搜索(自动处理横杠)
curl "http://localhost:8080/api/v1/videos/search?dvd_id=ABP001"

# 按演员搜索
curl "http://localhost:8080/api/v1/videos/search?actress_id=12345"
```

### 视频列表 `GET /api/v1/videos`
```bash
curl "http://localhost:8080/api/v1/videos?page=1&page_size=20"
```

### 视频详情 `GET /api/v1/videos/:content_id`
```bash
curl "http://localhost:8080/api/v1/videos/EBWH-001?service_code=FANZA"
```

### 演员列表 `GET /api/v1/actresses`
```bash
curl "http://localhost:8080/api/v1/actresses?page=1&page_size=50"
```

### 演员详情 `GET /api/v1/actresses/:id`
```bash
curl "http://localhost:8080/api/v1/actresses/12345"
```

### 演员作品 `GET /api/v1/actresses/:id/videos`
```bash
curl "http://localhost:8080/api/v1/actresses/12345/videos"
```

### 辅助数据
```bash
GET /api/v1/makers      # 厂商列表
GET /api/v1/labels      # 标签列表
GET /api/v1/series      # 系列列表
GET /api/v1/categories  # 分类列表
```

### 统计 `GET /api/v1/stats`
```bash
curl "http://localhost:8080/api/v1/stats"
```

### 健康检查 `GET /health`
```bash
curl "http://localhost:8080/health"
```

## 配置

通过环境变量或.env文件配置:

| 变量 | 默认值 | 说明 |
|------|--------|------|
| DB_HOST | localhost | 数据库地址 |
| DB_PORT | 5432 | 数据库端口 |
| DB_USER | kongmei | 数据库用户 |
| DB_PASSWORD | (空) | 数据库密码 |
| DB_NAME | r18 | 数据库名 |
| DB_MAX_CONN | 20 | 最大连接数 |
| DB_MIN_CONN | 5 | 最小连接数 |
| SERVER_HOST | 0.0.0.0 | 服务地址 |
| SERVER_PORT | 8080 | 服务端口 |

## 免责声明

本项目仅供技术学习与个人研究使用。

- 本项目**不提供**影片数据库，不存储、传播任何受版权保护的内容
- 数据来源为 r18.dev 公开数据库，本项目仅作为 API 查询工具
- 使用本项目所产生的一切行为，均由使用者自行承担全部责任
- 请勿将本项目用于任何商业盈利目的
