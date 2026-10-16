#!/usr/bin/env python3
"""Build CClear's multi-resolution Windows icon from the V3 logo master."""

from pathlib import Path

from PIL import Image


def main() -> None:
    root = Path(__file__).resolve().parents[1]
    source = root / "assets" / "cclear-logo-v3.png"
    output = root / "assets" / "cclear-v3.ico"
    sizes = (16, 20, 24, 32, 40, 48, 64, 128, 256)

    with Image.open(source) as image:
        rgba = image.convert("RGBA")
        bounds = rgba.getbbox()
        if bounds is None:
            raise SystemExit("logo master is fully transparent")
        mark = rgba.crop(bounds)
        side = max(mark.size)
        square = Image.new("RGBA", (side, side))
        square.alpha_composite(mark, ((side - mark.width) // 2, (side - mark.height) // 2))
        square.save(output, format="ICO", sizes=[(size, size) for size in sizes])

    print(output, output.stat().st_size)


if __name__ == "__main__":
    main()
