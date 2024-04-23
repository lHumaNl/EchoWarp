block_cipher = None

a = Analysis(['echowarp/main.py'],
             pathex=['.'],
             binaries=[],
             datas=[],
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
          exclude_binaries=True,
          name='EchoWarp',
          debug=False,
          bootloader_ignore_signals=False,
          strip=False,
          upx=True,
          console=True )

coll = COLLECT(exe,
               a.binaries,
               a.zipfiles,
               a.datas,
               strip=None,
               upx=True,
               name='EchoWarp',
               upx_exclude=[],
               runtime_tmpdir=None,
               console=True )
