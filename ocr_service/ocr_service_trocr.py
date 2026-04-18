"""
Improved OCR service using fine-tuned TrOCR.
Drop-in replacement for main.py — same API endpoint (/predict).

Requires Python 3.13 with:
    torch, transformers, opencv-python, pillow

Run:
    py -3.13 ocr_service_trocr.py
"""
import os
import re
import cv2
import numpy as np
import torch
from PIL import Image
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from transformers import TrOCRProcessor, VisionEncoderDecoderModel
import uvicorn

app = FastAPI()

MODEL_DIR = os.path.join(os.path.dirname(__file__), "..", "models", "trocr_container")

LOOK_LIKE_DIGIT = {"O":"0","Q":"0","D":"0","I":"1","L":"1","Z":"2","S":"5","B":"8","G":"6"}
LOOK_LIKE_LETTER = {"0":"O","1":"I","2":"Z","5":"S","8":"B"}

device = torch.device("cuda" if torch.cuda.is_available() else "cpu")
processor: TrOCRProcessor = None
model: VisionEncoderDecoderModel = None


def load_model():
    global processor, model
    if not os.path.exists(MODEL_DIR):
        raise RuntimeError(f"Model not found at {MODEL_DIR}. Run finetune_trocr.py first.")
    print(f"Loading TrOCR model from {MODEL_DIR} on {device}")
    processor = TrOCRProcessor.from_pretrained(MODEL_DIR)
    model = VisionEncoderDecoderModel.from_pretrained(MODEL_DIR).to(device)
    model.eval()
    print("Model loaded.")


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
    return "".join(out) if len(out) == 11 else ""


def build_variants(img_path: str) -> list[Image.Image]:
    img_bgr = cv2.imread(img_path)
    if img_bgr is None:
        return []

    h, w = img_bgr.shape[:2]
    variants = []

    def to_pil(bgr):
        return Image.fromarray(cv2.cvtColor(bgr, cv2.COLOR_BGR2RGB))

    # Original
    variants.append(to_pil(img_bgr))

    # 2x upscale
    up2 = cv2.resize(img_bgr, (w * 2, h * 2), interpolation=cv2.INTER_CUBIC)
    variants.append(to_pil(up2))

    # CLAHE + sharpen
    gray = cv2.cvtColor(img_bgr, cv2.COLOR_BGR2GRAY)
    clahe = cv2.createCLAHE(clipLimit=3.0, tileGridSize=(8, 8))
    eq = clahe.apply(gray)
    sharp = cv2.filter2D(eq, -1, np.array([[0,-1,0],[-1,5,-1],[0,-1,0]], np.float32))
    variants.append(to_pil(cv2.cvtColor(sharp, cv2.COLOR_GRAY2BGR)))

    # Vertical crops (container number usually in middle region)
    for top_f, bot_f in [(0.25, 0.65), (0.35, 0.75), (0.15, 0.55), (0.45, 0.85)]:
        crop = img_bgr[int(h * top_f):int(h * bot_f), :]
        if crop.shape[0] < 10:
            continue
        variants.append(to_pil(crop))
        # Upscaled crop
        crop_up = cv2.resize(crop, (crop.shape[1]*2, crop.shape[0]*2), interpolation=cv2.INTER_CUBIC)
        variants.append(to_pil(crop_up))

    # Denoised
    denoised = cv2.fastNlMeansDenoisingColored(img_bgr, None, 10, 10, 7, 21)
    variants.append(to_pil(denoised))

    return variants


def run_trocr(variants: list[Image.Image]) -> list[dict]:
    candidates = []
    for img in variants:
        pv = processor(images=img, return_tensors="pt").pixel_values.to(device)
        with torch.no_grad():
            ids = model.generate(pv, max_new_tokens=24, num_beams=4)
        raw = processor.batch_decode(ids, skip_special_tokens=True)[0].strip()
        forced = force_container(raw)
        if forced:
            candidates.append({"text": forced, "raw": raw, "score": 1.0})
        elif raw:
            candidates.append({"text": raw, "raw": raw, "score": 0.5})
    return candidates


class ImageRequest(BaseModel):
    image_path: str


@app.on_event("startup")
async def startup():
    load_model()


@app.post("/predict")
async def predict(req: ImageRequest):
    if not os.path.exists(req.image_path):
        raise HTTPException(status_code=404, detail="Image not found")

    variants = build_variants(req.image_path)
    if not variants:
        raise HTTPException(status_code=422, detail="Cannot read image")

    candidates = run_trocr(variants)

    # Prefer fully-forced container format
    forced_candidates = [c for c in candidates if len(c["text"]) == 11 and c["text"].isalnum()]
    final = forced_candidates[0]["text"] if forced_candidates else (candidates[0]["text"] if candidates else "")

    return {
        "text": final,
        "raw": candidates,
        "candidates": candidates[:10],
    }


if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8001)
