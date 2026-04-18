"""
Evaluate fine-tuned PaddleOCR on unseen test set WITH auto-crop.
Pipeline: detection -> crop each text region -> recognition -> extract container number.
Simulates "user crops the image" scenario.
"""

import os
import re
import csv
import cv2
import numpy as np
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


def crop_and_predict(img_path: str) -> str:
    img = cv2.imread(img_path)
    if img is None:
        return ""

    result = ocr.ocr(img_path, cls=True)
    if not result or not result[0]:
        return ""

    candidates = []
    for line in result[0]:
        box, (text, score) = line[0], line[1]
        if score < 0.3:
            continue

        # crop the detected region
        pts = np.array(box, dtype=np.float32)
        x_min, y_min = int(pts[:, 0].min()), int(pts[:, 1].min())
        x_max, y_max = int(pts[:, 0].max()), int(pts[:, 1].max())
        x_min, y_min = max(0, x_min), max(0, y_min)
        crop = img[y_min:y_max, x_min:x_max]

        if crop.size == 0:
            candidates.append((text, score))
            continue

        # re-run recognition on crop only
        tmp = os.path.join(os.environ.get("TEMP", BASE), "crop_eval.jpg")
        cv2.imwrite(tmp, crop)
        rec_result = ocr.ocr(tmp, det=False, cls=False)
        if rec_result and rec_result[0]:
            rec_text, rec_score = rec_result[0][0]
            candidates.append((rec_text, rec_score))
        else:
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

    for i, (fname, img_path, gt) in enumerate(rows, 1):
        pred = crop_and_predict(img_path)
        match = pred == gt
        if match:
            word_correct += 1
        else:
            errors.append((fname, gt, pred))
        tp, fp, fn = char_metrics(pred, gt)
        char_tp += tp
        char_fp += fp
        char_fn += fn
        print(f"[{i:3}/{len(rows)}] {fname:<10} GT: {gt:<12} PRED: {pred:<12} {'OK' if match else 'X'}")

    total = len(rows)
    word_acc = word_correct / total
    c_prec = char_tp / (char_tp + char_fp) if (char_tp + char_fp) else 0
    c_rec  = char_tp / (char_tp + char_fn) if (char_tp + char_fn) else 0
    c_f1   = (2 * c_prec * c_rec / (c_prec + c_rec)) if (c_prec + c_rec) else 0

    print(f"\n{'='*55}")
    print(f"  UNSEEN TEST SET — WITH AUTO CROP")
    print(f"{'='*55}")
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
        print(f"  {fname:<10} GT: {gt:<12} PRED: {pred if pred else '(empty)'}")


if __name__ == "__main__":
    main()
