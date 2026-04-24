# AWS DocumentDB (Instance-Based) — Users CRUD API

Go + Gin 프레임워크로 구현한 AWS DocumentDB instance-based 클러스터 연동 RESTful API 서버입니다.  
`users` 컬렉션에 대한 CRUD 작업을 HTTP 엔드포인트로 노출합니다.

---

## 목차

1. [기술 스택](#기술-스택)
2. [프로젝트 구조](#프로젝트-구조)
3. [아키텍처 및 계층 구조](#아키텍처-및-계층-구조)
4. [구성 요소 상세](#구성-요소-상세)
   - [config](#config)
   - [models](#models)
   - [repository](#repository)
   - [handler](#handler)
   - [main](#main)
5. [요청 흐름](#요청-흐름)
6. [API 엔드포인트](#api-엔드포인트)
7. [환경 변수](#환경-변수)
8. [AWS DocumentDB 연결 특이사항](#aws-documentdb-연결-특이사항)
9. [실행 방법](#실행-방법)
   - [로컬 실행](#로컬-실행)
   - [Docker 실행](#docker-실행)

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
instance_based/
├── main.go                      # 진입점: DB 연결, 서버 기동
├── go.mod / go.sum              # 모듈 정의 및 의존성 잠금
├── .env.example                 # 환경변수 예시 (실제 .env 파일로 복사하여 사용)
├── global-bundle.pem            # AWS DocumentDB TLS CA 번들
├── Dockerfile                   # 멀티스테이지 컨테이너 이미지 정의
├── .dockerignore
│
├── config/
│   └── config.go                # 환경변수 로딩 및 Config 구조체
│
├── models/
│   └── user.go                  # User 도메인 모델, UpdateUserRequest DTO
│
├── repository/
│   └── user_repository.go       # MongoDB 컬렉션 직접 조작 (CRUD)
│
└── handler/
    └── user_handler.go          # HTTP 요청/응답 처리, 라우트 등록
```

---

## 아키텍처 및 계층 구조

이 프로젝트는 관심사 분리를 위해 4개 계층으로 구성됩니다.

```
HTTP 요청
    │
    ▼
┌─────────────────────────────────┐
│           handler               │  HTTP 파싱, 유효성 검사, 응답 직렬화
│        (user_handler.go)        │
└─────────────────┬───────────────┘
                  │ 도메인 객체 전달
                  ▼
┌─────────────────────────────────┐
│          repository             │  MongoDB 드라이버 직접 호출
│      (user_repository.go)       │  bson 직렬화/역직렬화
└─────────────────┬───────────────┘
                  │ mongo-driver
                  ▼
┌─────────────────────────────────┐
│       AWS DocumentDB            │  users 컬렉션
│    (MongoDB 호환 클러스터)       │
└─────────────────────────────────┘
```

`config`는 애플리케이션 시작 시 한 번 로드되어 `main.go`에서 DB 연결에 사용됩니다.  
`models`는 계층 간 데이터 전달에 사용되는 공유 타입입니다.

---

## 구성 요소 상세

### config

**파일**: `config/config.go`

애플리케이션 전체에서 사용하는 설정값을 환경변수에서 읽어 `Config` 구조체로 반환합니다.  
환경변수가 없으면 하드코딩된 기본값이 사용됩니다.

```go
type Config struct {
    MongoURI   string  // DocumentDB 연결 URI
    DBName     string  // 사용할 데이터베이스 이름
    ServerPort string  // HTTP 서버 포트
    TLSCAFile  string  // TLS CA 파일 경로
}
```

| 환경변수 | 기본값 |
|---|---|
| `DOCDB_URI` | `mongodb://localhost:27017` |
| `DOCDB_DB_NAME` | `appdb` |
| `SERVER_PORT` | `8080` |
| `TLS_CA_FILE` | `global-bundle.pem` |

---

### models

**파일**: `models/user.go`

두 가지 타입을 정의합니다.

#### User

DocumentDB `users` 컬렉션의 한 도큐먼트를 표현합니다.  
`bson` 태그는 MongoDB 직렬화, `json` 태그는 HTTP 응답 직렬화에 사용됩니다.  
`binding` 태그는 Gin의 요청 유효성 검사 규칙입니다.

```go
type User struct {
    ID        primitive.ObjectID  // MongoDB ObjectID (_id)
    Name      string              // 필수 (required)
    Email     string              // 필수, 이메일 형식 검증
    CreatedAt time.Time           // 생성 시각 (자동 설정)
    UpdatedAt time.Time           // 수정 시각 (자동 갱신)
}
```

#### UpdateUserRequest

PATCH/PUT 요청 본문을 바인딩하는 DTO입니다.  
`omitempty`로 전달되지 않은 필드는 업데이트에서 제외됩니다.

```go
type UpdateUserRequest struct {
    Name  string  // 선택적
    Email string  // 선택적
}
```

---

### repository

**파일**: `repository/user_repository.go`

MongoDB 드라이버를 통해 `users` 컬렉션을 직접 조작합니다.  
`*mongo.Collection`을 보유하며, 모든 메서드는 `context.Context`를 첫 번째 인자로 받아 타임아웃/취소를 지원합니다.

| 메서드 | 동작 |
|---|---|
| `Create(ctx, *User)` | `InsertOne` — ObjectID·타임스탬프 자동 설정 후 삽입 |
| `FindAll(ctx)` | `Find` + `cursor.All` — 전체 도큐먼트 조회 |
| `FindByID(ctx, id)` | `FindOne` — ObjectID로 단건 조회, 없으면 `mongo.ErrNoDocuments` |
| `Update(ctx, id, *UpdateUserRequest)` | `UpdateOne` ($set) — 비어있지 않은 필드만 업데이트, 이후 `FindByID`로 갱신된 도큐먼트 반환 |
| `Delete(ctx, id)` | `DeleteOne` — 삭제 건수가 0이면 `mongo.ErrNoDocuments` 반환 |

**부분 업데이트 로직** (`Update` 메서드):

```go
update := bson.M{"$set": bson.M{"updated_at": time.Now()}}

if req.Name != ""  { update["$set"].(bson.M)["name"]  = req.Name  }
if req.Email != "" { update["$set"].(bson.M)["email"] = req.Email }
```

요청에 포함된 필드만 `$set`에 추가하여 나머지 필드를 그대로 보존합니다.

---

### handler

**파일**: `handler/user_handler.go`

Gin의 `*gin.RouterGroup`을 받아 라우트를 등록하고, HTTP 요청을 파싱하여 repository를 호출한 뒤 JSON 응답을 반환합니다.

**라우트 등록** (`RegisterRoutes`):

```
POST   /api/v1/users        → Create
GET    /api/v1/users        → FindAll
GET    /api/v1/users/:id    → FindByID
PUT    /api/v1/users/:id    → Update
DELETE /api/v1/users/:id    → Delete
```

**에러 처리 규칙**:

| 에러 종류 | HTTP 상태 코드 |
|---|---|
| JSON 바인딩 실패 / 잘못된 ID 형식 | `400 Bad Request` |
| `mongo.ErrNoDocuments` | `404 Not Found` |
| 그 외 DB 에러 | `500 Internal Server Error` |

---

### main

**파일**: `main.go`

애플리케이션 진입점입니다. 다음 순서로 동작합니다.

1. **`.env` 로드** — `godotenv.Load()`로 `.env` 파일을 환경변수로 등록 (파일 없으면 무시)
2. **설정 로드** — `config.Load()`로 `Config` 구조체 생성
3. **DocumentDB 연결** — `connectDocDB(cfg)` 호출
4. **의존성 조립** — `UserRepository` → `UserHandler` 순서로 생성
5. **라우터 등록** — `/health`, `/api/v1/users` 라우트 등록
6. **서버 기동** — `r.Run(":PORT")`

**TLS 연결 로직** (`connectDocDB`):

```go
// global-bundle.pem이 존재할 때만 TLS 설정 (로컬 개발 환경 호환)
if _, err := os.Stat(cfg.TLSCAFile); err == nil {
    tlsCfg, _ := buildTLSConfig(cfg.TLSCAFile)
    clientOpts.SetTLSConfig(tlsCfg)
}
```

`buildTLSConfig`는 PEM 파일에서 CA 인증서를 읽어 `x509.CertPool`을 구성하고 `tls.Config`에 주입합니다.

---

## 요청 흐름

사용자 생성(`POST /api/v1/users`)을 예시로 전체 흐름을 설명합니다.

```
클라이언트
  │
  │  POST /api/v1/users
  │  Body: {"name":"Alice","email":"alice@example.com"}
  │
  ▼
[Gin Router]
  │ 라우트 매칭 → UserHandler.Create 호출
  ▼
[handler.Create]
  │ c.ShouldBindJSON(&user)  → name, email 추출 + 유효성 검사
  │ 실패 시 400 반환
  ▼
[repository.Create]
  │ user.ID        = primitive.NewObjectID()
  │ user.CreatedAt = now
  │ user.UpdatedAt = now
  │ col.InsertOne(ctx, user)
  ▼
[DocumentDB]
  │ users 컬렉션에 도큐먼트 삽입
  ▼
[handler.Create]
  │ 201 Created + 삽입된 User 객체 JSON 반환
  ▼
클라이언트
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
# AWS 공식 다운로드
curl -O https://truststore.pki.us-east-1.amazonaws.com/global/global-bundle.pem
```

### 연결 흐름

```
Go 앱
  │
  │  mongo.Connect(URI + TLS Config)
  │  ─────────────────────────────────
  │  TLSConfig: x509.CertPool ← global-bundle.pem
  │
  ▼
AWS DocumentDB (Port 27017, TLS)
  │  replicaSet=rs0        ← primary + replica 구성 인식
  │  readPreference=secondaryPreferred ← 읽기는 replica 우선
  │  retryWrites=false     ← 분산 트랜잭션 미지원 우회
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
# .env 파일 내 DOCDB_URI, DOCDB_DB_NAME 등 실제 값으로 수정

# 3. 빌드 및 실행
go run .

# 또는 바이너리 빌드 후 실행
go build -o server .
./server
```

### Docker 실행

```bash
# 1. 이미지 빌드
docker build -t docdb-app .

# 2. 컨테이너 실행 (.env 파일로 환경변수 주입)
docker run --env-file .env -p 8080:8080 docdb-app
```

**Dockerfile 구조** (멀티스테이지 빌드):

| 스테이지 | 베이스 이미지 | 역할 |
|---|---|---|
| builder | `golang:1.22-alpine` | 소스 컴파일 (CGO 비활성화, 정적 바이너리) |
| runtime | `distroless/static-debian12` | 바이너리 + CA 번들만 포함 (~수 MB) |

> `.env` 파일은 이미지에 포함되지 않습니다. 반드시 `--env-file` 또는 `-e` 플래그로 런타임에 주입하세요.

---

### 동작 확인

서버 기동 후 헬스 체크:

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

사용자 생성:

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'
```

전체 조회:

```bash
curl http://localhost:8080/api/v1/users
```

단건 조회:

```bash
curl http://localhost:8080/api/v1/users/{id}
```

수정:

```bash
curl -X PUT http://localhost:8080/api/v1/users/{id} \
  -H "Content-Type: application/json" \
  -d '{"name":"Bob"}'
```

삭제:

```bash
curl -X DELETE http://localhost:8080/api/v1/users/{id}
```
