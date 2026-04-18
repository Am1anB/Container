FROM python:3.9-slim

RUN apt-get update && apt-get install -y \
    libgl1-mesa-glx \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    libxrender-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY ocr_service/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY ocr_service/main.py .
COPY inference/ ./inference/

EXPOSE 7860

CMD ["python", "main.py"]
