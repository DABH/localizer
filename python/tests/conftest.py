# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

import json
import pathlib

import pytest

CONFORMANCE = pathlib.Path(__file__).resolve().parents[2] / "testdata" / "conformance"


@pytest.fixture(scope="session")
def corpus():
    """Loads a conformance corpus file by name."""

    def load(name):
        if not CONFORMANCE.is_dir():  # an sdist carries the tests but not the repository's corpus
            pytest.skip("conformance corpus not present (run from a repository checkout)")
        with open(CONFORMANCE / name, encoding="utf-8") as f:
            return json.load(f)

    return load
