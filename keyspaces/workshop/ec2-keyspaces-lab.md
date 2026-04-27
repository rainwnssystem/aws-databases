# Amazon Keyspaces 실습: EC2에서 접근 및 쿼리

## 목표

1. Amazon Keyspaces 구성 (키스페이스 + 테이블 생성)
2. EC2 인스턴스에서 Keyspaces에 접근
3. CQL로 기본 쿼리 실행 (INSERT / SELECT / UPDATE / DELETE)

## 아키텍처

```
┌─────────────────────────────────────────────────────────┐
│  AWS Cloud                                              │
│                                                         │
│  ┌──────────────────┐    port 9142 (TLS)               │
│  │   EC2 Instance   │ ─────────────────────────────►   │
│  │                  │                                   │
│  │  cqlsh-expansion │         ┌─────────────────────┐  │
│  │  (CQL 클라이언트) │         │  Amazon Keyspaces   │  │
│  │                  │         │                     │  │
│  │  IAM Role 연결   │         │  keyspace: demo_keyspace │
│  └──────────────────┘         │  table: users       │  │
│                               └─────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

**핵심 포인트**
- EC2에 IAM Role을 붙여 자격증명 없이 SigV4 인증
- Keyspaces는 포트 9142 / TLS 필수
- cqlsh-expansion은 SigV4를 지원하는 확장 cqlsh 툴

---

## 사전 준비

- AWS 계정 및 콘솔 접근 권한
- 실습 리전: **ap-northeast-2 (서울)**

---

## Step 1. IAM Role 생성

EC2가 Keyspaces에 접근할 때 사용할 역할을 먼저 만듭니다.

### 1-1. IAM 콘솔에서 역할 생성

1. [IAM 콘솔](https://console.aws.amazon.com/iam) → **Roles** → **Create role**
2. **Trusted entity type**: AWS service
3. **Use case**: EC2 선택 → **Next**
4. **Permissions** 검색창에 `AmazonKeyspacesFullAccess` 입력 후 체크 → **Next**
5. **Role name**: `ec2-keyspaces-role`
6. **Create role**

> `AmazonKeyspacesFullAccess`는 실습용입니다. 프로덕션에서는 최소 권한 원칙을 적용하세요.

### 1-2. (대안) AWS CLI로 역할 생성

```bash
# Trust policy 파일 생성
cat > trust-policy.json << 'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": { "Service": "ec2.amazonaws.com" },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF

# 역할 생성
aws iam create-role \
  --role-name ec2-keyspaces-role \
  --assume-role-policy-document file://trust-policy.json

# 정책 연결
aws iam attach-role-policy \
  --role-name ec2-keyspaces-role \
  --policy-arn arn:aws:iam::aws:policy/AmazonKeyspacesFullAccess

# EC2 인스턴스 프로파일 생성 및 역할 연결
aws iam create-instance-profile \
  --instance-profile-name ec2-keyspaces-profile

aws iam add-role-to-instance-profile \
  --instance-profile-name ec2-keyspaces-profile \
  --role-name ec2-keyspaces-role
```

---

## Step 2. EC2 인스턴스 생성

### 2-1. 콘솔에서 생성

1. [EC2 콘솔](https://console.aws.amazon.com/ec2) → **Launch instance**
2. 다음 설정으로 구성:

| 항목 | 값 |
|------|-----|
| Name | `keyspaces-lab` |
| AMI | Amazon Linux 2023 |
| Instance type | t3.micro |
| Key pair | 기존 키페어 선택 또는 새로 생성 |
| VPC | default VPC |
| Subnet | 아무 public subnet |
| Auto-assign public IP | Enable |
| Security group | 아래 참고 |
| IAM instance profile | `ec2-keyspaces-profile` |

**Security Group 설정**

| 방향 | 프로토콜 | 포트 | 소스/대상 |
|------|---------|------|----------|
| Inbound | TCP | 22 (SSH) | 내 IP |
| Outbound | TCP | 9142 | 0.0.0.0/0 |
| Outbound | TCP | 443 | 0.0.0.0/0 |

> Outbound 9142: Keyspaces 연결 / Outbound 443: AWS API 호출 (IAM SigV4 인증)

3. **Launch instance**

### 2-2. AWS CLI로 생성

```bash
# 기본 VPC의 서브넷 ID 확인
SUBNET_ID=$(aws ec2 describe-subnets \
  --filters "Name=default-for-az,Values=true" \
  --query "Subnets[0].SubnetId" \
  --output text \
  --region ap-northeast-2)

# Security Group 생성
SG_ID=$(aws ec2 create-security-group \
  --group-name keyspaces-lab-sg \
  --description "Security group for Keyspaces lab" \
  --region ap-northeast-2 \
  --query "GroupId" --output text)

# Inbound: SSH
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID \
  --protocol tcp --port 22 \
  --cidr 0.0.0.0/0 \
  --region ap-northeast-2

# EC2 시작
aws ec2 run-instances \
  --image-id resolve:ssm:/aws/service/ami-amazon-linux-latest/al2023-ami-kernel-default-x86_64 \
  --instance-type t3.micro \
  --key-name <your-key-pair-name> \
  --security-group-ids $SG_ID \
  --subnet-id $SUBNET_ID \
  --associate-public-ip-address \
  --iam-instance-profile Name=ec2-keyspaces-profile \
  --tag-specifications 'ResourceType=instance,Tags=[{Key=Name,Value=keyspaces-lab}]' \
  --region ap-northeast-2
```

---

## Step 3. Amazon Keyspaces에 키스페이스와 테이블 생성

EC2에 접속하기 전에, Keyspaces 리소스를 먼저 생성합니다.

### 3-1. AWS CLI로 생성 (로컬 또는 CloudShell)

```bash
# 키스페이스 생성
aws keyspaces create-keyspace \
  --keyspace-name demo_keyspace \
  --region ap-northeast-2

# 생성 확인
aws keyspaces get-keyspace \
  --keyspace-name demo_keyspace \
  --region ap-northeast-2
```

예상 출력:
```json
{
    "keyspaceName": "demo_keyspace",
    "resourceArn": "arn:aws:cassandra:ap-northeast-2:111122223333:/keyspace/demo_keyspace/",
    "replicationStrategy": "SINGLE_REGION"
}
```

```bash
# 테이블 생성 (사용자 데이터)
aws keyspaces create-table \
  --keyspace-name demo_keyspace \
  --table-name users \
  --schema-definition '{
    "allColumns": [
      {"name": "uuid",  "type": "uuid"},
      {"name": "name",  "type": "text"},
      {"name": "email", "type": "text"}
    ],
    "partitionKeys": [{"name": "uuid"}]
  }' \
  --region ap-northeast-2

# 테이블 상태 확인 (ACTIVE가 될 때까지 30초~1분 소요)
aws keyspaces get-table \
  --keyspace-name demo_keyspace \
  --table-name users \
  --region ap-northeast-2 \
  --query "status"
```

**스키마 설계 이유**

| 컬럼 | 타입 | 역할 | 이유 |
|------|------|------|------|
| `uuid` | uuid | 파티션 키 (PRIMARY KEY) | 전역 유일한 식별자로 사용자를 구분 |
| `name` | text | 일반 컬럼 | 사용자 이름 |
| `email` | text | 일반 컬럼 | 사용자 이메일 |

> `uuid` 타입은 Cassandra/Keyspaces의 내장 UUID 타입입니다. `uuid()` 함수로 자동 생성할 수 있습니다.

### 3-2. 콘솔로 생성

1. [Keyspaces 콘솔](https://console.aws.amazon.com/keyspaces/home?region=ap-northeast-2) 접속
2. **Create keyspace** → name: `demo_keyspace` → **Create keyspace**
3. `demo_keyspace` 선택 → **Create table**
4. Table name: `users`
5. 컬럼 추가:
   - `uuid` (uuid) — Partition key
   - `name` (text)
   - `email` (text)
6. **Create table**

---

## Step 4. EC2에 접속하여 cqlsh-expansion 설치

### 4-1. EC2 접속

```bash
ssh -i <your-key.pem> ec2-user@<EC2-Public-IP>
```

### 4-2. 환경 설정 및 cqlsh-expansion 설치

```bash
# Python3 pip 확인
python3 --version
pip3 --version

# cqlsh-expansion 설치
pip3 install --user cqlsh-expansion

# PATH 추가
echo 'export PATH="$PATH:$HOME/.local/bin"' >> ~/.bashrc
source ~/.bashrc

# 버전 확인
cqlsh-expansion --version
# 출력: cqlsh 6.1.0

# 초기화 (인증서 + cqlshrc 자동 설정)
cqlsh-expansion.init
```

`cqlsh-expansion.init` 이 자동으로 처리하는 것:
- `~/.cassandra/` 디렉토리 생성
- SigV4 인증이 설정된 `cqlshrc` 파일 복사
- Amazon Keyspaces용 TLS 인증서 번들 복사

### 4-3. IAM Role 동작 확인

EC2의 IAM Role이 올바르게 연결되었는지 확인합니다.

```bash
# 인스턴스 메타데이터에서 역할 확인
curl -s http://169.254.169.254/latest/meta-data/iam/info | python3 -m json.tool

# AWS CLI로 현재 자격증명 확인
aws sts get-caller-identity --region ap-northeast-2
```

예상 출력:
```json
{
    "UserId": "AROAIOSFODNN7EXAMPLE:i-1234567890abcdef0",
    "Account": "111122223333",
    "Arn": "arn:aws:sts::111122223333:assumed-role/ec2-keyspaces-role/i-1234567890abcdef0"
}
```

---

## Step 5. Keyspaces 연결 및 쿼리 실행

### 5-1. 연결

```bash
cqlsh-expansion cassandra.ap-northeast-2.amazonaws.com 9142 --ssl
```

연결 성공 시:
```
Connected to Amazon Keyspaces at cassandra.ap-northeast-2.amazonaws.com:9142
[cqlsh 6.1.0 | Cassandra 3.11.2 | CQL spec 3.4.4 | Native protocol v4]
Use HELP for help.
cqlsh current consistency level is ONE.
cqlsh>
```

### 5-2. 키스페이스 및 테이블 확인

```sql
-- 키스페이스 목록 확인
SELECT keyspace_name FROM system_schema.keyspaces;

-- 테이블 목록 확인
SELECT table_name FROM system_schema.tables
  WHERE keyspace_name = 'demo_keyspace';

-- 테이블 스키마 확인
DESCRIBE TABLE demo_keyspace.users;
```

### 5-3. 데이터 삽입 (INSERT)

쓰기 작업 전에 일관성 수준을 설정합니다.

```sql
CONSISTENCY LOCAL_QUORUM;

-- uuid() 함수로 UUID 자동 생성하여 삽입
INSERT INTO demo_keyspace.users (uuid, name, email)
VALUES (uuid(), 'Alice', 'alice@example.com');

INSERT INTO demo_keyspace.users (uuid, name, email)
VALUES (uuid(), 'Bob', 'bob@example.com');

INSERT INTO demo_keyspace.users (uuid, name, email)
VALUES (uuid(), 'Charlie', 'charlie@example.com');

-- UUID를 직접 지정하여 삽입 (재현 가능한 테스트용)
INSERT INTO demo_keyspace.users (uuid, name, email)
VALUES (11111111-1111-1111-1111-111111111111, 'Dave', 'dave@example.com');
```

### 5-4. 데이터 조회 (SELECT)

```sql
-- UUID를 알고 있을 때 단건 조회 (가장 효율적)
SELECT * FROM demo_keyspace.users
  WHERE uuid = 11111111-1111-1111-1111-111111111111;

-- 특정 컬럼만 조회
SELECT uuid, name FROM demo_keyspace.users
  WHERE uuid = 11111111-1111-1111-1111-111111111111;

-- 전체 조회 (소규모 테이블 / 개발 확인용)
SELECT * FROM demo_keyspace.users;
```

예상 출력 (단건 조회):
```
 uuid                                 | email             | name
--------------------------------------+-------------------+------
 11111111-1111-1111-1111-111111111111 | dave@example.com  | Dave
```

> Keyspaces는 파티션 키 없이 전체 스캔을 할 경우 성능이 크게 저하됩니다. 프로덕션에서는 UUID를 사전에 저장해 두고 PRIMARY KEY로 조회하는 패턴을 권장합니다.

### 5-5. 데이터 수정 (UPDATE)

```sql
-- 이름 수정
UPDATE demo_keyspace.users
  SET name = 'David'
  WHERE uuid = 11111111-1111-1111-1111-111111111111;

-- 이메일 수정
UPDATE demo_keyspace.users
  SET email = 'david@example.com'
  WHERE uuid = 11111111-1111-1111-1111-111111111111;

-- 수정 확인
SELECT * FROM demo_keyspace.users
  WHERE uuid = 11111111-1111-1111-1111-111111111111;
```

> **주의**: `uuid`는 PRIMARY KEY이므로 수정 불가합니다.

### 5-6. 데이터 삭제 (DELETE)

```sql
-- 특정 컬럼(셀)만 삭제
DELETE email FROM demo_keyspace.users
  WHERE uuid = 11111111-1111-1111-1111-111111111111;

-- 행 전체 삭제
DELETE FROM demo_keyspace.users
  WHERE uuid = 11111111-1111-1111-1111-111111111111;

-- 삭제 확인
SELECT * FROM demo_keyspace.users
  WHERE uuid = 11111111-1111-1111-1111-111111111111;
```

### 5-7. TTL 활용 (선택 실습)

임시 사용자 데이터를 자동으로 만료시킵니다.

```sql
-- TTL 3600초(1시간)를 지정하여 삽입
INSERT INTO demo_keyspace.users (uuid, name, email)
VALUES (uuid(), 'TempUser', 'temp@example.com')
USING TTL 3600;

-- TTL 남은 시간 확인
SELECT uuid, name, TTL(name)
  FROM demo_keyspace.users
  WHERE uuid = <위에서 삽입된 uuid>;
```

---

## Step 6. 트러블슈팅

### 연결 실패 시 확인 순서

```bash
# 1. 엔드포인트 DNS 해석 확인
nslookup cassandra.ap-northeast-2.amazonaws.com

# 2. 포트 9142 연결 가능 여부 확인
nc -zv cassandra.ap-northeast-2.amazonaws.com 9142

# 3. IAM 자격증명 확인
aws sts get-caller-identity --region ap-northeast-2

# 4. Keyspaces 접근 권한 확인 (아래 명령이 성공하면 권한 OK)
aws keyspaces list-keyspaces --region ap-northeast-2
```

### 자주 발생하는 오류

| 오류 메시지 | 원인 | 해결 |
|------------|------|------|
| `Connection refused` | SG outbound 9142 차단 | Security Group outbound 9142 허용 |
| `AuthenticationFailed` | IAM Role 미연결 또는 권한 부족 | EC2에 IAM Role 확인, 정책 확인 |
| `Unauthorized` | Keyspaces 권한 없음 | `AmazonKeyspacesFullAccess` 정책 연결 확인 |
| `WriteFailure` | `CONSISTENCY LOCAL_QUORUM` 미설정 | 쓰기 전 `CONSISTENCY LOCAL_QUORUM;` 실행 |
| `InvalidQuery` | 파티션 키 없이 SELECT | `WHERE uuid = '...'` 조건 추가 |

---

## Step 7. 리소스 정리

실습 후 비용이 발생하지 않도록 삭제합니다.

### Keyspaces 리소스 삭제

```sql
-- cqlsh에서
DROP TABLE demo_keyspace.users;
DROP KEYSPACE demo_keyspace;
```

또는 AWS CLI로:

```bash
aws keyspaces delete-table \
  --keyspace-name demo_keyspace \
  --table-name users \
  --region ap-northeast-2

aws keyspaces delete-keyspace \
  --keyspace-name demo_keyspace \
  --region ap-northeast-2
```

### EC2 및 IAM 리소스 삭제

```bash
# EC2 인스턴스 ID 확인 후 종료
INSTANCE_ID=$(aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=keyspaces-lab" "Name=instance-state-name,Values=running" \
  --query "Reservations[0].Instances[0].InstanceId" \
  --output text --region ap-northeast-2)

aws ec2 terminate-instances \
  --instance-ids $INSTANCE_ID \
  --region ap-northeast-2

# IAM Role 정리
aws iam remove-role-from-instance-profile \
  --instance-profile-name ec2-keyspaces-profile \
  --role-name ec2-keyspaces-role

aws iam delete-instance-profile \
  --instance-profile-name ec2-keyspaces-profile

aws iam detach-role-policy \
  --role-name ec2-keyspaces-role \
  --policy-arn arn:aws:iam::aws:policy/AmazonKeyspacesFullAccess

aws iam delete-role \
  --role-name ec2-keyspaces-role
```

---

## 정리

| 단계 | 핵심 개념 |
|------|----------|
| IAM Role | EC2에 자격증명을 직접 넣지 않고 역할을 부여 (SigV4 자동 인증) |
| Security Group | Keyspaces는 포트 9142 / TLS 필수 |
| 파티션 키 설계 | `uuid` 타입으로 전역 유일 식별자를 파티션 키로 사용 |
| 쓰기 일관성 | `CONSISTENCY LOCAL_QUORUM` 설정 필수 |
| TTL | 임시 데이터 자동 만료에 유용 |
