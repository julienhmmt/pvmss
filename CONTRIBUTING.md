# How to contribute?

Thanks for your interest in this project! 🎉

If you'd like to contribute, here's how:

- have an idea or found a bug? great! please open an issue first to discuss it
- want to contribute code? let's do it, look at the [Quick contribution](#quick-contribution) section below

## Quick contribution

1. **Fork** this repository
2. **Create** a branch: `git checkout -b my-contribution`
3. **Commit** your changes: `git commit -m "Clear description"`
4. **Push** your branch: `git push origin my-contribution`
5. **Open** a Pull Request

## Types of contributions

- 🐛 Report a bug → Open an issue
- ✨ New feature → Open an issue
- 📝 Documentation → Direct PR welcome
- 🔧 Minor fixes → Direct PR welcome

## Simple guidelines

- Clear commit messages in English
- Test your code before submitting: `make server-test` (Go, uses the `fake` cluster - no Proxmox needed), `make web-test` and `make web-check` (SvelteKit), `make lint` for both
- Touching a user-facing feature? Update `WORKFLOWS.md`, `docs/FEATURES.md`, and the in-app pages under `server/internal/docs/seed/` (EN + FR) and the UI strings in `web/messages/` (EN, FR, DE, IT)
- Follow existing code style; conventions, commands and architecture are in [AGENTS.md](AGENTS.md) (read it before a non-trivial change)
- Touching logging or the SIEM sink? Update `docs/observability.md` (and the `PVMSS_SYSLOG_*` rows in the READMEs)
- Avoid the em-dash character (U+2014) in code and docs: use a spaced hyphen `-`
- Be respectful and constructive
- AI-generated code is welcome, but must be reviewed by a human before merging

## Development stack

- `make up` starts `pvmss-dev` behind five reverse proxies (HAProxy on
  `https://localhost:50000`, nginx `:50001`, Caddy `:50002`, Traefik `:50003`,
  Nginx Proxy Manager `https://pvmss.localhost:50004`). Configs live in
  [`dev/proxies/`](dev/proxies/). The stack runs mutual TLS on both hops:
  import `dev/proxies/certs/client.p12` (password `pvmss`) and trust `ca.crt`
  once. Details in [docs/tls.md](docs/tls.md).
- `make proxy-e2e` checks the VNC console WebSocket through each proxy
  (Playwright, fake cluster source, its own database).
- `make sonar` scans `pvmss-server` and `pvmss-web` into the shared local
  SonarQube (`~/git/gh/sonarqube`); results at `http://localhost:9000/projects`,
  stop it with `make sonar-down`.

## Need help?

Open an issue with the `question` label or reach out to us!

## Review process

PRs are typically reviewed within 2-3 days. Please be patient and responsive to feedback. If you don't get a response within 5 days, feel free to ping us.

Thanks for contributing! 🙏
