from setuptools import setup, find_packages

setup(
    name="nyx_package",
    version="0.1.0",
    py_modules=["cli"],
    packages=find_packages(),
    install_requires=[
        "click",
        "seleniumbase",
        "flask",
    ],
    entry_points={
        "console_scripts": [
            "nyx=app.cli:main",
        ],
    },
)
