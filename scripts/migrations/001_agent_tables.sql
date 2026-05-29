-- =============================================================================
-- Agent Module Tables Migration
-- Database: PostgreSQL
-- Description: Creates tables for the AI Agent module including sessions,
--              messages, built-in tools, MCP plugins, remote MCP configs,
--              and audit events.
-- =============================================================================

-- 1. cl_agent_sessions - Agent conversation sessions
CREATE TABLE IF NOT EXISTS cl_agent_sessions (
    id            SERIAL PRIMARY KEY,
    user_id       INTEGER NOT NULL,
    title         VARCHAR(200),
    model         VARCHAR(100),
    tool_count    INTEGER DEFAULT 0,
    message_count INTEGER DEFAULT 0,
    status        SMALLINT DEFAULT 1,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW(),
    deleted_at    BIGINT DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_user ON cl_agent_sessions(user_id, created_at DESC);

-- 2. cl_agent_messages - Messages within agent sessions
CREATE TABLE IF NOT EXISTS cl_agent_messages (
    id           BIGSERIAL PRIMARY KEY,
    session_id   VARCHAR(50) NOT NULL,
    role         VARCHAR(20) NOT NULL,
    content      TEXT NOT NULL,
    tool_calls   JSONB,
    tool_call_id VARCHAR(100),
    metadata     JSONB,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_agent_messages_session ON cl_agent_messages(session_id, created_at);

-- 3. cl_agent_builtin_tools - Built-in tools registry
CREATE TABLE IF NOT EXISTS cl_agent_builtin_tools (
    name         VARCHAR(100) PRIMARY KEY,
    display_name VARCHAR(200) NOT NULL,
    description  TEXT,
    category     VARCHAR(50),
    enabled      BOOLEAN DEFAULT TRUE,
    config       JSONB,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW()
);

-- Seed data for builtin tools (18 tools)
INSERT INTO cl_agent_builtin_tools (name, display_name, description, category) VALUES
('net.lsof',       'lsof',       '列出打开的文件和网络连接', 'network'),
('net.ss',         'ss',         '查看套接字统计', 'network'),
('net.netstat',    'netstat',    '网络连接、路由表、接口统计', 'network'),
('log.journalctl', 'journalctl', '查询 systemd 日志', 'log'),
('log.dmesg',      'dmesg',      '内核环形缓冲区日志', 'log'),
('log.tail',       'tail',       '追踪日志文件末尾', 'log'),
('proc.ps',        'ps',         '进程列表和详细信息', 'process'),
('proc.top',       'top',        '实时进程资源占用快照', 'process'),
('proc.pgrep',     'pgrep',      '按名称查找进程', 'process'),
('disk.df',        'df',         '文件系统磁盘空间', 'disk'),
('disk.du',        'du',         '目录磁盘占用', 'disk'),
('disk.iostat',    'iostat',     '磁盘 I/O 统计', 'disk'),
('sys.free',       'free',       '内存和交换空间', 'system'),
('sys.vmstat',     'vmstat',     '虚拟内存/CPU/IO 统计', 'system'),
('sys.uname',      'uname',      '系统信息', 'system'),
('svc.systemctl',  'systemctl',  '服务管理', 'service'),
('svc.uptime',     'uptime',     '运行时间和负载', 'service'),
('shell.exec',     'Shell',      '执行 Shell 命令', 'shell')
ON CONFLICT (name) DO NOTHING;

-- 4. cl_agent_mcp_plugins - MCP plugin marketplace
CREATE TABLE IF NOT EXISTS cl_agent_mcp_plugins (
    id           SERIAL PRIMARY KEY,
    name         VARCHAR(100) UNIQUE NOT NULL,
    display_name VARCHAR(200) NOT NULL,
    description  TEXT,
    version      VARCHAR(50) NOT NULL,
    author       VARCHAR(100),
    category     VARCHAR(50),
    tags         JSONB,
    icon_url     VARCHAR(500),
    homepage     VARCHAR(500),
    manifest     JSONB NOT NULL,
    binary_path  VARCHAR(500),
    binary_hash  VARCHAR(64),
    downloads    INTEGER DEFAULT 0,
    status       VARCHAR(20) DEFAULT 'active',
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW(),
    deleted_at   BIGINT DEFAULT 0
);

-- 5. cl_agent_mcp_plugin_installs - User plugin installations
CREATE TABLE IF NOT EXISTS cl_agent_mcp_plugin_installs (
    id         SERIAL PRIMARY KEY,
    plugin_id  INTEGER NOT NULL,
    user_id    INTEGER NOT NULL,
    config     JSONB,
    enabled    BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at BIGINT DEFAULT 0
);

-- 6. cl_agent_remote_mcp_configs - Remote MCP server configurations
CREATE TABLE IF NOT EXISTS cl_agent_remote_mcp_configs (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    transport   VARCHAR(20) NOT NULL,
    url         VARCHAR(500) NOT NULL,
    auth_type   VARCHAR(20) DEFAULT 'none',
    auth_config JSONB,
    enabled     BOOLEAN DEFAULT TRUE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW(),
    deleted_at  BIGINT DEFAULT 0
);

-- 7. cl_agent_audit_events - Tool execution audit trail
CREATE TABLE IF NOT EXISTS cl_agent_audit_events (
    id         BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(50) NOT NULL,
    user_id    INTEGER NOT NULL,
    tool_name  VARCHAR(100),
    tool_args  TEXT,
    risk_level VARCHAR(20),
    action     VARCHAR(20),
    result     TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_agent_audit_session ON cl_agent_audit_events(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_agent_audit_user ON cl_agent_audit_events(user_id, created_at);
