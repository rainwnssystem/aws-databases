# 📝 Amazon Keyspaces (for Apache Cassandra)

## 개요
---
- Scalable, highly available한 완전 관리형 Apache Cassandra 호환 데이터베이스 서비스
- 서버 프로비저닝, 패치, 소프트웨어 설치·운영 없이 사용 가능하며, 사용한 리소스에 대해서만 과금
- 트래픽에 따라 자동으로 스케일되며, virtually unlimited 처리량과 스토리지를 제공
- 기존 Cassandra application code와 driver를 그대로 사용하여 AWS 환경으로 워크로드를 이전·운영·확장 가능
> "Amazon Keyspaces (for Apache Cassandra) is a scalable, highly available, and managed Apache Cassandra–compatible database service. With Amazon Keyspaces, you don't have to provision, patch, or manage servers, and you don't have to install, maintain, or operate software."
>
> "You pay for only the resources that you use. The service automatically scales tables up and down in response to application traffic."
>
> "You can run your existing Cassandra workloads on AWS using the same Cassandra application code and developer tools that you use today."


## 구성 요소
---
### 클라이언트 연결 구조
- 클라이언트는 실제 내부 서버가 아닌 **로드 밸런서**에 연결되며, 로드 밸런서가 내부 스토리지 파티션으로 쿼리를 라우팅한다.
> "Amazon Keyspaces maps the nodes to load balancers that route your queries to one of the many underlying storage partitions."

- 연결 방식에 따라 클라이언트에 노출되는 노드 수가 다르다.
> "This is a connection established over any public endpoint. In this case, Amazon Keyspaces appears as a **nine-node** Apache Cassandra 3.11.2 cluster to the client."
>
> "This is a private connection established using an interface VPC endpoint. In this case, Amazon Keyspaces appears as a **three-node** Apache Cassandra 3.11.2 cluster to the client."

| 연결 방식 | 노출 노드 수 | 비고 |
|---|---|---|
| 퍼블릭 엔드포인트 | 9개 | Apache Cassandra 3.11.2 클러스터로 보임 |
| VPC Interface Endpoint | 3개 | Apache Cassandra 3.11.2 클러스터로 보임 |

- 노출 노드 수와 관계없이 실제 처리량과 스토리지는 virtually unlimited로 제공된다.
> "Independent of the connection type and the number of nodes that are visible to the client, Amazon Keyspaces provides virtually limitless throughput and storage."

### 스토리지 구조
- 데이터는 SSD 기반 파티션에 저장되며, AWS Region 내 **다수의 가용 영역(AZ)에 3중 복제**되어 고가용성을 보장한다.
> "Amazon Keyspaces stores data in partitions. A partition is an allocation of storage for a table, backed by solid state drives (SSDs). Amazon Keyspaces automatically replicates your data across multiple Availability Zones within an AWS Region for durability and high availability."

- 처리량 또는 스토리지 필요량이 증가하면 Amazon Keyspaces가 파티션 분할 및 추가를 자동으로 처리한다.
> "As your throughput or storage needs grow, Amazon Keyspaces handles the partition management for you and automatically provisions the required additional partitions."

### 데이터 모델
- Cassandra의 데이터 모델을 그대로 사용한다.
- **Partition Key** (필수): 데이터가 저장될 파티션을 결정. 단일 컬럼 또는 복합(compound) 컬럼으로 구성 가능
> "The partition key portion of the primary key is required and determines which partition of your cluster the data is stored in. The partition key can be a single column, or it can be a compound value composed of two or more columns."

- **Clustering Column** (선택): 파티션 내에서 데이터가 정렬되는 방식을 결정
> "The optional clustering column portion of your primary key determines how the data is clustered and sorted within each partition."

- **Primary Key**: Partition Key + Clustering Column의 조합. 테이블 내 모든 레코드에서 고유해야 함
> "Every Cassandra table must have a primary key, which is the unique key to each row in the table. The primary key is the composite of a required partition key and optional clustering columns. The data that comprises the primary key must be unique across all records in a table."

- JOINs이 없으므로, 각 접근 패턴에 맞게 테이블을 개별 설계하는 query-first design이 권장된다.
> "There are no JOINs in CQL. Therefore, you should design your tables with the shape of your data and how you need to access it for your business use cases. This might result in de-normalization with duplicated data. You should design each of your tables specifically for a particular access pattern."

### 처리량 용량 모드
- 두 가지 용량 모드 지원

  #### On-Demand 모드
  - 실제 수행된 읽기/쓰기에 대해서만 과금. 사전에 처리량 용량을 지정할 필요 없으며, 트래픽 변화에 따라 거의 즉각적으로 스케일
  > "With on-demand mode, you pay for only the reads and writes that your application actually performs. You do not need to specify your table's throughput capacity in advance. Amazon Keyspaces accommodates your application traffic almost instantly as it ramps up or down, making it a good option for applications with unpredictable traffic."

  #### Provisioned 모드
  - 예상 읽기/쓰기 수를 사전에 지정. 예측 가능한 트래픽에서 비용 최적화 가능하며, Auto Scaling 설정으로 자동 용량 조정 가능
  > "Provisioned capacity mode helps you optimize the price of throughput if you have predictable application traffic and can forecast your table's capacity requirements in advance. With provisioned capacity mode, you specify the number of reads and writes per second that you expect your application to perform. You can increase and decrease the provisioned capacity for your table automatically by enabling automatic scaling."

  - 하루 1회 On-Demand ↔ Provisioned 전환 가능
  > "You can change the capacity mode of your table once per day as you learn more about your workload's traffic patterns, or if you expect to have a large burst in traffic, such as from a major event that you anticipate will drive a lot of table traffic."


## 주요 특징
---
### 완전 관리형 서버리스
- 서버 프로비저닝, 패치, 소프트웨어 설치·운영이 불필요하다.
> "With Amazon Keyspaces, you don't have to provision, patch, or manage servers, and you don't have to install, maintain, or operate software."

- AWS Management Console 또는 코드 몇 줄로 keyspace와 table을 생성할 수 있다.
> "With just a few clicks on the AWS Management Console or a few lines of code, you can create keyspaces and tables in Amazon Keyspaces, without deploying any infrastructure or installing software."

- 기존 Apache Cassandra는 수백 개의 노드를 직접 프로비저닝·패치·운영해야 하며, 수백 대의 물리 서버와 하나 이상의 데이터 센터를 관리하는 운영 부담이 따른다.
> "A production Cassandra deployment might consist of hundreds of nodes, running on hundreds of physical computers across one or more physical data centers. This can cause an operational burden for application developers who need to provision, patch, and manage servers in addition to installing, maintaining, and operating software."

- Amazon Keyspaces는 클러스터, 호스트, JVM을 설정할 필요가 없다. Compaction, compression, caching, garbage collection 등의 설정은 적용되지 않으며 지정 시 무시된다.
> "Amazon Keyspaces is serverless—there are no clusters, hosts, or JVMs to configure. Cassandra's settings for compaction, compression, caching, garbage collection, and bloom filtering are not applicable and are ignored if specified."

### 자동 스케일링
- 애플리케이션 트래픽에 따라 테이블이 자동으로 스케일 업/다운된다.
> "The service automatically scales tables up and down in response to application traffic."

- 초당 수천 건의 요청을 처리하는 애플리케이션을 virtually unlimited 처리량과 스토리지로 구축할 수 있다.
> "You can build applications that serve thousands of requests per second with virtually unlimited throughput and storage."

### 고가용성 및 내구성
- 데이터를 여러 AZ에 3중 복제하여 저장하며, 최고 수준의 보안 요구사항을 충족하는 AWS 데이터 센터 및 네트워크 아키텍처 위에서 동작한다.
> "Amazon Keyspaces (for Apache Cassandra) stores three copies of your data in multiple Availability Zones for durability and high availability. In addition, you benefit from a data center and network architecture that is built to meet the requirements of the most security-sensitive organizations."

- 기존 Cassandra는 replication factor와 consistency level을 직접 설정해야 하지만, Amazon Keyspaces는 고가용성·내구성·단일 자릿수 밀리초 성능을 제공하도록 이를 자동 설정한다.
> "Amazon Keyspaces automatically configures settings such as replication factor and consistency level to provide you with high availability, durability, and single-digit-millisecond performance."

### 보안 (기본 내장)
- 테이블 생성 시 저장 데이터 암호화(Encryption at Rest)가 자동으로 활성화되며, 모든 클라이언트 연결은 TLS를 필수로 요구한다.
> "Encryption at rest is automatically enabled when you create a new Amazon Keyspaces table and all client connections require Transport Layer Security (TLS). Additional AWS security features include monitoring, AWS Identity and Access Management, and virtual private cloud (VPC) endpoints."

- 인증/인가는 AWS IAM을 통해 처리되며 Cassandra 자체 보안 명령은 지원하지 않는다.
> "Amazon Keyspaces uses AWS Identity and Access Management (IAM) for user authentication and authorization, supporting equivalent authorization policies as Apache Cassandra. Amazon Keyspaces does not support Apache Cassandra's security configuration commands."

### 단일 자릿수 밀리초 성능
- 읽기 쿼리 시 1MB 데이터를 읽으면 자동으로 페이지네이션을 적용하여 일관된 성능을 유지한다.
> "Amazon Keyspaces automatically paginates after reading 1 MB of data to provide consistent, single-digit millisecond read performance."

- 더 높은 탄력성과 낮은 지연 시간을 위해 Multi-Region replication을 제공한다.
> "For even more resiliency and low-latency local reads, Amazon Keyspaces offers multi-Region replication."

### 일관성 수준
- 쓰기는 모든 AZ에 3중 복제 후 확인 응답을 보내며, `LOCAL_QUORUM`만 지원한다.
> "Amazon Keyspaces replicates all write operations three times across multiple Availability Zones for durability and high availability. Writes are durably stored before they are acknowledged using the `LOCAL_QUORUM` consistency level."

- 읽기는 `ONE`, `LOCAL_ONE`, `LOCAL_QUORUM` 세 가지를 지원한다.
  - `LOCAL_QUORUM`: 직전 완료된 모든 쓰기 결과를 반영한 응답 보장
  - `ONE` / `LOCAL_ONE`: 성능과 가용성을 높이지만 최근 쓰기가 반영되지 않을 수 있음
> "Amazon Keyspaces supports three read consistency levels: `ONE`, `LOCAL_ONE`, and `LOCAL_QUORUM`. During a `LOCAL_QUORUM` read, Amazon Keyspaces returns a response reflecting the most recent updates from all prior successful write operations. Using the consistency level `ONE` or `LOCAL_ONE` can improve the performance and availability of your read requests, but the response might not reflect the results of a recently completed write."

- 기존 Cassandra는 `EACH_QUORUM`, `QUORUM`, `ALL`, `TWO`, `THREE`, `ANY`, `SERIAL`, `LOCAL_SERIAL` 등 다양한 일관성 수준을 지원하지만, Amazon Keyspaces에서 위 수준을 지정하면 예외(exception)가 발생한다.
> "`EACH_QUORUM`, `QUORUM`, `ALL`, `TWO`, `THREE`, `ANY`, `SERIAL`, `LOCAL_SERIAL` — will result in exceptions."

### 비용 모델
- 사용한 리소스에 대해서만 과금하며, 사전 인프라 프로비저닝이 필요 없다.
> "You pay for only the resources that you use."

- Github에 공개된 pricing calculator를 통해 기존 Cassandra 워크로드와의 직접 비용을 비교해볼 수 있다. (단, TCO 항목인 인프라 유지비, 운영 오버헤드, 지원 비용은 포함되지 않음)


## Apache Cassandra와의 차이
---
### DDL 비동기 처리
- Amazon Keyspaces는 keyspace·table·type 생성/삭제 등 DDL 작업을 **비동기**로 처리하며, 생성 완료 여부는 상태를 별도로 확인해야 한다.
> "Amazon Keyspaces performs data definition language (DDL) operations, such as creating and deleting keyspaces, tables, and types asynchronously. To monitor creation status, you must check keyspace and table creation status separately."

- 기존 Cassandra는 DDL 작업이 동기적으로 처리된다.

### 연결(Connection) 처리량 제한
- TCP 연결 1개당 초당 최대 3,000 CQL 쿼리를 처리하며, 기본적으로 9개의 peer IP를 노출하므로 기본 최대 처리량은 27,000 CQL 쿼리/초이다.
> "Amazon Keyspaces supports up to 3,000 CQL queries per TCP connection per second. Amazon Keyspaces exposes 9 peer IP addresses to drivers. Default maximum CQL query throughput: 27,000 CQL queries per second (3,000 × 9). To increase throughput, increase connections per IP address (e.g., 2 connections per IP doubles throughput to 54,000 queries/second)."

- 권장 설정: 연결당 500 CQL 쿼리/초 기준으로 계획
> "Configure drivers to use 500 CQL queries per second per connection. For 18,000 CQL queries per second, plan for 36 connections (4 connections across 9 endpoints)."

- 기존 Cassandra에는 per-connection 처리량 제한이 없다.

### Batch 제한
- Logged batch: 최대 100개 명령 / Unlogged batch: 최대 30개 명령 / batch 내에서는 `INSERT`, `UPDATE`, `DELETE`만 허용
> "Amazon Keyspaces supports: **Logged batch** – up to 100 commands per batch; **Unlogged batch** – up to 30 commands per batch. Only **INSERT**, **UPDATE**, or **DELETE** commands are permitted in a batch."

### 페이지네이션 방식
- 반환된 행(rows returned)이 아닌 **읽은 행(rows read)** 기준으로 페이지를 나누며, 필터링 쿼리의 경우 PAGE SIZE보다 적은 행이 반환될 수 있다.
> "Results are paginated based on rows **read** to process a request, not rows returned. Some pages may contain fewer rows than specified in PAGE SIZE for filtered queries."

- 기존 Cassandra는 반환된 행 수를 기준으로 페이지를 나눈다.

### Prepared Statements 제한
- DML 작업(읽기/쓰기)에 대해서만 prepared statements를 지원하며, DDL 작업은 prepared statements 밖에서 실행해야 한다.
> "**Supported:** DML operations (reading and writing data). **Not supported:** DDL operations (creating tables and keyspaces). DDL operations must be run outside of prepared statements."

### CDC(Change Data Capture) 차이
- Keyspaces CDC는 변경 전/후의 전체 행(row)을 캡처하며, 테이블 단위로 중복 제거 및 순서 보장된 변경 레코드를 제공한다. 각 CDC 스트림은 ARN을 가진 AWS 리소스이다.
> "Amazon Keyspaces CDC streams capture the version of the row before and after changes (vs. Cassandra showing only modified columns). CDC streams provide de-duplicated and ordered change records at the table level. Each CDC stream is an AWS resource with an ARN."

- 기존 Cassandra CDC는 변경된 컬럼만 캡처한다.

### 지원되지 않는 CQL 연산
| 미지원 항목 | 비고 |
|---|---|
| `CREATE INDEX` / `DROP INDEX` | Secondary index 미지원 |
| `ALTER TYPE` | UDT 수정 불가 |
| `CREATE TRIGGER` / `DROP TRIGGER` | 트리거 미지원 |
| `CREATE FUNCTION` / `DROP FUNCTION` | 사용자 정의 함수 미지원 |
| `CREATE AGGREGATE` / `DROP AGGREGATE` | 집계 함수 미지원 |
| `CREATE MATERIALIZED VIEW` | 구체화된 뷰 미지원 |
| `TRUNCATE` | 미지원 |

### Lightweight Transactions (LWT)
- `INSERT`, `UPDATE`, `DELETE`에서 compare-and-set 기능을 완전하게 지원하며, 서버리스 특성상 LWT 사용 시에도 성능 패널티가 없다.
> "Amazon Keyspaces fully supports compare-and-set functionality on **INSERT**, **UPDATE**, and **DELETE** commands (lightweight transactions/LWTs). As a serverless offering, Amazon Keyspaces provides consistent performance at scale with no performance penalty for using LWTs."

- 기존 Cassandra에서는 LWT 사용 시 성능 패널티가 발생한다.

### 기타 차이점 요약

| 항목 | Apache Cassandra | Amazon Keyspaces |
|---|---|---|
| 인프라 관리 | 직접 관리 (노드, 클러스터) | AWS 완전 관리 |
| 스케일링 | 수동 노드 추가/제거 | 자동 스케일링 |
| 인증/인가 | Cassandra 자체 보안 | AWS IAM |
| 데이터 복제 | 설정에 따라 다름 | 기본 3중 복제 (다중 AZ) |
| DDL 처리 | 동기 | 비동기 |
| 쓰기 일관성 | 다양한 레벨 | `LOCAL_QUORUM`만 |
| 읽기 일관성 | 다양한 레벨 | `ONE`, `LOCAL_ONE`, `LOCAL_QUORUM` |
| LWT 성능 패널티 | 있음 | 없음 |
| CDC 캡처 범위 | 변경 컬럼만 | 변경 전/후 전체 행 |
| Materialized View | 지원 | 미지원 |
| Secondary Index | 지원 | 미지원 |
| 클러스터 설정 | 직접 튜닝 | 자동 설정, 무시됨 |
| 접속 방법 | IP:Port | 서비스 엔드포인트:9142 + TLS |


## 참고
---
- [What is Amazon Keyspaces?](https://docs.aws.amazon.com/keyspaces/latest/devguide/what-is-keyspaces.html)
- [Amazon Keyspaces: How it works](https://docs.aws.amazon.com/keyspaces/latest/devguide/how-it-works.html)
- [Compare Amazon Keyspaces with Cassandra](https://docs.aws.amazon.com/keyspaces/latest/devguide/keyspaces-vs-cassandra.html)
- [Functional differences with Apache Cassandra](https://docs.aws.amazon.com/keyspaces/latest/devguide/functional-differences.html)
- [Supported Cassandra APIs, operations, functions, and data types](https://docs.aws.amazon.com/keyspaces/latest/devguide/cassandra-apis.html)
- [Supported Cassandra consistency levels](https://docs.aws.amazon.com/keyspaces/latest/devguide/consistency.html)
- [Amazon Keyspaces use cases](https://docs.aws.amazon.com/keyspaces/latest/devguide/use-cases.html)
