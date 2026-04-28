# VanPanel File Manager Design

## Goal

Add a 1Panel-inspired operations file manager to VanPanel. The first release manages files on the VanPanelBackend host machine and reserves API fields for future service-tree remote host support through SSH.

## Scope

The first release includes:

- Directory browsing with pagination, sorting, hidden-file toggle, and filename search.
- Directory tree browsing with bounded depth.
- File upload and download.
- Create file, create directory, rename, delete, copy, and move.
- Text preview and text editing with size and binary-file limits.
- File metadata display including size, mode, owner, group, type, and modification time.
- chmod and chown on Unix-like systems.
- zip and tar.gz compression and decompression.
- Safe-root enforcement with optional administrator-enabled full-disk mode.

The first release excludes:

- Remote host file management execution.
- Recycle bin.
- Public file sharing links.
- File edit history or version restore.
- AI file search.
- Database-backed file-manager configuration UI.

## Architecture

The backend adds a focused `internal/files` module instead of copying the full 1Panel implementation. The module follows the existing VanPanelBackend layering:

- `internal/files/api`: Gin handlers and `/api/files/*` route registration.
- `internal/files/service`: file-manager business logic, request validation, path policy enforcement, and operation orchestration.
- `internal/files/fs`: local filesystem adapter around `os`, `filepath`, `archive/zip`, and tar.gz utilities.
- `internal/files/model` or `internal/files/types`: request and response DTOs. This module does not need database models in the first release.

The module is registered through the existing dependency injection flow:

- Add providers to `pkg/di/wire.go`.
- Add handler parameters and route registration to `pkg/di/web.go`.
- Regenerate `pkg/di/wire_gen.go` during implementation if the project expects generated Wire output to be committed.

The frontend adds a single-page operations workbench:

- `apps/web-antd/src/router/routes/modules/files.ts`
- `apps/web-antd/src/api/core/files/files.ts`
- `apps/web-antd/src/views/files/FileManager.vue`
- `apps/web-antd/src/views/files/file-manager.css`

The UI uses Ant Design Vue and the existing `requestClient` abstraction.

## Target Model

Every API request that operates on files accepts target metadata:

```json
{
  "target_type": "local",
  "node_id": 0,
  "path": "/var/www",
  "page": 1,
  "size": 50
}
```

First release behavior:

- `target_type` must be `local`.
- `node_id` is accepted but ignored for local operations.
- Any non-local target returns `ErrFileOperationUnsupported`.

Future remote-host support can reuse the same request shape and swap the local filesystem adapter for an SSH/SFTP adapter.

## Security Model

The file manager defaults to safe-root mode. All file operations must resolve paths before execution and reject paths outside configured safe roots.

Path policy requirements:

- Clean every input path with `filepath.Clean`.
- Resolve absolute paths with `filepath.Abs`.
- Resolve symlinks for access checks where the operation follows symlinks.
- Reject empty paths.
- Reject paths outside configured roots when full-disk mode is disabled.
- Reject move and copy operations where source or destination escapes allowed roots.
- Reject invalid file names for create and rename.
- Preserve explicit backend enforcement even if the frontend hides unsafe paths.

Full-disk mode is available only when enabled in backend configuration. The first release assumes existing authentication and authorization middleware protects `/api/files/*`; implementation should also make the route private by default.

## Configuration

Add configuration to backend YAML files:

```yaml
file_manager:
  enabled: true
  allow_full_disk: false
  roots:
    - name: VanPanel
      path: .
    - name: Logs
      path: ./logs
    - name: Deploy
      path: ./deploy
  max_edit_size_mb: 5
  max_preview_size_mb: 10
  allowed_archive_types:
    - zip
    - tar.gz
```

Relative paths are resolved against the backend working directory at startup or service initialization time.

## Backend API

`GET /api/files/roots`

Returns enabled state, allowed roots, full-disk mode, OS type, and path separator.

`POST /api/files/list`

Lists files under a directory. Supports pagination, sorting, hidden-file toggle, and filename search.

`POST /api/files/tree`

Returns a bounded-depth directory tree for the left navigation.

`POST /api/files/content`

Reads file content for preview or edit. Binary files and files above configured limits return metadata plus an error indicating edit or preview is unavailable.

`POST /api/files/save`

Writes text content to a file and preserves existing permissions where possible.

`POST /api/files/create`

Creates a file or directory.

`POST /api/files/rename`

Renames a file or directory within the same parent directory unless the request explicitly provides a destination path.

`POST /api/files/delete`

Deletes a file or directory. First release deletes directly and does not use a recycle bin.

`POST /api/files/move`

Copies or moves files. Supports overwrite control.

`POST /api/files/chmod`

Changes mode on Unix-like systems. Windows returns `ErrFileOperationUnsupported`.

`POST /api/files/chown`

Changes owner and group on Unix-like systems. Windows returns `ErrFileOperationUnsupported`.

`POST /api/files/compress`

Creates zip or tar.gz archives.

`POST /api/files/decompress`

Extracts zip or tar.gz archives.

`POST /api/files/upload`

Uploads files with multipart form data.

`GET /api/files/download?path=...`

Downloads a single file. Directory download requires the user to compress the directory first.

## Error Handling

The handler layer uses the existing `base.HandleRequest` style. The service layer returns typed business errors that can be mapped to consistent API responses:

- `ErrFilePathDenied`: path is outside allowed roots or full-disk mode is disabled.
- `ErrFileNotFound`: path does not exist.
- `ErrFileAlreadyExists`: destination exists and overwrite is false.
- `ErrFileTooLarge`: preview or edit exceeds configured limits.
- `ErrFileBinaryUnsupported`: binary file cannot be edited.
- `ErrFileOperationUnsupported`: operation is unsupported for the current OS or target type.
- `ErrFileInvalidName`: file or directory name is invalid.
- `ErrFileArchiveUnsupported`: archive type is unsupported.

Write operations should include clear request fields such as `path`, `target_path`, `paths`, and `operation` so the existing request/audit logging can identify what changed.

## Frontend Design

The page is a single operations workbench.

Top bar:

- Target selector. First release shows only local host.
- Current path breadcrumb.
- Filename search input.
- Refresh action.

Left panel:

- Safe-root list.
- Directory tree.
- Common path shortcuts.

Main panel:

- Toolbar for upload, new file, new directory, copy, move, delete, compress, decompress, chmod, and chown.
- File table with name, type, size, mode, owner, group, modified time, and actions.
- Sorting, pagination, hidden-file toggle, and multi-select.

Right drawer:

- Text preview and editor.
- File properties.
- chmod/chown form.
- Operation result feedback for archive actions.

Dialogs:

- Copy or move target path.
- Compression and decompression parameters.
- Delete confirmation.
- Upload progress.

Frontend safety behavior:

- Dangerous actions require confirmation.
- chmod and chown are hidden or disabled on Windows.
- Binary and oversized files cannot enter edit mode.
- Backend denial errors are displayed directly and do not get masked as generic failures.

## Testing Strategy

Backend unit tests:

- Safe-root validation for empty path, `..`, symlink escape, cross-root copy/move, Windows-style paths, and normal allowed paths.
- File metadata construction.
- Create, rename, delete, copy, move, text read, text save, upload helper behavior, zip, tar.gz, decompress.
- chmod/chown supported and unsupported OS behavior where practical.

Backend handler tests:

- Successful roots, list, content, create, rename, delete, and download responses.
- Denied path response.
- Unsupported target response.
- Oversized or binary content response.

Frontend tests:

- API parameter construction.
- File size and mode formatting.
- Edit-disable logic for binary and oversized files.
- Dangerous operation confirmation state.

Manual verification:

- Browse configured roots.
- Upload and download a file.
- Create, rename, copy, move, and delete a file and directory.
- Preview and edit a text file.
- Confirm binary files cannot be edited.
- Compress and decompress zip and tar.gz.
- Verify chmod/chown behavior on the active OS.
- Verify denied path attempts fail.

## Phased Delivery

1. Backend foundation: configuration, DTOs, local filesystem adapter, path policy, roots, list, tree, metadata, and content read.
2. Backend write operations: create, save, upload, download, rename, delete, copy, and move.
3. Backend operations: chmod, chown, compress, and decompress.
4. Frontend workbench: route, API client, root selector, directory tree, file table, search, sorting, pagination, upload, download, preview, and edit drawer.
5. Frontend operations: copy, move, delete, compress, decompress, chmod, chown, and confirmation flows.
6. Verification and documentation: backend tests, frontend type check or tests, manual checklist, and implementation notes for future SSH remote target support.

## Open Decisions Resolved

- Product shape: operations file workbench.
- First release target: local host only, with remote target fields reserved.
- First release feature level: 1Panel common capabilities without recycle bin, sharing, history, or AI search.
- Safety model: safe roots by default, optional administrator-enabled full-disk mode.
- UI structure: single-page Ant Design Vue workbench with left navigation, central table, and right drawer.
