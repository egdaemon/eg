import os
from setuptools import setup, find_packages

setup(
  name="egpy",
  version=os.environ.get("EGPY_VERSION", "0.0.0"),
  packages=find_packages(),
)
