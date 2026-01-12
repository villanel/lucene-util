
- **健康检查**：`/healthz` 端点用于监控
- **服务信息**：`/info` 端点提供版本、git SHA、架构和主机名
- **Prometheus 指标**：`/metrics` 端点用于监控性能
- **分片分析**：`/analyze` 端点用于上传和分析分片归档
- **Lucene 段洞察**：从Lucene段中提取详细信息

## 项目结构

```
├── lucene-analyzer/          # 主服务代码
│   ├── lucene_parser.go      # Lucene解析逻辑
│   ├── main.go               # HTTP服务实现
│   ├── go.mod                # Go模块依赖
│   └── go.sum                # Go模块校验和
├── test/                     # 测试文件
│   ├── integration-test.sh   # 集成测试脚本
│   └── test-data/            # 测试数据归档
├── k8s/                      # Kubernetes部署配置
│   └── deployment.yml        # 完整的部署配置
├── .github/                  # GitHub Actions工作流
│   └── workflows/            # CI/CD流水线配置
├── docker-compose.yml        # Docker Compose配置
└── README.md                 # 项目文档
```

## 快速开始

### 开发条件

- docker
- Go 1.24.0

### 使用Docker运行

```bash
docker run -p 8080:8080 ghcr.io/villanel/lucene-shard-analyzer:latest
```

### 本地运行

```bash
# 克隆仓库
git clone https://github.com/villanel/lucene-util.git
cd lucene-util/lucene-analyzer

# 构建服务
go build -o lucene-shard-analyzer

# 运行服务
./lucene-shard-analyzer
```

#### 本地验证

**服务运行后，使用以下命令验证**：

```bash
# 测试健康检查
curl http://localhost:8080/healthz

# 测试服务信息
curl http://localhost:8080/info

# 使用示例分片归档测试分析功能
test_file=$(ls ../test/test-data/*.zip | head -1)
curl -X POST -H "Content-Type: application/zip" \
  --data-binary @"$test_file" \
  http://localhost:8080/analyze | jq
```

## API 文档

### GET /healthz

**响应**：
```
HTTP/1.1 200 OK
Content-Type: text/plain

ok
```

### GET /info

**响应**：
```json
{
  "version": "dev",
  "git_sha": "abc1234",
  "arch": "amd64",
  "hostname": "your-hostname"
}
```

### GET /metrics

**响应**：
Prometheus格式的指标，用于监控服务。
```json{
# HELP analyze_operation_duration_seconds Analyze operation duration in seconds
# TYPE analyze_operation_duration_seconds histogram
analyze_operation_duration_seconds_bucket{le="0.005"} 3
analyze_operation_duration_seconds_bucket{le="0.01"} 3
analyze_operation_duration_seconds_bucket{le="0.025"} 3
analyze_operation_duration_seconds_bucket{le="0.05"} 4
analyze_operation_duration_seconds_bucket{le="0.1"} 4
analyze_operation_duration_seconds_bucket{le="0.25"} 4
analyze_operation_duration_seconds_bucket{le="0.5"} 4
analyze_operation_duration_seconds_bucket{le="1"} 4
analyze_operation_duration_seconds_bucket{le="2.5"} 4
analyze_operation_duration_seconds_bucket{le="5"} 4
analyze_operation_duration_seconds_bucket{le="10"} 4
analyze_operation_duration_seconds_bucket{le="+Inf"} 4
analyze_operation_duration_seconds_sum 0.036709208
analyze_operation_duration_seconds_count 4
# HELP analyze_operations_total Total number of analyze operations
# TYPE analyze_operations_total counter
analyze_operations_total 4
# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{endpoint="/analyze",method="POST",status="200"} 1
http_requests_total{endpoint="/analyze",method="POST",status="400"} 3
http_requests_total{endpoint="/info",method="GET",status="200"} 1
# HELP promhttp_metric_handler_requests_in_flight Current number of scrapes being served.
# TYPE promhttp_metric_handler_requests_in_flight gauge
promhttp_metric_handler_requests_in_flight 1
# HELP promhttp_metric_handler_requests_total Total number of scrapes by HTTP status code.
# TYPE promhttp_metric_handler_requests_total counter
promhttp_metric_handler_requests_total{code="200"} 1
promhttp_metric_handler_requests_total{code="500"} 0
promhttp_metric_handler_requests_total{code="503"} 0

}
```
### POST /analyze

上传OpenSearch/Elasticsearch分片归档文件（tar/zip）并离线分析Lucene段。

**请求**：
- Content-Type: `multipart/form-data` 或直接文件上传
- 支持的文件格式：`.zip`、`.tar`、`.tar.gz`

**响应**：
```json
{
    "index_path": "/tmp/lucene-shard-3300541074/s_NL8E3ySUW7ittn8yvdDQ/0/index",
    "segments_file": "segments_7y8",
    "total_segments": 7,
    "total_docs": 10297,
    "total_deleted_docs": 0,
    "total_soft_deleted_docs": 3,
    "user_data": {
        "history_uuid": "WH_1FxqyTH-pha7nwGJGrQ",
        "local_checkpoint": "10307",
        "max_seq_no": "10307",
        "max_unsafe_auto_id_timestamp": "-1",
        "min_retained_seq_no": "10308",
        "translog_uuid": "5GkYT9tDS1adQu4QlM51tg"
    },
    "segments": [
        {
            "name": "_8rd",
            "seg_id": "bb0edc6ae2e4fb9767b1478cba55a928",
            "codec": "Lucene103",
            "max_doc": 10210,
            "compound": false,
            "del_gen": -1,
            "del_count": 0,
            "field_infos_gen": 2,
            "dv_gen": 2,
            "soft_del_count": 3,
            "sci_id": "bb0edc6ae2e4fb9767b1478cba55aaae",
        }
    ]
  }
```

## 构建与部署

### 构建Docker镜像

```bash
# 为本地架构构建
docker build -t lucene-shard-analyzer ./lucene-analyzer
```



## 测试

### 单元测试

单元测试用于验证核心功能的正确性。

**运行命令**：
```bash
cd lucene-analyzer
go test -v
```

### 集成测试

集成测试脚本用于验证Lucene Shard Analyzer Service在Kubernetes环境中的完整功能。

#### 测试概述

**测试内容**：
- 服务健康检查和可用性
- 多副本负载均衡验证
- 实际分片文件分析功能
- 服务稳定性测试

#### 测试方法

**运行命令**：
```bash
./test/integration-test.sh
```

**测试工作原理**：
1. 在Kubernetes集群中创建临时测试Pod
2. 安装必要的测试工具（curl、jq）
3. 遍历test-data目录中的所有测试分片文件
4. 对每个文件执行analyze请求
5. 验证分析结果包含预期字段
6. 执行负载均衡测试，验证请求分发到所有Pod
7. 生成测试结果汇总
8. 清理临时测试资源

#### 负载均衡验证

项目使用Kubernetes Service实现负载均衡，确保请求均匀分布到所有运行的Pod。

**验证方法**：
```bash
# 获取服务IP
service_ip=$(kubectl get service lucene-shard-analyzer-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# 多次调用服务信息端点，验证主机名变化
for i in {1..10}; do
  curl http://$service_ip/info | jq -r '.hostname'
done
```

**预期结果**：
```
lucene-shard-analyzer-7f5f9d7f9d-2h4k7
lucene-shard-analyzer-7f5f9d7f9d-5j7m2
lucene-shard-analyzer-7f5f9d7f9d-2h4k7
lucene-shard-analyzer-7f5f9d7f9d-5j7m2
```
#### 分析接口验证

项目使用Kubernetes Service实现负载均衡，确保请求均匀分布到所有运行的Pod。

**验证方法**：
```bash
# 获取服务IP
service_ip=$(kubectl get service lucene-shard-analyzer-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}')

# 多次调用服务信息端点，验证主机名变化
curl -s -X POST -H "Content-Type: $content_type" \
    --data-binary @"$file_path" \
    http://lucene-shard-analyzer-service/analyze 2>/dev/null
```


## CI/CD 流水线

项目使用GitHub Actions实现完整的CI/CD流水线，包括构建、推送、部署和测试。




### 工作流概述

| 流水线类型 | 触发条件 | 主要任务 | 工作流文件 |
|------------|----------|----------|------------|
| Docker构建 | 主分支推送、标签创建 | 多架构镜像构建、推送 | `.github/workflows/docker-build.yml` |
| Kubernetes部署测试 | 主分支推送、标签创建、Pull Request | Kind本地集群、多副本部署、自动化测试 | `.github/workflows/kubernetes-deploy-test.yml` |

### Docker构建流水线

- **构建内容**：为 `amd64` 和 `arm64` 构建多架构Docker镜像
- **推送目标**：GitHub Container Registry (GHCR)
- **标签策略**：
  - `latest` 用于main分支构建
  - `vX.Y.Z` 用于标签发布
  - `sha-<shortsha>` 用于提交特定构建

### Kubernetes部署和测试流水线

- **部署内容**：
  - 使用Kind创建本地Kubernetes集群
  - 部署多副本Lucene Shard Analyzer Service
  - 执行自动化集成测试
- **测试内容**：
  - 服务健康检查（`GET /healthz`）
  - 负载均衡验证（多次调用`GET /info`验证不同Pod响应）
  - 分片分析测试（`POST /analyze`使用示例分片归档）

### segment文件分析
```
/index# hexdump -C segments_7y8
00000000  3f d7 6c 17 08 73 65 67  6d 65 6e 74 73 00 00 00  |?.l..segments...|
00000010  0a bb 0e dc 6a e2 e4 fb  97 67 b1 47 8c ba 55 aa  |....j....g.G..U.|
00000020  c0 03 37 79 38 0a 03 02  0a 00 00 00 00 00 00 a9  |..7y8...........|
00000030  fe bc 59 00 00 00 07 0a  03 02 04 5f 38 72 64 bb  |..Y........_8rd.|
00000040  0e dc 6a e2 e4 fb 97 67  b1 47 8c ba 55 a9 28 09  |..j....g.G..U.(.|
00000050  4c 75 63 65 6e 65 31 30  33 ff ff ff ff ff ff ff  |Lucene103.......|
00000060  ff 00 00 00 00 00 00 00  00 00 00 00 02 00 00 00  |................|
00000070  00 00 00 00 02 00 00 00  03 01 bb 0e dc 6a e2 e4  |.............j..|
00000080  fb 97 67 b1 47 8c ba 55  aa ae 01 0a 5f 38 72 64  |..g.G..U...._8rd|
00000090  5f 32 2e 66 6e 6d 00 00  00 01 00 00 00 08 02 15  |_2.fnm..........|
000000a0  5f 38 72 64 5f 32 5f 4c  75 63 65 6e 65 39 30 5f  |_8rd_2_Lucene90_|
000000b0  30 2e 64 76 64 15 5f 38  72 64 5f 32 5f 4c 75 63  |0.dvd._8rd_2_Luc|
000000c0  65 6e 65 39 30 5f 30 2e  64 76 6d 04 5f 38 74 77  |ene90_0.dvm._8tw|
000000d0  bb 0e dc 6a e2 e4 fb 97  67 b1 47 8c ba 55 aa 8b  |...j....g.G..U..|
000000e0  09 4c 75 63 65 6e 65 31  30 33 ff ff ff ff ff ff  |.Lucene103......|
000000f0  ff ff 00 00 00 00 ff ff  ff ff ff ff ff ff ff ff  |................|
00000100  ff ff ff ff ff ff 00 00  00 00 01 bb 0e dc 6a e2  |..............j.|
00000110  e4 fb 97 67 b1 47 8c ba  55 aa 8d 00 00 00 00 00  |...g.G..U.......|
00000120  04 5f 38 74 78 bb 0e dc  6a e2 e4 fb 97 67 b1 47  |._8tx...j....g.G|
00000130  8c ba 55 aa 8f 09 4c 75  63 65 6e 65 31 30 33 ff  |..U...Lucene103.|
00000140  ff ff ff ff ff ff ff 00  00 00 00 ff ff ff ff ff  |................|
00000150  ff ff ff ff ff ff ff ff  ff ff ff 00 00 00 00 01  |................|
00000160  bb 0e dc 6a e2 e4 fb 97  67 b1 47 8c ba 55 aa 91  |...j....g.G..U..|
00000170  00 00 00 00 00 04 5f 38  74 79 bb 0e dc 6a e2 e4  |......_8ty...j..|
00000180  fb 97 67 b1 47 8c ba 55  aa 93 09 4c 75 63 65 6e  |..g.G..U...Lucen|
00000190  65 31 30 33 ff ff ff ff  ff ff ff ff 00 00 00 00  |e103............|
000001a0  ff ff ff ff ff ff ff ff  ff ff ff ff ff ff ff ff  |................|
000001b0  00 00 00 00 01 bb 0e dc  6a e2 e4 fb 97 67 b1 47  |........j....g.G|
000001c0  8c ba 55 aa 95 00 00 00  00 00 04 5f 38 74 7a bb  |..U........_8tz.|
000001d0  0e dc 6a e2 e4 fb 97 67  b1 47 8c ba 55 aa 97 09  |..j....g.G..U...|
000001e0  4c 75 63 65 6e 65 31 30  33 ff ff ff ff ff ff ff  |Lucene103.......|
000001f0  ff 00 00 00 00 ff ff ff  ff ff ff ff ff ff ff ff  |................|
00000200  ff ff ff ff ff 00 00 00  00 01 bb 0e dc 6a e2 e4  |.............j..|
00000210  fb 97 67 b1 47 8c ba 55  aa 99 00 00 00 00 00 04  |..g.G..U........|
00000220  5f 38 75 30 bb 0e dc 6a  e2 e4 fb 97 67 b1 47 8c  |_8u0...j....g.G.|
00000230  ba 55 aa 9b 09 4c 75 63  65 6e 65 31 30 33 ff ff  |.U...Lucene103..|
00000240  ff ff ff ff ff ff 00 00  00 00 ff ff ff ff ff ff  |................|
00000250  ff ff ff ff ff ff ff ff  ff ff 00 00 00 00 01 bb  |................|
00000260  0e dc 6a e2 e4 fb 97 67  b1 47 8c ba 55 aa 9d 00  |..j....g.G..U...|
00000270  00 00 00 00 04 5f 38 75  31 bb 0e dc 6a e2 e4 fb  |....._8u1...j...|
00000280  97 67 b1 47 8c ba 55 aa  9f 09 4c 75 63 65 6e 65  |.g.G..U...Lucene|
00000290  31 30 33 ff ff ff ff ff  ff ff ff 00 00 00 00 ff  |103.............|
000002a0  ff ff ff ff ff ff ff ff  ff ff ff ff ff ff ff 00  |................|
000002b0  00 00 00 01 bb 0e dc 6a  e2 e4 fb 97 67 b1 47 8c  |.......j....g.G.|
000002c0  ba 55 aa a1 00 00 00 00  00 06 0d 74 72 61 6e 73  |.U.........trans|
000002d0  6c 6f 67 5f 75 75 69 64  16 35 47 6b 59 54 39 74  |log_uuid.5GkYT9t|
000002e0  44 53 31 61 64 51 75 34  51 6c 4d 35 31 74 67 13  |DS1adQu4QlM51tg.|
000002f0  6d 69 6e 5f 72 65 74 61  69 6e 65 64 5f 73 65 71  |min_retained_seq|
00000300  5f 6e 6f 05 31 30 33 30  38 10 6c 6f 63 61 6c 5f  |_no.10308.local_|
00000310  63 68 65 63 6b 70 6f 69  6e 74 05 31 30 33 30 37  |checkpoint.10307|
00000320  0c 68 69 73 74 6f 72 79  5f 75 75 69 64 16 57 48  |.history_uuid.WH|
00000330  5f 31 46 78 71 79 54 48  2d 70 68 61 37 6e 77 47  |_1FxqyTH-pha7nwG|
00000340  4a 47 72 51 0a 6d 61 78  5f 73 65 71 5f 6e 6f 05  |JGrQ.max_seq_no.|
00000350  31 30 33 30 37 1c 6d 61  78 5f 75 6e 73 61 66 65  |10307.max_unsafe|
00000360  5f 61 75 74 6f 5f 69 64  5f 74 69 6d 65 73 74 61  |_auto_id_timesta|
00000370  6d 70 02 2d 31 c0 28 93  e8 00 00 00 00 00 00 00  |mp.-1.(.........|
00000380  00 54 97 ac 79                                    |.T..y|
00000385

文件头 (Header)
3f d7 6c 17: Codec Magic。对应代码中的 0x3fd76c17。

08: 字符串长度。

73 65 67 6d 65 6e 74 73: 字符串内容 "segments"。

00 00 00 0a: Format Version。十六进制 0x0A = 十进制 10。

bb 0e dc 6a e2 e4 fb 97 67 b1 47 8c ba 55 aa c0: 16 字节的 Index ID。

03: Suffix 长度。

37 79 38: Suffix 内容 "7y8"（即文件名 segments_7y8 的后缀）。

2. Lucene 版本与全局统计
0a 03 02: Lucene Version。10.3.2。

0a 00 00 00: Created Version。10.0.0。

00 00 00 00 00 00 a9 fe: SegInfo Version。

bc 59: Counter (VLong)。

00 00 00 07: NumSegments。十六进制 0x07 = 7。表示该索引当前由 7 个段 组成。

段信息循环解析 (以第一个段 _8rd 为例)
从偏移量 0x00000038 附近开始：

04: 段名长度。

5f 38 72 64: 段名 _8rd。

bb 0e dc 6a ...: 段的唯一 ID (16 bytes)。

09: Codec 字符串长度。

4c 75 63 65 6e 65 31 30 33: Codec 名称 "Lucene103"。

ff ff ff ff ff ff ff ff: DelGen。-1 表示没有挂起的删除（这是 Long 的补码表示）。

00 00 00 00: DelCount。0 个硬删除。

00 00 00 00: FieldInfosGen。0。

00 00 00 00: DVGen。0。

02: SoftDelCount。这里是 0x02，表示该段有 2 个软删除文档。

/index# hexdump -C _8rd.si
00000000  3f d7 6c 17 13 4c 75 63  65 6e 65 39 30 53 65 67  |?.l..Lucene90Seg|
00000010  6d 65 6e 74 49 6e 66 6f  00 00 00 00 bb 0e dc 6a  |mentInfo.......j|
00000020  e2 e4 fb 97 67 b1 47 8c  ba 55 a9 28 00 0a 00 00  |....g.G..U.(....|
00000030  00 03 00 00 00 02 00 00  00 01 0a 00 00 00 03 00  |................|
00000040  00 00 02 00 00 00 e2 27  00 00 ff ff 0a 06 73 6f  |.......'......so|
00000050  75 72 63 65 05 6d 65 72  67 65 07 6f 73 2e 61 72  |urce.merge.os.ar|
00000060  63 68 05 61 6d 64 36 34  14 6a 61 76 61 2e 72 75  |ch.amd64.java.ru|
00000070  6e 74 69 6d 65 2e 76 65  72 73 69 6f 6e 0c 32 35  |ntime.version.25|
00000080  2e 30 2e 31 2b 38 2d 4c  54 53 0b 6d 65 72 67 65  |.0.1+8-LTS.merge|
00000090  46 61 63 74 6f 72 02 31  31 02 6f 73 05 4c 69 6e  |Factor.11.os.Lin|
000000a0  75 78 0b 6a 61 76 61 2e  76 65 6e 64 6f 72 10 45  |ux.java.vendor.E|
000000b0  63 6c 69 70 73 65 20 41  64 6f 70 74 69 75 6d 0a  |clipse Adoptium.|
000000c0  6f 73 2e 76 65 72 73 69  6f 6e 22 35 2e 31 35 2e  |os.version"5.15.|
000000d0  31 34 36 2e 31 2d 6d 69  63 72 6f 73 6f 66 74 2d  |146.1-microsoft-|
000000e0  73 74 61 6e 64 61 72 64  2d 57 53 4c 32 09 74 69  |standard-WSL2.ti|
000000f0  6d 65 73 74 61 6d 70 0d  31 37 36 37 36 31 31 38  |mestamp.17676118|
00000100  30 38 37 34 34 13 6d 65  72 67 65 4d 61 78 4e 75  |08744.mergeMaxNu|
00000110  6d 53 65 67 6d 65 6e 74  73 02 2d 31 0e 6c 75 63  |mSegments.-1.luc|
00000120  65 6e 65 2e 76 65 72 73  69 6f 6e 06 31 30 2e 33  |ene.version.10.3|
00000130  2e 32 12 08 5f 38 72 64  2e 66 64 6d 13 5f 38 72  |.2.._8rd.fdm._8r|
00000140  64 5f 4c 75 63 65 6e 65  39 30 5f 30 2e 64 76 6d  |d_Lucene90_0.dvm|
00000150  08 5f 38 72 64 2e 6b 64  69 14 5f 38 72 64 5f 4c  |._8rd.kdi._8rd_L|
00000160  75 63 65 6e 65 31 30 33  5f 30 2e 74 6d 64 08 5f  |ucene103_0.tmd._|
00000170  38 72 64 2e 6b 64 64 14  5f 38 72 64 5f 4c 75 63  |8rd.kdd._8rd_Luc|
00000180  65 6e 65 31 30 33 5f 30  2e 70 73 6d 13 5f 38 72  |ene103_0.psm._8r|
00000190  64 5f 4c 75 63 65 6e 65  39 30 5f 30 2e 64 76 64  |d_Lucene90_0.dvd|
000001a0  07 5f 38 72 64 2e 73 69  08 5f 38 72 64 2e 6e 76  |._8rd.si._8rd.nv|
000001b0  6d 08 5f 38 72 64 2e 66  6e 6d 14 5f 38 72 64 5f  |m._8rd.fnm._8rd_|
000001c0  4c 75 63 65 6e 65 31 30  33 5f 30 2e 74 69 70 08  |Lucene103_0.tip.|
000001d0  5f 38 72 64 2e 6e 76 64  14 5f 38 72 64 5f 4c 75  |_8rd.nvd._8rd_Lu|
000001e0  63 65 6e 65 31 30 33 5f  30 2e 64 6f 63 14 5f 38  |cene103_0.doc._8|
000001f0  72 64 5f 4c 75 63 65 6e  65 31 30 33 5f 30 2e 74  |rd_Lucene103_0.t|
00000200  69 6d 08 5f 38 72 64 2e  66 64 78 14 5f 38 72 64  |im._8rd.fdx._8rd|
00000210  5f 4c 75 63 65 6e 65 31  30 33 5f 30 2e 70 6f 73  |_Lucene103_0.pos|
00000220  08 5f 38 72 64 2e 6b 64  6d 08 5f 38 72 64 2e 66  |._8rd.kdm._8rd.f|
00000230  64 74 01 1f 4c 75 63 65  6e 65 39 30 53 74 6f 72  |dt..Lucene90Stor|
00000240  65 64 46 69 65 6c 64 73  46 6f 72 6d 61 74 2e 6d  |edFieldsFormat.m|
00000250  6f 64 65 0a 42 45 53 54  5f 53 50 45 45 44 00 c0  |ode.BEST_SPEED..|
00000260  28 93 e8 00 00 00 00 00  00 00 00 ad 3c e4 d4     |(...........<..|
0000026f

1. Header (文件头)
3f d7 6c 17: Codec Magic。标识这是一个 Lucene 编码文件。

13: 字符串长度 (19)。

4c 75 ... 6f: 字符串内容 "Lucene90SegmentInfo"。

00 00 00 00: Version (0)。

bb 0e ... aa: Segment ID (16 字节)。必须与 segments_7y8 中记录的 ID 完全一致。

28: Suffix 长度 (40)。

2. 段核心属性 (Lucene 9.0+ 格式)
从偏移量 0x0000002d 开始，注意这里进入了 Little Endian (小端序) 区域：

0a 00 00 00: Version Major (10)。

03 00 00 00: Version Minor (3)。

02 00 00 00: Version Bugfix (2)。合起来就是 Lucene 10.3.2。

01: HasMinVersion (true)。表示后面还有一个最小版本号。

0a 00 00 00 ...: MinVersion (10.3.2)。

e2 27 00 00: DocCount (小端序)。十六进制 0x27e2 = 10210。这个段 _8rd 包含 10,210 条文档。

00: IsCompoundFile (false)。说明这个段不是复合格式，其数据散落在多个文件中（.fdt, .tim 等）。

```