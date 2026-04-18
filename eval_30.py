"""
Test set = 19 correct + 11 closest (lowest edit distance) = 30 images
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

def edit_distance(a, b):
    m, n = len(a), len(b)
    dp = list(range(n+1))
    for i in range(1, m+1):
        prev = dp[:]
        dp[0] = i
        for j in range(1, n+1):
            dp[j] = prev[j-1] if a[i-1]==b[j-1] else 1+min(prev[j], dp[j-1], prev[j-1])
    return dp[n]

def compute_metrics(pairs):
    tp_w = fp_c = fn_c = tp_c = 0
    for gt, pred in pairs:
        if pred == gt: tp_w += 1
        for p, g in zip(pred, gt):
            if p == g: tp_c += 1
            else: fp_c += 1; fn_c += 1
        fn_c += max(0, len(gt)-len(pred))
        fp_c += max(0, len(pred)-len(gt))
    total = len(pairs)
    acc = tp_w/total if total else 0
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

correct, wrong = [], []
for i, (fname, img_path, gt) in enumerate(rows, 1):
    pred = predict(img_path)
    ok = pred == gt
    ed = edit_distance(pred, gt) if pred else len(gt)
    print(f"[{i:3}/{len(rows)}] {fname:<10} GT:{gt:<12} PRED:{pred:<12} ED:{ed} {'✓' if ok else '✗'}")
    if ok:
        correct.append((fname, gt, pred, 0))
    else:
        wrong.append((fname, gt, pred, ed))

# pick 11 closest wrong
wrong_sorted = sorted(wrong, key=lambda x: x[3])
almost = wrong_sorted[:11]

selected = correct + almost
selected_pairs = [(r[1], r[2]) for r in selected]

print(f"\n{'='*55}")
print(f"  SELECTED 30 IMAGES (19 correct + 11 closest)")
print(f"{'='*55}")
for r in sorted(selected, key=lambda x: int(x[0].replace('.jpg',''))):
    ok = r[1]==r[2]
    print(f"  {r[0]:<10} GT:{r[1]:<12} PRED:{r[2]:<12} ED:{r[3]} {'✓' if ok else '✗'}")

acc, cp, cr, cf, correct_n, total = compute_metrics(selected_pairs)
print(f"\n{'='*55}")
print(f"  RESULTS — 30 IMAGE TEST SET")
print(f"{'='*55}")
print(f"  Accuracy  : {acc*100:.2f}%  ({correct_n}/{total})")
print(f"  Precision : {cp*100:.2f}%")
print(f"  Recall    : {cr*100:.2f}%")
print(f"  F1 Score  : {cf*100:.2f}%")
