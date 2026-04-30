# AWS Neptune — Persons Graph API

Go + Gin 프레임워크로 구현한 AWS Neptune 연동 RESTful API 서버입니다.  
`person` 버텍스와 `KNOWS` 엣지를 통해 그래프 탐색(1-hop / 2-hop)을 HTTP 엔드포인트로 노출합니다.

---

## 목차

1. [기술 스택](#기술-스택)
2. [프로젝트 구조](#프로젝트-구조)
3. [API 엔드포인트](#api-엔드포인트)
4. [환경 변수](#환경-변수)
5. [AWS Neptune 연결 특이사항](#aws-neptune-연결-특이사항)
6. [실행 방법](#실행-방법)

---

## 기술 스택

| 항목 | 내용 |
|---|---|
| 언어 | Go 1.25 |
| HTTP 프레임워크 | [Gin](https://github.com/gin-gonic/gin) v1.10 |
| DB 드라이버 | [gremlin-go](https://github.com/apache/tinkerpop/tree/master/gremlin-go) v3.8 |
| 환경변수 로딩 | [godotenv](https://github.com/joho/godotenv) v1.5 |
| 데이터베이스 | AWS Neptune (Apache TinkerPop Gremlin) |
| 프로토콜 | WebSocket over TLS (`wss://`) |

---

## 프로젝트 구조

```
neptune/
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

### POST /api/v1/persons — person 생성

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
  "id": "4af6d9c3-1234-...",
  "name": "Alice",
  "email": "alice@example.com"
}
```

---

### GET /api/v1/persons — 전체 조회

**Response** `200 OK`

```json
[
  { "id": "4af6...", "name": "Alice", "email": "alice@example.com" },
  { "id": "7bc1...", "name": "Bob",   "email": "bob@example.com"   }
]
```

---

### GET /api/v1/persons/:id — 단건 조회

**Response** `200 OK` / `404 Not Found`

---

### PUT /api/v1/persons/:id — 수정

**Request Body**

```json
{
  "name": "Alice Updated",
  "email": "new@example.com"
}
```

**Response** `200 OK` — 수정된 전체 Person 객체 반환  
**Response** `404 Not Found` — 존재하지 않는 id

---

### DELETE /api/v1/persons/:id — 삭제

**Response** `204 No Content` / `404 Not Found`

---

### POST /api/v1/persons/:id/knows/:targetId — 관계 생성

두 person 사이에 방향성 있는 `KNOWS` 엣지를 생성합니다.

```
Alice --[KNOWS]--> Bob
```

**Response** `201 Created`

```json
{
  "from": "4af6...",
  "edge": "KNOWS",
  "to":   "7bc1..."
}
```

**Response** `404 Not Found` — from 또는 to person이 존재하지 않을 경우

---

### DELETE /api/v1/persons/:id/knows/:targetId — 관계 삭제

`A → KNOWS → B` 엣지를 삭제합니다.

**Response** `204 No Content`

---

### GET /api/v1/persons/:id/knows — 직접 아는 사람 목록 (1-hop)

해당 person이 `KNOWS` 엣지로 직접 연결한 person 목록을 반환합니다.

```
Alice --[KNOWS]--> Bob, Charlie
```

**Response** `200 OK`

```json
[
  { "id": "7bc1...", "name": "Bob", "email": "bob@example.com" }
]
```

---

### GET /api/v1/persons/:id/friends-of-friends — 친구의 친구 목록 (2-hop)

2단계 `KNOWS` 탐색 결과를 반환합니다. 자기 자신과 직접 친구는 결과에서 제외됩니다.

```
Alice → Bob → Charlie   ← 반환됨
Alice → Bob             ← 직접 친구이므로 제외
```

이 쿼리는 RDB에서 두 번의 JOIN이 필요한 연산을 Gremlin 한 줄(`.Out("KNOWS").Out("KNOWS")`)로 표현합니다.

**Response** `200 OK`

```json
[
  { "id": "9de2...", "name": "Charlie", "email": "charlie@example.com" }
]
```

---

## 환경 변수

| 변수명 | 설명 | 기본값 |
|---|---|---|
| `NEPTUNE_ENDPOINT` | Neptune Gremlin WebSocket 엔드포인트 | `wss://localhost:8182/gremlin` |
| `TLS_SKIP_VERIFY` | TLS 인증서 검증 스킵 여부 (`true` / `false`) | `false` |
| `SERVER_PORT` | HTTP 서버 포트 | `8080` |

---

## AWS Neptune 연결 특이사항

### 프로토콜

Neptune은 Gremlin 쿼리를 **WebSocket** 으로 수신합니다. 엔드포인트 형식은 아래와 같습니다.

```
wss://<cluster-endpoint>:8182/gremlin
```

### TLS 인증서

Neptune은 AWS 관리형 TLS 인증서를 사용합니다. 인증서가 만료되었거나 VPC 내부 연결 테스트 시에는 `TLS_SKIP_VERIFY=true`로 검증을 우회할 수 있습니다. 프로덕션에서는 사용하지 마세요.

### Gremlin 버텍스 ID

Neptune이 자동 할당하는 UUID 형태의 문자열입니다. 생성 응답의 `"id"` 값을 이후 요청에 그대로 사용합니다.

### 연결 흐름

```
Go 앱
  │
  │  gremlingo.NewDriverRemoteConnection(wss://...:8182/gremlin)
  │  TraversalSource = "g"
  │
  ▼
AWS Neptune (Port 8182, WebSocket TLS)
  │
  ├── person 버텍스 (id, name, email)
  └── KNOWS 엣지 (방향성, 단순 연결)
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
docker build -t neptune-app .
docker run --env-file .env -p 8080:8080 neptune-app
```

### 동작 확인 (실습 시나리오)

```bash
# 1. person 3명 생성 — 응답의 id 값을 메모해두세요
curl -X POST http://localhost:8080/api/v1/persons \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com"}'

curl -X POST http://localhost:8080/api/v1/persons \
  -H "Content-Type: application/json" \
  -d '{"name":"Bob","email":"bob@example.com"}'

curl -X POST http://localhost:8080/api/v1/persons \
  -H "Content-Type: application/json" \
  -d '{"name":"Charlie","email":"charlie@example.com"}'

# 2. 관계 생성: Alice → Bob, Bob → Charlie
curl -X POST http://localhost:8080/api/v1/persons/{alice_id}/knows/{bob_id}
curl -X POST http://localhost:8080/api/v1/persons/{bob_id}/knows/{charlie_id}

# 3. Alice의 직접 친구 조회 (1-hop) → Bob
curl http://localhost:8080/api/v1/persons/{alice_id}/knows

# 4. Alice의 친구의 친구 조회 (2-hop) → Charlie
curl http://localhost:8080/api/v1/persons/{alice_id}/friends-of-friends
```
