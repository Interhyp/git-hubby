import io
import json
import tarfile
import tempfile
import unittest
from pathlib import Path

from pr_snapshot import create_publication, image_tag, update_chart, validate_archive, validate_crds, validate_publication


SHA = "a" * 40
DIGEST = "sha256:" + "b" * 64


def archive(path, entries=None):
    if entries is None:
        entries = [("oci-layout", b"{}"), ("index.json", b"{}"), ("manifest.json", b"[]"),
                   ("blobs/sha256/" + "a" * 64, b"blob")]
    with tarfile.open(path, "w") as tar:
        for name, data in entries:
            info = tarfile.TarInfo(name)
            if data is None:
                info.type = tarfile.SYMTYPE
                info.linkname = "../../outside"
                tar.addfile(info)
            else:
                info.size = len(data)
                tar.addfile(info, io.BytesIO(data))


class SnapshotTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.crds = self.root / "crds"
        self.crds.mkdir()
        (self.crds / "github.interhyp.de_repositories.yaml").write_text("kind: CustomResourceDefinition\n")
        self.publication = self.root / "publication"
        self.chart = self.root / "chart"
        (self.chart / "crds").mkdir(parents=True)
        (self.chart / "crds" / "old.yaml").write_text("old")
        (self.chart / "Chart.yaml").write_text('name: git-hubby\nappVersion: "1.0.0"\n')
        self.image = self.root / "image.tar"
        archive(self.image)

    def create(self):
        create_publication(self.crds, self.publication, "255", SHA, "123", "1", DIGEST)

    def test_records_publication_and_updates_trusted_chart(self):
        self.create()
        record = validate_publication(self.publication, "123")
        self.assertEqual(record["image_digest"], DIGEST)
        self.assertEqual(record["image_tag"], image_tag("255", SHA, "123", "1"))
        update_chart(self.publication, self.chart, "123")
        self.assertEqual([p.name for p in (self.chart / "crds").iterdir()], ["github.interhyp.de_repositories.yaml"])
        self.assertIn(f'appVersion: "{record["image_tag"]}"', (self.chart / "Chart.yaml").read_text())

    def test_rejects_modified_crds_and_wrong_run(self):
        self.create()
        with self.assertRaises(ValueError):
            validate_publication(self.publication, "124")
        (self.publication / "crds" / "github.interhyp.de_repositories.yaml").write_text("tampered")
        with self.assertRaises(ValueError):
            update_chart(self.publication, self.chart, "123")
        self.assertTrue((self.chart / "crds" / "old.yaml").exists())

    def test_rejects_modified_image_reference_and_invalid_fields(self):
        self.create()
        path = self.publication / "publication.json"
        record = json.loads(path.read_text())
        for key, value in [("image_tag", "latest"), ("sha", "bad\nsha"), ("attempt", 1), ("image_digest", "bad")]:
            with self.subTest(key=key):
                modified = dict(record)
                modified[key] = value
                path.write_text(json.dumps(modified))
                with self.assertRaises(ValueError):
                    validate_publication(self.publication, "123")

    def test_rejects_archive_path_traversal_and_links(self):
        validate_archive(self.image)
        for name, data in [("../escape", b"bad"), ("/absolute", b"bad"), ("blobs/sha256/" + "a" * 64, None)]:
            with self.subTest(name=name):
                archive(self.image, [(name, data)])
                with self.assertRaises(ValueError):
                    validate_archive(self.image)

    def test_rejects_duplicate_and_unexpected_archive_entries(self):
        for entries in [[("index.json", b"{}"), ("index.json", b"{}")], [("run.sh", b"bad")]]:
            with self.subTest(entries=entries):
                archive(self.image, entries)
                with self.assertRaises(ValueError):
                    validate_archive(self.image)

    def test_rejects_symlink_and_unexpected_crd(self):
        (self.crds / "github.interhyp.de_repositories.yaml").unlink()
        (self.crds / "github.interhyp.de_repositories.yaml").symlink_to(self.chart / "Chart.yaml")
        with self.assertRaises(ValueError):
            validate_crds(self.crds)
        (self.crds / "github.interhyp.de_repositories.yaml").unlink()
        (self.crds / "run.sh").write_text("bad")
        with self.assertRaises(ValueError):
            validate_crds(self.crds)

    def test_rejects_unexpected_chart_before_changing_crds(self):
        self.create()
        (self.chart / "Chart.yaml").write_text("different chart\n")
        with self.assertRaises(ValueError):
            update_chart(self.publication, self.chart, "123")
        self.assertTrue((self.chart / "crds" / "old.yaml").exists())


if __name__ == "__main__":
    unittest.main()
