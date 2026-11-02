# Changelog

## [0.1.0](https://github.com/tanvinhnguyen2504/auth-service/releases/tag/0.0.1) - 2026-11-02
- [#PR1](https://github.com/tanvinhnguyen2504/auth-service/pull/1)
  - feat(ci): add CI and Artifact Registry release workflows

## 2026-10-31

- feat(middleware): add check require `app-id` from header/query in handler to server multi mini apps

## 2026-10-25

- feat(auth): add logic auth by oauth2 of google
- feat(secret): add AES-256-GCM box for sealed values
- feat(platform): add config, database pool, error envelope and security middleware
- chore: add Go module, Makefile and container build