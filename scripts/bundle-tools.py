#!/usr/bin/env python3
"""Stage pinned, checksum-verified executables; never bundle yt-dlp."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import struct
import tarfile
import tempfile
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parent.parent
LOCK = ROOT / 'packaging/tools.lock.json'


def download(item):
    checksum = item['sha256']
    if len(checksum) != 64:
        raise ValueError('A pinned SHA-256 checksum is required')
    cache = ROOT / '.cache/media-tools' / checksum
    if cache.is_file() and hashlib.sha256(cache.read_bytes()).hexdigest() == checksum:
        return cache.read_bytes()
    print('Downloading ' + item['url'], flush=True)
    request = urllib.request.Request(item['url'], headers={'User-Agent': 'MediaCLI-build'})
    with urllib.request.urlopen(request, timeout=180) as response:
        chunks = []
        received = 0
        while True:
            chunk = response.read(8 * 1024 * 1024)
            if not chunk:
                break
            chunks.append(chunk)
            received += len(chunk)
            print(f'  {received // (1024 * 1024)} MiB', flush=True)
        data = b''.join(chunks)
    if hashlib.sha256(data).hexdigest() != checksum:
        raise ValueError('SHA-256 mismatch: ' + item['url'])
    cache.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=cache.parent, delete=False) as output:
        output.write(data)
        temporary = output.name
    os.replace(temporary, cache)
    return data


def archive_files(data):
    if zipfile.is_zipfile(io.BytesIO(data)):
        with zipfile.ZipFile(io.BytesIO(data)) as archive:
            for entry in archive.infolist():
                if not entry.is_dir():
                    yield entry.filename, archive.read(entry)
    else:
        with tarfile.open(fileobj=io.BytesIO(data)) as archive:
            for entry in archive:
                if entry.isfile():
                    yield entry.name, archive.extractfile(entry).read()


def executable_target(data):
    if data[:4] == b'\x7fELF':
        return 'linux', {62: 'x64', 183: 'arm64'}.get(struct.unpack_from('<H', data, 18)[0])
    if data[:4] == b'\xcf\xfa\xed\xfe':
        return 'darwin', {0x1000007: 'x64', 0x100000c: 'arm64'}.get(struct.unpack_from('<I', data, 4)[0])
    if data[:2] == b'MZ':
        offset = struct.unpack_from('<I', data, 60)[0]
        if data[offset:offset + 4] != b'PE\0\0':
            raise ValueError('Invalid PE executable')
        return 'win32', {0x8664: 'x64', 0xaa64: 'arm64'}.get(struct.unpack_from('<H', data, offset + 4)[0])
    raise ValueError('Unknown executable format')


def stage(platform, arch, destination):
    key = platform + '-' + arch
    lock = json.loads(LOCK.read_text())[key]
    if destination.exists() and any(path.name.startswith('yt-dlp') for path in destination.iterdir()):
        raise ValueError('yt-dlp must never be included in a build')
    destination.mkdir(parents=True, exist_ok=True)
    receipt = {'platform': key, 'files': {}, 'sources': lock}
    extension = '.exe' if platform == 'win32' else ''
    for item in lock['files']:
        data = download(item)
        if item['format'] == 'archive':
            expected = {tool + extension for tool in item['tools']}
            found = set()
            for name, content in archive_files(data):
                basename = Path(name).name
                if basename in expected:
                    if basename in found:
                        raise ValueError('Duplicate tool in archive: ' + basename)
                    output = destination / basename
                    output.write_bytes(content)
                    found.add(basename)
                elif item.get('license_directory') and '/bin/' not in name and not basename.startswith('.'):
                    # Keep upstream licenses, build configuration and documentation.
                    parts = Path(name).parts[1:]
                    if not parts or '..' in parts:
                        continue
                    output = destination / item['license_directory'] / Path(*parts)
                    output.parent.mkdir(parents=True, exist_ok=True)
                    output.write_bytes(content)
            if expected != found:
                raise ValueError('Missing tools in archive: ' + repr(expected - found))
        else:
            output = destination / item['output']
            output.parent.mkdir(parents=True, exist_ok=True)
            output.write_bytes(gzip.decompress(data) if item['format'] == 'gzip' else data)
    for tool in ['ffmpeg', 'ffprobe', 'deno']:
        output = destination / (tool + extension)
        if executable_target(output.read_bytes()) != (platform, arch):
            raise ValueError('Wrong executable architecture: ' + str(output))
        output.chmod(0o755)
    shutil.copytree(ROOT / 'packaging/licenses', destination / 'licenses/mediacli-bundle', dirs_exist_ok=True)
    for output in sorted(destination.rglob('*')):
        if output.is_file() and output.name != 'tools.json' and output.name not in ['mediacli', 'mediacli.exe', 'build.json']:
            receipt['files'][output.relative_to(destination).as_posix()] = hashlib.sha256(output.read_bytes()).hexdigest()
    (destination / 'tools.json').write_text(json.dumps(receipt, indent=2) + '\n')
    print('Bundled FFmpeg, ffprobe and Deno for ' + key, flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('platform', choices=['linux', 'darwin', 'win32'])
    parser.add_argument('arch', choices=['x64', 'arm64'])
    parser.add_argument('destination', type=Path)
    arguments = parser.parse_args()
    stage(arguments.platform, arguments.arch, arguments.destination)
