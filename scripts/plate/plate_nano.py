# YOLOX-Nano 번호판 설정. 데이터·증강·학습 횟수 등은 plate_tiny.py 그대로이고 모델만 Nano다.
# Nano는 폭 0.25와 depthwise 합성곱이라 Tiny보다 계산이 훨씬 적다(CPU 분석 속도용, 10-05).
# 사전학습 가중치는 yolox_nano.pth다. YOLOX가 이 파일 폴더를 sys.path에 넣어 plate_tiny를 불러온다.

from torch import nn

from plate_tiny import Exp as TinyExp


class Exp(TinyExp):
    def __init__(self):
        super().__init__()
        self.width = 0.25
        self.mosaic_prob = 0.5
        self.exp_name = "plate_nano"

    def get_model(self):
        def init_yolo(module):
            for m in module.modules():
                if isinstance(m, nn.BatchNorm2d):
                    m.eps = 1e-3
                    m.momentum = 0.03

        if "model" not in self.__dict__:
            from yolox.models import YOLOX, YOLOPAFPN, YOLOXHead
            in_channels = [256, 512, 1024]
            backbone = YOLOPAFPN(self.depth, self.width, in_channels=in_channels, act=self.act, depthwise=True)
            head = YOLOXHead(self.num_classes, self.width, in_channels=in_channels, act=self.act, depthwise=True)
            self.model = YOLOX(backbone, head)
        self.model.apply(init_yolo)
        self.model.head.initialize_biases(1e-2)
        return self.model
