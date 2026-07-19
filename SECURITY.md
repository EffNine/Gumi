# Security Policy

## Supported Versions

Use this section to tell people about which versions of your project are currently being supported with security updates.

| Version | Supported          |
| ------- | ------------------ |
| 0.2.x   | :white_check_mark: |
| < 0.2   | :x:                |

## Reporting a Vulnerability

Gumi takes security seriously. We welcome responsible disclosure of vulnerabilities and appreciate your efforts to help keep the project safe.

### What to Report

Please report security vulnerabilities if they relate to:

- Authentication or authorization bypass
- Injection flaws (SQL, command injection, etc.)
- Secrets or credentials exposed in logs, configs, or responses
- Unsafe deserialization or file handling
- Cryptographic weaknesses
- Supply-chain concerns in dependencies or build process

### What Not to Report

Use a regular [bug report](../../issues/new?template=bug_report.md) for:

- UI glitches or styling issues
- Performance problems that don't involve data exposure
- Feature requests or enhancement suggestions (use [feature request](../../issues/new?template=feature_request.md) instead)
- Vulnerabilities in upstream dependencies that don't affect Gumi's threat model

### How to Report

**Preferred method: [GitHub Security Advisories](../../security/advisories/new)**

This creates a private channel where we can discuss the issue without exposing it publicly.

**Alternative: Private vulnerability reporting**

If you prefer, you can use GitHub's private vulnerability reporting feature on the repository.

### What to Include

When reporting, please provide:

1. A clear description of the vulnerability
2. Steps to reproduce (POC code, commands, or screenshots)
3. Your assessment of the impact
4. Any suggested mitigations (optional)

### Response Timeline

- **Acknowledgment:** We will acknowledge receipt within **5 business days** and assign a primary reviewer.
- **Assessment:** Within **15 business days**, we will provide an initial assessment and let you know if we agree it's a vulnerability.
- **Fix & Disclosure:** If confirmed, we will work on a fix and coordinate public disclosure with you. We aim to resolve critical issues within **30 days**.

### Privacy & Credit

- We will **not** disclose your identity without your permission.
- You may request anonymity and we will respect it.
- We are happy to **credit reporters** in release notes and advisories (unless you prefer to remain anonymous).
- You may also choose to remain unnamed — that's fine too.

## Security Best Practices for Contributors

- Run `gitleaks` or `trufflehog` locally before committing to avoid accidental secret leaks.
- Never hardcode credentials, API keys, or tokens in source code.
- Use environment variables or the project's configuration file (`gumi.yaml`) for secrets.
- Keep dependencies updated and review security advisories regularly.

## Contact

For questions about Gumi's security practices, open a [discussion](../../discussions) or reach out to the maintainer.
