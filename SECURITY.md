# Security policy

## Supported versions

Security fixes are applied to the latest code on `main` and the latest tagged
release. Older development snapshots are not supported.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Use GitHub's private
vulnerability reporting flow under **Security → Advisories → Report a
vulnerability** for this repository. Include the affected command or file,
impact, reproduction steps, and any proposed mitigation. Do not include real
recordings, transcripts, credentials, or unrelated personal data.

If private vulnerability reporting is temporarily unavailable, contact the
maintainer through the repository owner's GitHub profile and ask for a private
reporting channel without disclosing technical details publicly.

You should receive an acknowledgement within seven days. A fix timeline will
depend on severity and reproducibility. Please allow time for a coordinated fix
before publishing details.

## Security boundaries

Vox protects local runtime files, avoids automatic submission, verifies pinned
downloads, and performs inference locally. X11 does not isolate applications:
another client in the same X11 session may observe input, windows, or clipboard
changes. Vox is not designed to defend against a compromised user account,
desktop session, model binary, kernel, or GPU driver. See
`docs/security-audit.md` for the complete threat model and residual risks.
