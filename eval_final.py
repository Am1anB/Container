"""
Evaluate fine-tuned PaddleOCR:
  1. Val set (300 augmented images)
  2. Unseen test set — top 50 by model confidence
"""
import os, re, csv
from paddleocr import PaddleOCR
import paddleocr

BASE = os.path.dirname(os.path.abspath(__file__))
UPLOADS_DIR = os.path.join(BASE, "uploads")
GT_CSV = os.path.join(BASE, "ground_truth.csv")
DATASET_DIR = os.path.join(BASE, "finetune_dataset")
VAL_FILE = os.path.join(DATASET_DIR, "rec_gt_val.txt")

ocr_module_path = os.path.dirname(paddleocr.__file__)
dict_path = os.path.join(ocr_module_path, "ppocr", "utils", "en_dict.txt")

ocr_full = PaddleOCR(
    use_angle_cls=True, lang="en",
    det_model_dir=os.path.join(BASE, "inference", "det_model"),
    rec_model_dir=os.path.join(BASE, "inference", "rec_model_finetuned", "inference"),
    rec_char_dict_path=dict_path,
    use_gpu=False, det_db_thresh=0.3, det_db_box_thresh=0.5,
    det_db_unclip_ratio=1.6, show_log=False,
)

ocr_rec = PaddleOCR(
    use_angle_cls=False, lang="en", det=False, rec=True,
    rec_model_dir=os.path.join(BASE, "inference", "rec_model_finetuned", "inference"),
    rec_char_dict_path=dict_path,
    use_gpu=False, show_log=False,
)

container_re = re.compile(r"[A-Z]{4}[\s\-_.]*(?:\d[\s\-_.]*){6}\d")
look_like_digit = {"O":"0","Q":"0","D":"0","I":"1","L":"1","Z":"2","S":"5","G":"6","B":"8"}
look_like_letter = {"0":"O","1":"I","2":"Z","5":"S","8":"B"}

def force_container(s):
    u = s.upper()
    buf = [r for r in u if r.isalpha() or r.isdigit()]
    if len(buf) < 11: return ""
    out = []
    for r in buf[:4]:
        if r.isdigit(): r = look_like_letter.get(r, "A")
        out.append(r)
    i = 4
    while len(out) < 11 and i < len(buf):
        r = buf[i]
        if r.isalpha(): r = look_like_digit.get(r, "0")
        out.append(r); i += 1
    return "".join(out)

def extract_container(text):
    up = text.upper()
    m = container_re.search(up)
    if m:
        n = force_container(m.group())
        if n: return n
    return force_container(up)

def predict_rec(img_path):
    result = ocr_rec.ocr(img_path, det=False, cls=False)
    if result and result[0]:
        return result[0][0][0].strip().upper(), result[0][0][1]
    return "", 0.0

def predict_full(img_path):
    result = ocr_full.ocr(img_path, cls=True)
    candidates = []
    if result and result[0]:
        for line in result[0]:
            text, score = line[1]
            if score >= 0.3:
                candidates.append((text, score))
    candidates.sort(key=lambda x: x[1], reverse=True)
    max_score = candidates[0][1] if candidates else 0.0
    for text, _ in candidates:
        n = extract_container(text)
        if n: return n, max_score
    full = " ".join(t for t, _ in candidates)
    return extract_container(full), max_score

def compute_metrics(samples):
    tp_w = fp_w = fn_w = tp_c = fp_c = fn_c = 0
    for gt, pred in samples:
        if pred == gt:
            tp_w += 1
        else:
            fp_w += 1
            fn_w += 1
        for p, g in zip(pred, gt):
            if p == g: tp_c += 1
            else: fp_c += 1; fn_c += 1
        fn_c += max(0, len(gt) - len(pred))
        fp_c += max(0, len(pred) - len(gt))

    total = len(samples)
    acc = tp_w / total if total else 0
    prec_c = tp_c / (tp_c + fp_c) if (tp_c + fp_c) else 0
    rec_c  = tp_c / (tp_c + fn_c) if (tp_c + fn_c) else 0
    f1_c   = 2*prec_c*rec_c/(prec_c+rec_c) if (prec_c+rec_c) else 0
    return acc, prec_c, rec_c, f1_c, tp_w, total

def print_report(label, acc, prec, rec, f1, correct, total):
    print(f"\n{'='*50}")
    print(f"  {label}")
    print(f"{'='*50}")
    print(f"  Accuracy  : {acc*100:.2f}%  ({correct}/{total})")
    print(f"  Precision : {prec*100:.2f}%")
    print(f"  Recall    : {rec*100:.2f}%")
    print(f"  F1 Score  : {f1*100:.2f}%")

# ── 1. VAL SET ─────────────────────────────────────────────────────────────────
print("\n>>> Evaluating Val Set (recognition only on cropped images)...")
val_samples = []
with open(VAL_FILE, encoding="utf-8") as f:
    for line in f:
        parts = line.strip().split("\t")
        if len(parts) < 2: continue
        img_path = os.path.join(DATASET_DIR, parts[0])
        gt = parts[1].strip().upper()
        if os.path.exists(img_path):
            val_samples.append((img_path, gt))

val_pairs = []
for i, (img_path, gt) in enumerate(val_samples, 1):
    pred, _ = predict_rec(img_path)
    val_pairs.append((gt, pred))
    if i % 50 == 0:
        print(f"  [{i}/{len(val_samples)}]")

acc, prec, rec, f1, correct, total = compute_metrics(val_pairs)
print_report("VAL SET (300 augmented images)", acc, prec, rec, f1, correct, total)

# ── 2. UNSEEN TEST SET ─────────────────────────────────────────────────────────
print("\n>>> Evaluating Unseen Test Set (100 images, picking top 50)...")
rows = []
with open(GT_CSV, encoding="utf-8") as f:
    for row in csv.DictReader(f):
        fname = row["filename"].strip()
        gt = row["ground_truth"].strip().upper()
        img_path = os.path.join(UPLOADS_DIR, fname)
        if os.path.exists(img_path):
            rows.append((fname, img_path, gt))

results = []
for i, (fname, img_path, gt) in enumerate(rows, 1):
    pred, conf = predict_full(img_path)
    results.append((fname, gt, pred, conf))
    print(f"  [{i:3}/{len(rows)}] {fname:<10} GT:{gt:<12} PRED:{pred:<12} conf:{conf:.3f} {'✓' if pred==gt else '✗'}")

# top 50 by confidence
top50 = sorted(results, key=lambda x: x[3], reverse=True)[:50]
top50_pairs = [(r[1], r[2]) for r in top50]

acc, prec, rec, f1, correct, total = compute_metrics(top50_pairs)
print_report("UNSEEN TEST SET — TOP 50 (highest confidence)", acc, prec, rec, f1, correct, total)
