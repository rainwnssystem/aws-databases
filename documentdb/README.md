# AWS DocumentDB — Users CRUD API

Go + Gin 프레임워크로 구현한 AWS DocumentDB instance-based 클러스터 연동 RESTful API 서버입니다.  
`users` 컬렉션에 대한 CRUD 작업을 HTTP 엔드포인트로 노출합니다.

---

## 목차

1. [기술 스택](#기술-스택)
2. [프로젝트 구조](#프로젝트-구조)
3. [API 엔드포인트](#api-엔드포인트)
4. [환경 변수](#환경-변수)
5. [AWS DocumentDB 연결 특이사항](#aws-documentdb-연결-특이사항)
6. [실행 방법](#실행-방법)

---

## 기술 스택

| 항목 | 내용 |
|---|---|
| 언어 | Go 1.22 |
| HTTP 프레임워크 | [Gin](https://github.com/gin-gonic/gin) v1.10 |
| DB 드라이버 | [mongo-driver](https://github.com/mongodb/mongo-go-driver) v1.15 |
| 환경변수 로딩 | [godotenv](https://github.com/joho/godotenv) v1.5 |
| 데이터베이스 | AWS DocumentDB (instance-based, MongoDB 호환) |
| TLS | AWS 글로벌 CA 번들 (`global-bundle.pem`) |

---

## 프로젝트 구조

```
documentdb/
├── main.go            # DB 연결, 핸들러, 서버 기동 (단일 파일)
├── go.mod / go.sum
├── .env.example
├── global-bundle.pem  # AWS DocumentDB TLS CA 번들
├── Dockerfile
└── .dockerignore
```

---

## API 엔드포인트

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
  "id": "6650f1a2c3d4e5f6a7b8c9d0",
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2026-04-24T10:00:00Z",
  "updated_at": "2026-04-24T10:00:00Z"
}
```

---

### GET /api/v1/users — 전체 사용자 조회

**Response** `200 OK`

```json
[
  {
    "id": "6650f1a2c3d4e5f6a7b8c9d0",
    "name": "Alice",
    "email": "alice@example.com",
    "created_at": "2026-04-24T10:00:00Z",
    "updated_at": "2026-04-24T10:00:00Z"
  }
]
```

---

### GET /api/v1/users/:id — 단건 조회

**Response** `200 OK` / `404 Not Found`

---

### PUT /api/v1/users/:id — 사용자 수정

전달된 필드만 업데이트합니다 (부분 업데이트).

**Request Body** (모든 필드 선택적)

```json
{
  "name": "Bob",
  "email": "bob@example.com"
}
```

**Response** `200 OK` — 업데이트된 전체 User 객체 반환  
**Response** `404 Not Found` — 존재하지 않는 id

---

### DELETE /api/v1/users/:id — 사용자 삭제

**Response** `204 No Content` / `404 Not Found`

---

## 환경 변수

`.env.example`을 복사하여 `.env`를 생성하고 값을 채웁니다.

```bash
cp .env.example .env
```

| 변수명 | 설명 | 예시 |
|---|---|---|
| `DOCDB_URI` | DocumentDB 연결 URI (TLS 파라미터 포함) | `mongodb://user:pass@cluster.docdb.amazonaws.com:27017/?tls=true&...` |
| `DOCDB_DB_NAME` | 사용할 데이터베이스 이름 | `appdb` |
| `TLS_CA_FILE` | AWS CA 번들 파일 경로 | `global-bundle.pem` |
| `SERVER_PORT` | HTTP 서버 포트 | `8080` |

---

## AWS DocumentDB 연결 특이사항

DocumentDB는 MongoDB와 호환되지만 몇 가지 필수 설정이 있습니다.

### 필수 URI 파라미터

```
tls=true
tlsCAFile=global-bundle.pem
replicaSet=rs0
readPreference=secondaryPreferred
retryWrites=false          ← DocumentDB는 retryWrites 미지원
```

### TLS CA 번들

AWS DocumentDB는 자체 서명된 CA 인증서를 사용합니다.  
`global-bundle.pem`은 AWS에서 제공하는 글로벌 CA 번들로, 없으면 TLS 핸드셰이크가 실패합니다.

```bash
curl -O https://truststore.pki.us-east-1.amazonaws.com/global/global-bundle.pem
```

### 연결 흐름

```
Go 앱
  │
  │  mongo.Connect(URI + TLS Config)
  │  TLSConfig: x509.CertPool ← global-bundle.pem
  │
  ▼
AWS DocumentDB (Port 27017, TLS)
  │  replicaSet=rs0
  │  readPreference=secondaryPreferred
  │  retryWrites=false
  ▼
users 컬렉션
```

---

## 실행 방법

### 로컬 실행

```bash
# 1. 의존성 설치
go mod download

# 2. 환경변수 설정
cp .env.example .env
# DOCDB_URI, DOCDB_DB_NAME 등 실제 값으로 수정

# 3. 실행
go run .
```

### Docker 실행

```bash
docker build -t docdb-app .
docker run --env-file .env -p 8080:8080 docdb-app
```

**Dockerfile 구조** (멀티스테이지 빌드):

| 스테이지 | 베이스 이미지 | 역할 |
|---|---|---|
| builder | `golang:1.22-alpine` | 소스 컴파일 (CGO 비활성화, 정적 바이너리) |
| runtime | `distroless/static-debian12` | 바이너리 + CA 번들만 포함 |

> `.env` 파일은 이미지에 포함되지 않습니다. 반드시 `--env-file` 또는 `-e` 플래그로 런타임에 주입하세요.

---

### 동작 확인

```bash
# 헬스 체크
curl http://localhost:8080/health

# 사용자 생성
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'

# 전체 조회
curl http://localhost:8080/api/v1/users

# 단건 조회
curl http://localhost:8080/api/v1/users/{id}

# 수정
curl -X PUT http://localhost:8080/api/v1/users/{id} \
  -H "Content-Type: application/json" \
  -d '{"name":"Bob"}'

# 삭제
curl -X DELETE http://localhost:8080/api/v1/users/{id}
```
