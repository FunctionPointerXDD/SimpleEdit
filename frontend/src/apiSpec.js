// cmd/apispec이 docs/api.json과 함께 만든다. 손으로 고치지 말고 go run ./cmd/apispec 로 다시 만든다.

export const EVENTS = /** @type {const} */ ({
    analyzeProgress: 'analyze:progress',
    downloadProgress: 'download:progress',
    exportProgress: 'export:progress',
});

export const MESSAGES = {
    msgAgeLimit: '나이 제한이 있는 영상이라 받을 수 없습니다. 다른 영상 주소를 넣어 주세요.',
    msgAnalyzeFail: '자동으로 찾는 중 문제가 생겼습니다. 다시 눌러 주세요.',
    msgAutoStale: '자른 구간 일부의 얼굴 찾기가 끝나지 않았습니다. "이전"을 눌러 3단계로 돌아가면 나머지를 이어서 찾습니다.',
    msgBadRange: '끝 지점이 시작 지점보다 앞에 있습니다. "여기서 끝"을 다시 눌러 주세요.',
    msgBadURL: '유튜브 영상 주소가 아닙니다. 유튜브에서 주소를 다시 복사해 칸에 붙여넣은 뒤 "영상 받기"를 눌러 주세요.',
    msgBlocked: '유튜브가 잠시 다운로드를 막았습니다. 몇 분 뒤에 다시 눌러 주세요.',
    msgBusy: '다른 작업이 진행 중입니다. 끝날 때까지 기다려 주세요.',
    msgCanceled: '작업을 취소했습니다.',
    msgDownloadFail: '영상을 받지 못했습니다. 잠시 뒤 다시 눌러 주세요.',
    msgExportFail: '영상을 저장하지 못했습니다. 다시 "저장하기"를 눌러 주세요.',
    msgLive: '생방송 중인 영상은 받을 수 없습니다. 방송이 끝난 뒤 다시 해 주세요.',
    msgNameBadChar: '파일 이름에 쓸 수 없는 글자(\\ / : * ? " < > |)가 있습니다. 지우고 다시 눌러 주세요.',
    msgNameEmpty: '파일 이름이 비어 있습니다. 이름을 적은 뒤 다시 눌러 주세요.',
    msgNameReserved: '이 이름은 Windows에서 쓸 수 없습니다. 다른 이름을 적어 주세요.',
    msgNameTooLong: '파일 이름이 너무 깁니다. 100글자 안으로 줄여 주세요.',
    msgNetwork: '영상을 받지 못했습니다. 인터넷 연결을 확인하고 다시 눌러 주세요.',
    msgNoSpace: '컴퓨터 저장 공간이 부족합니다. 필요 없는 파일을 지운 뒤 다시 눌러 주세요.',
    msgNoVideo: '먼저 영상을 불러와 주세요.',
    msgNotExtracted: '압축을 풀지 않고 실행하셨습니다. zip 파일을 마우스 오른쪽 버튼으로 눌러 "압축 풀기"를 한 뒤, 풀린 폴더의 운전영상편집기.exe를 실행해 주세요.',
    msgOpenFail: '이 파일은 열 수 없습니다. 다른 동영상 파일을 골라 주세요.',
    msgOverwriteFail: '기존 파일을 새 내용으로 바꾸지 못했습니다. 그 영상을 틀어 둔 프로그램이 있으면 닫은 뒤 다시 눌러 주세요.',
    msgPrivate: '비공개 영상이라 받을 수 없습니다. 다른 영상 주소를 넣어 주세요.',
    msgRecentGone: '받아 둔 영상을 열 수 없습니다. 7일이 지나 지워졌을 수 있으니 유튜브 주소로 다시 받아 주세요.',
    msgRuntimeMiss: '얼굴 찾기에 필요한 Windows 구성 요소(Visual C++ 재배포 가능 패키지)가 없습니다. 설치한 뒤 다시 눌러 주세요.',
    msgTooManyRects: '가릴 곳은 10개까지 정할 수 있습니다. 필요 없는 것을 지워 주세요.',
    msgTooShort: '자를 구간이 너무 짧습니다. 시작과 끝을 1초 이상 떨어지게 정해 주세요.',
    msgToolMissing: '프로그램 파일 일부가 없습니다. 프로그램 폴더를 처음 받은 그대로 다시 풀어 주세요.',
    msgUnavailable: '삭제되었거나 볼 수 없는 영상입니다. 다른 영상 주소를 넣어 주세요.',
};

/**
 * @typedef {Object} AnalyzeRequest
 * @property {string} kind
 * @property {number} start
 * @property {number} end
 * @property {boolean} fresh
 */

/**
 * @typedef {Object} AutoMask
 * @property {string} kind
 * @property {number} start
 * @property {number} duration
 * @property {number} fps
 * @property {number} found
 * @property {number} tracks
 * @property {[number, number, number, number, number][][]} frames
 */

/**
 * @typedef {Object} ExportRequest
 * @property {number} start
 * @property {number} end
 * @property {MaskRect[]} rects
 * @property {string} fileName
 * @property {string[]} auto
 * @property {boolean} mute
 * @property {Record<string, number[]>} exclude
 * @property {boolean} overwrite
 */

/**
 * @typedef {Object} MaskRect
 * @property {number} x
 * @property {number} y
 * @property {number} w
 * @property {number} h
 * @property {number} start
 * @property {number} end
 */

/**
 * @typedef {Object} ProgressEvent
 * @property {number} percent
 * @property {number} remaining
 * @property {string} stage
 */

/**
 * @typedef {Object} RecentVideo
 * @property {string} id
 * @property {string} title
 * @property {number} duration
 * @property {number} savedAt
 */

/**
 * @typedef {Object} VideoInfo
 * @property {string} url
 * @property {number} duration
 * @property {number} width
 * @property {number} height
 * @property {string} name
 * @property {boolean} reused
 * @property {boolean} lowered
 */
