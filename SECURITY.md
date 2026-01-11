# Security Policy

## Supported Versions

dsops follows semantic versioning. Security updates are provided for:

| Version | Supported          |
|---------|--------------------|
| 0.x     | :white_check_mark: |

> As dsops is pre-1.0, all 0.x releases receive security updates. Once 1.0 is released, this table will specify which major/minor versions are actively supported.

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### Reporting Methods

1. **GitHub Security Advisories** (Preferred)
   - Navigate to the [Security tab](https://github.com/systmms/dsops/security/advisories) in our repository
   - Click "Report a vulnerability"
   - This allows private discussion and coordinated disclosure

2. **Email**
   - Send details to: security@systmms.com
   - Use the subject line: `[dsops] Security Report: <brief description>`

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Any suggested fixes (optional)

### Response Timeline

| Phase | Timeframe |
|-------|-----------|
| Initial acknowledgment | Within 48 hours |
| Severity assessment and timeline | Within 7 days |
| Fix development | Varies by complexity |
| Coordinated disclosure | 90 days from initial report |

We follow a 90-day coordinated disclosure policy. If a fix cannot be deployed within 90 days, we will work with the reporter on an appropriate disclosure timeline.

### What to Expect

1. **Acknowledgment**: You'll receive confirmation that we've received your report within 48 hours
2. **Assessment**: We'll evaluate the severity and determine a fix timeline within 7 days
3. **Updates**: We'll keep you informed of our progress
4. **Credit**: With your permission, we'll acknowledge your contribution in the release notes
5. **Disclosure**: Once a fix is released, we'll publish a security advisory

### Credit and Acknowledgment

We believe in recognizing the valuable contributions of security researchers. Unless you prefer to remain anonymous, we will:

- Credit you in the security advisory
- Include your name/handle in the release notes
- Add you to our [security acknowledgments](#acknowledgments) section
- Link to your profile or website (if provided)

**Hall of Fame Eligibility**: Reports that result in a fix for Critical or High severity issues will be highlighted in our Hall of Fame section.

**Anonymity**: If you prefer to remain anonymous, simply let us know in your report. We will never disclose reporter identities without explicit permission.

## Out of Scope

The following issues are generally considered out of scope:

- **Denial of Service (DoS)** without additional security impact
- **Social engineering** attacks against project maintainers
- **Physical attacks** against infrastructure
- **Attacks requiring physical access** to a user's device
- **Issues in dependencies** without a demonstrated attack vector in dsops
- **Theoretical vulnerabilities** without proof-of-concept
- **Issues in unmaintained versions**

If you're unsure whether an issue is in scope, please report it anyway. We'd rather receive reports that turn out to be low-risk than miss genuine vulnerabilities.

## Report Triage Process

When we receive a vulnerability report, it goes through the following triage process:

### Assessment Categories

| Category | Response | Timeline |
|----------|----------|----------|
| **Critical** | Immediate escalation, expedited fix | Fix within days |
| **High** | Prioritized for next release | Fix within 2 weeks |
| **Medium** | Scheduled for upcoming release | Fix within 30 days |
| **Low** | Added to backlog | Fix within 90 days |
| **Informational** | Documented for future reference | No fix required |
| **Invalid/Out of Scope** | Closed with explanation | N/A |

### Invalid Report Handling

If a report is determined to be invalid or out of scope:

1. We will respond within 7 days explaining why
2. We will provide guidance on what would make it a valid report (if applicable)
3. We welcome follow-up questions or additional information
4. Reporters can request reconsideration if they disagree with the assessment

Common reasons reports may be marked invalid:
- Vulnerability requires conditions that are outside our threat model
- Issue is a known limitation documented in our [threat model](https://github.com/systmms/dsops/blob/main/docs/content/security/threat-model.md)
- Report lacks sufficient detail to reproduce
- Issue has already been reported and is being addressed

We treat all reporters with respect, regardless of whether their report results in a fix.

## Security Contacts

The security team can be reached via:

- **Primary**: [GitHub Security Advisories](https://github.com/systmms/dsops/security/advisories)
- **Email**: security@systmms.com
- **PGP Key**: Available upon request for encrypted communications

> **Note**: The security@systmms.com alias is monitored by project maintainers. For routine questions, please use GitHub Discussions instead.

## Security Features

dsops is designed with security as a core principle. Key security features include:

- **Ephemeral-first design**: Secrets are injected directly into process environments, never written to disk by default
- **Automatic log redaction**: All logging uses `logging.Secret()` to automatically mask sensitive values
- **Process isolation**: Parent processes never see secret values; only child processes receive them
- **Memory protection**: Secret values are protected from memory dumps using mlock
- **Signed releases**: All release artifacts are signed using Sigstore cosign
- **Software Bill of Materials**: Every release includes an SBOM for dependency transparency

## Acknowledgments

We thank the following individuals and organizations for responsibly disclosing security issues:

*No vulnerabilities have been reported yet. This section will be updated as we receive and address reports.*

---

This security policy is based on [GitHub's recommended security policy template](https://docs.github.com/en/code-security/getting-started/adding-a-security-policy-to-your-repository) and [OWASP guidelines](https://owasp.org/www-project-vulnerability-disclosure/).
