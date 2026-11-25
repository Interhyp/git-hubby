#!/usr/bin/env python3
"""Handle snapshot artifacts as data; never execute content from a pull request."""

import argparse
import hashlib
import json
import re
import shutil
import tarfile
from pathlib import Path, PurePosixPath

MAX_ARCHIVE_SIZE = 4 * 1024**3
MAX_CRD_SIZE = 10 * 1024**2
MAX_CRDS = 100
BLOB = re.compile(r"blobs/sha256/[0-9a-f]{64}\Z")
CRD = re.compile(r"[a-z0-9][a-z0-9.-]*_[a-z0-9][a-z0-9.-]*\.yaml\Z")
SHA = re.compile(r"[0-9a-f]{40}\Z")
DIGEST = re.compile(r"sha256:[0-9a-f]{64}\Z")
NUMBER = re.compile(r"[1-9][0-9]{0,18}\Z")


def validate_archive(path: Path) -> None:
    if path.is_symlink() or not path.is_file() or path.stat().st_size > MAX_ARCHIVE_SIZE:
        raise ValueError("image archive is missing, a symlink, or too large")
    seen = set()
    total = 0
    with tarfile.open(path, "r:") as archive:
        for member in archive:
            name = member.name.removeprefix("./").rstrip("/")
            if not name or name == ".":
                if member.isdir():
                    continue
                raise ValueError("invalid image archive entry")
            if PurePosixPath(name).is_absolute() or ".." in PurePosixPath(name).parts:
                raise ValueError(f"unsafe archive path: {name}")
            if name in seen:
                raise ValueError(f"duplicate archive entry: {name}")
            seen.add(name)
            if member.isdir():
                if name not in {"blobs", "blobs/sha256"}:
                    raise ValueError(f"unexpected image directory: {name}")
                continue
            if not member.isfile() or not (name in {"oci-layout", "index.json", "manifest.json"} or BLOB.fullmatch(name)):
                raise ValueError(f"unexpected image archive entry: {name}")
            total += member.size
            if total > MAX_ARCHIVE_SIZE or len(seen) > 10000:
                raise ValueError("image archive exceeds limits")
    if not {"oci-layout", "index.json"}.issubset(seen) or not any(BLOB.fullmatch(name) for name in seen):
        raise ValueError("incomplete OCI image archive")


def validate_crds(directory: Path) -> list[Path]:
    if directory.is_symlink() or not directory.is_dir():
        raise ValueError("CRD directory is missing or a symlink")
    paths = sorted(directory.iterdir())
    if not paths or len(paths) > MAX_CRDS:
        raise ValueError("invalid number of CRD files")
    for path in paths:
        if (not CRD.fullmatch(path.name) or path.is_symlink() or not path.is_file()
                or path.stat().st_size > MAX_CRD_SIZE):
            raise ValueError(f"invalid CRD file: {path.name}")
    return paths


def crd_hashes(directory: Path) -> dict[str, str]:
    return {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in validate_crds(directory)}


def image_tag(pr: str, sha: str, run_id: str, attempt: str) -> str:
    if not all(NUMBER.fullmatch(value) for value in (pr, run_id, attempt)) or not SHA.fullmatch(sha):
        raise ValueError("invalid PR, SHA, run ID, or attempt")
    return f"snapshot-pr-{pr}-{sha}-{run_id}-{attempt}"


def create_publication(crds: Path, destination: Path, pr: str, sha: str,
                       run_id: str, attempt: str, digest: str) -> None:
    tag = image_tag(pr, sha, run_id, attempt)
    if not DIGEST.fullmatch(digest):
        raise ValueError("invalid image digest")
    hashes = crd_hashes(crds)
    destination.mkdir(parents=True, exist_ok=False)
    target = destination / "crds"
    target.mkdir()
    for name in hashes:
        shutil.copyfile(crds / name, target / name)
    if crd_hashes(target) != hashes:
        raise ValueError("CRDs changed while preparing publication")
    record = {"pr": pr, "sha": sha, "run_id": run_id, "attempt": attempt,
              "image_tag": tag, "image_digest": digest, "crds": hashes}
    (destination / "publication.json").write_text(json.dumps(record, sort_keys=True) + "\n", encoding="utf-8")


def validate_publication(directory: Path, run_id: str) -> dict:
    record_path = directory / "publication.json"
    if directory.is_symlink() or record_path.is_symlink() or not record_path.is_file() or record_path.stat().st_size > 32768:
        raise ValueError("missing or invalid publication record")
    record = json.loads(record_path.read_text(encoding="utf-8"))
    if not isinstance(record, dict) or set(record) != {"pr", "sha", "run_id", "attempt", "image_tag", "image_digest", "crds"}:
        raise ValueError("invalid publication record")
    if not all(isinstance(record[key], str) for key in ("pr", "sha", "run_id", "attempt", "image_tag", "image_digest")):
        raise ValueError("invalid publication fields")
    expected_tag = image_tag(record["pr"], record["sha"], record["run_id"], record["attempt"])
    if record["run_id"] != run_id or record["image_tag"] != expected_tag or not DIGEST.fullmatch(record["image_digest"]):
        raise ValueError("publication does not match the requested run or image")
    if record["crds"] != crd_hashes(directory / "crds"):
        raise ValueError("CRDs do not match the published record")
    return record


def update_chart(publication: Path, chart: Path, run_id: str) -> None:
    record = validate_publication(publication, run_id)
    chart_crds = chart / "crds"
    chart_yaml = chart / "Chart.yaml"
    if chart_crds.is_symlink() or not chart_crds.is_dir() or chart_yaml.is_symlink() or not chart_yaml.is_file():
        raise ValueError("trusted chart is missing or contains symlinks")
    content = chart_yaml.read_text(encoding="utf-8")
    if len(re.findall(r"^appVersion:.*$", content, flags=re.MULTILINE)) != 1:
        raise ValueError("trusted chart appVersion has changed; review the workflow")
    for old_crd in chart_crds.iterdir():
        if old_crd.is_symlink() or not old_crd.is_file():
            raise ValueError("unexpected entry in trusted chart CRD directory")
        old_crd.unlink()
    for name in record["crds"]:
        shutil.copyfile(publication / "crds" / name, chart_crds / name)
    content = re.sub(r"^appVersion:.*$", f'appVersion: "{record["image_tag"]}"', content, flags=re.MULTILINE)
    chart_yaml.write_text(content, encoding="utf-8")


def main() -> None:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    image = sub.add_parser("validate-image")
    image.add_argument("archive", type=Path)
    crds = sub.add_parser("validate-crds")
    crds.add_argument("directory", type=Path)
    create = sub.add_parser("create-publication")
    create.add_argument("crds", type=Path)
    create.add_argument("destination", type=Path)
    for name in ("pr", "sha", "run_id", "attempt", "digest"):
        create.add_argument(name)
    inspect = sub.add_parser("validate-publication")
    inspect.add_argument("directory", type=Path)
    inspect.add_argument("run_id")
    update = sub.add_parser("update-chart")
    update.add_argument("directory", type=Path)
    update.add_argument("chart", type=Path)
    update.add_argument("run_id")
    args = parser.parse_args()
    if args.command == "validate-image":
        validate_archive(args.archive)
    elif args.command == "validate-crds":
        validate_crds(args.directory)
    elif args.command == "create-publication":
        create_publication(args.crds, args.destination, args.pr, args.sha, args.run_id, args.attempt, args.digest)
    elif args.command == "validate-publication":
        record = validate_publication(args.directory, args.run_id)
        for key in ("pr", "sha", "attempt", "image_tag", "image_digest"):
            print(f"{key}={record[key]}")
    else:
        update_chart(args.directory, args.chart, args.run_id)


if __name__ == "__main__":
    main()
