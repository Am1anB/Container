"""
Augment crop_img data to expand training set.
Reads train.txt + val.txt, generates ~10 variants per image.
Output: augmented_crops/ folder + augmented_labels.txt
"""
import os
import random
import numpy as np
from PIL import Image, ImageEnhance, ImageFilter

CROP_DIR = "../../crop_img"
OUT_DIR = "augmented_crops"
LABEL_FILES = ["../../train.txt", "../../val.txt"]
OUT_LABEL = "augmented_labels.txt"

random.seed(42)
np.random.seed(42)


def load_labels():
    pairs = []
    for path in LABEL_FILES:
        if not os.path.exists(path):
            continue
        with open(path, encoding="utf-8") as f:
            lines = [l.strip() for l in f if l.strip()]
        i = 0
        while i < len(lines) - 1:
            p0 = lines[i].split("\t")
            p1 = lines[i + 1].split("\t")
            if "_crop_0" in p0[0] and "_crop_1" in p1[0]:
                base = p0[0].replace("_crop_0.jpg", "")
                label = p0[1] + p1[1]
                pairs.append((base, label))
                i += 2
            else:
                i += 1
    return pairs


def augment(img: Image.Image):
    variants = [img]

    # Rotation
    for angle in [-10, -5, 5, 10]:
        variants.append(img.rotate(angle, expand=False, fillcolor=(128, 128, 128)))

    # Brightness
    for factor in [0.65, 0.8, 1.2, 1.4]:
        variants.append(ImageEnhance.Brightness(img).enhance(factor))

    # Contrast
    for factor in [0.7, 1.4]:
        variants.append(ImageEnhance.Contrast(img).enhance(factor))

    # Sharpness
    variants.append(ImageEnhance.Sharpness(img).enhance(2.5))

    # Blur
    variants.append(img.filter(ImageFilter.GaussianBlur(radius=1)))

    # Noise
    arr = np.array(img, dtype=np.float32)
    noise = np.random.normal(0, 12, arr.shape)
    noisy = np.clip(arr + noise, 0, 255).astype(np.uint8)
    variants.append(Image.fromarray(noisy))

    # Grayscale → RGB
    gray = img.convert("L").convert("RGB")
    variants.append(gray)

    return variants


def main():
    os.makedirs(OUT_DIR, exist_ok=True)
    pairs = load_labels()
    print(f"Loaded {len(pairs)} label pairs")

    label_lines = []
    saved = 0

    for base, label in pairs:
        for suffix in ["_crop_0", "_crop_1"]:
            src = os.path.join(CROP_DIR, f"{os.path.basename(base)}{suffix}.jpg")
            if not os.path.exists(src):
                continue
            img = Image.open(src).convert("RGB")

            # Original
            out_name = f"{os.path.basename(base)}{suffix}_orig.jpg"
            out_path = os.path.join(OUT_DIR, out_name)
            img.save(out_path)
            part_label = label[:4] if suffix == "_crop_0" else label[4:]
            label_lines.append(f"{out_path}\t{part_label}")
            saved += 1

            # Augmented variants
            for idx, aug_img in enumerate(augment(img)[1:], 1):
                out_name = f"{os.path.basename(base)}{suffix}_aug{idx}.jpg"
                out_path = os.path.join(OUT_DIR, out_name)
                aug_img.save(out_path)
                label_lines.append(f"{out_path}\t{part_label}")
                saved += 1

    with open(OUT_LABEL, "w", encoding="utf-8") as f:
        for line in label_lines:
            f.write(line + "\n")

    print(f"Saved {saved} images to {OUT_DIR}/")
    print(f"Label file: {OUT_LABEL} ({len(label_lines)} lines)")


if __name__ == "__main__":
    main()
