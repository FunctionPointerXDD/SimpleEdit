# 학습한 체크포인트를 ONNX로 내보낸다. 입력 "images" [1,3,H,W]는 높이·너비 가변(32의 배수),
# 출력 "output" [1,N,6]은 그리드 기준 값 그대로다(앱의 Go 코드가 stride 8/16/32로 디코딩).
# YOLOX의 tools/export_onnx.py는 최신 PyTorch에 없는 torch.onnx._export를 써서 따로 둔다.
# SiLU는 YOLOX가 내보내기용으로 둔 SiLU 모듈로 바꾸고, 예전 방식(dynamo=False) 내보내기를 쓴다.
# 학습 체크포인트에는 NumPy 실수(best_ap 등)가 있어 weights_only=False로 읽는다(직접 만든 파일만).
# 사용: python export.py plate_tiny.py best_ckpt.pth plate.onnx

import sys

import torch
from torch import nn
from yolox.exp import get_exp
from yolox.models.network_blocks import SiLU
from yolox.utils import replace_module

OPSET = 13


def main():
    exp_file, ckpt_file, out = sys.argv[1:4]
    exp = get_exp(exp_file, None)
    model = exp.get_model()
    ckpt = torch.load(ckpt_file, map_location="cpu", weights_only=False)
    model.load_state_dict(ckpt.get("model", ckpt))
    model.eval()
    model = replace_module(model, nn.SiLU, SiLU)
    model.head.decode_in_inference = False
    dummy = torch.zeros(1, 3, exp.test_size[0], exp.test_size[1])
    torch.onnx.export(
        model, dummy, out,
        input_names=["images"], output_names=["output"],
        dynamic_axes={"images": {2: "height", 3: "width"}, "output": {1: "anchors"}},
        opset_version=OPSET, dynamo=False,
    )
    print("내보냄:", out)


if __name__ == "__main__":
    main()
