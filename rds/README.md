# AWS RDS (MySQL) — Users CRUD API

Go + Gin 프레임워크로 구현한 AWS RDS MySQL 연동 RESTful API 서버입니다.  
`users` 테이블에 대한 CRUD 작업을 HTTP 엔드포인트로 노출합니다.

---

## 목차

1. [기술 스택](#기술-스택)
2. [프로젝트 구조](#프로젝트-구조)
3. [API 엔드포인트](#api-엔드포인트)
4. [환경 변수](#환경-변수)
5. [AWS RDS 연결 특이사항](#aws-rds-연결-특이사항)
6. [실행 방법](#실행-방법)

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
rds/
├── main.go          # DB 연결, 핸들러, 서버 기동 (단일 파일)
├── go.mod / go.sum
├── .env.example
├── Dockerfile
├── .dockerignore
└── sql/
    ├── schema.sql   # users 테이블 DDL
    └── seed.sql     # 초기 데이터
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
  "id": 1,
  "name": "Alice",
  "email": "alice@example.com",
  "created_at": "2026-04-24T10:00:00Z"
}
```

---

### GET /api/v1/users — 전체 사용자 조회

**Response** `200 OK`

```json
[
  {
    "id": 1,
    "name": "Alice",
    "email": "alice@example.com",
    "created_at": "2026-04-24T10:00:00Z"
  }
]
```

---

### GET /api/v1/users/:id — 단건 조회

**Response** `200 OK` / `404 Not Found`

---

### PUT /api/v1/users/:id — 사용자 수정

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
# DB_HOST, DB_USER, DB_PASSWORD 등 실제 값으로 수정

# 2. 의존성 설치
go mod download

# 3. 실행
go run .
```

### Docker 실행

```bash
docker build -t rds-app .
docker run --env-file .env -p 8080:8080 rds-app
```

**Dockerfile 구조** (멀티스테이지 빌드):

| 스테이지 | 베이스 이미지 | 역할 |
|---|---|---|
| builder | `golang:1.22-alpine` | 소스 컴파일 (CGO 비활성화, 정적 바이너리) |
| runtime | `alpine:3.20` | 바이너리만 포함 |

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
curl http://localhost:8080/api/v1/users/1

# 수정
curl -X PUT http://localhost:8080/api/v1/users/1 \
  -H "Content-Type: application/json" \
  -d '{"name":"Bob","email":"bob@example.com"}'

# 삭제
curl -X DELETE http://localhost:8080/api/v1/users/1
```
