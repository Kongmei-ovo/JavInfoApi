# JavInfoApi

A self-hosted metadata aggregation API with multi-source data support, optimized for high-performance queries and scalable deployment.

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-336791?logo=postgresql)

## Features

- **Multi-Source Metadata** — Aggregates content metadata from public DMM/FANZA-compatible sources
- **Flexible Search** — Query by ID, title, performer, maker, series, or category with full-text support
- **High Performance** — Built with Go for concurrent request handling
- **Rich Relationships** — Supports performers, makers, labels, series, and category associations
- **RESTful API** — Clean API design with OpenAPI specification

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 15+

### Installation

```bash
# Clone repository
git clone https://github.com/Kongmei-ovo/JavInfoApi.git
cd JavInfoApi

# Copy configuration
cp .env.example .env
# Edit .env with your database credentials

# Build
go build -o JavInfoApi .

# Run
./JavInfoApi
```

The API will be available at `http://localhost:8080`.

## API Endpoints

### Content Search

```bash
GET /api/v1/videos/search?q=keyword          # Full-text search
GET /api/v1/videos/search?dvd_id=ABC-123    # By DVD identifier
GET /api/v1/videos/search?maker_id=1001     # By maker
GET /api/v1/videos/search?actress_name=Yui # By performer name
```

### Content Details

```bash
GET /api/v1/videos/{content_id}            # Full metadata
GET /api/v1/videos?page=1&page_size=20    # Paginated list
```

### Reference Data

```bash
GET /api/v1/actresses        # Performer directory
GET /api/v1/makers            # Maker directory
GET /api/v1/labels            # Label directory
GET /api/v1/series            # Series directory
GET /api/v1/categories        # Category directory
GET /api/v1/categories/stats  # Category statistics
```

### System

```bash
GET /api/v1/stats             # Database statistics
GET /health                   # Health check
```

## Configuration

Configure via environment variables or `.env` file:

| Variable | Default | Description |
|----------|---------|-------------|
| DB_HOST | localhost | Database host |
| DB_PORT | 5432 | Database port |
| DB_USER | kongmei | Database user |
| DB_PASSWORD | (empty) | Database password |
| DB_NAME | r18 | Database name |
| DB_MAX_CONN | 20 | Max connections |
| DB_MIN_CONN | 5 | Min connections |
| SERVER_HOST | 0.0.0.0 | Listen address |
| SERVER_PORT | 8080 | Listen port |

## Project Structure

```
JavInfoApi/
├── main.go          # Application entry point
├── API.md           # API documentation
├── openapi.json     # OpenAPI specification
└── .env.example     # Configuration template
```

## License

MIT
