# Cloud Sandbox for Coding Agents - 实现指南

> 为 AI 编码 Agent 构建的云端沙箱执行环境完整实现指南

## 目录

1. [项目概述](#1-项目概述)
   - [1.1 目标](#11-目标)
   - [1.2 使用场景](#12-使用场景)
   - [1.3 使用方式](#13-使用方式)
   - [1.4 典型集成示例](#14-典型集成示例)
   - [1.5 技术栈](#15-技术栈)
2. [快速开始](#2-快速开始)
3. [架构设计](#3-架构设计)
4. [核心模块实现](#4-核心模块实现)
5. [部署指南](#5-部署指南)
6. [API 参考](#6-api-参考)
7. [安全配置](#7-安全配置)
8. [监控运维](#8-监控运维)
9. [开发路线图](#9-开发路线图)

---

## 1. 项目概述

### 1.1 目标

构建一个生产级云端沙箱系统，为 AI 编码 Agent 提供：

| 特性 | 目标指标 |
|------|----------|
| 启动时间 | < 200ms |
| 会话恢复 | < 500ms |
| 并发沙箱 | 1000+ |
| 安全隔离 | 硬件级 |
| 可用性 | 99.9% |

### 1.2 使用场景

#### 场景总览

```
┌─────────────────────────────────────────────────────────────────┐
│                     谁在使用云端沙箱？                            │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  场景 A: AI 编码助手产品 (Claude Code / Cursor / Copilot)       │
│  ┌─────────┐      ┌─────────┐      ┌─────────┐                 │
│  │  用户   │ ───→ │   LLM   │ ───→ │  沙箱   │                 │
│  │ "帮我写 │      │ 生成代码 │      │ 执行代码 │                 │
│  │ 一个爬虫"│      │         │      │ 返回结果 │                 │
│  └─────────┘      └─────────┘      └─────────┘                 │
│                                                                 │
│  场景 B: 在线 IDE / Notebook (Replit / Colab / JupyterHub)     │
│  ┌─────────┐      ┌─────────┐                                  │
│  │  用户   │ ───→ │  沙箱   │                                  │
│  │ 在浏览器 │      │ 运行代码 │                                  │
│  │ 写代码  │      │ 实时反馈 │                                  │
│  └─────────┘      └─────────┘                                  │
│                                                                 │
│  场景 C: 自主 Agent (Manus / Devin / OpenDevin)                │
│  ┌─────────┐      ┌─────────┐      ┌─────────┐                 │
│  │ AI Agent│ ───→ │  沙箱   │ ───→ │ 完成任务 │                 │
│  │ 自动规划 │      │ 执行步骤 │      │ 返回结果 │                 │
│  └─────────┘      └─────────┘      └─────────┘                 │
│                                                                 │
│  场景 D: 在线编程教育 (LeetCode / Codecademy)                   │
│  ┌─────────┐      ┌─────────┐      ┌─────────┐                 │
│  │  学生   │ ───→ │  沙箱   │ ───→ │ 判题系统 │                 │
│  │ 提交代码 │      │ 执行测试 │      │ 评分反馈 │                 │
│  └─────────┘      └─────────┘      └─────────┘                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

#### 场景 A：AI 编码助手（类似 Claude Artifacts）

```
用户: "帮我分析这个 CSV 文件，画一个销售趋势图"

┌──────────────────────────────────────────────────────────┐
│                    AI 助手处理流程                         │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  1. LLM 理解需求                                         │
│     ↓                                                    │
│  2. 生成 Python 代码:                                    │
│     ```python                                            │
│     import pandas as pd                                  │
│     import matplotlib.pyplot as plt                      │
│     df = pd.read_csv('/workspace/sales.csv')            │
│     df.plot(x='date', y='revenue')                      │
│     plt.savefig('/workspace/trend.png')                 │
│     ```                                                  │
│     ↓                                                    │
│  3. 发送到云端沙箱执行                                    │
│     ↓                                                    │
│  4. 沙箱返回: stdout + 生成的图片                         │
│     ↓                                                    │
│  5. AI 助手展示结果给用户                                 │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

#### 场景 B：自主 Agent（类似 Manus / Devin）

```
用户: "帮我创建一个 Todo 应用的后端 API"

┌──────────────────────────────────────────────────────────┐
│                    Agent 自主执行流程                     │
├──────────────────────────────────────────────────────────┤
│                                                          │
│  Agent 自动规划任务:                                      │
│  ├── Step 1: 初始化项目结构                              │
│  ├── Step 2: 创建数据模型                                │
│  ├── Step 3: 实现 API 端点                               │
│  ├── Step 4: 编写测试                                    │
│  └── Step 5: 运行测试验证                                │
│                                                          │
│  每个步骤都在沙箱中执行:                                  │
│                                                          │
│  [沙箱] mkdir -p src/routes src/models                   │
│  [沙箱] vim src/models/todo.py  # 创建文件               │
│  [沙箱] vim src/routes/api.py   # 创建文件               │
│  [沙箱] pip install fastapi uvicorn pytest               │
│  [沙箱] pytest tests/ -v        # 运行测试               │
│                                                          │
│  Agent 根据执行结果决定下一步:                            │
│  - 成功 → 继续下一步                                     │
│  - 失败 → 分析错误，修复代码，重试                        │
│                                                          │
└──────────────────────────────────────────────────────────┘
```

#### 场景 C：在线编程教育平台

```
学生在浏览器中学习编程:

┌─────────────────────────────────────────────────────────┐
│  📚 Python 入门课程 - 第3课：循环                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  练习题：打印 1-10 的平方数                              │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │ # 在这里写代码                                   │   │
│  │ for i in range(1, 11):                          │   │
│  │     print(i ** 2)                               │   │
│  │                                                  │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  [▶ 运行]  [📤 提交]                                    │
│                                                         │
│  输出:                                                  │
│  ┌─────────────────────────────────────────────────┐   │
│  │ 1                                                │   │
│  │ 4                                                │   │
│  │ 9                                                │   │
│  │ 16                                               │   │
│  │ ...                                              │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  ✅ 正确！进入下一题                                     │
│                                                         │
└─────────────────────────────────────────────────────────┘

后台: 每个学生的代码都在独立沙箱中执行，互不干扰，安全隔离
```

#### 场景 D：会话持久化 - 跨天继续工作

```
场景：用户今天开始一个数据分析项目，明天继续

Day 1 - 下午 3 点:
┌────────────────────────────────────────┐
│ 用户: "帮我分析这个销售数据集"          │
│                                        │
│ [沙箱状态]                              │
│ ├── /workspace/sales.csv (用户上传)    │
│ ├── /workspace/analysis.py (AI生成)    │
│ ├── 已安装: pandas, numpy, matplotlib  │
│ └── 内存变量: df (已加载的数据框)       │
│                                        │
│ 用户: "今天先到这里，保存进度"          │
│ → sandbox.pause()                      │
│ → 返回 session_id: sess_abc123         │
└────────────────────────────────────────┘
                    │
                    │  (过了一夜)
                    ↓
Day 2 - 上午 10 点:
┌────────────────────────────────────────┐
│ 用户: "继续昨天的数据分析"              │
│ → sandbox.resume("sess_abc123")        │
│                                        │
│ [沙箱状态 - 完全恢复，约 500ms]         │
│ ├── /workspace/sales.csv ✓             │
│ ├── /workspace/analysis.py ✓           │
│ ├── pandas, numpy, matplotlib ✓        │
│ └── df 变量 ✓ (数据已在内存中)         │
│                                        │
│ 用户: "在昨天基础上加个销售预测模型"    │
│ → 直接继续工作，无需重新配置环境        │
└────────────────────────────────────────┘
```

### 1.3 使用方式

#### 方式 A：通过 Python SDK 集成

```python
from cloud_sandbox import Sandbox

# 1. 创建新沙箱（约 150ms）
sandbox = Sandbox.create(
    spec={"cpu": 2, "memory": 2048, "image": "python:3.11"}
)

# 或恢复已有会话
sandbox = Sandbox.resume(session_id="sess_abc123")

# 2. 执行代码
result = sandbox.run_code("""
import pandas as pd
df = pd.DataFrame({'a': [1,2,3], 'b': [4,5,6]})
print(df.describe())
""")
print(result.stdout)

# 3. 文件操作
sandbox.files.write("/workspace/data.csv", csv_content)
content = sandbox.files.read("/workspace/output.txt")
files = sandbox.files.list("/workspace")

# 4. 执行 Shell 命令
sandbox.run_command("pip install scikit-learn")
sandbox.run_command("python train.py")

# 5. 暂停会话（保存完整状态）
sandbox.pause()
print(f"下次恢复用: {sandbox.session_id}")

# 6. 完全销毁（不保存）
sandbox.destroy()
```

#### 方式 B：通过 REST API 调用

```bash
# 1. 分配沙箱
curl -X POST https://api.sandbox.example.com/v1/sandbox/allocate \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "spec": {
      "cpu": 2,
      "memory": 2048,
      "image": "python:3.11"
    }
  }'

# 响应:
# {
#   "session_id": "sess_abc123",
#   "sandbox_id": "sb_xyz789",
#   "status": "running"
# }

# 2. 执行代码
curl -X POST https://api.sandbox.example.com/v1/execute \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "session_id": "sess_abc123",
    "code": "print(sum(range(100)))",
    "language": "python",
    "timeout": 30
  }'

# 响应:
# {
#   "stdout": "4950\n",
#   "stderr": "",
#   "exit_code": 0,
#   "execution_time_ms": 23
# }

# 3. 上传文件
curl -X PUT "https://api.sandbox.example.com/v1/files/sess_abc123?path=/workspace/data.csv" \
  -H "Authorization: Bearer $TOKEN" \
  --data-binary @local_data.csv

# 4. 暂停会话（保存状态，释放资源）
curl -X POST https://api.sandbox.example.com/v1/sandbox/release \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"session_id": "sess_abc123", "pause": true}'

# 5. 恢复会话
curl -X POST https://api.sandbox.example.com/v1/sandbox/allocate \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"session_id": "sess_abc123"}'  # 传入之前的 session_id 即可恢复
```

#### 方式 C：流式输出（适用于长时间任务）

```python
import httpx

# 流式执行，实时获取输出
async with httpx.AsyncClient() as client:
    async with client.stream(
        "POST",
        "https://api.sandbox.example.com/v1/execute/stream",
        json={
            "session_id": "sess_abc123",
            "code": "for i in range(10): print(i); time.sleep(1)",
            "language": "python"
        },
        headers={"Authorization": f"Bearer {token}"}
    ) as response:
        async for line in response.aiter_lines():
            # 实时打印输出
            print(line)
```

### 1.4 典型集成示例

#### 示例 1：集成到 LangChain Agent

```python
from langchain.tools import Tool
from cloud_sandbox import Sandbox

class CodeExecutionTool(Tool):
    name = "code_executor"
    description = "执行 Python 代码并返回结果"
    
    def __init__(self):
        self.sandbox = None
    
    def _run(self, code: str) -> str:
        if not self.sandbox:
            self.sandbox = Sandbox.create()
        
        result = self.sandbox.run_code(code)
        
        if result.exit_code != 0:
            return f"Error: {result.stderr}"
        return result.stdout

# 在 Agent 中使用
from langchain.agents import initialize_agent

agent = initialize_agent(
    tools=[CodeExecutionTool()],
    llm=llm,
    agent="zero-shot-react-description"
)

agent.run("计算斐波那契数列的第 20 项")
```

#### 示例 2：构建在线 IDE 后端

```python
from fastapi import FastAPI, WebSocket
from cloud_sandbox import Sandbox

app = FastAPI()

# 用户会话管理
user_sandboxes: dict[str, Sandbox] = {}

@app.websocket("/ws/{user_id}")
async def websocket_endpoint(websocket: WebSocket, user_id: str):
    await websocket.accept()
    
    # 获取或创建用户沙箱
    if user_id not in user_sandboxes:
        user_sandboxes[user_id] = Sandbox.create()
    
    sandbox = user_sandboxes[user_id]
    
    while True:
        data = await websocket.receive_json()
        
        if data["type"] == "execute":
            result = sandbox.run_code(data["code"])
            await websocket.send_json({
                "type": "output",
                "stdout": result.stdout,
                "stderr": result.stderr
            })
        
        elif data["type"] == "save":
            sandbox.pause()
            await websocket.send_json({"type": "saved"})
```

#### 示例 3：AI 课程实验环境

```python
class CourseLabEnvironment:
    """为 AI 工程课程提供标准化实验环境"""
    
    def __init__(self, student_id: str, course: str):
        self.sandbox = Sandbox.create_or_resume(
            user_id=student_id,
            template=f"course-{course}"  # 预装课程所需依赖
        )
    
    def setup_week6_crewai(self):
        """Week 6: 配置 CrewAI 多智能体环境"""
        self.sandbox.run_command("pip install crewai langchain")
        self.sandbox.files.write(
            "/workspace/crew_config.py",
            CREWAI_STARTER_TEMPLATE
        )
        return "CrewAI 环境已就绪，可以开始实验"
    
    def setup_week7_tools(self):
        """Week 7: 配置 Agent 工具集成环境"""
        self.sandbox.run_command("pip install langchain-community")
        return "工具集成环境已就绪"
    
    def submit_assignment(self, code: str) -> dict:
        """提交作业并自动评分"""
        result = self.sandbox.run_code(code)
        test_result = self.sandbox.run_command("pytest tests/ -v")
        
        return {
            "output": result.stdout,
            "tests_passed": "PASSED" in test_result.stdout,
            "score": self._calculate_score(test_result)
        }
    
    def export_workspace(self) -> bytes:
        """导出学生作业"""
        return self.sandbox.files.download_zip("/workspace")
```

### 1.5 技术栈

```
┌─────────────────────────────────────────────────────────────┐
│                     技术栈总览                               │
├─────────────────────────────────────────────────────────────┤
│ 语言     │ Go 1.22+ (核心服务), Python 3.11+ (Agent)        │
│ 虚拟化   │ Firecracker microVM / gVisor (备选)              │
│ 编排     │ Kubernetes 1.29+                                 │
│ 存储     │ PostgreSQL + Redis + MinIO + NFS                │
│ 网关     │ Kong / APISIX / Traefik                         │
│ 监控     │ Prometheus + Grafana + Loki                     │
│ CI/CD    │ GitHub Actions / GitLab CI                      │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 快速开始

### 2.1 环境要求

```bash
# 操作系统: Linux with KVM support
$ lscpu | grep Virtualization
Virtualization: VT-x

$ ls /dev/kvm
/dev/kvm

# 内存: >= 16GB, 磁盘: >= 100GB SSD
```

### 2.2 一键启动开发环境

```bash
# 克隆项目
git clone https://github.com/your-org/cloud-sandbox.git
cd cloud-sandbox

# 安装依赖
make install-deps

# 启动开发环境 (Docker Compose)
make dev-up

# 运行测试
make test

# 访问
# API: http://localhost:8080
# Grafana: http://localhost:3000
```

### 2.3 项目结构

```
cloud-sandbox/
├── cmd/                    # 服务入口
│   ├── gateway/           # API 网关
│   ├── scheduler/         # 沙箱调度器
│   ├── session-manager/   # 会话管理
│   └── sandbox-agent/     # 沙箱内代理
│
├── internal/              # 内部实现
│   ├── sandbox/          # 沙箱管理 (Firecracker)
│   ├── session/          # 会话状态
│   ├── scheduler/        # 调度逻辑
│   ├── storage/          # 存储后端
│   └── security/         # 安全模块
│
├── api/                   # API 定义
│   ├── proto/            # gRPC
│   └── openapi/          # REST
│
├── deploy/               # 部署配置
│   ├── docker/          # Docker 镜像
│   ├── kubernetes/      # K8s 配置
│   └── terraform/       # IaC
│
├── scripts/              # 工具脚本
├── images/              # 沙箱镜像
└── docs/                # 文档
```

---

## 3. 架构设计

### 3.1 系统架构图

```
                              ┌──────────────────┐
                              │   Load Balancer  │
                              └────────┬─────────┘
                                       │
                              ┌────────▼─────────┐
                              │    API Gateway   │
                              │  认证/限流/路由   │
                              └────────┬─────────┘
                                       │
           ┌───────────────────────────┼───────────────────────────┐
           │                           │                           │
   ┌───────▼───────┐          ┌───────▼───────┐          ┌───────▼───────┐
   │Session Manager│          │   Scheduler   │          │Metrics Service│
   │  会话生命周期  │          │   沙箱调度    │          │   指标采集    │
   └───────┬───────┘          └───────┬───────┘          └───────────────┘
           │                          │
           │          ┌───────────────┴───────────────┐
           │          │                               │
   ┌───────▼──────────▼───────────────────────────────▼───────┐
   │                    Sandbox Pool                          │
   │   ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐           │
   │   │  VM 1  │ │  VM 2  │ │  VM 3  │ │  VM N  │  ...      │
   │   │ (Idle) │ │(Active)│ │(Paused)│ │ (Idle) │           │
   │   └────────┘ └────────┘ └────────┘ └────────┘           │
   └──────────────────────────┬───────────────────────────────┘
                              │
   ┌──────────────────────────┴───────────────────────────────┐
   │                   Shared Storage                         │
   │  ┌──────────┐  ┌──────────┐  ┌──────────┐               │
   │  │PostgreSQL│  │  Redis   │  │  MinIO   │               │
   │  │(Sessions)│  │ (Cache)  │  │(Workspace)│              │
   │  └──────────┘  └──────────┘  └──────────┘               │
   └──────────────────────────────────────────────────────────┘
```

### 3.2 核心流程

```
用户请求 → Gateway → 认证 → 限流检查
                         ↓
              ┌──────────┴──────────┐
              │   有 session_id?    │
              └──────────┬──────────┘
                    ↓          ↓
                   Yes         No
                    ↓          ↓
            查询现有会话    创建新会话
                    ↓          ↓
              会话活跃?    从池获取沙箱
              ↓      ↓         ↓
             Yes     No    分配给会话
              ↓      ↓         ↓
         直接返回  恢复会话←───┘
                    ↓
               执行任务
                    ↓
              更新活跃时间
                    ↓
         ┌────────┴────────┐
         │   用户主动释放?  │
         └────────┬────────┘
              ↓        ↓
            Yes        No
              ↓        ↓
         保存状态   等待超时
              ↓        ↓
         释放沙箱   自动清理
```

---

## 4. 核心模块实现

### 4.1 Firecracker 沙箱管理

#### 配置文件

```yaml
# config/sandbox.yaml
sandbox:
  kernel_path: /opt/firecracker/vmlinux-5.10
  rootfs_path: /opt/firecracker/rootfs.ext4
  vcpu_count: 2
  mem_size_mb: 2048
  
pool:
  min_size: 5
  max_size: 100
  warmup_size: 10
  idle_timeout: 30m
  cleanup_interval: 5m

snapshot:
  enabled: true
  storage_path: /var/lib/sandbox/snapshots
```

#### 沙箱管理器核心代码

```go
// internal/sandbox/manager.go
package sandbox

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "time"
)

type Manager struct {
    config      Config
    sandboxDir  string
    snapshotDir string
}

type Sandbox struct {
    ID           string
    Status       Status
    IP           string
    SocketPath   string
    CreatedAt    time.Time
    LastActiveAt time.Time
    process      *exec.Cmd
    client       *http.Client
}

type Status string

const (
    StatusIdle    Status = "idle"
    StatusActive  Status = "active"
    StatusPaused  Status = "paused"
    StatusStopped Status = "stopped"
)

// Create 创建新沙箱
func (m *Manager) Create(ctx context.Context) (*Sandbox, error) {
    id := generateID()
    sandboxPath := filepath.Join(m.sandboxDir, id)
    socketPath := filepath.Join(sandboxPath, "firecracker.sock")
    
    // 1. 创建目录
    if err := os.MkdirAll(sandboxPath, 0755); err != nil {
        return nil, err
    }
    
    // 2. 复制根文件系统 (CoW)
    rootfsPath := filepath.Join(sandboxPath, "rootfs.ext4")
    if err := copyFile(m.config.RootFSPath, rootfsPath); err != nil {
        return nil, err
    }
    
    // 3. 启动 Firecracker
    cmd := exec.CommandContext(ctx, "firecracker", "--api-sock", socketPath)
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    
    // 4. 等待 socket 就绪
    if err := waitForSocket(socketPath, 5*time.Second); err != nil {
        cmd.Process.Kill()
        return nil, err
    }
    
    sandbox := &Sandbox{
        ID:           id,
        Status:       StatusIdle,
        SocketPath:   socketPath,
        CreatedAt:    time.Now(),
        LastActiveAt: time.Now(),
        process:      cmd,
        client:       createUnixClient(socketPath),
    }
    
    // 5. 配置虚拟机
    if err := m.configureVM(ctx, sandbox, rootfsPath); err != nil {
        m.Stop(ctx, sandbox)
        return nil, err
    }
    
    // 6. 启动虚拟机
    if err := m.startVM(ctx, sandbox); err != nil {
        m.Stop(ctx, sandbox)
        return nil, err
    }
    
    return sandbox, nil
}

// configureVM 配置虚拟机
func (m *Manager) configureVM(ctx context.Context, sb *Sandbox, rootfsPath string) error {
    // 配置内核
    if err := m.apiCall(sb, "PUT", "/boot-source", map[string]interface{}{
        "kernel_image_path": m.config.KernelPath,
        "boot_args":         "console=ttyS0 reboot=k panic=1 pci=off",
    }); err != nil {
        return err
    }
    
    // 配置磁盘
    if err := m.apiCall(sb, "PUT", "/drives/rootfs", map[string]interface{}{
        "drive_id":       "rootfs",
        "path_on_host":   rootfsPath,
        "is_root_device": true,
        "is_read_only":   false,
    }); err != nil {
        return err
    }
    
    // 配置资源
    if err := m.apiCall(sb, "PUT", "/machine-config", map[string]interface{}{
        "vcpu_count":   m.config.VCPUCount,
        "mem_size_mib": m.config.MemSizeMB,
    }); err != nil {
        return err
    }
    
    return nil
}

// startVM 启动虚拟机
func (m *Manager) startVM(ctx context.Context, sb *Sandbox) error {
    return m.apiCall(sb, "PUT", "/actions", map[string]interface{}{
        "action_type": "InstanceStart",
    })
}

// Pause 暂停沙箱
func (m *Manager) Pause(ctx context.Context, sb *Sandbox) error {
    if err := m.apiCall(sb, "PATCH", "/vm", map[string]interface{}{
        "state": "Paused",
    }); err != nil {
        return err
    }
    sb.Status = StatusPaused
    return nil
}

// Resume 恢复沙箱
func (m *Manager) Resume(ctx context.Context, sb *Sandbox) error {
    if err := m.apiCall(sb, "PATCH", "/vm", map[string]interface{}{
        "state": "Resumed",
    }); err != nil {
        return err
    }
    sb.Status = StatusActive
    sb.LastActiveAt = time.Now()
    return nil
}

// CreateSnapshot 创建快照
func (m *Manager) CreateSnapshot(ctx context.Context, sb *Sandbox, snapshotID string) error {
    // 暂停
    if err := m.Pause(ctx, sb); err != nil {
        return err
    }
    defer m.Resume(ctx, sb)
    
    snapshotPath := filepath.Join(m.snapshotDir, sb.ID, snapshotID)
    os.MkdirAll(snapshotPath, 0755)
    
    return m.apiCall(sb, "PUT", "/snapshot/create", map[string]interface{}{
        "snapshot_path": filepath.Join(snapshotPath, "state"),
        "mem_file_path": filepath.Join(snapshotPath, "memory"),
        "snapshot_type": "Full",
    })
}

// RestoreFromSnapshot 从快照恢复
func (m *Manager) RestoreFromSnapshot(ctx context.Context, sandboxID, snapshotID string) (*Sandbox, error) {
    snapshotPath := filepath.Join(m.snapshotDir, sandboxID, snapshotID)
    
    // 创建新沙箱实例
    sb, err := m.createEmptySandbox(ctx)
    if err != nil {
        return nil, err
    }
    
    // 加载快照
    if err := m.apiCall(sb, "PUT", "/snapshot/load", map[string]interface{}{
        "snapshot_path": filepath.Join(snapshotPath, "state"),
        "mem_file_path": filepath.Join(snapshotPath, "memory"),
        "resume_vm":     true,
    }); err != nil {
        m.Stop(ctx, sb)
        return nil, err
    }
    
    sb.Status = StatusActive
    return sb, nil
}

// Stop 停止沙箱
func (m *Manager) Stop(ctx context.Context, sb *Sandbox) error {
    if sb.process != nil {
        sb.process.Process.Kill()
    }
    os.RemoveAll(filepath.Join(m.sandboxDir, sb.ID))
    sb.Status = StatusStopped
    return nil
}

// 辅助函数
func (m *Manager) apiCall(sb *Sandbox, method, path string, body interface{}) error {
    // 实现 HTTP 调用到 Firecracker API
    return nil
}

func createUnixClient(socketPath string) *http.Client {
    return &http.Client{
        Transport: &http.Transport{
            DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
                return net.Dial("unix", socketPath)
            },
        },
        Timeout: 10 * time.Second,
    }
}

func waitForSocket(path string, timeout time.Duration) error {
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if conn, err := net.Dial("unix", path); err == nil {
            conn.Close()
            return nil
        }
        time.Sleep(10 * time.Millisecond)
    }
    return fmt.Errorf("timeout waiting for socket")
}
```

### 4.2 沙箱池

```go
// internal/sandbox/pool.go
package sandbox

import (
    "context"
    "sync"
    "time"
)

type Pool struct {
    config  PoolConfig
    manager *Manager
    
    mu     sync.RWMutex
    idle   []*Sandbox
    active map[string]*Sandbox
    
    stopCh chan struct{}
}

type PoolConfig struct {
    MinSize         int
    MaxSize         int
    WarmupSize      int
    IdleTimeout     time.Duration
    CleanupInterval time.Duration
}

func NewPool(config PoolConfig, manager *Manager) *Pool {
    p := &Pool{
        config:  config,
        manager: manager,
        idle:    make([]*Sandbox, 0),
        active:  make(map[string]*Sandbox),
        stopCh:  make(chan struct{}),
    }
    
    go p.warmupLoop()
    go p.cleanupLoop()
    
    return p
}

// Acquire 获取沙箱
func (p *Pool) Acquire(ctx context.Context) (*Sandbox, error) {
    p.mu.Lock()
    
    // 从空闲池获取
    if len(p.idle) > 0 {
        sb := p.idle[len(p.idle)-1]
        p.idle = p.idle[:len(p.idle)-1]
        p.active[sb.ID] = sb
        p.mu.Unlock()
        
        sb.Status = StatusActive
        sb.LastActiveAt = time.Now()
        return sb, nil
    }
    
    // 检查是否可创建
    if len(p.active) >= p.config.MaxSize {
        p.mu.Unlock()
        return nil, ErrPoolExhausted
    }
    p.mu.Unlock()
    
    // 创建新沙箱
    sb, err := p.manager.Create(ctx)
    if err != nil {
        return nil, err
    }
    
    p.mu.Lock()
    p.active[sb.ID] = sb
    p.mu.Unlock()
    
    sb.Status = StatusActive
    return sb, nil
}

// Release 释放沙箱
func (p *Pool) Release(ctx context.Context, id string) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    sb, ok := p.active[id]
    if !ok {
        return ErrNotFound
    }
    
    delete(p.active, id)
    
    // 重置并放回池
    if len(p.idle) < p.config.MaxSize {
        sb.Status = StatusIdle
        p.idle = append(p.idle, sb)
    } else {
        p.manager.Stop(ctx, sb)
    }
    
    return nil
}

// warmupLoop 预热
func (p *Pool) warmupLoop() {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-p.stopCh:
            return
        case <-ticker.C:
            p.mu.RLock()
            needed := p.config.WarmupSize - len(p.idle)
            p.mu.RUnlock()
            
            if needed > 0 {
                ctx := context.Background()
                for i := 0; i < needed; i++ {
                    if sb, err := p.manager.Create(ctx); err == nil {
                        p.mu.Lock()
                        p.idle = append(p.idle, sb)
                        p.mu.Unlock()
                    }
                }
            }
        }
    }
}

// cleanupLoop 清理
func (p *Pool) cleanupLoop() {
    ticker := time.NewTicker(p.config.CleanupInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-p.stopCh:
            return
        case <-ticker.C:
            p.cleanup()
        }
    }
}

func (p *Pool) cleanup() {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    ctx := context.Background()
    now := time.Now()
    
    // 清理超时活跃沙箱
    for id, sb := range p.active {
        if now.Sub(sb.LastActiveAt) > p.config.IdleTimeout {
            p.manager.Stop(ctx, sb)
            delete(p.active, id)
        }
    }
    
    // 缩减空闲池
    for len(p.idle) > p.config.MinSize {
        sb := p.idle[len(p.idle)-1]
        p.idle = p.idle[:len(p.idle)-1]
        p.manager.Stop(ctx, sb)
    }
}

// Stats 统计信息
func (p *Pool) Stats() map[string]int {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    return map[string]int{
        "idle":   len(p.idle),
        "active": len(p.active),
        "max":    p.config.MaxSize,
    }
}
```

### 4.3 会话管理

```go
// internal/session/manager.go
package session

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/redis/go-redis/v9"
    "gorm.io/gorm"
)

type Session struct {
    ID           string    `gorm:"primaryKey"`
    UserID       string    `gorm:"index"`
    SandboxID    string
    Status       Status
    WorkspaceURL string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    ExpiresAt    time.Time
}

type Status string

const (
    StatusActive  Status = "active"
    StatusPaused  Status = "paused"
    StatusExpired Status = "expired"
)

type Manager struct {
    db      *gorm.DB
    redis   *redis.Client
    storage StorageBackend
    ttl     time.Duration
}

type StorageBackend interface {
    SaveWorkspace(ctx context.Context, sessionID, sandboxID string) error
    RestoreWorkspace(ctx context.Context, sessionID, sandboxID string) error
    DeleteWorkspace(ctx context.Context, sessionID string) error
}

func NewManager(db *gorm.DB, redis *redis.Client, storage StorageBackend) *Manager {
    db.AutoMigrate(&Session{})
    return &Manager{
        db:      db,
        redis:   redis,
        storage: storage,
        ttl:     24 * time.Hour,
    }
}

// Create 创建会话
func (m *Manager) Create(ctx context.Context, userID, sandboxID string) (*Session, error) {
    session := &Session{
        ID:        generateID(),
        UserID:    userID,
        SandboxID: sandboxID,
        Status:    StatusActive,
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
        ExpiresAt: time.Now().Add(m.ttl),
    }
    
    if err := m.db.Create(session).Error; err != nil {
        return nil, err
    }
    
    m.cache(ctx, session)
    return session, nil
}

// Get 获取会话
func (m *Manager) Get(ctx context.Context, id string) (*Session, error) {
    // 从缓存获取
    if session := m.getFromCache(ctx, id); session != nil {
        return session, nil
    }
    
    // 从数据库获取
    session := &Session{}
    if err := m.db.First(session, "id = ?", id).Error; err != nil {
        return nil, err
    }
    
    if time.Now().After(session.ExpiresAt) {
        return nil, ErrExpired
    }
    
    m.cache(ctx, session)
    return session, nil
}

// Pause 暂停会话
func (m *Manager) Pause(ctx context.Context, id string) error {
    session, err := m.Get(ctx, id)
    if err != nil {
        return err
    }
    
    // 保存工作区
    if err := m.storage.SaveWorkspace(ctx, id, session.SandboxID); err != nil {
        return err
    }
    
    session.Status = StatusPaused
    session.SandboxID = ""
    session.UpdatedAt = time.Now()
    
    if err := m.db.Save(session).Error; err != nil {
        return err
    }
    
    m.cache(ctx, session)
    return nil
}

// Resume 恢复会话
func (m *Manager) Resume(ctx context.Context, id, sandboxID string) error {
    session, err := m.Get(ctx, id)
    if err != nil {
        return err
    }
    
    // 恢复工作区
    if err := m.storage.RestoreWorkspace(ctx, id, sandboxID); err != nil {
        return err
    }
    
    session.Status = StatusActive
    session.SandboxID = sandboxID
    session.UpdatedAt = time.Now()
    session.ExpiresAt = time.Now().Add(m.ttl)
    
    if err := m.db.Save(session).Error; err != nil {
        return err
    }
    
    m.cache(ctx, session)
    return nil
}

// cache 缓存会话
func (m *Manager) cache(ctx context.Context, session *Session) {
    data, _ := json.Marshal(session)
    m.redis.Set(ctx, "session:"+session.ID, data, m.ttl)
}

// getFromCache 从缓存获取
func (m *Manager) getFromCache(ctx context.Context, id string) *Session {
    data, err := m.redis.Get(ctx, "session:"+id).Bytes()
    if err != nil {
        return nil
    }
    
    session := &Session{}
    if err := json.Unmarshal(data, session); err != nil {
        return nil
    }
    
    return session
}
```

### 4.4 存储后端

```go
// internal/storage/minio.go
package storage

import (
    "archive/tar"
    "compress/gzip"
    "context"
    "io"
    "os"
    "path/filepath"
    
    "github.com/minio/minio-go/v7"
)

type MinIOStorage struct {
    client *minio.Client
    bucket string
}

// SaveWorkspace 保存工作区
func (s *MinIOStorage) SaveWorkspace(ctx context.Context, sessionID, sandboxID string) error {
    workspacePath := fmt.Sprintf("/var/lib/sandbox/%s/workspace", sandboxID)
    
    // 压缩
    tmpFile, _ := os.CreateTemp("", "workspace-*.tar.gz")
    defer os.Remove(tmpFile.Name())
    
    if err := s.compress(workspacePath, tmpFile); err != nil {
        return err
    }
    
    // 上传
    tmpFile.Seek(0, 0)
    _, err := s.client.PutObject(ctx, s.bucket,
        fmt.Sprintf("sessions/%s/workspace.tar.gz", sessionID),
        tmpFile, -1,
        minio.PutObjectOptions{ContentType: "application/gzip"},
    )
    
    return err
}

// RestoreWorkspace 恢复工作区
func (s *MinIOStorage) RestoreWorkspace(ctx context.Context, sessionID, sandboxID string) error {
    object, err := s.client.GetObject(ctx, s.bucket,
        fmt.Sprintf("sessions/%s/workspace.tar.gz", sessionID),
        minio.GetObjectOptions{},
    )
    if err != nil {
        return err
    }
    defer object.Close()
    
    workspacePath := fmt.Sprintf("/var/lib/sandbox/%s/workspace", sandboxID)
    os.RemoveAll(workspacePath)
    os.MkdirAll(workspacePath, 0755)
    
    return s.decompress(object, workspacePath)
}

func (s *MinIOStorage) compress(srcDir string, dst io.Writer) error {
    gw := gzip.NewWriter(dst)
    defer gw.Close()
    tw := tar.NewWriter(gw)
    defer tw.Close()
    
    return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        
        header, _ := tar.FileInfoHeader(info, "")
        header.Name, _ = filepath.Rel(srcDir, path)
        
        tw.WriteHeader(header)
        
        if !info.IsDir() {
            file, _ := os.Open(path)
            defer file.Close()
            io.Copy(tw, file)
        }
        
        return nil
    })
}

func (s *MinIOStorage) decompress(src io.Reader, dstDir string) error {
    gr, _ := gzip.NewReader(src)
    defer gr.Close()
    tr := tar.NewReader(gr)
    
    for {
        header, err := tr.Next()
        if err == io.EOF {
            break
        }
        
        targetPath := filepath.Join(dstDir, header.Name)
        
        if header.Typeflag == tar.TypeDir {
            os.MkdirAll(targetPath, 0755)
        } else {
            os.MkdirAll(filepath.Dir(targetPath), 0755)
            file, _ := os.Create(targetPath)
            io.Copy(file, tr)
            file.Close()
        }
    }
    
    return nil
}
```

---

## 5. 部署指南

### 5.1 Docker Compose（开发环境）

```yaml
# docker-compose.yml
version: '3.8'

services:
  gateway:
    build: ./cmd/gateway
    ports:
      - "8080:8080"
    depends_on:
      - scheduler
      - session-manager
    environment:
      - SCHEDULER_URL=scheduler:9090
      - SESSION_URL=session-manager:9090

  scheduler:
    build: ./cmd/scheduler
    ports:
      - "9090:9090"
    depends_on:
      - redis
    volumes:
      - /dev/kvm:/dev/kvm
      - sandbox-data:/var/lib/sandbox
    privileged: true
    environment:
      - REDIS_URL=redis://redis:6379

  session-manager:
    build: ./cmd/session-manager
    ports:
      - "9091:9090"
    depends_on:
      - postgres
      - redis
      - minio
    environment:
      - DATABASE_URL=postgres://postgres:postgres@postgres:5432/sandbox
      - REDIS_URL=redis://redis:6379
      - MINIO_ENDPOINT=minio:9000

  postgres:
    image: postgres:16
    environment:
      - POSTGRES_DB=sandbox
      - POSTGRES_PASSWORD=postgres
    volumes:
      - postgres-data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    volumes:
      - redis-data:/data

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      - MINIO_ROOT_USER=minioadmin
      - MINIO_ROOT_PASSWORD=minioadmin
    volumes:
      - minio-data:/data

  prometheus:
    image: prom/prometheus
    ports:
      - "9092:9090"
    volumes:
      - ./deploy/prometheus:/etc/prometheus

  grafana:
    image: grafana/grafana
    ports:
      - "3000:3000"
    depends_on:
      - prometheus

volumes:
  sandbox-data:
  postgres-data:
  redis-data:
  minio-data:
```

### 5.2 Kubernetes（生产环境）

```yaml
# deploy/kubernetes/scheduler.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sandbox-scheduler
spec:
  replicas: 3
  selector:
    matchLabels:
      app: sandbox-scheduler
  template:
    spec:
      containers:
        - name: scheduler
          image: cloud-sandbox/scheduler:latest
          ports:
            - containerPort: 8080
            - containerPort: 9090
          resources:
            requests:
              cpu: "500m"
              memory: "512Mi"
            limits:
              cpu: "2"
              memory: "2Gi"
          envFrom:
            - configMapRef:
                name: sandbox-config
            - secretRef:
                name: sandbox-secrets
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: scheduler-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: sandbox-scheduler
  minReplicas: 3
  maxReplicas: 20
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

---

## 6. API 参考

### 6.1 REST API

```
POST /api/v1/sandbox/allocate
  请求: { "session_id": "可选", "spec": { "cpu": 2, "memory": 2048 } }
  响应: { "session_id": "xxx", "sandbox_id": "xxx", "sandbox_ip": "xxx" }

POST /api/v1/sandbox/release
  请求: { "session_id": "xxx", "pause": true }
  响应: { "success": true }

GET /api/v1/sandbox/{session_id}/status
  响应: { "status": "active", "sandbox_id": "xxx", "expires_at": "xxx" }

POST /api/v1/execute
  请求: { "session_id": "xxx", "code": "print('hello')", "language": "python" }
  响应: { "stdout": "hello\n", "stderr": "", "exit_code": 0 }

PUT /api/v1/files/{session_id}?path=/workspace/test.py
  请求: <file content>
  响应: { "success": true }

GET /api/v1/files/{session_id}?path=/workspace
  响应: { "files": [{ "name": "test.py", "size": 100, "is_dir": false }] }
```

---

## 7. 安全配置

### 7.1 网络隔离脚本

```bash
#!/bin/bash
# scripts/setup-network.sh

SANDBOX_ID=$1
TAP="tap_${SANDBOX_ID}"
BRIDGE="sandbox-br0"

# 创建 bridge
ip link add $BRIDGE type bridge 2>/dev/null || true
ip addr add 172.16.0.1/16 dev $BRIDGE 2>/dev/null || true
ip link set $BRIDGE up

# 创建 TAP
ip tuntap add dev $TAP mode tap
ip link set $TAP master $BRIDGE
ip link set $TAP up

# 防火墙规则
iptables -A FORWARD -i $TAP -o eth0 -j DROP
iptables -A FORWARD -i $TAP -o eth0 -p tcp --dport 443 -j ACCEPT
iptables -A FORWARD -i $TAP -o eth0 -p tcp --dport 80 -j ACCEPT
iptables -A FORWARD -i $TAP -o eth0 -p udp --dport 53 -j ACCEPT

# NAT
iptables -t nat -A POSTROUTING -s 172.16.0.0/16 -o eth0 -j MASQUERADE
```

### 7.2 命令白名单

```go
var allowedCommands = map[string]bool{
    "ls": true, "cat": true, "head": true, "tail": true,
    "grep": true, "find": true, "wc": true,
    "python": true, "python3": true, "pip": true,
    "node": true, "npm": true, "npx": true,
    "git": true, "curl": true, "wget": true,
}

var blockedPatterns = []string{
    `rm\s+-rf\s+/`,
    `sudo`, `su\b`,
    `/etc/passwd`, `/etc/shadow`,
    `iptables`, `netcat`,
}
```

---

## 8. 监控运维

### 8.1 关键指标

```
# 沙箱
sandbox_pool_size{status="idle|active"}
sandbox_startup_duration_seconds
sandbox_restore_duration_seconds

# 会话
session_total{status="active|paused|expired"}
session_lifetime_seconds

# API
api_requests_total{method,path,status}
api_request_duration_seconds

# 资源
sandbox_cpu_usage_percent
sandbox_memory_usage_bytes
```

### 8.2 告警规则

```yaml
groups:
  - name: sandbox
    rules:
      - alert: PoolExhausted
        expr: sandbox_pool_size{status="idle"} == 0
        for: 5m
        
      - alert: HighStartupLatency
        expr: histogram_quantile(0.99, sandbox_startup_duration_seconds) > 1
        for: 5m
        
      - alert: SessionErrorRate
        expr: rate(session_errors_total[5m]) > 0.01
        for: 5m
```

---

## 9. 开发路线图

### Phase 1: MVP (4-6周)
- [ ] Docker 容器沙箱
- [ ] 基础会话管理
- [ ] 代码执行 API
- [ ] JWT 认证

### Phase 2: 生产就绪 (6-8周)
- [ ] Firecracker 集成
- [ ] 快照/恢复
- [ ] 沙箱池
- [ ] K8s 部署

### Phase 3: 企业级 (8-12周)
- [ ] 网络隔离
- [ ] 审计日志
- [ ] 多租户
- [ ] 监控告警

---

## 参考资源

- [Firecracker](https://github.com/firecracker-microvm/firecracker)
- [E2B](https://github.com/e2b-dev/E2B)
- [Coder](https://github.com/coder/coder)
- [gVisor](https://gvisor.dev/)

---

## License

MIT License
