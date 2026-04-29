# VanPanel System Terminal Design

## Goal

Add a 1Panel-style terminal feature to VanPanel with a unified system module in the WebUI. The first implementation supports both the local Linux shell on the VanPanelBackend host and SSH terminals for existing managed hosts.

The feature must use one frontend terminal experience and one structured WebSocket protocol across local and SSH targets. It must not introduce privilege escalation in the first version.

## Scope

In scope:

- Move the file manager route under `/system/files`.
- Move existing file manager WebUI code from `apps/web-antd/src/views/files` to `apps/web-antd/src/views/system`.
- Add a new terminal page at `/system/terminal`.
- Add a backend terminal module for local shell and SSH terminal sessions.
- Use a unified JSON WebSocket protocol for input, output, resize, heartbeat, errors, and close events.
- Record terminal session audit metadata only.

Out of scope for the first version:

- Recording command input or terminal output.
- Running shells as arbitrary selected system users.
- sudo, su, or any built-in privilege escalation flow.
- Split panes.
- Terminal session replay.
- Opening a terminal from the current file manager directory.
- Interactive PowerShell support on Windows.

## Frontend Routing And Structure

The WebUI will expose a top-level system route:

```text
/system
```

Child pages:

```text
/system/files      File manager
/system/terminal   Terminal manager
```

The frontend code will be organized under:

```text
apps/web-antd/src/views/system/FileManager.vue
apps/web-antd/src/views/system/TerminalManager.vue
apps/web-antd/src/views/system/file-manager.css
apps/web-antd/src/views/system/file-manager-utils.ts
```

The existing file manager behavior does not change as part of this migration. The change is only path and ownership: the file manager becomes part of the system module instead of a standalone files module.

The terminal page follows the 1Panel terminal concept:

- Target selector for local shell and SSH hosts.
- Multi-tab terminal sessions.
- xterm.js terminal rendering.
- Resize support.
- Connection states: connecting, connected, disconnected, and error.
- Quick command input that sends text to the active terminal.
- Active session close action.

## Backend Architecture

Add a new backend module:

```text
internal/terminal/model
internal/terminal/service
internal/terminal/pty
internal/terminal/ssh
internal/terminal/api
```

Responsibilities:

- `model`: WebSocket message types, session metadata, target metadata, audit records.
- `service`: session lifecycle, target validation, permission checks, timeout handling, audit writes.
- `pty`: local shell PTY adapter.
- `ssh`: SSH terminal adapter that reuses existing managed host connection data.
- `api`: HTTP and WebSocket endpoints.

Backend API:

```text
GET  /api/system/terminal/targets
WS   /api/system/terminal/connect
POST /api/system/terminal/sessions/:id/close
GET  /api/system/terminal/sessions
```

Target behavior:

- `local` target starts a shell on the VanPanelBackend host.
- `ssh` targets are existing managed hosts from the service tree/local resource data.
- The frontend cannot submit arbitrary SSH host, username, password, or key material to the terminal connect endpoint.

## Local Shell Behavior

The local shell runs as the current VanPanelBackend process user. The first version does not switch users and does not call sudo or su.

On Linux, the shell selection order is:

```text
$SHELL
/bin/bash
/bin/sh
```

If none are available, the connection fails with a structured error message.

On Windows, the local shell target returns a clear unsupported response in the first version. SSH targets can still work if the existing SSH implementation supports them.

## Unified WebSocket Protocol

All terminal targets use JSON messages.

Client to server:

```json
{ "type": "connect", "targetType": "local", "targetId": "local" }
{ "type": "input", "sessionId": "session-id", "data": "ls -la\r" }
{ "type": "resize", "sessionId": "session-id", "cols": 120, "rows": 32 }
{ "type": "ping", "sessionId": "session-id" }
{ "type": "close", "sessionId": "session-id", "reason": "client_closed" }
```

Server to client:

```json
{ "type": "connected", "sessionId": "session-id", "targetType": "local", "targetName": "Local Shell" }
{ "type": "output", "sessionId": "session-id", "data": "..." }
{ "type": "pong", "sessionId": "session-id" }
{ "type": "error", "sessionId": "session-id", "message": "..." }
{ "type": "closed", "sessionId": "session-id", "reason": "server_closed" }
```

Protocol rules:

- The first client message must be `connect`.
- Input before a successful `connected` message is rejected.
- Resize messages are forwarded to the active PTY or SSH session.
- Ping/pong is used to detect dead connections.
- Close messages should trigger cleanup and audit finalization.

## Security Model

Terminal access is high-risk and must be explicitly authenticated.

Security rules:

- Only authenticated users can access terminal APIs.
- WebSocket connections must validate the same token model as the rest of the system.
- The terminal WebSocket route must not be a naked unauthenticated whitelist endpoint.
- Local shell runs as the backend process user.
- SSH targets must come from existing managed host records.
- No arbitrary host connection details are accepted from the frontend.
- Idle sessions close automatically after a configured timeout, initially 30 minutes.
- Session cleanup must close the WebSocket, PTY or SSH session, and process resources.

## Audit Model

The first version records session metadata only:

- User ID.
- Username.
- Source IP.
- Target type.
- Target ID.
- Target name.
- Start time.
- End time.
- Exit reason.
- Error summary when applicable.

The first version does not record:

- Command input.
- Terminal output.
- Replay data.

This avoids storing passwords, tokens, private keys, and other sensitive data that commonly pass through terminals.

## Error Handling

Errors are returned as structured protocol messages where possible:

```json
{ "type": "error", "sessionId": "session-id", "message": "target not found" }
```

Expected errors:

- Invalid target type.
- Target not found.
- Permission denied.
- Local shell unsupported on current OS.
- Shell executable not found.
- SSH connection failed.
- PTY allocation failed.
- Session timeout.
- WebSocket closed by client.

Errors should finalize the audit record with an error summary, but should not include secrets or command content.

## Testing

Backend tests:

- Message parsing and validation.
- Local shell selection.
- Unsupported OS behavior.
- Session lifecycle.
- Target validation.
- Audit metadata finalization.
- Timeout cleanup.

Frontend tests:

- Terminal message encode/decode utilities.
- Session/tab state transitions.
- Route registration for `/system/files` and `/system/terminal`.
- Existing file manager utilities after migration.

Manual verification:

- Open `/system/files` and confirm file manager behavior is unchanged.
- Open `/system/terminal` on Linux and start a local shell.
- Open an SSH target from existing host data.
- Resize the browser and confirm resize messages are sent.
- Send quick commands to the active terminal.
- Close a terminal tab and confirm backend session cleanup.
- Leave a session idle and confirm timeout cleanup.
