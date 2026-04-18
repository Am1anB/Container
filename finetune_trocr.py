"""
Fine-tune TrOCR on augmented container crop data.

Run with Python 3.13:
    py -3.13 finetune_trocr.py

Requirements (already installed):
    torch, transformers, pillow

Output: models/trocr_container/
"""
import os
import json
import torch
from torch.utils.data import Dataset, DataLoader, random_split
from transformers import (
    TrOCRProcessor,
    VisionEncoderDecoderModel,
    Seq2SeqTrainer,
    Seq2SeqTrainingArguments,
    default_data_collator,
)
from PIL import Image

LABEL_FILE = "augmented_labels.txt"
MODEL_OUT = "models/trocr_container"
BASE_MODEL = "microsoft/trocr-base-printed"
EPOCHS = 10
BATCH_SIZE = 8
LR = 5e-5
MAX_LEN = 16

CHAR_SET = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"


def load_pairs(label_file):
    pairs = []
    with open(label_file, encoding="utf-8") as f:
        for line in f:
            line = line.strip()
            if not line:
                continue
            parts = line.split("\t")
            if len(parts) >= 2:
                img_path, label = parts[0], parts[1].strip().upper()
                if os.path.exists(img_path) and all(c in CHAR_SET for c in label):
                    pairs.append((img_path, label))
    return pairs


class CropDataset(Dataset):
    def __init__(self, pairs, processor):
        self.pairs = pairs
        self.processor = processor

    def __len__(self):
        return len(self.pairs)

    def __getitem__(self, idx):
        img_path, label = self.pairs[idx]
        img = Image.open(img_path).convert("RGB")
        pixel_values = self.processor(images=img, return_tensors="pt").pixel_values.squeeze(0)
        labels = self.processor.tokenizer(
            label,
            padding="max_length",
            max_length=MAX_LEN,
            return_tensors="pt",
        ).input_ids.squeeze(0)
        labels[labels == self.processor.tokenizer.pad_token_id] = -100
        return {"pixel_values": pixel_values, "labels": labels}


def compute_metrics_fn(processor):
    def compute_metrics(pred):
        label_ids = pred.label_ids
        pred_ids = pred.predictions

        if isinstance(pred_ids, tuple):
            pred_ids = pred_ids[0]

        pred_ids = pred_ids.argmax(-1) if pred_ids.ndim == 3 else pred_ids
        label_ids[label_ids == -100] = processor.tokenizer.pad_token_id

        pred_strs = processor.tokenizer.batch_decode(pred_ids, skip_special_tokens=True)
        label_strs = processor.tokenizer.batch_decode(label_ids, skip_special_tokens=True)

        correct = sum(p.strip().upper() == l.strip().upper() for p, l in zip(pred_strs, label_strs))
        return {"exact_match": correct / len(pred_strs)}

    return compute_metrics


def main():
    if not os.path.exists(LABEL_FILE):
        print(f"ERROR: {LABEL_FILE} not found. Run augment_data.py first.")
        return

    print("Loading labels...")
    pairs = load_pairs(LABEL_FILE)
    print(f"Valid pairs: {len(pairs)}")

    print(f"Loading model: {BASE_MODEL}")
    processor = TrOCRProcessor.from_pretrained(BASE_MODEL)
    model = VisionEncoderDecoderModel.from_pretrained(BASE_MODEL)

    model.config.decoder_start_token_id = processor.tokenizer.cls_token_id
    model.config.pad_token_id = processor.tokenizer.pad_token_id
    model.config.vocab_size = model.config.decoder.vocab_size
    model.config.eos_token_id = processor.tokenizer.sep_token_id
    model.config.max_length = MAX_LEN
    model.config.early_stopping = True
    model.config.no_repeat_ngram_size = 3
    model.config.length_penalty = 2.0
    model.config.num_beams = 4

    dataset = CropDataset(pairs, processor)

    val_size = max(1, int(len(dataset) * 0.1))
    train_size = len(dataset) - val_size
    train_ds, val_ds = random_split(dataset, [train_size, val_size])
    print(f"Train: {train_size}, Val: {val_size}")

    os.makedirs(MODEL_OUT, exist_ok=True)

    training_args = Seq2SeqTrainingArguments(
        output_dir=MODEL_OUT,
        num_train_epochs=EPOCHS,
        per_device_train_batch_size=BATCH_SIZE,
        per_device_eval_batch_size=BATCH_SIZE,
        learning_rate=LR,
        warmup_steps=100,
        eval_strategy="epoch",
        save_strategy="epoch",
        load_best_model_at_end=True,
        metric_for_best_model="exact_match",
        greater_is_better=True,
        predict_with_generate=True,
        logging_steps=50,
        fp16=False,
        dataloader_num_workers=0,
        report_to="none",
        save_total_limit=2,
    )

    trainer = Seq2SeqTrainer(
        model=model,
        args=training_args,
        train_dataset=train_ds,
        eval_dataset=val_ds,
        data_collator=default_data_collator,
        compute_metrics=compute_metrics_fn(processor),
    )

    print("Starting fine-tuning...")
    trainer.train()

    model.save_pretrained(MODEL_OUT)
    processor.save_pretrained(MODEL_OUT)
    print(f"Model saved to {MODEL_OUT}/")


if __name__ == "__main__":
    main()
