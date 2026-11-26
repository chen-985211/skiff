"""Stage the native MyGo packages and write the tag-specific release introduction to stdout."""
import os
from pathlib import Path
import re
import shutil
import sys


def prepare(tag, source, output):
    if not re.fullmatch(r"v[0-9]+\.[0-9]+\.[0-9]+", tag):
        raise SystemExit(f"Expected a vMAJOR.MINOR.PATCH tag, got {tag}")
    version = tag[1:]
    dmg = f"skiff-{version}-macos-arm64.dmg"
    packages = {
        f"skiff-{version}-arm64.dmg": dmg,
        f"skiff-setup-{version}-amd64.exe": f"skiff-{version}-windows-x64.exe",
        f"skiff-{version}-linux-amd64.tar.gz": f"skiff-{version}-linux-x64.tar.gz",
        f"skiff_{version}_amd64.deb": f"skiff-{version}-linux-x64.deb",
        f"skiff-{version}-1.x86_64.rpm": f"skiff-{version}-linux-x64.rpm",
    }
    for original in packages:
        path = source / original
        if not path.is_file() or not path.stat().st_size:
            raise SystemExit(f"Missing or empty package: {path}. Check the version in mygo.json.")

    readme_path = Path(__file__).resolve().parents[1] / "README.zh-CN.md"
    readme = readme_path.read_text(encoding="utf-8")
    heading = "### macOS 首次打开"
    section = re.search(rf"^{heading}\n(.+?)(?=^#{{1,3}} |\Z)", readme, re.MULTILINE | re.DOTALL)
    if not section or "[macos-dmg]" not in section[1]:
        raise SystemExit(f"{readme_path.name} is missing the macOS first-open guide or its [macos-dmg] link.")
    url = f"https://github.com/{os.environ['GITHUB_REPOSITORY']}/releases/download/{tag}/{dmg}"

    output.mkdir()
    for original, published in packages.items():
        shutil.copyfile(source / original, output / published)
    print(f"## macOS 首次打开\n\n{section[1].strip()}\n\n[macos-dmg]: {url}")


if __name__ == "__main__":
    tag, source, output = sys.argv[1:]
    prepare(tag, Path(source), Path(output))
