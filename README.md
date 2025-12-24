# Cloud Sandbox for Coding Agents

> 为 AI 编码 Agent 构建的云端沙箱执行环境

[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://go.dev/)
[![Python Version](https://img.shields.io/badge/Python-3.9+-green.svg)](https://python.org/)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

## 项目概述

Cloud Sandbox 是一个开源的云端代码执行沙箱系统，专为 AI 编码助手、自主 Agent 和在线 IDE 设计。它提供安全隔离的执行环境，支持会话持久化和快速恢复。

### 核心特性

| 特性 | 描述 |
|------|------|
| **安全隔离** | 每个沙箱独立运行，互不干扰 |
| **会话持久化** | 支持暂停/恢复，跨天继续工作 |
| **沙箱池** | 预热池实现快速分配 |
| **多语言支持** | Python、Node.js、Go 等 |
| **完整 SDK** | Python SDK 开箱即用 |

### 使用场景

```
┌─────────────────────────────────────────────────────────────────┐
│  场景 A: AI 编码助手 (Claude Code / Cursor / Copilot)           │
│  用户请求 → LLM 生成代码 → 沙箱执行 → 返回结果                    │
│                                                                 │
│  场景 B: 自主 Agent (Manus / Devin / OpenDevin)                 │
│  Agent 规划任务 → 沙箱执行每步 → 根据结果决策                     │
│                                                                 │
│  场景 C: 在线 IDE / Notebook (Replit / Colab)                   │
│  用户在浏览器写代码 → 沙箱实时执行 → 即时反馈                      │
│                                                                 │
│  场景 D: 编程教育平台 (LeetCode / Codecademy)                    │
│  学生提交代码 → 沙箱运行测试 → 自动评分                           │
└─────────────────────────────────────────────────────────────────┘
```

## 快速开始

### 环境要求

- Docker 20.10+
- Go 1.22+ (构建)
- Python 3.9+ (SDK)

### 1. 克隆项目

```bash
git clone https://github.com/cloud-sandbox/cloud-sandbox.git
cd cloud-sandbox
```

### 2. 构建并启动

```bash
# 构建所有服务
make build

# 启动服务
./bin/scheduler &      # 沙箱调度器 (端口 9090)
./bin/session-manager & # 会话管理器 (端口 9091)
./bin/gateway          # API 网关 (端口 8080)

# 或使用一键启动
make run-all
```

### 3. 验证服务

```bash
curl http://localhost:8080/health
# {"service":"gateway","status":"ok"}
```

### 4. 使用 Python SDK

```bash
pip install -e sdk/python
```

```python
from cloud_sandbox import Sandbox

# 创建沙箱并执行代码
with Sandbox.create(
    base_url="http://localhost:8080",
    user_id="my-user",
) as sandbox:
    # 执行 Python 代码
    result = sandbox.run_code("print('Hello, Cloud Sandbox!')")
    print(result.stdout)  # Hello, Cloud Sandbox!

    # 执行 Shell 命令
    result = sandbox.run_command("ls -la /workspace")

    # 文件操作
    sandbox.write_file("/workspace/test.py", "x = 42")
    files = sandbox.list_files("/workspace")
```

### 5. 使用 REST API

```bash
# 获取认证 Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test", "role": "user"}' | jq -r '.access_token')

# 获取沙箱
SANDBOX=$(curl -s -X POST http://localhost:8080/api/v1/sandbox/acquire \
  -H "Authorization: Bearer $TOKEN" | jq -r '.sandbox_id')

# 执行代码
curl -s -X POST http://localhost:8080/api/v1/execute \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"sandbox_id\": \"$SANDBOX\", \"code\": \"print('Hello!')\", \"language\": \"python\"}"

# 释放沙箱
curl -s -X POST http://localhost:8080/api/v1/sandbox/release \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"sandbox_id\": \"$SANDBOX\"}"
```

## 架构设计

```
                              ┌──────────────────┐
                              │   Load Balancer  │
                              └────────┬─────────┘
                                       │
                              ┌────────▼─────────┐
                              │    API Gateway   │
                              │  JWT认证/限流/路由 │
                              └────────┬─────────┘
                                       │
           ┌───────────────────────────┼───────────────────────────┐
           │                           │                           │
   ┌───────▼───────┐          ┌───────▼───────┐          ┌───────▼───────┐
   │Session Manager│          │   Scheduler   │          │    Metrics    │
   │  会话生命周期  │          │   沙箱调度    │          │   Prometheus  │
   └───────┬───────┘          └───────┬───────┘          └───────────────┘
           │                          │
           │          ┌───────────────┴───────────────┐
           │          │         Sandbox Pool          │
           │          │  ┌────┐ ┌────┐ ┌────┐ ┌────┐  │
           │          │  │ C1 │ │ C2 │ │ C3 │ │ CN │  │
           │          │  └────┘ └────┘ └────┘ └────┘  │
           │          └───────────────────────────────┘
           │
   ┌───────┴───────────────────────────────────────────┐
   │                 Storage Layer                     │
   │  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
   │  │PostgreSQL│  │  Redis   │  │  MinIO   │        │
   │  │ Sessions │  │  Cache   │  │Workspace │        │
   │  └──────────┘  └──────────┘  └──────────┘        │
   └───────────────────────────────────────────────────┘
```

## 项目结构

```
cloud-sandbox/
├── cmd/                      # 服务入口
│   ├── gateway/             # API 网关
│   ├── scheduler/           # 沙箱调度器
│   └── session-manager/     # 会话管理器
│
├── internal/                 # 内部实现
│   ├── sandbox/             # 沙箱管理 (Docker)
│   ├── session/             # 会话状态
│   ├── auth/                # JWT 认证
│   ├── middleware/          # 中间件 (限流/日志)
│   └── metrics/             # Prometheus 指标
│
├── sdk/                      # 客户端 SDK
│   └── python/              # Python SDK
│
├── deploy/                   # 部署配置
│   ├── docker/              # Docker 镜像
│   └── k8s/                 # Kubernetes 配置
│       ├── base/            # 基础配置
│       └── overlays/        # 环境覆盖 (dev/prod)
│
├── tests/                    # 测试
│   └── e2e/                 # 端到端测试
│
├── docs/                     # 文档
│   ├── user-guide.md        # 用户指南
│   ├── quick-start.md       # 快速开始
│   └── architecture-comparison.md
│
└── config/                   # 配置文件
```

## API 参考

| 端点 | 方法 | 描述 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/api/v1/auth/token` | POST | 获取 JWT Token |
| `/api/v1/sessions` | GET/POST | 会话管理 |
| `/api/v1/sessions/{id}` | GET/DELETE | 单个会话操作 |
| `/api/v1/sessions/{id}/pause` | POST | 暂停会话 |
| `/api/v1/sessions/{id}/resume` | POST | 恢复会话 |
| `/api/v1/sandbox/acquire` | POST | 获取沙箱 |
| `/api/v1/sandbox/release` | POST | 释放沙箱 |
| `/api/v1/sandbox/stats` | GET | 沙箱池统计 |
| `/api/v1/execute` | POST | 执行代码 |
| `/api/v1/files` | GET/PUT/DELETE | 文件操作 |

详细 API 文档见 [docs/user-guide.md](docs/user-guide.md)

## 部署

### Docker Compose (开发环境)

```bash
# 启动基础设施
docker compose up -d

# 启动服务
make run-all
```

### Kubernetes (生产环境)

```bash
# 开发环境
make k8s-dev

# 生产环境
make k8s-prod
```

## 配置

### 环境变量

| 变量 | 默认值 | 描述 |
|------|--------|------|
| `JWT_SECRET` | `cloud-sandbox-secret` | JWT 签名密钥 |
| `POOL_MIN_SIZE` | `5` | 沙箱池最小数量 |
| `POOL_MAX_SIZE` | `50` | 沙箱池最大数量 |
| `SANDBOX_IMAGE` | `python:3.11-slim` | 默认容器镜像 |

## 开发路线图

### Phase 1: MVP ✅ (已完成)

- [x] Docker 容器沙箱
- [x] 沙箱池管理 (预热/清理)
- [x] 基础会话管理 (创建/暂停/恢复/删除)
- [x] 代码执行 API
- [x] 文件操作 API
- [x] JWT 认证
- [x] 请求限流
- [x] Python SDK
- [x] 端到端测试
- [x] Kubernetes 部署配置
- [x] Prometheus 监控指标

### Phase 2: 生产就绪 (计划中)

- [ ] Firecracker 微虚拟机集成
- [ ] 快照/恢复功能
- [ ] gRPC API 支持
- [ ] 水平自动扩缩容
- [ ] 多租户支持

### Phase 3: 企业级 (规划中)

- [ ] 网络隔离增强
- [ ] 审计日志
- [ ] RBAC 权限控制
- [ ] 监控告警完善
- [ ] 多集群部署

## 性能目标

| 指标 | 目标 | 当前状态 |
|------|------|---------|
| 沙箱启动时间 | < 200ms | ~150ms (Docker) |
| 会话恢复时间 | < 500ms | 开发中 |
| 并发沙箱数 | 1000+ | 受限于主机资源 |
| API P99 延迟 | < 100ms | ~50ms |

## 贡献指南

欢迎贡献！请查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解如何参与项目。

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 提交 Pull Request

## 参考项目

- [E2B](https://github.com/e2b-dev/E2B) - AI 代码执行沙箱
- [Firecracker](https://github.com/firecracker-microvm/firecracker) - AWS 微虚拟机
- [gVisor](https://gvisor.dev/) - 容器运行时沙箱

## License

MIT License - 详见 [LICENSE](LICENSE) 文件

---

**Cloud Sandbox** - 让 AI Agent 安全地执行代码
