"""
Evaluate F1 / Precision / Recall on unseen test set (uploads/ + ground_truth.csv).
Uses the full OCR pipeline: detection + recognition + container extraction.

Usage:
    python eval_unseen.py
"""

import os
import csv
import re
from paddleocr import PaddleOCR
import paddleocr

BASE = os.path.dirname(os.path.abspath(__file__))
UPLOADS_DIR = os.path.join(BASE, "uploads")
GT_CSV = os.path.join(BASE, "ground_truth.csv")

ocr_module_path = os.path.dirname(paddleocr.__file__)
dict_path = os.path.join(ocr_module_path, "ppocr", "utils", "en_dict.txt")

ocr = PaddleOCR(
    use_angle_cls=True,
    lang="en",
    det_model_dir=os.path.join(BASE, "inference", "det_model"),
    rec_model_dir=os.path.join(BASE, "inference", "rec_model_finetuned", "inference"),
    rec_char_dict_path=dict_path,
    use_gpu=False,
    det_db_thresh=0.3,
    det_db_box_thresh=0.5,
    det_db_unclip_ratio=1.6,
    show_log=False,
)

container_re = re.compile(r"[A-Z]{4}[\s\-_.]*(?:\d[\s\-_.]*){6}\d")
look_like_digit = {"O": "0", "Q": "0", "D": "0", "I": "1", "L": "1",
                   "Z": "2", "S": "5", "G": "6", "B": "8"}
look_like_letter = {"0": "O", "1": "I", "2": "Z", "5": "S", "8": "B"}


def force_container(s: str) -> str:
    u = s.upper()
    buf = [r for r in u if r.isalpha() or r.isdigit()]
    if len(buf) < 11:
        return ""
    out = []
    for r in buf[:4]:
        if r.isdigit():
            r = look_like_letter.get(r, "A")
        out.append(r)
    i = 4
    while len(out) < 11 and i < len(buf):
        r = buf[i]
        if r.isalpha():
            r = look_like_digit.get(r, "0")
        out.append(r)
        i += 1
    return "".join(out)


def extract_container(text: str) -> str:
    up = text.upper()
    m = container_re.search(up)
    if m:
        n = force_container(m.group())
        if n:
            return n
    return force_container(up)


def predict(img_path: str) -> str:
    result = ocr.ocr(img_path, cls=True)
    candidates = []
    if result and result[0]:
        for line in result[0]:
            text, score = line[1]
            if score >= 0.3:
                candidates.append((text, score))
    candidates.sort(key=lambda x: x[1], reverse=True)
    for text, _ in candidates:
        n = extract_container(text)
        if n:
            return n
    full = " ".join(t for t, _ in candidates)
    return extract_container(full)


def char_metrics(pred: str, gt: str):
    tp = sum(p == g for p, g in zip(pred, gt))
    fp = max(0, len(pred) - tp)
    fn = max(0, len(gt) - tp)
    return tp, fp, fn


def main():
    rows = []
    with open(GT_CSV, encoding="utf-8") as f:
        for row in csv.DictReader(f):
            fname = row["filename"].strip()
            gt = row["ground_truth"].strip().upper()
            img_path = os.path.join(UPLOADS_DIR, fname)
            if os.path.exists(img_path):
                rows.append((fname, img_path, gt))

    print(f"Test set: {len(rows)} images\n")

    char_tp = char_fp = char_fn = 0
    word_correct = 0
    errors = []

    for fname, img_path, gt in rows:
        pred = predict(img_path)
        if pred == gt:
            word_correct += 1
        else:
            errors.append((fname, gt, pred))
        tp, fp, fn = char_metrics(pred, gt)
        char_tp += tp
        char_fp += fp
        char_fn += fn

    total = len(rows)

    word_acc = word_correct / total
    c_prec = char_tp / (char_tp + char_fp) if (char_tp + char_fp) else 0
    c_rec  = char_tp / (char_tp + char_fn) if (char_tp + char_fn) else 0
    c_f1   = (2 * c_prec * c_rec / (c_prec + c_rec)) if (c_prec + c_rec) else 0

    print("=" * 55)
    print("  UNSEEN TEST SET RESULTS")
    print("=" * 55)
    print(f"\n[Word / Sequence level]")
    print(f"  Accuracy  : {word_acc*100:.2f}%  ({word_correct}/{total})")
    print(f"  Precision : {word_acc*100:.2f}%")
    print(f"  Recall    : {word_acc*100:.2f}%")
    print(f"  F1 Score  : {word_acc*100:.2f}%")

    print(f"\n[Character level]")
    print(f"  Precision : {c_prec*100:.2f}%")
    print(f"  Recall    : {c_rec*100:.2f}%")
    print(f"  F1 Score  : {c_f1*100:.2f}%")

    print(f"\n[Errors — {len(errors)} images]")
    for fname, gt, pred in errors:
        print(f"  {fname:<10}  GT: {gt:<12}  PRED: {pred if pred else '(empty)'}")


if __name__ == "__main__":
    main()
