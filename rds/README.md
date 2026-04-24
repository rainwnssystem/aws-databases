# AWS RDS (MySQL) — Users CRUD API

Go + Gin 프레임워크로 구현한 AWS RDS MySQL 연동 RESTful API 서버입니다.  
`users` 테이블에 대한 CRUD 작업을 HTTP 엔드포인트로 노출합니다.

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
8. [AWS RDS 연결 특이사항](#aws-rds-연결-특이사항)
9. [실행 방법](#실행-방법)
   - [사전 준비 (테이블 생성)](#사전-준비-테이블-생성)
   - [로컬 실행](#로컬-실행)
   - [Docker 실행](#docker-실행)

---

## 기술 스택

| 항목 | 내용 |
|---|---|
| 언어 | Go 1.22 |
| HTTP 프레임워크 | [Gin](https://github.com/gin-gonic/gin) v1.10 |
| DB 드라이버 | [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql) v1.8 |
| 환경변수 로딩 | [godotenv](https://github.com/joho/godotenv) v1.5 |
| 데이터베이스 | AWS RDS MySQL |

---

## 프로젝트 구조

```
demo-app/
├── main.go                      # 진입점: DB 연결, 서버 기동
├── go.mod / go.sum              # 모듈 정의 및 의존성 잠금
├── .env.example                 # 환경변수 예시 (실제 .env 파일로 복사하여 사용)
├── Dockerfile                   # 멀티스테이지 컨테이너 이미지 정의
├── .dockerignore
│
├── config/
│   └── config.go                # 환경변수 로딩 및 DSN 조합
│
├── models/
│   └── user.go                  # User 도메인 모델, Create/UpdateUserRequest DTO
│
├── repository/
│   └── user_repository.go       # database/sql 기반 CRUD (prepared statement)
│
├── handler/
│   └── user_handler.go          # HTTP 요청/응답 처리, 라우트 등록
│
└── sql/
    ├── schema.sql               # users 테이블 DDL
    └── seed.sql                 # Alice, Bob 초기 데이터
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
│          repository             │  database/sql 직접 호출
│      (user_repository.go)       │  prepared statement (? placeholder)
└─────────────────┬───────────────┘
                  │ go-sql-driver/mysql
                  ▼
┌─────────────────────────────────┐
│         AWS RDS MySQL           │  users 테이블
└─────────────────────────────────┘
```

`config`는 애플리케이션 시작 시 한 번 로드되어 `main.go`에서 DB 연결에 사용됩니다.  
`models`는 계층 간 데이터 전달에 사용되는 공유 타입입니다.

---

## 구성 요소 상세

### config

**파일**: `config/config.go`

환경변수에서 DB 접속 정보를 읽어 MySQL DSN 문자열로 조합하고 `Config` 구조체로 반환합니다.

```go
type Config struct {
    DSN        string  // MySQL DSN (host:port/dbname?options)
    ServerPort string  // HTTP 서버 포트
}
```

DSN 형식:

```
{DB_USER}:{DB_PASSWORD}@tcp({DB_HOST}:{DB_PORT})/{DB_NAME}?parseTime=true&charset=utf8mb4
```

| 환경변수 | 기본값 |
|---|---|
| `DB_HOST` | `localhost` |
| `DB_PORT` | `3306` |
| `DB_USER` | `admin` |
| `DB_PASSWORD` | `password` |
| `DB_NAME` | `appdb` |
| `SERVER_PORT` | `8080` |

---

### models

**파일**: `models/user.go`

세 가지 타입을 정의합니다.

#### User

`users` 테이블의 한 행을 표현합니다.  
`json` 태그는 HTTP 응답 직렬화에 사용됩니다.

```go
type User struct {
    ID        int       // AUTO_INCREMENT PK
    Name      string    // 필수
    Email     string    // 필수, UNIQUE
    CreatedAt time.Time // DEFAULT CURRENT_TIMESTAMP
    UpdatedAt time.Time // ON UPDATE CURRENT_TIMESTAMP
}
```

#### CreateUserRequest / UpdateUserRequest

요청 본문을 바인딩하는 DTO입니다.  
`binding:"required,email"` 태그로 Gin이 유효성 검사를 수행합니다.

```go
type CreateUserRequest struct {
    Name  string  // 필수
    Email string  // 필수, 이메일 형식
}

type UpdateUserRequest struct {
    Name  string  // 필수
    Email string  // 필수, 이메일 형식
}
```

---

### repository

**파일**: `repository/user_repository.go`

`database/sql`을 통해 `users` 테이블을 직접 조작합니다.  
모든 쿼리는 `?` placeholder를 사용한 prepared statement로 SQL 인젝션을 방지합니다.

| 메서드 | 동작 |
|---|---|
| `FindAll()` | `SELECT` 전체 행 조회, `ORDER BY id` |
| `FindByID(id)` | `SELECT` 단건 조회, 없으면 `ErrNotFound` |
| `Create(name, email)` | `INSERT` 후 `LastInsertId()`로 생성된 행 반환 |
| `Update(id, name, email)` | `UPDATE` 후 `RowsAffected()`가 0이면 `ErrNotFound` |
| `Delete(id)` | `DELETE` 후 `RowsAffected()`가 0이면 `ErrNotFound` |

`ErrNotFound`는 패키지 내 sentinel error로 정의되어 handler에서 404 응답에 사용됩니다.

---

### handler

**파일**: `handler/user_handler.go`

Gin의 `*gin.Engine`을 받아 `/users` 라우트 그룹을 등록하고, HTTP 요청을 파싱하여 repository를 호출한 뒤 JSON 응답을 반환합니다.

**라우트 등록** (`RegisterRoutes`):

```
GET    /users        → listUsers
GET    /users/:id    → getUser
POST   /users        → createUser
PUT    /users/:id    → updateUser
DELETE /users/:id    → deleteUser
```

**에러 처리 규칙**:

| 에러 종류 | HTTP 상태 코드 |
|---|---|
| JSON 바인딩 실패 / 잘못된 ID 형식 | `400 Bad Request` |
| `ErrNotFound` | `404 Not Found` |
| 그 외 DB 에러 | `500 Internal Server Error` |

---

### main

**파일**: `main.go`

애플리케이션 진입점입니다. 다음 순서로 동작합니다.

1. **`.env` 로드** — `godotenv.Load()`로 `.env` 파일을 환경변수로 등록 (파일 없으면 무시)
2. **설정 로드** — `config.Load()`로 DSN 조합 및 `Config` 구조체 생성
3. **RDS 연결** — `sql.Open("mysql", cfg.DSN)` + `db.Ping()`으로 연결 확인
4. **의존성 조립** — `UserRepository` → `UserHandler` 순서로 생성
5. **라우터 등록** — `/users` 라우트 등록
6. **서버 기동** — `r.Run(":PORT")`

---

## 요청 흐름

사용자 생성(`POST /users`)을 예시로 전체 흐름을 설명합니다.

```
클라이언트
  │
  │  POST /users
  │  Body: {"name":"Alice","email":"alice@example.com"}
  │
  ▼
[Gin Router]
  │ 라우트 매칭 → UserHandler.createUser 호출
  ▼
[handler.createUser]
  │ c.ShouldBindJSON(&req)  → name, email 추출 + 유효성 검사
  │ 실패 시 400 반환
  ▼
[repository.Create]
  │ INSERT INTO users (name, email) VALUES (?, ?)
  │ LastInsertId() → 생성된 id 획득
  │ FindByID(id)   → 생성된 행 조회
  ▼
[AWS RDS MySQL]
  │ users 테이블에 행 삽입
  │ created_at / updated_at DEFAULT로 자동 설정
  ▼
[handler.createUser]
  │ 201 Created + 삽입된 User 객체 JSON 반환
  ▼
클라이언트
```

---

## API 엔드포인트

### POST /users — 사용자 생성

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
  "created_at": "2026-04-24T10:00:00Z",
  "updated_at": "2026-04-24T10:00:00Z"
}
```

---

### GET /users — 전체 사용자 조회

**Response** `200 OK`

```json
[
  {
    "id": 1,
    "name": "Alice",
    "email": "alice@example.com",
    "created_at": "2026-04-24T10:00:00Z",
    "updated_at": "2026-04-24T10:00:00Z"
  },
  {
    "id": 2,
    "name": "Bob",
    "email": "bob@example.com",
    "created_at": "2026-04-24T10:00:00Z",
    "updated_at": "2026-04-24T10:00:00Z"
  }
]
```

---

### GET /users/:id — 단건 조회

**Response** `200 OK` / `404 Not Found`

```json
{
  "id": 1,
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2026-04-24T10:00:00Z",
  "updated_at": "2026-04-24T10:00:00Z"
}
```

---

### PUT /users/:id — 사용자 수정

name, email 모두 교체합니다 (전체 업데이트).

**Request Body**

```json
{
  "name": "Bob",
  "email": "bob@example.com"
}
```

**Response** `200 OK` — 업데이트된 전체 User 객체 반환  
**Response** `404 Not Found` — 존재하지 않는 id

---

### DELETE /users/:id — 사용자 삭제

**Response** `204 No Content` / `404 Not Found`

---

## 환경 변수

`.env.example`을 복사하여 `.env`를 생성하고 값을 채웁니다.

```bash
cp .env.example .env
```

| 변수명 | 설명 | 예시 |
|---|---|---|
| `DB_HOST` | RDS 엔드포인트 | `mydb.xxxxxx.ap-northeast-2.rds.amazonaws.com` |
| `DB_PORT` | MySQL 포트 | `3306` |
| `DB_USER` | DB 사용자명 | `admin` |
| `DB_PASSWORD` | DB 비밀번호 | `yourpassword` |
| `DB_NAME` | 데이터베이스 이름 | `appdb` |
| `SERVER_PORT` | HTTP 서버 포트 | `8080` |

---

## AWS RDS 연결 특이사항

### VPC 및 Security Group

RDS 인스턴스는 VPC 내부에 위치합니다.  
애플리케이션(EC2, ECS 등)이 같은 VPC에 있거나, RDS Security Group이 해당 소스의 3306 포트 인바운드를 허용해야 합니다.

```
앱 (EC2 / ECS)
  │
  │  TCP 3306
  │  ─────────────────────────────────
  │  Security Group: allow from app SG
  ▼
AWS RDS MySQL (Private Subnet)
```

### DSN 옵션

`parseTime=true` — MySQL `DATETIME` 컬럼을 Go `time.Time`으로 자동 변환합니다.  
없으면 `Scan`에서 타입 오류가 발생합니다.

```
parseTime=true&charset=utf8mb4
```

### TLS (선택)

RDS는 기본적으로 TLS 연결을 지원합니다. 강제하려면 DSN에 아래를 추가합니다.

```
tls=true
```

AWS RDS CA 번들은 아래에서 다운로드할 수 있습니다.

```bash
curl -O https://truststore.pki.us-east-1.amazonaws.com/global/global-bundle.pem
```

---

## 실행 방법

### 사전 준비 (테이블 생성)

앱 실행 전 RDS에 테이블을 생성합니다.

```bash
# 테이블 생성
mysql -h <RDS_ENDPOINT> -u admin -p appdb < sql/schema.sql

# 초기 데이터 삽입 (선택)
mysql -h <RDS_ENDPOINT> -u admin -p appdb < sql/seed.sql
```

### 로컬 실행

```bash
# 1. 환경변수 설정
cp .env.example .env
# .env 파일 내 DB_HOST, DB_USER, DB_PASSWORD 등 실제 값으로 수정

# 2. 의존성 설치
go mod download

# 3. 실행
go run .

# 또는 바이너리 빌드 후 실행
go build -o server .
./server
```

### Docker 실행

```bash
# 1. 이미지 빌드
docker build -t rds-demo-app .

# 2. 컨테이너 실행 (.env 파일로 환경변수 주입)
docker run --env-file .env -p 8080:8080 rds-demo-app

# 또는 -e 플래그로 직접 주입
docker run -p 8080:8080 \
  -e DB_HOST=mydb.xxxxxx.ap-northeast-2.rds.amazonaws.com \
  -e DB_USER=admin \
  -e DB_PASSWORD=yourpassword \
  -e DB_NAME=appdb \
  rds-demo-app
```

**Dockerfile 구조** (멀티스테이지 빌드):

| 스테이지 | 베이스 이미지 | 역할 |
|---|---|---|
| builder | `golang:1.22-alpine` | 소스 컴파일 (CGO 비활성화, 정적 바이너리) |
| runtime | `alpine:3.20` | 바이너리만 포함 |

> `.env` 파일은 이미지에 포함되지 않습니다. 반드시 `--env-file` 또는 `-e` 플래그로 런타임에 주입하세요.

---

### 동작 확인

서버 기동 후 사용자 생성:

```bash
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'
```

전체 조회:

```bash
curl http://localhost:8080/users
```

단건 조회:

```bash
curl http://localhost:8080/users/1
```

수정:

```bash
curl -X PUT http://localhost:8080/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Bob","email":"bob@example.com"}'
```

삭제:

```bash
curl -X DELETE http://localhost:8080/users/1
```
