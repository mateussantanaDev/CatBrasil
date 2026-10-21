#!/usr/bin/env python3
"""Check frontend image health in disposable, network-isolated containers."""

import argparse
import json
import subprocess
import sys
import time
import uuid
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]


def docker(*args):
    return subprocess.run(["docker", *args], check=True, capture_output=True, text=True).stdout.strip()


def verify(name, image):
    container = f"catbrasil-health-{name}-{uuid.uuid4().hex[:10]}"
    try:
        docker("run", "--rm", "-d", "--network", "none", "--name", container, image)
        raw_test = docker("inspect", container, "--format", "{{json .Config.Healthcheck.Test}}")
        test = json.loads(raw_test) if raw_test else None
        if not test or not isinstance(test, list) or "http://127.0.0.1/healthz" not in test or any("localhost" in item for item in test):
            raise RuntimeError(f"{name}: imagem não usa healthcheck IPv4 em formato exec")
        docker("exec", container, "wget", "--no-verbose", "--tries=1", "--spider", "http://127.0.0.1/healthz")
        deadline = time.monotonic() + 45
        while time.monotonic() < deadline:
            raw_state = docker("inspect", container, "--format", "{{json .State.Health}}")
            state = json.loads(raw_state) if raw_state else None
            if state and isinstance(state, dict):
                if state.get("Status") == "healthy":
                    print(f"{name}: /healthz respondeu e Docker marcou healthy")
                    return
                if state.get("Status") == "unhealthy":
                    raise RuntimeError(f"{name}: Docker marcou unhealthy")
            time.sleep(1)
        raise RuntimeError(f"{name}: healthcheck não ficou healthy em 45 segundos")
    finally:
        subprocess.run(["docker", "rm", "-f", container], capture_output=True, text=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--build", action="store_true", help="reconstrói as duas imagens a partir dos Dockerfiles locais")
    parser.add_argument("--client-image", default="catbrasil/frontend-client:p0.9-local")
    parser.add_argument("--admin-image", default="catbrasil/frontend-admin:p0.9-local")
    args = parser.parse_args()
    images = {"client": args.client_image, "admin": args.admin_image}
    temporary = []
    try:
        if args.build:
            for name in images:
                image = f"catbrasil-health-{name}:{uuid.uuid4().hex[:10]}"
                docker("build", "-f", f"frontend/Dockerfile.{name}", "-t", image, str(ROOT))
                images[name] = image
                temporary.append(image)
        for name, image in images.items():
            verify(name, image)
    finally:
        for image in temporary:
            subprocess.run(["docker", "image", "rm", image], capture_output=True, text=True)


if __name__ == "__main__":
    try:
        main()
    except (subprocess.CalledProcessError, RuntimeError) as exc:
        print(f"Falha na validação do frontend: {exc}", file=sys.stderr)
        sys.exit(1)
