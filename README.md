# Lucene Shard Analyzer Service

一个无状态的HTTP服务，用于分析OpenSearch/Elasticsearch分片归档文件，离线提取Lucene段的有用信息。

## 特性

- **多架构支持**：支持 `amd64` 和 `arm64` 架构构建
- **健康检查**：`/healthz` 端点用于监控
- **服务信息**：`/info` 端点提供版本、git SHA、架构和主机名
- **Prometheus 指标**：`/metrics` 端点用于监控性能
- **分片分析**：`/analyze` 端点用于上传和分析分片归档
- **Lucene 段洞察**：从Lucene段中提取详细信息

## 快速开始

### 前提条件

- 系统上已安装Docker
- Go 1.24.0或更高版本（用于本地开发）

### 使用Docker运行

```bash
docker run -p 8080:8080 ghcr.io/your-username/lucene-shard-analyzer:latest
```

### 本地运行

```bash
# 克隆仓库
git clone https://github.com/your-username/lucene-shard-analyzer.git
cd lucene-shard-analyzer/lucene-analyzer

# 构建服务
go build -o lucene-shard-analyzer

# 运行服务
./lucene-shard-analyzer
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

### POST /analyze

上传OpenSearch/Elasticsearch分片归档文件（tar/zip）并离线分析Lucene段。

**请求**：
- Content-Type: `multipart/form-data` 或直接文件上传
- 支持的文件格式：`.zip`、`.tar`、`.tar.gz`

**响应**：
```json
{
  "index_path": "/path/to/index",
  "segments_file": "segments_1",
  "total_segments": 2,
  "total_docs": 1000,
  "total_deleted_docs": 100,
  "total_soft_deleted_docs": 50,
  "user_data": {},
  "segments": [
    {
      "name": "_0",
      "seg_id": "0123456789abcdef",
      "codec": "Lucene90",
      "max_doc": 500,
      "compound": false,
      "del_gen": 1,
      "del_count": 50,
      "field_infos_gen": 0,
      "dv_gen": 0,
      "soft_del_count": 25,
      "diagnostics": {}
    },
    {
      "name": "_1",
      "seg_id": "fedcba9876543210",
      "codec": "Lucene90",
      "max_doc": 500,
      "compound": false,
      "del_gen": 1,
      "del_count": 50,
      "field_infos_gen": 0,
      "dv_gen": 0,
      "soft_del_count": 25,
      "diagnostics": {}
    }
  ],
  "notes": "Parsed per Lucene90SegmentInfoFormat"
}
```

## 构建和部署

### 构建Docker镜像

```bash
# 为本地架构构建
docker build -t lucene-shard-analyzer ./lucene-analyzer

# 为多个架构构建
docker buildx build --platform linux/amd64,linux/arm64 -t lucene-shard-analyzer ./lucene-analyzer
```

### 推送到容器注册表

```bash
# 标记镜像
docker tag lucene-shard-analyzer ghcr.io/your-username/lucene-shard-analyzer:latest

# 推送到GHCR
docker push ghcr.io/your-username/lucene-shard-analyzer:latest
```

## CI/CD 流水线

项目使用GitHub Actions进行CI/CD，工作流如下：

1. **构建**：为 `amd64` 和 `arm64` 构建多架构Docker镜像
2. **推送**：将镜像推送到GitHub Container Registry (GHCR)
3. **标签策略**：
   - `latest` 用于main分支构建
   - `vX.Y.Z` 用于标签发布
   - `sha-<shortsha>` 用于提交特定构建

### GitHub Actions 工作流

工作流定义在 `.github/workflows/build.yml` 中，包括：

- 推送到main分支时自动构建
- 创建标签时自动构建
- 多架构构建支持
- 镜像推送到GHCR

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
├── docker-compose.yml        # Docker Compose配置
└── README.md                 # 此文件
```

## 测试

### 运行单元测试

```bash
cd lucene-analyzer
go test -v
```

### 运行集成测试

```bash
./test/integration-test.sh
```

## 贡献

1. Fork 仓库
2. 创建功能分支 (`git checkout -b feature/your-feature`)
3. 提交更改 (`git commit -am 'Add some feature'`)
4. 推送到分支 (`git push origin feature/your-feature`)
5. 创建 Pull Request

## 许可证

本项目采用MIT许可证 - 详见LICENSE文件。

## 项目相关

- [Apache Lucene](https://lucene.apache.org/) - 为Elasticsearch和OpenSearch提供支持的搜索库
- [Prometheus](https://prometheus.io/) - 监控和告警工具包
- [Go](https://golang.org/) - 用于开发此服务的编程语言
