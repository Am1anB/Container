"""
Prepare augmented crop data into PaddleX OCR recognition dataset format.

Output structure:
  finetune_dataset/
    rec_gt_train.txt
    rec_gt_val.txt
    images/  (symlinked from augmented_crops/)

Run: py -3.13 prepare_finetune_dataset.py
  OR: .venv/Scripts/python.exe prepare_finetune_dataset.py
"""
import os
import shutil
import random

AUGMENTED_LABEL = "augmented_labels.txt"
DATASET_DIR = "finetune_dataset"
IMAGES_DIR = os.path.join(DATASET_DIR, "images")
TRAIN_FILE = os.path.join(DATASET_DIR, "rec_gt_train.txt")
VAL_FILE = os.path.join(DATASET_DIR, "rec_gt_val.txt")
VAL_RATIO = 0.1

random.seed(42)


def main():
    if not os.path.exists(AUGMENTED_LABEL):
        print(f"ERROR: {AUGMENTED_LABEL} not found. Run augment_data.py first.")
        return

    os.makedirs(IMAGES_DIR, exist_ok=True)

    pairs = []
    with open(AUGMENTED_LABEL, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            parts = line.split("\t")
            if len(parts) >= 2:
                src_path = parts[0]
                label = parts[1].strip().upper()
                if os.path.exists(src_path) and label:
                    pairs.append((src_path, label))

    print(f"Loaded {len(pairs)} pairs")

    # Copy images to dataset/images/
    print("Copying images...")
    new_pairs = []
    for src, label in pairs:
        fname = os.path.basename(src)
        dst = os.path.join(IMAGES_DIR, fname)
        if not os.path.exists(dst):
            shutil.copy2(src, dst)
        new_pairs.append((f"images/{fname}", label))

    # Split train/val (by unique base image, not by augmented variant)
    # Group by original image id so augmented variants stay in same split
    from collections import defaultdict
    groups = defaultdict(list)
    for rel_path, label in new_pairs:
        fname = os.path.basename(rel_path)
        # base key: strip augmentation suffix
        base = fname.split("_aug")[0].split("_orig")[0]
        groups[base].append((rel_path, label))

    base_keys = list(groups.keys())
    random.shuffle(base_keys)
    val_n = max(1, int(len(base_keys) * VAL_RATIO))
    val_keys = set(base_keys[:val_n])
    train_keys = set(base_keys[val_n:])

    train_lines = []
    val_lines = []
    for key in train_keys:
        for rel_path, label in groups[key]:
            train_lines.append(f"{rel_path}\t{label}")
    for key in val_keys:
        for rel_path, label in groups[key]:
            val_lines.append(f"{rel_path}\t{label}")

    with open(TRAIN_FILE, "w", encoding="utf-8") as f:
        f.write("\n".join(train_lines) + "\n")
    with open(VAL_FILE, "w", encoding="utf-8") as f:
        f.write("\n".join(val_lines) + "\n")

    print(f"Train: {len(train_lines)} samples")
    print(f"Val:   {len(val_lines)} samples")
    print(f"Dataset ready at: {DATASET_DIR}/")


if __name__ == "__main__":
    main()
