# -*- mode: python ; coding: utf-8 -*-

block_cipher = None

a = Analysis(['./echowarp/main.py'],
             pathex=['./echowarp/'],
             binaries=[],
             datas=[('echowarp/*', 'echowarp')],
             hiddenimports=['pyaudio', 'cryptography'],
             hookspath=[],
             runtime_hooks=[],
             excludes=[],
             cipher=block_cipher,
             noarchive=False)

pyz = PYZ(a.pure, a.zipped_data,
          cipher=block_cipher)

exe = EXE(pyz,
          a.scripts,
          [],
          exclude_binaries=True,
          name='EchoWarp',
          debug=False,
          bootloader_ignore_signals=False,
          strip=False,
          upx=True,
          console=True )
