# VanPanel Lit WebUI Refactor Design

## Goal

Replace the existing React frontend (VanPanelWebUI, separate repository) with a new Lit-based SPA embedded directly in VanPanelBackend. The new UI takes design inspiration from OpenClaw's Control UI (Lit + Vite + dark theme) but is tailor-built for the operations management domain, communicating with the existing Go backend via REST + SSE — no OpenClaw Gateway protocol adaptation needed.

## Architecture

```text
Go Binary (VanPanelBackend)
  ├── /api/*  ──→ Gin HTTP Server (existing REST + SSE APIs)
  └── /*      ──→ embed.FS static files (Lit SPA, SPA fallback)

Lit SPA (webui/)
  ├── /login     ──→ Login page → JWT token
  ├── /chat      ──→ Agent chat (SSE streaming + Tool Cards)
  ├── /k8s       ──→ K8s cluster management
  ├── /prometheus──→ Prometheus alerts/rules
  ├── /workorder ──→ ITIL ticketing
  ├── /tree      ──→ CMDB service tree
  ├── /files     ──→ File manager
  └── /system    ──→ Users, RBAC, audit logs, config
```

## Technology Stack

| Layer | Choice | Rationale |
|-------|--------|-----------|
| Framework | Lit 3.x (Web Components) | Same as OpenClaw, lightweight, standards-based |
| Build | Vite 8 | Same as OpenClaw, fast dev + production chunking |
| Markdown | `@create-markdown/preview` + `marked` + `highlight.js` | Same as OpenClaw, code highlighting + tables |
| XSS | DOMPurify | Same as OpenClaw, sanitizes rendered HTML |
| Theme | CSS custom properties (dark/light/system) | Inspired by OpenClaw's three-theme system |
| i18n | Custom Lit ReactiveController (zh/en) | Simplified version, no 19-language overhead |
| State | Lit `@state()` decorators (no external library) | Same pattern as OpenClaw's single-App model |
| API | REST + SSE (no WebSocket) | Zero backend protocol changes |
| Auth | Existing JWT (login → token → Authorization header) | Zero backend auth changes |
| Bundle | Go `embed.FS` | Single binary deployment, no Node.js runtime |

## Hard Boundaries

| Boundary | Decision | Why |
|----------|----------|-----|
| Route prefix | UI at `/ui/*`, APIs at `/api/*` | Physical isolation, no route conflicts |
| Auth | Reuse existing JWT, zero changes | Frontend just needs `fetch` + `Authorization: Bearer <token>` |
| Business APIs | K8s/Prometheus/Workorder/Tree/Files APIs **untouched** | Frontend adapts to existing response shapes |
| OpenClaw protocol | Not implemented | ~180 RPC methods over WebSocket — too large, too domain-mismatched |
| Channels/Voice/Dreaming | Excluded | Out of scope for ops management panel |

## From OpenClaw (Design Inspiration Only)

| What we borrow | Source file (reference) | How we use it |
|----------------|------------------------|---------------|
| Lit `@state()` pattern | `openclaw/ui/src/ui/app.ts` | Reference pattern, write own AppRoot |
| CSS dark theme variables | `openclaw/ui/src/styles/base.css` | Extract CSS custom property definitions |
| Markdown rendering | `openclaw/ui/src/ui/markdown.ts` | Copy core pipeline, adapt to our API |
| Tool Card UI | `openclaw/ui/src/ui/chat/tool-cards.ts` | Reference UI, rewrite data binding |
| SSE delta merging | `openclaw/ui/src/ui/chat/stream-text.ts` | Reference delta accumulate logic |
| Sidebar navigation | `openclaw/ui/src/ui/navigation.ts` | Reference structure, replace with ops menus |
| Theme toggle | `openclaw/ui/src/ui/theme.ts` | Reference dark/light/system switching |
| DOMPurify sanitization | `openclaw/ui/src/ui/markdown.ts` | Reuse sanitization pattern |

## Request Flow (Agent Chat)

```text
User types message in Lit UI
  → fetch POST /api/system/agent/query/stream (SSE)
  → SSE: event "delta"     → [render typing text]
  → SSE: event "tool_call" → [create Tool Card ⏳]
  → SSE: event "tool_result" → [update Tool Card ✅/❌]
  → SSE: event "delta"     → [render text after tool]
  → SSE: event "done"      → [stop spinner, final answer]
```

## First-Version Scope

In scope:
- Agent chat page with SSE streaming and Tool Cards (like OpenClaw)
- Sidebar navigation with ops domain tabs
- Dark/light theme with CSS custom properties
- i18n (Chinese/English)
- Markdown rendering with code highlighting
- Session management (create/switch/delete)
- Tool list display with enable/disable
- K8s cluster overview (read-only tables)
- File manager browse/upload
- System user/RBAC management

Out of scope for first version:
- K8s YAML editor with diff/apply
- Prometheus charts (iframe embed acceptable)
- Workorder full workflow
- CMDB tree drag-and-drop
- Voice/call integration
- Real-time WebSocket push
- Mobile responsiveness optimization
- E2E Playwright tests
