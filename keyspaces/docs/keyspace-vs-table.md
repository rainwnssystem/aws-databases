# Keyspace vs Table

> 참고: [Amazon Keyspaces Developer Guide](https://docs.aws.amazon.com/keyspaces/latest/devguide/)

---

## 한 줄 요약

| | Keyspace | Table |
|--|----------|-------|
| 역할 | 네임스페이스 (컨테이너) | 실제 데이터 저장소 |
| RDBMS 대응 | Database | Table |
| 포함 관계 | 여러 Table을 포함 | 하나의 Keyspace에 속함 |

---

## Keyspace

테이블들을 논리적으로 묶는 **네임스페이스**. 복제 전략(Replication Strategy)을 정의하는 단위이며, 그 안의 모든 테이블에 동일하게 적용된다.

### 속성

| 속성 | 설명 |
|------|------|
| `keyspace_name` | 고유 이름 (영문자·숫자·`_`, 최대 48자) |
| Replication Strategy | 데이터를 어떻게 복제할지 정의 |
| Tags | 비용 추적·관리용 리소스 태그 |

### Replication Strategy 종류

| 전략 | 설명 |
|------|------|
| `SingleRegionStrategy` | 단일 리전 내 3개 AZ에 자동 3중 복제. **Amazon Keyspaces 기본값** |
| `SimpleStrategy` | Apache Cassandra 호환용. Keyspaces에서는 `SingleRegionStrategy`와 동일하게 동작 |
| `NetworkTopologyStrategy` | 멀티 리전 복제 시 사용 |

### CQL

```sql
-- 생성
CREATE KEYSPACE my_keyspace
  WITH REPLICATION = {'class': 'SingleRegionStrategy'};

-- 조회
SELECT * FROM system_schema.keyspaces;

-- 삭제 (포함된 테이블 전체 삭제됨)
DROP KEYSPACE my_keyspace;
```

### 제약

- 생성·삭제는 **비동기** 처리 → 완료 여부를 별도로 확인해야 함
- 키스페이스 이름은 **소문자**로 저장 (따옴표로 감싸지 않으면 대문자 입력도 소문자로 변환)
- 계정당 키스페이스 수 한도: 기본 256개

---

## Table

실제 데이터가 저장되는 구조. 반드시 하나의 키스페이스에 속해야 하며, **Primary Key 설계**가 성능의 핵심이다.

### 구성 요소

```
Table
├── Primary Key
│   ├── Partition Key   (필수, 1개 이상)  → 어느 파티션에 저장할지 결정
│   └── Clustering Columns (선택)        → 파티션 내 정렬 순서 결정
└── Regular Columns                      → 나머지 데이터
```

### Primary Key 상세

**Partition Key**
- 데이터가 저장될 파티션을 결정
- 단일 컬럼 또는 복합 컬럼(복수 컬럼을 이중 괄호로 묶음)
- 파티션당 처리량 한도: 읽기 3,000 RCU / 쓰기 1,000 WCU

**Clustering Columns**
- 파티션 내 데이터의 물리적 정렬 순서
- `ASC`(기본) 또는 `DESC` 지정 가능
- 범위 쿼리(`>`, `<`, `BETWEEN`) 지원

```sql
-- 단일 파티션 키
PRIMARY KEY (device_id)

-- 복합 파티션 키
PRIMARY KEY ((year, award))

-- 파티션 키 + 클러스터링 컬럼
PRIMARY KEY (device_id, recorded_at)

-- 복합 파티션 키 + 복수 클러스터링 컬럼
PRIMARY KEY ((year, award), category, rank)
```

### 테이블 속성

| 속성 | 설명 |
|------|------|
| Capacity mode | On-demand(기본) 또는 Provisioned |
| Encryption | AWS 관리 키(기본) 또는 고객 관리 KMS 키 |
| PITR | Point-in-Time Recovery, 최대 35일 복원 |
| TTL | 행/컬럼 단위 자동 만료 |
| Tags | 리소스 태그 |

### CQL

```sql
-- 생성
CREATE TABLE my_keyspace.sensor_data (
    device_id   text,
    recorded_at timestamp,
    temperature double,
    humidity    double,
    PRIMARY KEY (device_id, recorded_at)
) WITH CLUSTERING ORDER BY (recorded_at DESC);

-- 스키마 확인
DESCRIBE TABLE my_keyspace.sensor_data;

-- 시스템 카탈로그로 확인
SELECT * FROM system_schema.columns
  WHERE keyspace_name = 'my_keyspace'
    AND table_name    = 'sensor_data';

-- 삭제
DROP TABLE my_keyspace.sensor_data;
```

### 제약

- 테이블 생성·삭제도 **비동기** 처리 (`status: CREATING → ACTIVE`)
- Primary Key 컬럼은 `UPDATE`로 변경 불가 (삭제 후 재삽입 필요)
- Secondary Index 미지원 → 다른 접근 패턴은 별도 테이블로 설계
- `TRUNCATE` 미지원
- 테이블 이름 최대 48자, 영문자·숫자·`_`만 허용

---

## Keyspace vs Table 비교

| 항목 | Keyspace | Table |
|------|----------|-------|
| 역할 | 네임스페이스 | 데이터 저장 |
| RDBMS 비유 | Database (Schema) | Table |
| 포함 관계 | 다수의 Table 포함 | 하나의 Keyspace 소속 |
| 설정 범위 | 복제 전략 (모든 테이블에 적용) | 용량 모드, 암호화, TTL, PITR |
| DDL 처리 | 비동기 | 비동기 |
| 삭제 시 영향 | 하위 Table 전체 삭제 | 해당 Table만 삭제 |
| 이름 최대 길이 | 48자 | 48자 |

---

## 계층 구조 예시

```
AWS Account
└── ap-northeast-2 (리전)
    ├── Keyspace: iotdata
    │   ├── Table: sensor_data      → device_id | recorded_at DESC
    │   └── Table: device_registry  → device_id
    └── Keyspace: ecommerce
        ├── Table: orders_by_user   → user_id | order_date DESC
        └── Table: orders_by_status → status  | order_date DESC
```

키스페이스는 보통 **애플리케이션 또는 도메인 단위**로 구분하고, 테이블은 **액세스 패턴 단위**로 설계한다.
