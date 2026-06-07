# AI-CloudOps 开发指南

## 如何运行项目

### 环境要求
- Go 1.21+
- Node.js 21.x
- pnpm (latest)
- Docker & Docker Compose
- Python 3.11+ (用于AI模块)

### 运行步骤

#### 1. 获取代码
```bash
# 克隆后端项目
git clone https://github.com/rizxfrog/VanPanelBackend.git
cd AI-CloudOps

# 克隆前端项目
git clone https://github.com/rizxfrog/VanPanelBackend-web.git

# 克隆AI模块项目
git clone https://github.com/rizxfrog/VanPanelBackend-aiops.git
```

#### 2. 启动基础服务
```bash
cd AI-CloudOps
# 启动数据库和中间件
docker-compose -f docker-compose-env.yaml up -d

# 配置环境变量
cp env.example .env

# 检查服务状态
docker-compose -f docker-compose-env.yaml ps
```

#### 3. 启动前端服务
```bash
cd AI-CloudOps-web
# 安装依赖
pnpm install

# 启动开发服务器
pnpm run dev
```
前端将在 [http://localhost:3000](http://localhost:3000) 启动

#### 4. 启动后端服务
```bash
cd AI-CloudOps
# 安装依赖
go mod tidy

# 启动后端服务
go run main.go
```
后端服务地址：[http://localhost:8000](http://localhost:8000)

#### 5. 启动AI服务（可选）
```bash
cd AI-CloudOps-aiops
# 配置环境变量
cp env.example .env

# 安装依赖
pip install -r requirements.txt

# 训练初始模型
cd data/ && python machine-learning.py && cd ..

# 启动AI服务
python app/main.py
```
AI服务地址：[http://localhost:8001](http://localhost:8001)

## 项目使用的技术

### 后端技术
- **语言**: Go 1.21+
- **Web框架**: Gin
- **ORM**: GORM
- **数据库**: MySQL
- **缓存**: Redis
- **日志**: Zap
- **配置管理**: Viper
- **依赖注入**: Google Wire
- **验证**: Go Playground Validator v10
- **JWT认证**: golang-jwt/jwt v5
- **分布式任务**: Hibiken/Asynq
- **WebSocket**: Gorilla WebSocket
- **Prometheus客户端**: Prometheus client_golang
- **Kubernetes客户端**: client-go
- **其他**: Casbin (权限控制), Godotenv (环境变量)

### 前端技术
- **框架**: Vue 3
- **语言**: TypeScript
- **UI组件库**: Ant Design Vue
- **构建工具**: Vite (基于pnpm)
- **状态管理**: Vuex/Pinia
- **路由**: Vue Router

### AI/机器学习模块
- **语言**: Python 3.11+
- **Web框架**: FastAPI
- **机器学习库**: scikit-learn

### 基础设施与DevOps
- **容器化**: Docker & Docker Compose
- **编排**: Kubernetes
- **监控**: Prometheus + Grafana
- **CI/CD**: GitHub Actions

项目由三个主要仓库组成：
1. **AI-CloudOps** - 核心后端服务 (Go)
2. **AI-CloudOps-web** - 前端界面 (Vue 3)
3. **AI-CloudOps-aiops** - AI智能分析模块 (Python)