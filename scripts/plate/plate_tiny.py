# YOLOX-Tiny 번호판 1종류 학습 설정. COCO 사전학습 가중치(yolox_tiny.pth)에서 이어 학습한다.
# 데이터 폴더는 PLATE_DATA, 학습 횟수는 PLATE_EPOCHS, 결과 폴더는 PLATE_OUT 환경 변수로 받는다.
# 결과 폴더를 Google 드라이브에 두면 Colab 연결이 끊겨도 체크포인트가 남아 이어서 학습할 수 있다.
# 회차별 체크포인트는 드라이브를 채워 남기지 않는다(latest·last_epoch·best만 남음).
# 학습 데이터가 근접 사진 위주라 mosaic_scale 아래쪽을 0.3까지 내려 작은 번호판을 더 만든다.
# 입력 640이고 multiscale_range 5면 학습 중 480~800px 사이로 크기를 바꿔 가며 배운다.
# 번호판 글자가 좌우로 뒤집혀도 위치 찾기에는 상관없어 flip은 기본값(0.5)을 둔다.

import os

from yolox.exp import Exp as MyExp


class Exp(MyExp):
    def __init__(self):
        super().__init__()
        self.depth = 0.33
        self.width = 0.375
        self.num_classes = 1
        self.input_size = (640, 640)
        self.test_size = (640, 640)
        self.multiscale_range = 5
        self.mosaic_scale = (0.3, 1.5)
        self.enable_mixup = False
        self.degrees = 5.0

        self.data_dir = os.environ.get("PLATE_DATA", "plate_data")
        self.train_ann = "instances_train2017.json"
        self.val_ann = "instances_val2017.json"

        self.max_epoch = int(os.environ.get("PLATE_EPOCHS", "60"))
        self.no_aug_epochs = max(1, self.max_epoch // 6)
        self.warmup_epochs = min(3, self.max_epoch)
        self.eval_interval = max(1, self.max_epoch // 10)
        self.data_num_workers = int(os.environ.get("PLATE_WORKERS", "2"))
        self.save_history_ckpt = False
        self.output_dir = os.environ.get("PLATE_OUT", "./YOLOX_outputs")
        self.exp_name = "plate_tiny"
