#!/usr/bin/env bash
# Gets the VM ready for the E2E stack: installs Go, Node, tsx and Docker when the agent did not boot
# from .cursor/Dockerfile (same versions as its ARGs), starts Docker, and maps fake-zoom's hostname
# to 127.0.0.1 so the agent's own browser reaches it during the OAuth flow and for join links (the
# Playwright suite does the same with Chromium's --host-resolver-rules). Mattermost reaches it
# through extra_hosts in e2e/playwright/docker-compose.yml, as in CI.
#
# Hardened like the Playbooks and core repos' start hooks: falls back to starting dockerd directly
# when the service script fails, fixes socket access for the current session, prints Docker's logs
# on failure, and logs in to Docker Hub when DOCKERHUB_USERNAME and DOCKERHUB_TOKEN are set as
# Cursor environment secrets (anonymous pulls can hit rate limits).
set -euo pipefail

# Prints a prefixed status message.
log() { echo "[cloud-agent-start] $*"; }

cd "$(dirname "$0")/.."

# Reads an ARG default from .cursor/Dockerfile.
image_arg() { sed -n "s/^ARG $1=//p" .cursor/Dockerfile; }

# Installs the given apt packages that are not already installed.
apt_install() {
    local missing=()
    for pkg in "$@"; do
        dpkg -s "$pkg" >/dev/null 2>&1 || missing+=("$pkg")
    done
    [[ ${#missing[@]} -eq 0 ]] && return 0
    if ! command -v apt-get >/dev/null 2>&1; then
        log "Missing ${missing[*]} and apt-get is not available; install them by hand." >&2
        exit 1
    fi
    log "Installing ${missing[*]}"
    sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends "${missing[@]}" >/dev/null
}

# Installs Go at the Dockerfile's GO_VERSION when it is missing.
ensure_go() {
    command -v go >/dev/null 2>&1 && return 0
    local version
    version="$(image_arg GO_VERSION)"
    log "Go is missing; installing go$version."
    apt_install build-essential ca-certificates curl git make
    sudo rm -rf /usr/local/go
    curl -fsSL "https://go.dev/dl/go${version}.linux-$(dpkg --print-architecture).tar.gz" | sudo tar -C /usr/local -xz
    for bin in go gofmt; do sudo ln -sf /usr/local/go/bin/$bin /usr/local/bin/$bin; done
}

# Installs Node at the Dockerfile's NODE_VERSION when it is missing.
ensure_node() {
    command -v node >/dev/null 2>&1 && return 0
    local version arch
    version="$(image_arg NODE_VERSION)"
    arch="$(dpkg --print-architecture)"
    [[ "$arch" == amd64 ]] && arch=x64
    log "Node is missing; installing v$version."
    apt_install ca-certificates curl xz-utils
    sudo rm -rf /usr/local/node
    sudo mkdir -p /usr/local/node
    curl -fsSL "https://nodejs.org/dist/v${version}/node-v${version}-linux-${arch}.tar.xz" |
        sudo tar -C /usr/local/node -xJ --strip-components=1
    for bin in node npm npx; do sudo ln -sf /usr/local/node/bin/$bin /usr/local/bin/$bin; done
}

# Installs tsx at the Dockerfile's TSX_VERSION when it is missing.
ensure_tsx() {
    command -v tsx >/dev/null 2>&1 && return 0
    log "tsx is missing; installing it."
    sudo env "PATH=$PATH" npm install -g --silent "tsx@$(image_arg TSX_VERSION)"
    sudo ln -sf "$(npm prefix -g)/bin/tsx" /usr/local/bin/tsx
}

# Installs Docker Engine and Compose when either is missing.
ensure_docker() {
    if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
        return 0
    fi
    log "Docker is missing; installing it."
    apt_install acl ca-certificates curl fuse-overlayfs gnupg iptables
    local codename
    # shellcheck disable=SC1091
    codename="$(. /etc/os-release && echo "$VERSION_CODENAME")"
    sudo install -m 0755 -d /etc/apt/keyrings
    sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $codename stable" |
        sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
    sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends \
        docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin >/dev/null
    # Same nested-Docker setup as .cursor/Dockerfile; an existing daemon.json is left alone.
    if [[ ! -f /etc/docker/daemon.json ]]; then
        sudo mkdir -p /etc/docker
        echo '{"storage-driver": "fuse-overlayfs"}' | sudo tee /etc/docker/daemon.json >/dev/null
        sudo update-alternatives --set iptables /usr/sbin/iptables-legacy >/dev/null || true
        sudo update-alternatives --set ip6tables /usr/sbin/ip6tables-legacy >/dev/null || true
    fi
    sudo usermod -aG docker "$(id -un)" || true
}

# Grants the current user read/write access to the Docker socket.
ensure_socket_access() {
    [[ -S /var/run/docker.sock ]] || return 0
    sudo chgrp docker /var/run/docker.sock 2>/dev/null || true
    sudo chmod g+rw /var/run/docker.sock 2>/dev/null || true
    if command -v setfacl >/dev/null 2>&1; then
        sudo setfacl -m "u:$(id -un):rw" /var/run/docker.sock 2>/dev/null || true
    fi
}

# Logs in to Docker Hub when credentials are set.
docker_login_if_configured() {
    if [[ -z "${DOCKERHUB_USERNAME:-}" || -z "${DOCKERHUB_TOKEN:-}" ]]; then
        log "Docker Hub credentials not configured; anonymous pulls may hit rate limits."
        return 0
    fi
    if echo "${DOCKERHUB_TOKEN}" | docker login -u "${DOCKERHUB_USERNAME}" --password-stdin >/tmp/docker-login.log 2>&1; then
        log "Logged in to Docker Hub as ${DOCKERHUB_USERNAME}."
    else
        log "Docker Hub login failed; see /tmp/docker-login.log." >&2
    fi
}

# Succeeds when the Docker daemon and Compose are usable.
ready() {
    docker info >/dev/null 2>&1 && docker compose version >/dev/null 2>&1
}

apt_install acl jq openssl procps
ensure_go
ensure_node
ensure_tsx
ensure_docker

if ! grep -qE '^127\.0\.0\.1[[:space:]]+fake-zoom$' /etc/hosts; then
    echo "127.0.0.1 fake-zoom" | sudo tee -a /etc/hosts >/dev/null
fi

ensure_socket_access
if ! ready; then
    sudo sh -c 'service docker start >/tmp/docker-service-start.log 2>&1' ||
        log "service docker start failed; starting dockerd directly."
    if ! pgrep -x dockerd >/dev/null 2>&1; then
        sudo rm -f /var/run/docker.pid
        sudo sh -c 'nohup dockerd --host=unix:///var/run/docker.sock >/tmp/dockerd.log 2>&1 &'
    fi
    for _ in $(seq 1 60); do
        ensure_socket_access
        ready && break
        sleep 1
    done
fi

if ! ready; then
    log "Docker did not become ready within 60 seconds." >&2
    for f in /tmp/docker-service-start.log /var/log/docker.log /tmp/dockerd.log; do
        if [[ -f "$f" ]]; then
            log "$f:" >&2
            sudo tail -n 80 "$f" >&2 || true
        fi
    done
    exit 1
fi

docker_login_if_configured
log "Docker is ready."
