#!/usr/bin/env python3
"""Generate a Cursor-style DMG background with a 3D perspective grid."""

from __future__ import annotations

from pathlib import Path

from PIL import Image, ImageDraw

WINDOW_W = 660
WINDOW_H = 400
GRID = (210, 210, 210)
ARROW = (20, 20, 20)
LINE = 1


def render(scale: int) -> Image.Image:
    width = WINDOW_W * scale
    height = WINDOW_H * scale
    horizon_y = int(height * 0.58)
    spacing = 40 * scale

    img = Image.new("RGB", (width, height), "white")
    draw = ImageDraw.Draw(img)

    for x in range(0, width + 1, spacing):
        draw.line([(x, 0), (x, horizon_y)], fill=GRID, width=LINE)
    for y in range(0, horizon_y + 1, spacing):
        draw.line([(0, y), (width, y)], fill=GRID, width=LINE)

    vanish_x = width // 2
    floor_h = height - horizon_y
    rows = max(8, floor_h // spacing)
    for i in range(rows + 1):
        t = i / rows
        y = horizon_y + int(floor_h * t)
        spread = int(width * (0.08 + 0.42 * t))
        draw.line(
            [(vanish_x - spread, y), (vanish_x + spread, y)],
            fill=GRID,
            width=LINE,
        )

    cols = width // spacing + 2
    for i in range(-cols // 2, cols // 2 + 1):
        x_top = vanish_x + i * spacing
        x_bottom = vanish_x + int(i * spacing * 2.8)
        draw.line([(x_top, horizon_y), (x_bottom, height)], fill=GRID, width=LINE)

    arrow_x = int(((180 + 64) + (480 + 64)) / 2 * scale)
    arrow_y = int((168 + 64) * scale)
    length = 60 * scale
    head = 18 * scale
    shaft = max(2, 2 * scale)
    x1 = arrow_x - length // 2
    x2 = arrow_x + length // 2
    draw.line([(x1, arrow_y), (x2 - head, arrow_y)], fill=ARROW, width=shaft)
    draw.polygon(
        [
            (x2, arrow_y),
            (x2 - head, arrow_y - head // 2),
            (x2 - head, arrow_y + head // 2),
        ],
        fill=ARROW,
    )
    return img


def main() -> None:
    out_dir = Path(__file__).resolve().parent
    for scale, name in ((1, "dmg-background.png"), (2, "dmg-background@2x.png")):
        img = render(scale)
        path = out_dir / name
        img.save(path, "PNG", dpi=(72, 72))
        print(f"Wrote {path} ({img.width}x{img.height})")


if __name__ == "__main__":
    main()
