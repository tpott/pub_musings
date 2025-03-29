from setuptools import setup, find_packages

setup(
    name="werewolf",
    version="0.1.0",
    packages=find_packages(),
    install_requires=[
        "requests>=2.25.0",
    ],
    entry_points={
        "console_scripts": [
            "werewolf=main:main",
        ],
    },
)
