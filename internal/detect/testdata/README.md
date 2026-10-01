# 검출 테스트 데이터

| 파일 | 원본 | 라이선스 |
| --- | --- | --- |
| `sts135_640.png` | [STS-135 crew portrait](https://commons.wikimedia.org/wiki/File:STS-135_crew_portrait.jpg) (NASA) | Public domain |
| `apollo11_640.png` | [Apollo 11 Crew](https://commons.wikimedia.org/wiki/File:Apollo_11_Crew.jpg) (NASA) | Public domain |

두 이미지는 1280px 썸네일을 긴 변 640으로 줄여(INTER_AREA) 640×640 검은 캔버스 왼쪽 위에 붙였습니다.
JPEG 디코더 차이를 없애려고 PNG로 저장했습니다.

`yunet_golden.json`은 OpenCV 5.0.0 `cv2.FaceDetectorYN`(모델 `face_detection_yunet_2023mar.onnx`, 가중치가 같은 2026may로도 일치,
입력 640×640, score 0.5, NMS 0.3)으로 위 PNG를 검출한 결과입니다. 형식: `box` = x, y, w, h (픽셀), `score`.
