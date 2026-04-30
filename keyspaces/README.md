# AWS Keyspaces — Users CRUD API

Go + Gin 프레임워크로 구현한 AWS Keyspaces (Apache Cassandra 호환) 연동 RESTful API 서버입니다.  
`users` 테이블에 대한 CRUD 작업을 HTTP 엔드포인트로 노출합니다.

---

## 목차

1. [기술 스택](#기술-스택)
2. [프로젝트 구조](#프로젝트-구조)
3. [API 엔드포인트](#api-엔드포인트)
4. [환경 변수](#환경-변수)
5. [AWS Keyspaces 연결 특이사항](#aws-keyspaces-연결-특이사항)
6. [실행 방법](#실행-방법)

---

## 기술 스택

| 항목 | 내용 |
|---|---|
| 언어 | Go 1.22 |
| HTTP 프레임워크 | [Gin](https://github.com/gin-gonic/gin) v1.10 |
| DB 드라이버 | [gocql](https://github.com/gocql/gocql) v1.7 |
| AWS 인증 | [aws-sigv4-auth-cassandra-gocql-driver-plugin](https://github.com/aws/aws-sigv4-auth-cassandra-gocql-driver-plugin) v1.1 |
| 환경변수 로딩 | [godotenv](https://github.com/joho/godotenv) v1.5 |
| 데이터베이스 | AWS Keyspaces (Apache Cassandra 호환) |
| 인증 방식 | AWS SigV4 (IAM 자격증명) |

---

## 프로젝트 구조

```
keyspaces/
├── main.go        # DB 연결, 핸들러, 서버 기동 (단일 파일)
├── go.mod / go.sum
├── .env
└── Dockerfile
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
  "uuid": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Alice",
  "email": "alice@example.com"
}
```

> ID는 서버에서 `TimeUUID`로 자동 생성됩니다.

---

### GET /api/v1/users — 전체 조회

**Response** `200 OK`

```json
[
  {
    "uuid": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Alice",
    "email": "alice@example.com"
  }
]
```

---

### GET /api/v1/users/:uuid — 단건 조회

**Response** `200 OK` / `404 Not Found`

---

### PUT /api/v1/users/:uuid — 사용자 수정

**Request Body**

```json
{
  "name": "Bob",
  "email": "bob@example.com"
}
```

**Response** `200 OK` — 수정된 전체 User 객체 반환  
**Response** `404 Not Found` — 존재하지 않는 uuid

---

### DELETE /api/v1/users/:uuid — 사용자 삭제

**Response** `204 No Content` / `404 Not Found`

---

## 환경 변수

| 변수명 | 설명 | 기본값 |
|---|---|---|
| `AWS_REGION` | AWS 리전 | `ap-northeast-2` |
| `KEYSPACES_ENDPOINT` | Keyspaces 엔드포인트 호스트 | `cassandra.<region>.amazonaws.com` |
| `SERVER_PORT` | HTTP 서버 포트 | `8080` |

> AWS 자격증명은 환경변수(`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`) 또는 IAM Role(EC2/ECS)로 주입합니다.

---

## AWS Keyspaces 연결 특이사항

### 포트 및 TLS

Keyspaces는 표준 Cassandra 포트(9042) 대신 **9142** 포트를 사용하며, TLS가 필수입니다.

```go
cluster.Port = 9142
cluster.SslOpts = &gocql.SslOptions{
    EnableHostVerification: true,
    Config: &tls.Config{ServerName: endpoint},
}
```

### 인증 — SigV4

Keyspaces는 Cassandra 기본 인증(username/password) 대신 **AWS SigV4 서명**을 사용합니다.  
별도의 서비스 계정 없이 IAM 자격증명으로 접근할 수 있습니다.

```go
auth := sigv4.NewAwsAuthenticator()
auth.Region = region
cluster.Authenticator = auth
```

### 일관성 수준

Keyspaces는 `LOCAL_ONE` 또는 `LOCAL_QUORUM`을 지원합니다. 이 서버는 `LOCAL_QUORUM`을 사용합니다.

### Keyspace / 테이블 사전 생성

Keyspaces는 DDL을 코드에서 자동 실행하지 않습니다. 애플리케이션 실행 전에 아래 CQL을 먼저 실행해야 합니다.

```cql
CREATE KEYSPACE IF NOT EXISTS demo_keyspace
  WITH replication = {'class': 'SingleRegionStrategy'};

CREATE TABLE IF NOT EXISTS demo_keyspace.users (
  uuid  UUID PRIMARY KEY,
  name  TEXT,
  email TEXT
);
```

### 연결 흐름

```
Go 앱
  │
  │  gocql.NewCluster(cassandra.<region>.amazonaws.com)
  │  Port: 9142, TLS, SigV4 Auth
  │
  ▼
AWS Keyspaces (Cassandra 호환, Port 9142)
  │
  └── demo_keyspace.users (uuid, name, email)
```

---

## 실행 방법

### 로컬 실행

```bash
# 1. 의존성 설치
go mod download

# 2. 환경변수 설정
vi .env

# 3. 실행
go run .
```

### Docker 실행

```bash
docker build -t keyspaces-app .
docker run --env-file .env -p 8080:8080 keyspaces-app
```

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
curl http://localhost:8080/api/v1/users/{uuid}

# 수정
curl -X PUT http://localhost:8080/api/v1/users/{uuid} \
  -H "Content-Type: application/json" \
  -d '{"name":"Bob","email":"bob@example.com"}'

# 삭제
curl -X DELETE http://localhost:8080/api/v1/users/{uuid}
```
