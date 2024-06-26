from setuptools import setup, find_packages
import version

setup(
    name='echowarp',
    version=version.__version__,
    packages=find_packages(include=['echowarp', 'echowarp.*']),
    include_package_data=True,
    package_data={
        '': ['../version.py'],
    },
    install_requires=[
        'PyAudio',
        'cryptography',
        'chardet'
    ],
    entry_points={
        'console_scripts': [
            'echowarp=echowarp.main:main',
        ],
    },
    author='lHumaNl',
    author_email='fisher_sam@mail.ru',
    description='Network audio streaming tool using UDP',
    long_description=open('README.md').read(),
    long_description_content_type='text/markdown',
    url='https://github.com/lHumaNl/EchoWarp',
    license='MIT',
    classifiers=[
        'Development Status :: 3 - Alpha',
        'Intended Audience :: Developers',
        'Topic :: Multimedia :: Sound/Audio',
        'License :: OSI Approved :: MIT License',
        'Programming Language :: Python :: 3',
        'Programming Language :: Python :: 3.6',
    ],
    keywords='audio streaming network',
)
