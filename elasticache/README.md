# AWS ElastiCache — Users CRUD API (Cache-Aside 패턴)

Go + Gin 프레임워크로 구현한 AWS ElastiCache 연동 RESTful API 서버입니다.  
RDS MySQL을 주 저장소로, ElastiCache를 캐시 레이어로 사용하는 **Cache-Aside** 패턴을 구현합니다.

ElastiCache 구성 방식별로 4가지 변형이 있습니다.

---

## 목차

1. [변형별 비교](#변형별-비교)
2. [기술 스택](#기술-스택)
3. [프로젝트 구조](#프로젝트-구조)
4. [API 엔드포인트](#api-엔드포인트)
5. [환경 변수](#환경-변수)
6. [Cache-Aside 동작 방식](#cache-aside-동작-방식)
7. [변형별 연결 특이사항](#변형별-연결-특이사항)
8. [실행 방법](#실행-방법)

---

## 변형별 비교

| 폴더 | 엔진 | 모드 | 클라이언트 타입 | TLS |
|---|---|---|---|---|
| `valkey_serverless/` | Valkey (Redis 호환) | Serverless | `redis.ClusterClient` | 항상 활성화 |
| `valkey_node_based/` | Valkey (Redis 호환) | Node-based (단일 노드) | `redis.Client` | 옵션 (`ELASTICACHE_TLS=true`) |
| `valkey_node_based_cluster/` | Valkey (Redis 호환) | Node-based (클러스터 모드) | `redis.ClusterClient` | 옵션 (`ELASTICACHE_TLS=true`) |
| `memcached_serverless/` | Memcached | Serverless | `memcache.Client` | 항상 활성화 (TLS Dialer) |

> **Valkey vs Memcached**: Valkey(Redis 호환)는 JSON 직렬화로 구조체를 저장하고, Memcached는 동일하게 JSON을 사용하지만 컨텍스트(Context) 지원이 없습니다.

---

## 기술 스택

| 항목 | 내용 |
|---|---|
| 언어 | Go 1.22 |
| HTTP 프레임워크 | [Gin](https://github.com/gin-gonic/gin) v1.10 |
| RDS 드라이버 | [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) v1.8 |
| Valkey 클라이언트 | [go-redis/redis](https://github.com/redis/go-redis) v9 |
| Memcached 클라이언트 | [bradfitz/gomemcache](https://github.com/bradfitz/gomemcache) |
| 환경변수 로딩 | [godotenv](https://github.com/joho/godotenv) v1.5 |
| 캐시 전략 | Cache-Aside (Read-Through + Write-Invalidate) |

---

## 프로젝트 구조

```
elasticache/
├── valkey_serverless/
│   ├── main.go
│   ├── go.mod / go.sum
│   └── Dockerfile
├── valkey_node_based/
│   ├── main.go
│   ├── go.mod / go.sum
│   └── Dockerfile
├── valkey_node_based_cluster/
│   ├── main.go
│   ├── go.mod / go.sum
│   └── Dockerfile
└── memcached_serverless/
    ├── main.go
    ├── go.mod / go.sum
    └── Dockerfile
```

---

## API 엔드포인트

4가지 변형 모두 동일한 엔드포인트를 제공합니다.

### GET /health

서버 상태 확인.

```
200 OK
{"status": "ok"}
```

---

### POST /api/v1/users — 사용자 생성

**Request Body**

```json
{
  "name": "Alice",
  "email": "alice@example.com"
}
```

**Response** `201 Created`

```json
{
  "id": 1,
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2026-04-30T10:00:00Z"
}
```

생성 후 캐시에도 즉시 저장합니다.

---

### GET /api/v1/users — 전체 조회

캐시를 거치지 않고 RDS에서 직접 조회합니다.

**Response** `200 OK`

```json
[
  {
    "id": 1,
    "name": "Alice",
    "email": "alice@example.com",
    "created_at": "2026-04-30T10:00:00Z"
  }
]
```

---

### GET /api/v1/users/:id — 단건 조회

**캐시 히트 시** → ElastiCache에서 즉시 반환  
**캐시 미스 시** → RDS 조회 후 캐시에 저장 (TTL: 30초)

서버 로그에서 `cache HIT` / `cache MISS`로 확인할 수 있습니다.

**Response** `200 OK` / `404 Not Found`

---

### PUT /api/v1/users/:id — 사용자 수정

RDS 업데이트 후 캐시를 삭제하고 새 데이터로 다시 저장합니다.

**Response** `200 OK` / `404 Not Found`

---

### DELETE /api/v1/users/:id — 사용자 삭제

RDS에서 삭제 후 캐시도 함께 삭제합니다.

**Response** `204 No Content` / `404 Not Found`

---

## 환경 변수

| 변수명 | 설명 | 기본값 |
|---|---|---|
| `DB_HOST` | RDS MySQL 호스트 | `localhost` |
| `DB_PORT` | RDS MySQL 포트 | `3306` |
| `DB_USER` | DB 사용자명 | `admin` |
| `DB_PASSWORD` | DB 비밀번호 | `password` |
| `DB_NAME` | DB 이름 | `appdb` |
| `ELASTICACHE_HOST` | ElastiCache 엔드포인트 호스트 | `localhost` |
| `ELASTICACHE_TLS` | TLS 활성화 여부 (valkey_node_based 계열만) | `false` |
| `SERVER_PORT` | HTTP 서버 포트 | `8080` |

> `valkey_serverless`, `memcached_serverless`는 TLS가 항상 활성화되어 있어 `ELASTICACHE_TLS` 변수가 없습니다.

---

## Cache-Aside 동작 방식

```
GET /api/v1/users/:id
  │
  ├─[캐시 히트]─→ ElastiCache에서 즉시 반환
  │
  └─[캐시 미스]─→ RDS MySQL 조회
                    │
                    └─→ ElastiCache에 저장 (TTL 30초)
                          │
                          └─→ 클라이언트에 반환

POST / PUT / DELETE
  │
  ├─→ RDS MySQL 쓰기
  └─→ ElastiCache 갱신 또는 삭제
```

캐시 키 형식: `user:{id}`

---

## 변형별 연결 특이사항

### valkey_serverless

Serverless는 클러스터 모드로 동작하므로 `ClusterClient`를 사용하고 TLS가 필수입니다.

```go
rdb = redis.NewClusterClient(&redis.ClusterOptions{
    Addrs:     []string{host + ":6379"},
    TLSConfig: &tls.Config{},
})
```

### valkey_node_based

단일 노드(클러스터 모드 비활성화) 구성입니다. `Client`를 사용하며 TLS는 선택적입니다.

```go
rdb = redis.NewClient(&redis.Options{
    Addr:      host + ":6379",
    TLSConfig: &tls.Config{}, // ELASTICACHE_TLS=true일 때만
})
```

### valkey_node_based_cluster

노드 기반이지만 클러스터 모드가 활성화된 구성입니다. `ClusterClient`를 사용하며 TLS는 선택적입니다.

```go
rdb = redis.NewClusterClient(&redis.ClusterOptions{
    Addrs:     []string{host + ":6379"},
    TLSConfig: &tls.Config{}, // ELASTICACHE_TLS=true일 때만
})
```

### memcached_serverless

`gomemcache`는 TLS를 기본 지원하지 않아 커스텀 `DialContext`로 TLS를 주입합니다.  
또한 Go의 `context.Context`를 지원하지 않아 캐시 헬퍼 함수에 ctx 파라미터가 없습니다.

```go
mc.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
    return (&tls.Dialer{Config: &tls.Config{ServerName: host}}).DialContext(ctx, network, address)
}
```

---

## 실행 방법

### 로컬 실행 (예: valkey_serverless)

```bash
cd elasticache/valkey_serverless

# 1. 의존성 설치
go mod download

# 2. 환경변수 설정
vi .env

# 3. 실행
go run .
```

### Docker 실행

```bash
docker build -t elasticache-app .
docker run --env-file .env -p 8080:8080 elasticache-app
```

### 동작 확인

```bash
# 헬스 체크
curl http://localhost:8080/health

# 사용자 생성
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'

# 단건 조회 — 첫 번째: cache MISS (RDS 조회 후 캐시 저장)
curl http://localhost:8080/api/v1/users/1

# 단건 조회 — 두 번째: cache HIT (ElastiCache에서 즉시 반환)
curl http://localhost:8080/api/v1/users/1

# 수정 (캐시 무효화 → 재저장)
curl -X PUT http://localhost:8080/api/v1/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Bob","email":"bob@example.com"}'

# 삭제 (캐시도 함께 삭제)
curl -X DELETE http://localhost:8080/api/v1/users/1
```
