# prepare.py·plate_tiny.py·export.py를 셀에 그대로 넣어 Colab 학습 노트북(plate_train.ipynb)을 만든다.
# 저장소가 비공개라 Colab이 직접 받을 수 없어서다. 스크립트를 고치면 이 파일을 다시 실행한다.
# 데이터는 Google 드라이브의 plate_train 폴더에 둔 Roboflow COCO zip들이고, 결과도 그 폴더 out/에 남는다.
# 이름에 korea가 든 zip은 학습 사진을 KOREA_REPEAT번 넣는다(한국 데이터가 적어 비중을 높이려고).
# 학습 중 검증(COCO 평가)이 C++ 확장을 바로 컴파일하므로 ninja를 설치한다(없으면 3회째에서 멈춤).
# 이어 학습할 체크포인트에는 NumPy 값이 있어 TORCH_FORCE_NO_WEIGHTS_ONLY_LOAD=1로 읽게 한다.
# YOLOX는 pip install -e가 Colab에서 실패해 PYTHONPATH로 불러온다.
# 사용: python make_notebook.py [출력 경로=plate_train.ipynb]

import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
YOLOX_COMMIT = "6ddff48"


def md(text):
    return {"cell_type": "markdown", "metadata": {}, "source": text.strip("\n").splitlines(True)}


def code(text):
    return {"cell_type": "code", "metadata": {}, "execution_count": None, "outputs": [],
            "source": text.strip("\n").splitlines(True)}


def embed(name):
    with open(os.path.join(HERE, name)) as f:
        return code(f"%%writefile {name}\n" + f.read())


def main():
    out = sys.argv[1] if len(sys.argv) > 1 else "plate_train.ipynb"
    cells = [
        md("""
# 번호판 검출 모델 학습 (YOLOX-Tiny)

1. 메뉴 **런타임 → 런타임 유형 변경 → T4 GPU**를 고르고 저장합니다.
2. Google 드라이브 **내 드라이브/plate_train** 폴더에 Roboflow에서 받은 COCO zip 파일들을 넣어 둡니다.
   같은 사진을 복사한 데이터셋은 하나만 넣습니다(예: Korea Car License Plate는 hyunjin 것만).
3. 메뉴 **런타임 → 모두 실행**을 누릅니다. 드라이브 연결을 물으면 허용합니다.
4. 끝나면 **plate_train/out/plate_int8.onnx**(앱용 INT8)와 plate.onnx(FP32)가 생깁니다. 둘 다 내려받아 전달해 주세요.

학습 중 연결이 끊기면 다시 **모두 실행**하면 됩니다. 드라이브에 남은 체크포인트에서 이어서 학습합니다.
"""),
        code("""
!nvidia-smi -L
"""),
        code("""
import glob, os, shlex
from google.colab import drive
drive.mount('/content/drive')
DRIVE = '/content/drive/MyDrive/plate_train'
EPOCHS = 30
KOREA_REPEAT = 8
zips = sorted(glob.glob(f'{DRIVE}/*.zip'))
print('\\n'.join(zips))
assert zips, 'plate_train 폴더에 zip 파일이 없습니다'
ZIPS = ' '.join(shlex.quote(z + (f':{KOREA_REPEAT}' if 'korea' in z.lower() else '')) for z in zips)
os.makedirs(f'{DRIVE}/out', exist_ok=True)
"""),
        code(f"""
%cd /content
!test -d YOLOX || git clone -q https://github.com/Megvii-BaseDetection/YOLOX.git
!git -C YOLOX checkout -q {YOLOX_COMMIT}
!pip install -q loguru thop tabulate pycocotools onnx onnxruntime ninja
!test -f yolox_tiny.pth || wget -q https://github.com/Megvii-BaseDetection/YOLOX/releases/download/0.1.1rc0/yolox_tiny.pth
"""),
        embed("prepare.py"),
        embed("plate_tiny.py"),
        embed("export.py"),
        embed("detect.py"),
        embed("quantize.py"),
        code("""
!test -d /content/plate_data || python prepare.py /content/plate_data {ZIPS} --dedupe
"""),
        code("""
os.environ.update(PLATE_DATA='/content/plate_data', PLATE_EPOCHS=str(EPOCHS), PLATE_OUT=f'{DRIVE}/out', PLATE_WORKERS='2',
                  TORCH_FORCE_NO_WEIGHTS_ONLY_LOAD='1')
latest = f'{DRIVE}/out/plate_tiny/latest_ckpt.pth'
start = f'--resume -c "{latest}"' if os.path.exists(latest) else '-c yolox_tiny.pth'
print('시작:', start)
!PYTHONPATH=YOLOX python YOLOX/tools/train.py -f plate_tiny.py -d 1 -b 32 --fp16 {start}
"""),
        code("""
!PYTHONPATH=YOLOX python export.py plate_tiny.py "{DRIVE}/out/plate_tiny/best_ckpt.pth" "{DRIVE}/out/plate.onnx"
CALIB = ' '.join(shlex.quote(f'{z}:{30 if "korea" in z.lower() else 70}') for z in zips)
!python quantize.py "{DRIVE}/out/plate.onnx" "{DRIVE}/out/plate_int8.onnx" {CALIB}
!ls -la "{DRIVE}/out"
"""),
    ]
    nb = {"cells": cells, "metadata": {"accelerator": "GPU", "colab": {"provenance": []},
                                       "kernelspec": {"name": "python3", "display_name": "Python 3"}},
          "nbformat": 4, "nbformat_minor": 0}
    with open(out, "w") as f:
        json.dump(nb, f, ensure_ascii=False, indent=1)
    print("만듦:", out)


if __name__ == "__main__":
    main()
