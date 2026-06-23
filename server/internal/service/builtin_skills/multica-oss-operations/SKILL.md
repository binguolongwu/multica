---
name: multica-oss-operations
description: "Upload, download, and list files in the workspace Object Storage Service (OSS)."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Multica OSS Operations

You have access to a workspace-level Object Storage Service (OSS). Use it
to store files that need to persist beyond the current session — generated
reports, logs, screenshots, reference documents, and build artifacts.

## When to use OSS

- Save task outputs (reports, charts, logs, screenshots) for the user to review
- Store reference documents that agents in other tasks or sessions may need
- Share downloadable files with workspace members via public URLs

## Commands

```bash
# Upload a file — returns the public URL
multica oss upload --file <local_path> [--key <oss_key>]

# List uploaded files
multica oss list [--prefix <dir>]

# Download a file
multica oss download --key <oss_key> --output <local_path>
```

## Best practices

- Upload generated reports, logs, and screenshots at the end of each task
- Use descriptive keys with a directory structure:
  `reports/2026-06-24-sales-summary.pdf`, `logs/task-123-build.log`
- Reference OSS URLs in issue comments using markdown:
  `[Download report](https://files.example.com/reports/...)`
- Check `multica oss list` before overwriting an existing key
- Do NOT store credentials, secrets, or .env files in OSS
