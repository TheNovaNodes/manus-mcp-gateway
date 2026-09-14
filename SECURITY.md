# Security Policy

## Supported Versions

We release patches and security updates for the current major version.

| Version | Supported          |
| ------- | ------------------ |
| 1.x     | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

The NovaNodes Collective takes the security of our open-source software and infrastructure seriously.

If you discover a security vulnerability in `manus-mcp-gateway`:

1. **DO NOT** create a public GitHub Issue.
2. Please report the issue privately by opening a [GitHub Security Advisory](https://github.com/TheNovaNodes/manus-mcp-gateway/security/advisories/new) or contacting the maintainers at `security@novanodes.ai`.
3. Provide a detailed description of the vulnerability, reproduction steps, and potential impact.

### Response Timeline

- **Initial Response:** Within 24 hours acknowledging receipt.
- **Triage & Status Update:** Within 72 hours with an initial assessment.
- **Fix & Disclosure:** Coordinated release and disclosure schedule aligned with standard industry practices.

## Credential Safety Guidelines

- **Never commit `.env` or API keys:** All keys should be supplied via environment variables (`MANUS_KEYS` or `MANUS_API_KEY`).
- **File Permissions:** Ensure local key files have strict permissions (`chmod 600 .env`).
- **Output Redaction:** The gateway automatically masks API keys in status outputs and logs.
