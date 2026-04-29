# File Manager

The file manager exposes local-host file operations through `/api/files/*`.

Default safety mode restricts operations to configured `file_manager.roots`. Set `file_manager.allow_full_disk` to `true` only for trusted administrator deployments.

First release supports the local target only:

```json
{ "target_type": "local", "node_id": 0 }
```

Remote service-tree nodes are reserved for a later SSH/SFTP adapter.

## Configuration

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

Relative roots are resolved against the backend working directory. All read and write operations are checked against the resolved safe roots.
