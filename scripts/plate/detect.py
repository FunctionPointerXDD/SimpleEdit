# 번호판 ONNX(export.py 출력)로 영상 프레임을 검출해 장면 모음 그림으로 저장한다(학습 결과 눈 확인용).
# 입력은 BGR 0~255 그대로, 긴 변을 size로 줄여 왼쪽 위에 두고 오른쪽·아래를 114로 채워 32의 배수로 맞춘다.
# 출력 [1,N,6]은 (cx,cy,w,h,obj,cls) 그리드 값이다. stride 8/16/32 격자 순서대로
# cx=(x+g)*s, w=exp(w)*s, 점수=sigmoid(obj)*sigmoid(cls) — 내보낸 모델은 sigmoid까지 들어 있다.
# 사용: python detect.py plate.onnx 영상.mp4 출력.jpg 시각1,시각2,... [size=1280] [점수=0.4]

import sys

import cv2 as cv
import numpy as np
import onnxruntime as ort

STRIDES = (8, 16, 32)


def letterbox(img, size):
    h, w = img.shape[:2]
    k = size / max(h, w)
    nw, nh = int(round(w * k)), int(round(h * k))
    pw, ph = (nw + 31) // 32 * 32, (nh + 31) // 32 * 32
    canvas = np.full((ph, pw, 3), 114, np.uint8)
    canvas[:nh, :nw] = cv.resize(img, (nw, nh), interpolation=cv.INTER_AREA)
    return canvas, k


def decode(out, pw, ph):
    grids, strides = [], []
    for s in STRIDES:
        gy, gx = np.meshgrid(np.arange(ph // s), np.arange(pw // s), indexing="ij")
        grids.append(np.stack((gx, gy), 2).reshape(-1, 2))
        strides.append(np.full((grids[-1].shape[0], 1), s))
    grids, strides = np.concatenate(grids), np.concatenate(strides)
    xy = (out[:, :2] + grids) * strides
    wh = np.exp(out[:, 2:4]) * strides
    return np.concatenate([xy, wh, out[:, 4:]], 1)


def detect(sess, img, size, thr):
    canvas, k = letterbox(img, size)
    blob = canvas.transpose(2, 0, 1)[None].astype(np.float32)
    out = decode(sess.run(None, {"images": blob})[0][0], canvas.shape[1], canvas.shape[0])
    score = out[:, 4] * out[:, 5]
    keep = score >= thr
    boxes = np.stack([out[keep, 0] - out[keep, 2] / 2, out[keep, 1] - out[keep, 3] / 2, out[keep, 2], out[keep, 3]], 1) / k
    idx = cv.dnn.NMSBoxes(boxes.tolist(), score[keep].tolist(), thr, 0.45)
    return [(boxes[i], float(score[keep][i])) for i in np.array(idx).flatten()] if len(idx) else []


def main():
    model, video, out, times = sys.argv[1:5]
    size = int(sys.argv[5]) if len(sys.argv) > 5 else 1280
    thr = float(sys.argv[6]) if len(sys.argv) > 6 else 0.4
    sess = ort.InferenceSession(model)
    cap = cv.VideoCapture(video)
    tiles = []
    for t in (float(x) for x in times.split(",")):
        cap.set(cv.CAP_PROP_POS_MSEC, t * 1000)
        ok, img = cap.read()
        if not ok:
            continue
        dets = detect(sess, img, size, thr)
        for (x, y, w, h), s in dets:
            cv.rectangle(img, (int(x), int(y)), (int(x + w), int(y + h)), (0, 0, 255), 3)
            cv.putText(img, f"{s:.2f}", (int(x), int(y) - 6), 0, 1, (0, 255, 255), 2)
        cv.putText(img, f"{t:.0f}s n={len(dets)}", (10, 50), 0, 1.5, (255, 255, 0), 3)
        tiles.append(cv.resize(img, (960, 540)))
        print(f"{t:.0f}s 번호판 {len(dets)}개 {[round(s, 2) for _, s in dets]}")
    rows = [np.hstack(tiles[i:i + 2] + [np.zeros_like(tiles[0])] * (2 - len(tiles[i:i + 2]))) for i in range(0, len(tiles), 2)]
    cv.imwrite(out, np.vstack(rows))


if __name__ == "__main__":
    main()
