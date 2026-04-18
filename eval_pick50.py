"""
Run fine-tuned PaddleOCR on all 100 unseen images,
then pick top 50 by model confidence and report metrics.
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
    max_score = candidates[0][1] if candidates else 0.0
    for text, _ in candidates:
        n = extract_container(text)
        if n: return n, max_score
    full = " ".join(t for t, _ in candidates)
    return extract_container(full), max_score

def char_metrics(pred, gt):
    tp = sum(p == g for p, g in zip(pred, gt))
    fp = max(0, len(pred) - tp)
    fn = max(0, len(gt) - tp)
    return tp, fp, fn

def report(samples, label):
    char_tp = char_fp = char_fn = word_correct = 0
    errors = []
    for fname, gt, pred in samples:
        if pred == gt: word_correct += 1
        else: errors.append((fname, gt, pred))
        tp, fp, fn = char_metrics(pred, gt)
        char_tp += tp; char_fp += fp; char_fn += fn
    total = len(samples)
    wacc = word_correct / total
    cp = char_tp/(char_tp+char_fp) if (char_tp+char_fp) else 0
    cr = char_tp/(char_tp+char_fn) if (char_tp+char_fn) else 0
    cf = (2*cp*cr/(cp+cr)) if (cp+cr) else 0
    print(f"\n{'='*55}\n  {label} ({total} images)\n{'='*55}")
    print(f"\n[Word level]")
    print(f"  Accuracy  : {wacc*100:.2f}%  ({word_correct}/{total})")
    print(f"  Precision : {wacc*100:.2f}%")
    print(f"  Recall    : {wacc*100:.2f}%")
    print(f"  F1 Score  : {wacc*100:.2f}%")
    print(f"\n[Character level]")
    print(f"  Precision : {cp*100:.2f}%")
    print(f"  Recall    : {cr*100:.2f}%")
    print(f"  F1 Score  : {cf*100:.2f}%")
    if errors:
        print(f"\n[Errors — {len(errors)}]")
        for fname, gt, pred in errors[:15]:
            print(f"  {fname:<10} GT:{gt:<12} PRED:{pred if pred else '(empty)'}")

def main():
    rows = []
    with open(GT_CSV, encoding="utf-8") as f:
        for row in csv.DictReader(f):
            fname = row["filename"].strip()
            gt = row["ground_truth"].strip().upper()
            img_path = os.path.join(UPLOADS_DIR, fname)
            if os.path.exists(img_path):
                rows.append((fname, img_path, gt))

    print(f"Running fine-tuned PaddleOCR on {len(rows)} images...")
    results = []
    for i, (fname, img_path, gt) in enumerate(rows, 1):
        pred, conf = predict(img_path)
        correct = pred == gt
        results.append((fname, gt, pred, conf, correct))
        print(f"[{i:3}/{len(rows)}] {fname:<10} GT:{gt:<12} PRED:{pred:<12} conf:{conf:.3f} {'✓' if correct else '✗'}")

    # sort by confidence descending, pick top 50
    results_sorted = sorted(results, key=lambda x: x[3], reverse=True)
    top50 = results_sorted[:50]

    all_samples  = [(r[0], r[1], r[2]) for r in results]
    top50_samples = [(r[0], r[1], r[2]) for r in top50]

    report(all_samples,  "ALL 100 IMAGES")
    report(top50_samples, "TOP 50 (highest confidence)")

    print(f"\n[Top 50 selected images]")
    for r in sorted(top50, key=lambda x: int(x[0].replace('.jpg',''))):
        print(f"  {r[0]:<10} conf:{r[3]:.3f}  {'✓' if r[4] else '✗'}")

if __name__ == "__main__":
    main()
