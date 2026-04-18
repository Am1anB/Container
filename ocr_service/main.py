from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from paddleocr import PaddleOCR
import uvicorn
import os
import cv2
import numpy as np
import paddleocr
import base64

app = FastAPI()

ocr_module_path = os.path.dirname(paddleocr.__file__)
dict_path = os.path.join(ocr_module_path, 'ppocr', 'utils', 'en_dict.txt')

print(f"Loading Dictionary from: {dict_path}")

ocr = PaddleOCR(
    use_angle_cls=True,
    lang='en',
    det_model_dir='./inference/det_model/',
    rec_model_dir='./inference/rec_model_finetuned/inference/',
    rec_char_dict_path=dict_path,
    use_gpu=False,
    det_db_thresh=0.3,
    det_db_box_thresh=0.5,
    det_db_unclip_ratio=1.6,
)

CONFIDENCE_THRESHOLD = 0.55


def preprocess_image(img: np.ndarray) -> np.ndarray:
    gray = cv2.cvtColor(img, cv2.COLOR_BGR2GRAY)
    clahe = cv2.createCLAHE(clipLimit=2.5, tileGridSize=(8, 8))
    enhanced = clahe.apply(gray)
    kernel = np.array([[0, -1, 0], [-1, 5, -1], [0, -1, 0]], dtype=np.float32)
    sharpened = cv2.filter2D(enhanced, -1, kernel)
    return cv2.cvtColor(sharpened, cv2.COLOR_GRAY2BGR)


def extract_candidates(result) -> list[dict]:
    candidates = []
    if result and result[0]:
        for line in result[0]:
            text, score = line[1]
            if score >= CONFIDENCE_THRESHOLD:
                candidates.append({"text": text, "score": round(float(score), 4)})
    return candidates


class ImageRequest(BaseModel):
    image_b64: str


@app.get("/health")
def health():
    return {"ok": True}


@app.post("/predict")
async def predict(req: ImageRequest):
    try:
        img_bytes = base64.b64decode(req.image_b64)
        img_array = np.frombuffer(img_bytes, np.uint8)
        img = cv2.imdecode(img_array, cv2.IMREAD_COLOR)
    except Exception as e:
        raise HTTPException(status_code=400, detail=f"Invalid image data: {e}")

    if img is None:
        raise HTTPException(status_code=400, detail="Could not decode image")

    all_candidates: list[dict] = []

    result = ocr.ocr(img, cls=True)
    all_candidates.extend(extract_candidates(result))

    preprocessed = preprocess_image(img)
    pre_result = ocr.ocr(preprocessed, cls=True)
    all_candidates.extend(extract_candidates(pre_result))

    all_candidates.sort(key=lambda x: x["score"], reverse=True)
    seen: set[str] = set()
    unique: list[dict] = []
    for c in all_candidates:
        if c["text"] not in seen:
            seen.add(c["text"])
            unique.append(c)

    full_text = " ".join(c["text"] for c in unique)

    return {
        "text": full_text,
        "raw": unique,
        "candidates": unique[:20],
    }


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 7860))
    uvicorn.run(app, host="0.0.0.0", port=port)
