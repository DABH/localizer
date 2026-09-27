# Copyright (c) 2026 Snizyx Software LLC. All rights reserved.
# SPDX-License-Identifier: NCSA

"""Render a Typer, Click or argparse CLI's own strings in the user's language.

The usual integration is one line before the app runs::

    import localizer
    localizer.localize(app, "yourcli.locales")

where ``yourcli/locales/`` holds one ``<language>.json`` catalog per language (see
https://locale.dev/reference/catalogs/). Language selection follows ``LOCALIZER_LANG`` (or an
application-specific variable given as ``env_var``), then ``LC_ALL``, ``LC_MESSAGES``, ``LANG``, then the
operating system's preferred languages; ``LOCALIZER_LANG=off`` disables localization and
``LOCALIZER_LANG=qps`` pseudo-localizes every known string.

Messages your own code prints go through :func:`t` (or :func:`tf` for format strings) at your output
chokepoints; :func:`error` translates an exception for display. Everything fails open to English.
"""

from ._api import ENV_LANG, Mode, error, init, lang, localize, t, tf, translate, uninstall

try:
    from ._version import __version__
except ImportError:  # a source checkout without the build hook
    __version__ = "0.0.0"

__all__ = ["ENV_LANG", "Mode", "__version__", "error", "init", "lang", "localize", "t", "tf", "translate", "uninstall"]
