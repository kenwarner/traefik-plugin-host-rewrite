# Traefik Plugin Host Rewrite

Traefik middleware plugin that rewrites request `Host` values with regex capture rules.

Example:

```text
preview-api.example.com -> api.internal.example.net
```

## Configuration

| Option | Default | Description |
| --- | --- | --- |
| `rules` | required | Ordered rewrite rules. First match wins. |
| `rules[].pattern` |  | Go regular expression matched against the incoming hostname, without the port. |
| `rules[].replacement` |  | Go regexp replacement string. Supports `$1`, `$name`, and related forms. |
| `allowedSuffixes` | empty | Optional allow-list for rewritten host suffixes. |
| `preserveForwardedHost` | `false` | Set `X-Forwarded-Host` to the original public host. |
| `originalHostHeader` | empty | Header used to store the original host. Set to an empty string to disable. |

The middleware lowercases the incoming host before matching, compiles regex rules once during startup, updates `req.Host`, and also sets the `Host` request header for compatibility.

## Catalog Install

Set the `traefik-plugin` topic on the public GitHub repository and tag releases, for example `v0.1.0`. If you fork or rename the repository, update the module path in `go.mod`, `.traefik.yml`, and the examples below before tagging.

Static configuration:

```yaml
experimental:
  plugins:
    hostRewrite:
      moduleName: github.com/kenwarner/traefik-plugin-host-rewrite
      version: v0.1.0
```

Dynamic configuration:

```yaml
http:
  routers:
    app-router:
      rule: HostRegexp(`^preview-[a-z0-9-]+\\.example\\.com$`)
      entryPoints:
        - websecure
      tls:
        certResolver: default
      service: app-service
      middlewares:
        - rewrite-preview-host

  middlewares:
    rewrite-preview-host:
      plugin:
        hostRewrite:
          rules:
            - pattern: "^preview-([a-z0-9-]+)\\.example\\.com$"
              replacement: "$1.internal.example.net"
          allowedSuffixes:
            - ".internal.example.net"
          preserveForwardedHost: true
          originalHostHeader: X-Original-Host

  services:
    app-service:
      loadBalancer:
        servers:
          - url: "http://app-backend:8080"
```

Make sure the rewritten host resolves correctly from Traefik's runtime environment. For containerized deployments, validate DNS from inside the Traefik container or network namespace, not only from the host machine.

## Local Development

Traefik local mode expects plugins under `./plugins-local/src/<module-name>`. For this module:

```text
plugins-local/
  src/
    github.com/
      kenwarner/
        traefik-plugin-host-rewrite/
```

Static configuration for local mode:

```yaml
experimental:
  localPlugins:
    hostRewrite:
      moduleName: github.com/kenwarner/traefik-plugin-host-rewrite
```

Dynamic configuration is the same as catalog mode.

## Development

```sh
go test ./...
```
