# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Backend (Go)
- **Start development server**: `go run main.go`
- **Install/update dependencies**: `go mod tidy`
- **Run tests**: `go test ./...`
- **Run single test**: `go test -run TestFunctionName ./...`
- **Build binary**: `go build -o ai-cloudops main.go`
- **Check for lint issues**: `golangci-lint run` (if installed)

### Frontend (Vue 3)
- **Start development server**: `pnpm run dev` (in AI-CloudOps-web directory)
- **Install dependencies**: `pnpm install`
- **Build for production**: `pnpm run build`
- **Run tests**: `pnpm run test`

### AI Module (Python)
- **Install dependencies**: `pip install -r requirements.txt`
- **Start AI service**: `python app/main.py`
- **Train initial model**: `cd data/ && python machine-learning.py && cd ..`

### Infrastructure
- **Start development services** (database, middleware): `docker-compose -f docker-compose-env.yaml up -d`
- **Start all services** (production): `docker-compose up -d`
- **Check service status**: `docker-compose ps`
- **View logs**: `docker-compose logs -f`
- **Deploy to Kubernetes**: `kubectl apply -f deploy/kubernetes/`

## Code Architecture

### Backend Structure
```
AI-CloudOps/
├── cmd/                    # Command-line tools and application entry points
├── config/                 # Configuration files (development, production)
├── internal/               # Core business logic (private to this application)
│   ├── middleware/         # HTTP middleware (authentication, logging, etc.)
│   ├── model/              # Data models and database schemas
│   ├── k8s/                # Kubernetes management functionality
│   ├── user/               # User authentication and authorization
│   ├── prometheus/         # Monitoring and metrics collection
│   ├── workorder/          # Work order/ticketing system
│   ├── tree/               # Service tree/CMDB functionality
│   └── system/             # System configuration and management
├── pkg/                    # Public packages that can be imported by other projects
├── docs/                   # API documentation
└── deploy/                 # Deployment configurations (Kubernetes, Docker, etc.)
```

### Key Technical Stack
- **Backend**: Go 1.24+ with Gin web framework, GORM ORM, Redis, MySQL
- **Frontend**: Vue 3 + TypeScript + Ant Design Vue (separate repository: AI-CloudOps-web)
- **AI Module**: Python + FastAPI + scikit-learn (separate repository: AI-CloudOps-aiops)
- **Infrastructure**: Kubernetes, Prometheus, Grafana, Docker Compose

### Development Workflow
1. Clone the three repositories: AI-CloudOps (backend), AI-CloudOps-web (frontend), AI-CloudOps-aiops (AI module)
2. Start infrastructure services using docker-compose
3. Configure environment variables by copying env.example to .env
4. Start each service in its respective directory:
   - Backend: `go run main.go`
   - Frontend: `pnpm run dev`
   - AI Module: `python app/main.py` (after training initial model)

### Important Notes
- The backend runs on port 8000 by default
- The frontend runs on port 3000 by default
- The AI module runs on port 8001 by default
- API documentation is available in the docs/ directory
- Follow Conventional Commits for commit messages: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `ci`