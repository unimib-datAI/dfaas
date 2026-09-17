#!/usr/bin/env python3
# /// script
# dependencies = [
#   "pymupdf",
# ]
# ///

import argparse
import math

import pymupdf


def choose_layout(n):
    """Choose columns and rows automatically."""
    cols = math.ceil(math.sqrt(n))
    rows = math.ceil(n / cols)

    # Prefer wider layouts for readability
    if n <= 2:
        cols = 1
    elif n <= 6:
        cols = 2

    rows = math.ceil(n / cols)
    return cols, rows


def combine_pdfs(files, output):
    n = len(files)

    cols, rows = choose_layout(n)

    # Size per plot panel (points)
    panel_width = 280
    panel_height = 220

    margin = 30
    spacing = 15

    # Automatically size the page
    page_width = 2 * margin + cols * panel_width + (cols - 1) * spacing

    page_height = 2 * margin + rows * panel_height + (rows - 1) * spacing

    doc = pymupdf.open()
    page = doc.new_page(
        width=page_width,
        height=page_height,
    )

    for i, pdf_file in enumerate(files):
        row = i // cols
        col = i % cols

        x0 = margin + col * (panel_width + spacing)
        y0 = margin + row * (panel_height + spacing)

        rect = pymupdf.Rect(
            x0,
            y0,
            x0 + panel_width,
            y0 + panel_height,
        )

        src = pymupdf.open(pdf_file)
        page.show_pdf_page(rect, src, 0)
        src.close()

    doc.save(output)
    doc.close()

    print(
        f"Created {output}: "
        f"{n} plots, {cols} columns × {rows} rows, "
        f"page {page_width:.0f}×{page_height:.0f} pt"
    )


def main():
    parser = argparse.ArgumentParser(
        description="Combine PDF plots into an automatically sized page"
    )

    parser.add_argument(
        "plots",
        nargs="+",
        help="PDF plot files",
    )

    parser.add_argument(
        "-o",
        "--output",
        default="combined_plots.pdf",
    )

    args = parser.parse_args()

    combine_pdfs(args.plots, args.output)


if __name__ == "__main__":
    main()
