#!/usr/bin/env python3
import argparse
import copy
import json
from pathlib import Path
import tempfile
from urllib.parse import urlsplit, urlunsplit

EXTENSION_ID = "nngceckbapebfimnlniiiahkandclblb"
VALID_MODES = {"off", "assist", "managed-extension"}
ROOT = Path(__file__).resolve().parents[1]
DEFAULT_TEMPLATE = ROOT / "deploy" / "oauth-browser" / "policies" / "bitwarden.json.tmpl"


def normalize_base_url(value: str) -> str:
    value = (value or "").strip()
    if not value:
        return ""
    parsed = urlsplit(value)
    if parsed.scheme.lower() != "https" or not parsed.hostname:
        raise ValueError("Bitwarden base URL must be HTTPS")
    if parsed.username is not None or parsed.password is not None:
        raise ValueError("Bitwarden base URL must not contain credentials")
    if parsed.query or parsed.fragment:
        raise ValueError("Bitwarden base URL must not contain query or fragment")
    path = parsed.path.rstrip("/")
    return urlunsplit(("https", parsed.netloc, path, "", ""))


def load_template(path: Path) -> dict:
    data = json.loads(path.read_text(encoding="utf-8"))
    settings = data.get("ExtensionSettings", {}).get(EXTENSION_ID, {})
    if settings.get("installation_mode") != "force_installed":
        raise ValueError("Bitwarden template must force-install the official extension")
    if settings.get("update_url") != "https://clients2.google.com/service/update2/crx":
        raise ValueError("Bitwarden template uses an unexpected update URL")
    return data


def render(mode: str, base_url: str, template: Path) -> dict:
    mode = mode.strip().lower()
    if mode not in VALID_MODES:
        raise ValueError("mode must be off, assist, or managed-extension")
    base_url = normalize_base_url(base_url)
    if mode == "off":
        return {}

    policy = copy.deepcopy(load_template(template)) if mode == "managed-extension" else {}
    if base_url:
        policy["3rdparty"] = {
            "extensions": {
                EXTENSION_ID: {
                    "environment": {
                        "base": base_url,
                    }
                }
            }
        }
    return policy


def atomic_write(path: Path, payload: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    text = json.dumps(payload, indent=2, sort_keys=True) + "\n"
    with tempfile.NamedTemporaryFile(
        "w", encoding="utf-8", dir=path.parent, delete=False
    ) as fh:
        fh.write(text)
        tmp = Path(fh.name)
    tmp.chmod(0o644)
    tmp.replace(path)


def main() -> int:
    parser = argparse.ArgumentParser(description="Render Sidekick browser policy")
    parser.add_argument("--mode", required=True)
    parser.add_argument("--base-url", default="")
    parser.add_argument("--template", type=Path, default=DEFAULT_TEMPLATE)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        payload = render(args.mode, args.base_url, args.template)
        atomic_write(args.output, payload)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        parser.error(str(exc))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
