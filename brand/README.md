# Localizer brand

One mark, used everywhere: a terminal prompt next to a globe, white on indigo.

| File | Use |
| --- | --- |
| `logo.svg` | The mark with rounded corners: the website header and favicon (`site/src/assets/logo.svg` and `site/public/favicon.svg` are copies), the README header, documents. |
| `avatar.svg` | The same mark on a full square, for places that round or crop avatars themselves: the GitHub App, the Polar organization, social profiles. |
| `logo-512.png`, `logo-1024.png` | PNG exports of `logo.svg` with transparent corners. |
| `avatar-512.png` | PNG export of `avatar.svg`: the GitHub App logo. |

Colors: indigo `#4f46e5` (the site's accent in the light theme) and white. Keep the mark's proportions and
colors; don't add effects or place it on busy backgrounds.

To regenerate the PNGs after editing an SVG (needs `rsvg-convert` from librsvg):

```sh
rsvg-convert -w 512 -h 512 brand/logo.svg -o brand/logo-512.png
rsvg-convert -w 1024 -h 1024 brand/logo.svg -o brand/logo-1024.png
rsvg-convert -w 512 -h 512 brand/avatar.svg -o brand/avatar-512.png
rsvg-convert -w 180 -h 180 brand/avatar.svg -o site/public/apple-touch-icon.png
rsvg-convert -w 32 -h 32 brand/logo.svg -o site/public/favicon-32.png
cp brand/logo.svg site/src/assets/logo.svg && cp brand/logo.svg site/public/favicon.svg
```

The social image (`site/public/og.png`) and the ICO favicon (`site/public/favicon.ico`) are made from these
PNGs by `uv run --with pillow python site/scripts/images.py`.
