# Security & Redaction Guidelines

This repository contains an **unofficial**, **reverse‑engineered** API client and spec.

- Do **not** commit secrets: tokens, cookies, API keys, private emails, phone numbers.
- Keep raw traffic captures local — `captures/` is gitignored because HAR files contain live tokens and PII. Only **redacted** material belongs in `examples/` or the spec.
- Redact IDs that can be tied to real people. Use placeholders like `REDACTED`.
- Avoid including full timestamps if unnecessary; consider truncation.
- If you believe something sensitive slipped in, remove it immediately (and rotate any exposed token).

If you represent Skylight and want something removed, open an issue and it will be addressed.
