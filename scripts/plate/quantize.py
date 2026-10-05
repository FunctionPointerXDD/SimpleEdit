# export.py로 만든 FP32 ONNX를 정적 INT8(QDQ)로 바꾼다. 앱은 이 INT8 파일을 plate_yolox_tiny.onnx로 쓴다.
# 가중치는 INT8(출력 채널별·대칭), 활성값은 UINT8(층마다 하나·비대칭)이고 입력·최종 출력은 float32 그대로다.
# 활성값 범위는 학습 zip의 train 사진을 detect.py와 같은 letterbox(긴 변 960, 114 채움)로 넣어 잰다.
# 입력 크기가 가변이라 symbolic shape 추론은 건너뛴다(실패함).
# 2026-10-05 측정(960px, 6스레드): FP32 91ms → INT8 41ms, 60초 영상 분석 57초 → 32초,
# 추적 뒤 가림 박스 97%가 FP32와 같은 자리. Nano는 0.7 이상 번호판을 11개 중 7개만 잡아 쓰지 않음.
# 사용: python quantize.py 입력.onnx 출력.onnx a.zip:70 b.zip:30   (zip:N = 그 zip에서 N장, 고정 시드)

import os
import random
import sys
import zipfile

import cv2 as cv
import numpy as np
from onnxruntime.quantization import CalibrationDataReader, QuantFormat, QuantType, quantize_static
from onnxruntime.quantization.shape_inference import quant_pre_process

from detect import letterbox

SIZE = 960
SEED = 0


class Reader(CalibrationDataReader):
    def __init__(self, blobs):
        self.it = iter(blobs)

    def get_next(self):
        x = next(self.it, None)
        return None if x is None else {"images": x}


def calibration(args):
    blobs = []
    for arg in args:
        path, _, n = arg.rpartition(":")
        with zipfile.ZipFile(path) as z:
            names = sorted(x for x in z.namelist() if x.startswith("train/") and x.endswith(".jpg"))
            random.seed(SEED)
            for name in random.sample(names, min(int(n), len(names))):
                img = cv.imdecode(np.frombuffer(z.read(name), np.uint8), cv.IMREAD_COLOR)
                canvas, _ = letterbox(img, SIZE)
                blobs.append(canvas.transpose(2, 0, 1)[None].astype(np.float32))
    return blobs


def main():
    src, dst = sys.argv[1:3]
    blobs = calibration(sys.argv[3:])
    pre = dst + ".pre.onnx"
    quant_pre_process(src, pre, skip_symbolic_shape=True)
    quantize_static(pre, dst, Reader(blobs), quant_format=QuantFormat.QDQ,
                    activation_type=QuantType.QUInt8, weight_type=QuantType.QInt8, per_channel=True)
    os.remove(pre)
    print(f"보정 사진 {len(blobs)}장, 만듦: {dst} ({os.path.getsize(dst) / 1e6:.1f}MB)")


if __name__ == "__main__":
    main()
