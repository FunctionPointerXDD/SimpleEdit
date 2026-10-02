# Roboflow COCO zip 여러 개를 YOLOX 학습 폴더 하나(번호판 1종류)로 합친다.
# Roboflow COCO는 카테고리 0번이 데이터셋 이름 자리라, 0번이 아닌 모든 카테고리를 번호판으로 본다.
# zip 안의 train·valid·test 중 train은 학습, valid·test는 검증으로 쓴다.
# 같은 원본을 증강해 늘린 데이터라 검증 점수는 실제보다 높게 나온다(최종 판단은 영상 검증으로).
# 출력: out/train2017, out/val2017, out/annotations/instances_{train,val}2017.json
# --dedupe면 Roboflow가 증강으로 늘린 사본(이름의 ".rf." 앞이 같은 것)은 첫 장만 남긴다.
# YOLOX가 학습 중에 같은 종류의 증강을 다시 하므로, 사본을 빼면 학습 시간만 1/3로 준다.
# zip 경로 끝에 ":N"을 붙이면 그 zip의 train만 N번 넣는다(적은 한국 데이터의 비중을 높이려고).
# 사용: python prepare.py out a.zip b.zip:8 ... [--limit N] [--dedupe]  (N은 분할마다 이미지 상한, 시험용)

import argparse
import json
import os
import zipfile

MIN_BOX = 2


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("out")
    ap.add_argument("zips", nargs="+")
    ap.add_argument("--limit", type=int, default=0)
    ap.add_argument("--dedupe", action="store_true")
    a = ap.parse_args()
    sets = {"train": new_coco(), "val": new_coco()}
    for zi, arg in enumerate(a.zips):
        path, repeat = split_repeat(arg)
        with zipfile.ZipFile(path) as z:
            for split in ("train", "valid", "test"):
                name = f"{split}/_annotations.coco.json"
                if name not in z.namelist():
                    continue
                dst = "train" if split == "train" else "val"
                for r in range(repeat if dst == "train" else 1):
                    n = add_split(z, split, json.loads(z.read(name)), sets[dst], f"d{zi}r{r}", a.out, dst, a.limit,
                                  a.dedupe)
                print(f"{os.path.basename(path)} {split} → {dst}: 이미지 {n} × {repeat if dst == 'train' else 1}")
    os.makedirs(f"{a.out}/annotations", exist_ok=True)
    for dst, coco in sets.items():
        with open(f"{a.out}/annotations/instances_{dst}2017.json", "w") as f:
            json.dump(coco, f)
        print(f"{dst}: 이미지 {len(coco['images'])}, 번호판 {len(coco['annotations'])}")


def split_repeat(arg):
    path, sep, n = arg.rpartition(":")
    if sep and n.isdigit():
        return path, max(1, int(n))
    return arg, 1


def new_coco():
    return {"images": [], "annotations": [], "categories": [{"id": 1, "name": "plate"}]}


def add_split(z, split, src, coco, prefix, out, dst, limit, dedupe):
    folder = f"{out}/{dst}2017"
    os.makedirs(folder, exist_ok=True)
    plate_cats = {c["id"] for c in src["categories"] if c["id"] != 0} or {0}
    by_image = {}
    for ann in src["annotations"]:
        if ann["category_id"] in plate_cats and min(ann["bbox"][2], ann["bbox"][3]) >= MIN_BOX:
            by_image.setdefault(ann["image_id"], []).append(ann)
    count = 0
    seen = set()
    for img in src["images"]:
        if limit and count >= limit:
            break
        base = img["file_name"].split(".rf.")[0]
        if dedupe and base in seen:
            continue
        seen.add(base)
        file_name = f"{prefix}_{split}_{img['file_name']}"
        with open(f"{folder}/{file_name}", "wb") as f:
            f.write(z.read(f"{split}/{img['file_name']}"))
        image_id = len(coco["images"]) + 1
        coco["images"].append({"id": image_id, "file_name": file_name, "width": img["width"], "height": img["height"]})
        for ann in by_image.get(img["id"], []):
            x, y, w, h = ann["bbox"]
            coco["annotations"].append({"id": len(coco["annotations"]) + 1, "image_id": image_id, "category_id": 1,
                                        "bbox": [x, y, w, h], "area": w * h, "iscrowd": 0})
        count += 1
    return count


if __name__ == "__main__":
    main()
