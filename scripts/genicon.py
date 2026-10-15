#!/usr/bin/env python3
"""Write a simple multi-size ICO for CClear."""

import math
import struct
from pathlib import Path


def in_round_rect(x, y, left, top, right, bottom, radius):
    if x < left or x > right or y < top or y > bottom:
        return False
    if left + radius <= x <= right - radius or top + radius <= y <= bottom - radius:
        return True
    for cx, cy in (
        (left + radius, top + radius),
        (right - radius, top + radius),
        (left + radius, bottom - radius),
        (right - radius, bottom - radius),
    ):
        if (x - cx) ** 2 + (y - cy) ** 2 <= radius ** 2:
            return True
    return False


def pixel(size, x, y):
    fx, fy = x + 0.5, y + 0.5
    margin = size * 0.08
    if not in_round_rect(fx, fy, margin, margin, size - margin, size - margin, size * 0.22):
        return (0, 0, 0, 0)
    cx = cy = size / 2
    dist = math.hypot(fx - cx, fy - cy)
    angle = math.atan2(fy - cy, fx - cx)
    gap = abs(angle) < 0.9 and fx > cx
    if size * 0.16 <= dist <= size * 0.30 and not gap:
        return (255, 255, 255, 255)
    return (15, 118, 110, 255)


def dib(size):
    xor_rows = bytearray()
    mask_stride = ((size + 31) // 32) * 4
    mask = bytearray(mask_stride * size)
    for y in range(size - 1, -1, -1):
        for x in range(size):
            r, g, b, a = pixel(size, x, y)
            xor_rows += bytes((b, g, r, a))
            if a < 128:
                mask_index = (size - 1 - y) * mask_stride + (x // 8)
                mask[mask_index] |= 0x80 >> (x % 8)
    header = struct.pack(
        "<IiiHHIIiiII",
        40,
        size,
        size * 2,
        1,
        32,
        0,
        len(xor_rows) + len(mask),
        0,
        0,
        0,
        0,
    )
    return header + xor_rows + mask


def main():
    sizes = (16, 32, 48, 64, 256)
    images = [dib(size) for size in sizes]
    header = struct.pack("<HHH", 0, 1, len(sizes))
    offset = 6 + 16 * len(sizes)
    entries = b""
    blob = b""
    for size, image in zip(sizes, images):
        dim = 0 if size == 256 else size
        entries += struct.pack("<BBBBHHII", dim, dim, 0, 0, 1, 32, len(image), offset)
        offset += len(image)
        blob += image
    out = Path(__file__).resolve().parents[1] / "assets" / "cclear.ico"
    out.write_bytes(header + entries + blob)
    print(out, out.stat().st_size)


if __name__ == "__main__":
    main()
