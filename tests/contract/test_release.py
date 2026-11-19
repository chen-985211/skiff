from pathlib import Path
import os
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
TAG = "v2.3.4"
PACKAGES = {
    "skiff-2.3.4-arm64.dmg": "skiff-2.3.4-macos-arm64.dmg",
    "skiff-setup-2.3.4-amd64.exe": "skiff-2.3.4-windows-x64.exe",
    "skiff-2.3.4-linux-amd64.tar.gz": "skiff-2.3.4-linux-x64.tar.gz",
    "skiff_2.3.4_amd64.deb": "skiff-2.3.4-linux-x64.deb",
    "skiff-2.3.4-1.x86_64.rpm": "skiff-2.3.4-linux-x64.rpm",
}


class ReleasePreparation(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.source = Path(temp.name) / "dist"
        self.output = Path(temp.name) / "release"
        self.source.mkdir()
        for name in PACKAGES:
            (self.source / name).write_bytes(name.encode())

    def prepare(self):
        return subprocess.run(
            [sys.executable, "scripts/prepare-release.py", TAG, self.source, self.output],
            cwd=ROOT, capture_output=True, text=True,
            env={**os.environ, "GITHUB_REPOSITORY": "example/skiff"},
        )

    def test_versioned_packages_and_first_open_notes(self):
        result = self.prepare()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual({p.name for p in self.output.iterdir()}, set(PACKAGES.values()))
        for original, published in PACKAGES.items():
            self.assertEqual((self.output / published).read_bytes(), original.encode())
        self.assertIn("macOS 首次打开", result.stdout)
        self.assertIn("仍要打开", result.stdout)
        self.assertIn("xattr -dr com.apple.quarantine /Applications/Skiff.app", result.stdout)
        self.assertIn("https://github.com/example/skiff/releases/download/v2.3.4/skiff-2.3.4-macos-arm64.dmg", result.stdout)
        self.assertNotIn("v0.1.0", result.stdout)

    def test_incomplete_set_fails_before_staging(self):
        (self.source / "skiff_2.3.4_amd64.deb").write_bytes(b"")
        result = self.prepare()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("skiff_2.3.4_amd64.deb", result.stderr)
        self.assertFalse(self.output.exists())
