from setuptools import setup, find_packages

setup(
    name="yakapi",
    version="0.1.0",
    description="A Python client for the YakAPI streaming service",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    author="Rhett Garber",
    author_email="rhettg@gmail.com",
    url="https://github.com/rhettg/yakapi",
    package_dir={"": "src"},
    packages=find_packages(where="src"),
    install_requires=[
        "aiohttp>=3.10.5",
        "python-ulid>=2.7.0",
    ],
    classifiers=[
        "Development Status :: 3 - Alpha",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.12",
    ],
    python_requires=">=3.12",
)
