"""
Run fine-tuned PaddleOCR on 100 unseen images.
Show which images are correct, then report metrics on correct-only subset.
"""
import os, re, csv
from paddleocr import PaddleOCR
import paddleocr

BASE = os.path.dirname(os.path.abspath(__file__))
UPLOADS_DIR = os.path.join(BASE, "uploads")
GT_CSV = os.path.join(BASE, "ground_truth.csv")

ocr_module_path = os.path.dirname(paddleocr.__file__)
dict_path = os.path.join(ocr_module_path, "ppocr", "utils", "en_dict.txt")

ocr = PaddleOCR(
    use_angle_cls=True, lang="en",
    det_model_dir=os.path.join(BASE, "inference", "det_model"),
    rec_model_dir=os.path.join(BASE, "inference", "rec_model_finetuned", "inference"),
    rec_char_dict_path=dict_path,
    use_gpu=False, det_db_thresh=0.3, det_db_box_thresh=0.5,
    det_db_unclip_ratio=1.6, show_log=False,
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

def predict(img_path):
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
        if n: return n
    return extract_container(" ".join(t for t, _ in candidates))

def compute_metrics(pairs):
    tp_w = fp_w = fn_w = tp_c = fp_c = fn_c = 0
    for gt, pred in pairs:
        if pred == gt: tp_w += 1
        else: fp_w += 1; fn_w += 1
        for p, g in zip(pred, gt):
            if p == g: tp_c += 1
            else: fp_c += 1; fn_c += 1
        fn_c += max(0, len(gt) - len(pred))
        fp_c += max(0, len(pred) - len(gt))
    total = len(pairs)
    acc = tp_w / total if total else 0
    cp = tp_c/(tp_c+fp_c) if (tp_c+fp_c) else 0
    cr = tp_c/(tp_c+fn_c) if (tp_c+fn_c) else 0
    cf = 2*cp*cr/(cp+cr) if (cp+cr) else 0
    return acc, cp, cr, cf, tp_w, total

rows = []
with open(GT_CSV, encoding="utf-8") as f:
    for row in csv.DictReader(f):
        fname = row["filename"].strip()
        gt = row["ground_truth"].strip().upper()
        img_path = os.path.join(UPLOADS_DIR, fname)
        if os.path.exists(img_path):
            rows.append((fname, img_path, gt))

print(f"Running on {len(rows)} images...\n")

correct_files = []
all_pairs = []
for i, (fname, img_path, gt) in enumerate(rows, 1):
    pred = predict(img_path)
    ok = pred == gt
    all_pairs.append((gt, pred))
    if ok:
        correct_files.append(fname)
    print(f"[{i:3}/{len(rows)}] {fname:<10} GT:{gt:<12} PRED:{pred:<12} {'✓' if ok else '✗'}")

print(f"\n{'='*50}")
print(f"  CORRECT: {len(correct_files)}/100 images")
print(f"  {correct_files}")
print(f"{'='*50}")

acc, cp, cr, cf, correct, total = compute_metrics(all_pairs)
print(f"\n[ALL 100 images]")
print(f"  Accuracy : {acc*100:.2f}%  Precision : {cp*100:.2f}%  Recall : {cr*100:.2f}%  F1 : {cf*100:.2f}%")
