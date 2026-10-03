#!/usr/bin/env python3
"""Regenerates the site's social image and favicon.ico from the brand assets.

    uv run --with pillow python site/scripts/images.py

Writes site/public/og.png (1200×630: the mark, the name, the tagline and the domain on the site's dark
background) and site/public/favicon.ico (16, 32 and 48 px, from brand/logo-512.png). Text is set in the
first font of FONTS present on the machine, so the image differs slightly between systems; the committed
file was made on macOS with SF Pro.
"""

from __future__ import annotations

import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parents[2]
BRAND = ROOT / "brand"
PUBLIC = ROOT / "site" / "public"

WIDTH, HEIGHT = 1200, 630
MARGIN = 100
BACKGROUND = "#17181c"  # Starlight's dark --sl-color-black, hsl(224 10% 10%)
NAME = "#ffffff"
TAGLINE = "#c7d2fe"  # --sl-color-accent-high in the dark theme
DOMAIN = "#a5b4fc"  # the prompt color in the home page's terminal

# (bold face, regular face); a face is a path and either a collection index or a variable-font instance.
FONTS = [
    (("/System/Library/Fonts/SFNS.ttf", "Bold"), ("/System/Library/Fonts/SFNS.ttf", "Regular")),  # macOS
    (("/System/Library/Fonts/HelveticaNeue.ttc", 1), ("/System/Library/Fonts/HelveticaNeue.ttc", 0)),
    (
        ("/System/Library/Fonts/Supplemental/Arial Bold.ttf", None),
        ("/System/Library/Fonts/Supplemental/Arial.ttf", None),
    ),
    (
        ("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", None),  # Debian, Ubuntu
        ("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", None),
    ),
    (
        ("/usr/share/fonts/truetype/liberation/LiberationSans-Bold.ttf", None),
        ("/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf", None),
    ),
]


def load(face: tuple[str, int | str | None], size: int) -> ImageFont.FreeTypeFont:
    path, variant = face
    if isinstance(variant, int):
        return ImageFont.truetype(path, size, index=variant)
    font = ImageFont.truetype(path, size)
    if variant:
        font.set_variation_by_name(variant)
    return font


def fonts():
    """The first (bold, regular) pair of FONTS that loads, as functions of the size."""
    for bold, regular in FONTS:
        try:
            load(bold, 10), load(regular, 10)
        except OSError:
            continue
        return (lambda size: load(bold, size)), (lambda size: load(regular, size))
    sys.exit("images.py: none of the fonts in FONTS is installed; add one")


def fit(draw: ImageDraw.ImageDraw, text: str, font, size: int, width: int):
    """The font at `size`, or smaller if `text` would be wider than `width`."""
    while size > 20 and draw.textlength(text, font=font(size)) > width:
        size -= 2
    return font(size)


def social_image() -> Path:
    bold, regular = fonts()
    image = Image.new("RGB", (WIDTH, HEIGHT), BACKGROUND)
    draw = ImageDraw.Draw(image)
    width = WIDTH - 2 * MARGIN

    mark = Image.open(BRAND / "logo-1024.png").convert("RGBA").resize((136, 136), Image.LANCZOS)
    top = 128
    image.paste(mark, (MARGIN, top), mark)
    draw.text((MARGIN + 136 + 40, top + 68), "Localizer", font=bold(112), fill=NAME, anchor="lm")

    tagline = "Your CLI, in your users’ language"
    draw.text((MARGIN, 386), tagline, font=fit(draw, tagline, regular, 60, width), fill=TAGLINE, anchor="ls")
    draw.text((MARGIN, HEIGHT - MARGIN), "locale.dev", font=bold(42), fill=DOMAIN, anchor="ls")

    out = PUBLIC / "og.png"
    image.save(out, optimize=True)
    return out


def favicon() -> Path:
    out = PUBLIC / "favicon.ico"
    logo = Image.open(BRAND / "logo-512.png").convert("RGBA")
    logo.save(out, sizes=[(16, 16), (32, 32), (48, 48)], bitmap_format="bmp")
    return out


if __name__ == "__main__":
    for path in (social_image(), favicon()):
        print(f"wrote {path.relative_to(ROOT)} ({path.stat().st_size:,} bytes)")
