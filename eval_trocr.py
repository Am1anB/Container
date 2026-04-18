"""
Evaluate fine-tuned TrOCR on unseen container images.

Run with Python 3.13:
    py -3.13 eval_trocr.py

Tests on 38 images that were NOT used in training.
"""
import os
import re
import csv
import cv2
import numpy as np
from PIL import Image
import torch
from transformers import TrOCRProcessor, VisionEncoderDecoderModel

MODEL_DIR = "models/trocr_container"
UPLOADS_DIR = "uploads"
GROUND_TRUTH = "ground_truth.csv"
OUT_CSV = "eval_trocr_results.csv"

UNSEEN = {
    "3.jpg","14.jpg","15.jpg","16.jpg","17.jpg","18.jpg","19.jpg","20.jpg",
    "21.jpg","22.jpg","23.jpg","24.jpg","26.jpg","34.jpg","37.jpg","38.jpg",
    "39.jpg","40.jpg","41.jpg","42.jpg","43.jpg","44.jpg","45.jpg","46.jpg",
    "47.jpg","48.jpg","49.jpg","50.jpg","51.jpg","52.jpg","53.jpg","54.jpg",
    "55.jpg","56.jpg","57.jpg","58.jpg","59.jpg","60.jpg",
}

LOOK_LIKE_DIGIT = {"O":"0","Q":"0","D":"0","I":"1","L":"1","Z":"2","S":"5","B":"8","G":"6"}
LOOK_LIKE_LETTER = {"0":"O","1":"I","2":"Z","5":"S","8":"B"}


def force_container(s: str) -> str:
    s = s.upper()
    buf = [c for c in s if c.isalpha() or c.isdigit()]
    if len(buf) < 11:
        return ""
    out = []
    for i in range(4):
        c = buf[i]
        if c.isdigit():
            c = LOOK_LIKE_LETTER.get(c, "A")
        out.append(c)
    for i in range(4, min(11, len(buf))):
        c = buf[i]
        if c.isalpha():
            c = LOOK_LIKE_DIGIT.get(c, "0")
        out.append(c)
    return "".join(out)


def cer(pred: str, gt: str) -> float:
    if not gt:
        return 1.0
    n, m = len(gt), len(pred)
    dp = list(range(m + 1))
    for i in range(1, n + 1):
        new_dp = [i] + [0] * m
        for j in range(1, m + 1):
            if gt[i - 1] == pred[j - 1]:
                new_dp[j] = dp[j - 1]
            else:
                new_dp[j] = 1 + min(dp[j], new_dp[j - 1], dp[j - 1])
        dp = new_dp
    return dp[m] / n


def preprocess_variants(img_path: str):
    img_bgr = cv2.imread(img_path)
    if img_bgr is None:
        return []

    variants = []
    h, w = img_bgr.shape[:2]

    def pil(arr):
        return Image.fromarray(cv2.cvtColor(arr, cv2.COLOR_BGR2RGB))

    # Original
    variants.append(pil(img_bgr))

    # Upscaled 2x
    up = cv2.resize(img_bgr, (w * 2, h * 2), interpolation=cv2.INTER_CUBIC)
    variants.append(pil(up))

    # CLAHE enhanced
    gray = cv2.cvtColor(img_bgr, cv2.COLOR_BGR2GRAY)
    clahe = cv2.createCLAHE(clipLimit=3.0, tileGridSize=(8, 8))
    enhanced = clahe.apply(gray)
    sharp_kernel = np.array([[0,-1,0],[-1,5,-1],[0,-1,0]], dtype=np.float32)
    sharpened = cv2.filter2D(enhanced, -1, sharp_kernel)
    enhanced_bgr = cv2.cvtColor(sharpened, cv2.COLOR_GRAY2BGR)
    variants.append(pil(enhanced_bgr))

    # Middle vertical crop (container number usually in center)
    for top_frac, bot_frac in [(0.3, 0.7), (0.2, 0.6), (0.4, 0.8)]:
        top = int(h * top_frac)
        bot = int(h * bot_frac)
        crop = img_bgr[top:bot, :]
        if crop.shape[0] > 10:
            variants.append(pil(crop))
            # Upscaled crop
            crop_up = cv2.resize(crop, (crop.shape[1]*2, crop.shape[0]*2), interpolation=cv2.INTER_CUBIC)
            variants.append(pil(crop_up))

    return variants


def predict_image(processor, model, img_path: str, device) -> str:
    variants = preprocess_variants(img_path)
    best = ""
    for img in variants:
        pixel_values = processor(images=img, return_tensors="pt").pixel_values.to(device)
        with torch.no_grad():
            ids = model.generate(pixel_values, max_new_tokens=20, num_beams=4)
        text = processor.batch_decode(ids, skip_special_tokens=True)[0]
        forced = force_container(text)
        if forced and (not best or len(forced) > len(best)):
            best = forced
    return best


def main():
    if not os.path.exists(MODEL_DIR):
        print(f"ERROR: Model not found at {MODEL_DIR}/")
        print("Run finetune_trocr.py first.")
        return

    print(f"Loading model from {MODEL_DIR}/")
    device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    print(f"Device: {device}")
    processor = TrOCRProcessor.from_pretrained(MODEL_DIR)
    model = VisionEncoderDecoderModel.from_pretrained(MODEL_DIR).to(device)
    model.eval()

    gt_map = {}
    with open(GROUND_TRUTH, encoding="utf-8") as f:
        for row in csv.DictReader(f):
            gt_map[row["filename"]] = row["ground_truth"]

    results = []
    total = exact = has_pred = 0

    for fname in sorted(UNSEEN, key=lambda x: int(x.replace(".jpg",""))):
        if fname not in gt_map:
            continue
        gt = gt_map[fname]
        img_path = os.path.join(UPLOADS_DIR, fname)
        if not os.path.exists(img_path):
            print(f"  MISSING: {img_path}")
            continue

        pred = predict_image(processor, model, img_path, device)
        score = cer(pred, gt)
        total += 1
        if pred:
            has_pred += 1
        if pred == gt:
            exact += 1

        status = "OK" if pred == gt else ("~" if pred else "MISS")
        print(f"  [{status}] {fname:8} GT:{gt:12} Pred:{pred or '(none)':12} CER:{score*100:.1f}%")
        results.append({"filename": fname, "ground_truth": gt, "prediction": pred, "CER": f"{score*100:.2f}%"})

    print(f"\n--- Results ({total} unseen images) ---")
    print(f"Exact match : {exact}/{total} ({exact/total*100:.1f}%)")
    print(f"Has prediction: {has_pred}/{total} ({has_pred/total*100:.1f}%)")
    print(f"No prediction : {total-has_pred}/{total} ({(total-has_pred)/total*100:.1f}%)")

    with open(OUT_CSV, "w", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=["filename","ground_truth","prediction","CER"])
        writer.writeheader()
        writer.writerows(results)
    print(f"\nResults saved to {OUT_CSV}")


if __name__ == "__main__":
    main()
