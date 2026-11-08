import importlib.util
import io
import json
from pathlib import Path
import struct
import tempfile
import unittest
from unittest.mock import patch
import zipfile

spec = importlib.util.spec_from_file_location('bundle_tools', Path(__file__).with_name('bundle-tools.py'))
bundle = importlib.util.module_from_spec(spec)
spec.loader.exec_module(bundle)


class BundleToolsTests(unittest.TestCase):
    def test_binary_headers_distinguish_platforms_and_architectures(self):
        elf = bytearray(64)
        elf[:4] = b'\x7fELF'
        struct.pack_into('<H', elf, 18, 183)
        self.assertEqual(bundle.executable_target(elf), ('linux', 'arm64'))
        pe = bytearray(128)
        pe[:2] = b'MZ'
        struct.pack_into('<I', pe, 60, 64)
        pe[64:68] = b'PE\0\0'
        struct.pack_into('<H', pe, 68, 0x8664)
        self.assertEqual(bundle.executable_target(pe), ('win32', 'x64'))
        mach = b'\xcf\xfa\xed\xfe' + struct.pack('<I', 0x100000c)
        self.assertEqual(bundle.executable_target(mach), ('darwin', 'arm64'))
        with self.assertRaises(ValueError):
            bundle.executable_target(b'not an executable')

    def test_wrong_architecture_and_archive_paths_are_rejected_or_contained(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            archive = io.BytesIO()
            elf = bytearray(64)
            elf[:4] = b'\x7fELF'
            struct.pack_into('<H', elf, 18, 183)  # ARM64 binary in an x64 package.
            with zipfile.ZipFile(archive, 'w') as output:
                output.writestr('root/bin/ffmpeg', elf)
                output.writestr('root/../../escaped', 'must not escape')
            lock = directory / 'lock.json'
            lock.write_text(json.dumps({'linux-x64': {'files': [{'format': 'archive', 'tools': ['ffmpeg'], 'license_directory': 'licenses/ffmpeg'}]}}))
            with patch.object(bundle, 'LOCK', lock), patch.object(bundle, 'download', return_value=archive.getvalue()):
                with self.assertRaisesRegex(ValueError, 'Wrong executable architecture'):
                    bundle.stage('linux', 'x64', directory / 'output')
            self.assertFalse((directory / 'escaped').exists())

    def test_ytdlp_cannot_be_accidentally_shipped(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            (directory / 'yt-dlp.exe').write_bytes(b'not bundled')
            with self.assertRaisesRegex(ValueError, 'never be included'):
                bundle.stage('linux', 'x64', directory)


if __name__ == '__main__':
    unittest.main()
