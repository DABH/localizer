# localizer-py → localizer

Localizer's Python runtime is published on PyPI as **[`localizer`](https://pypi.org/project/localizer/)**.
`localizer-py` was the name of its first release, when `localizer` still belonged to an unrelated project;
that project's author handed the name over.

This distribution contains no code. It depends on `localizer`, so `pip install localizer-py` keeps working
and installs the real package. Switch your dependency to `localizer` when convenient; the import name has
always been `localizer`.

## Upgrading from localizer-py 0.5.0

Release 0.5.0 of `localizer-py` contained the `localizer` package itself. `pip install -U localizer-py`
installs `localizer` and then uninstalls 0.5.0, whose file list covers the files just installed, so pip
can remove them and leave `import localizer` broken. Upgrade in two steps instead:

```sh
pip uninstall -y localizer-py && pip install localizer
```

If the package is already broken, `pip install --force-reinstall --no-deps localizer` puts the files back.

Documentation: https://locale.dev/
