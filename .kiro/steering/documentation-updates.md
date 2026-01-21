# Documentation Updates

**Status:** Authoritative
**Applies to:** All spec completions
**Audience:** Kiro

---

## Purpose

This document ensures documentation stays current when features are implemented.

---

## Requirements

When a spec is completed (all tasks marked done), Kiro MUST:

1. **Update README.md** with:
   - New commands or flags in the usage section
   - New features in the features list
   - Updated examples showing new functionality

2. **Update inline help** if the CLI has `--help` or usage text:
   - Add new commands/subcommands
   - Add new flags with descriptions
   - Add usage examples

3. **Check for other docs** that may need updates:
   - CHANGELOG.md (if present)
   - API documentation
   - Configuration documentation

---

## Checklist

Before marking a spec fully complete, verify:

- [ ] README reflects new features
- [ ] CLI help text is updated
- [ ] Examples are accurate and tested
- [ ] No stale documentation references old behavior
