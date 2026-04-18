"""
Evaluate F1 / Precision / Recall on val set.
Runs twice:
  1. "After augment"  — all 300 val samples (orig + aug)
  2. "Before augment" — only the 20 _orig samples

Metrics are computed at two levels:
  - Character level  (treats each char as a token)
  - Word (sequence) level  (full container number must match exactly)

Usage:
    python eval_metrics.py
"""

import os
import sys
from paddleocr import PaddleOCR
import paddleocr

# ── paths ──────────────────────────────────────────────────────────────────────
BASE = os.path.dirname(os.path.abspath(__file__))
DATASET_DIR = os.path.join(BASE, "finetune_dataset")
VAL_FILE = os.path.join(DATASET_DIR, "rec_gt_val.txt")

ocr_module_path = os.path.dirname(paddleocr.__file__)
dict_path = os.path.join(ocr_module_path, "ppocr", "utils", "en_dict.txt")

ocr = PaddleOCR(
    use_angle_cls=False,
    lang="en",
    det=False,          # recognition only — images are already cropped
    rec=True,
    rec_model_dir=os.path.join(BASE, "inference", "rec_model_finetuned", "inference"),
    rec_char_dict_path=dict_path,
    use_gpu=False,
    show_log=False,
)


def predict_text(img_path: str) -> str:
    result = ocr.ocr(img_path, det=False, cls=False)
    if result and result[0]:
        return result[0][0][0].strip().upper()
    return ""


def char_metrics(pred: str, gt: str):
    """Character-level TP/FP/FN treating each position as a binary label."""
    tp = sum(p == g for p, g in zip(pred, gt))
    fp = max(0, len(pred) - tp)
    fn = max(0, len(gt) - tp)
    return tp, fp, fn


def evaluate(samples: list[tuple[str, str]], label: str):
    print(f"\n{'='*60}")
    print(f"  {label}  ({len(samples)} samples)")
    print(f"{'='*60}")

    char_tp = char_fp = char_fn = 0
    word_correct = 0
    errors = []

    for img_rel, gt in samples:
        img_path = os.path.join(DATASET_DIR, img_rel)
        if not os.path.exists(img_path):
            continue
        pred = predict_text(img_path)

        # word level
        if pred == gt:
            word_correct += 1
        else:
            errors.append((img_rel, gt, pred))

        # char level
        tp, fp, fn = char_metrics(pred, gt)
        char_tp += tp
        char_fp += fp
        char_fn += fn

    total = len(samples)

    # ── Word-level ─────────────────────────────────────────────────────────────
    word_acc = word_correct / total if total else 0
    # word precision/recall/F1 (binary: correct=TP, wrong=FP+FN)
    w_precision = word_correct / total if total else 0   # same as accuracy here
    w_recall    = word_correct / total if total else 0
    w_f1 = w_precision  # same when TP/(TP+FP) == TP/(TP+FN)

    # ── Char-level ─────────────────────────────────────────────────────────────
    c_precision = char_tp / (char_tp + char_fp) if (char_tp + char_fp) else 0
    c_recall    = char_tp / (char_tp + char_fn) if (char_tp + char_fn) else 0
    c_f1 = (2 * c_precision * c_recall / (c_precision + c_recall)
            if (c_precision + c_recall) else 0)

    print(f"\n[Word / Sequence level]")
    print(f"  Accuracy  : {word_acc*100:.2f}%  ({word_correct}/{total})")
    print(f"  Precision : {w_precision*100:.2f}%")
    print(f"  Recall    : {w_recall*100:.2f}%")
    print(f"  F1 Score  : {w_f1*100:.2f}%")

    print(f"\n[Character level]")
    print(f"  Precision : {c_precision*100:.2f}%")
    print(f"  Recall    : {c_recall*100:.2f}%")
    print(f"  F1 Score  : {c_f1*100:.2f}%")

    if errors:
        print(f"\n[Sample errors — first 10]")
        for img, gt, pred in errors[:10]:
            print(f"  GT: {gt:<12}  PRED: {pred:<12}  ({os.path.basename(img)})")

    return {
        "word_acc": word_acc, "word_f1": w_f1,
        "char_precision": c_precision, "char_recall": c_recall, "char_f1": c_f1,
    }


def load_val_samples(orig_only=False):
    samples = []
    with open(VAL_FILE, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            parts = line.split("\t")
            if len(parts) < 2:
                continue
            img_rel, gt = parts[0], parts[1].strip().upper()
            if orig_only and "_orig" not in img_rel:
                continue
            samples.append((img_rel, gt))
    return samples


if __name__ == "__main__":
    print("Loading val set …")
    all_samples  = load_val_samples(orig_only=False)
    orig_samples = load_val_samples(orig_only=True)

    evaluate(all_samples,  "AFTER AUGMENT  (all val: orig + augmented)")
    evaluate(orig_samples, "BEFORE AUGMENT (original images only)")
