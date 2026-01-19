# 사용 가이드

CLI Runner를 이용한 AI CLI 요청 흐름 가이드입니다.

## 기본 사용 흐름

```
1. POST /run → processId 획득
2. GET /stream/{id} → SSE 스트림 구독
3. SSE에서 result 이벤트 수신 → 최종 결과 확인
4. (선택) GET /result-data/{id} → 놓친 결과 조회
```

---

## Step 1: 프로세스 실행

```bash
curl -X POST http://localhost:4001/api/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "connector": "claude",
    "prompt": "Hello, how are you?"
  }'
```

**응답**
```json
{
  "processId": "abc123-def456-..."
}
```

---

## Step 2: SSE 스트림 구독

```bash
curl -N http://localhost:4001/api/v1/stream/abc123-def456-...
```

**스트림 출력 예시**
```
event: output
data: {"type":"output","data":"Thinking...","timestamp":"..."}

event: result
data: {"type":"result","data":{"result":"I'm doing well!","usage":{...}},"timestamp":"..."}

event: done
data: {"type":"done","data":null,"timestamp":"..."}
```

---

## Step 3: Result 이벤트 처리

SSE 스트림에서 `event: result`를 수신하면 `data` 필드의 JSON을 파싱하여 사용합니다.

```javascript
// JavaScript EventSource 예시
const es = new EventSource('/api/v1/stream/' + processId);

es.addEventListener('result', (e) => {
  const data = JSON.parse(e.data);
  console.log('Result:', data.data);
});

es.addEventListener('done', () => {
  es.close();
});
```

---

## Step 4: 놓친 결과 조회

SSE 연결이 끊기거나 `result` 이벤트를 놓친 경우:

```bash
curl http://localhost:4001/api/v1/result-data/abc123-def456-...
```

> ⚠️ Result 데이터는 **10분간**만 캐시됩니다.

---

## 추가 사용 예시

### 프로세스 상태 확인
```bash
curl http://localhost:4001/api/v1/process/abc123-def456-...
```

### 프로세스 목록 조회
```bash
curl http://localhost:4001/api/v1/processes
```

### 프로세스 강제 종료
```bash
curl -X DELETE http://localhost:4001/api/v1/process/abc123-def456-...
```

### 사용 가능한 커넥터 확인
```bash
curl http://localhost:4001/api/v1/connectors
```

### 작업 디렉토리 지정
```bash
curl -X POST http://localhost:4001/api/v1/run \
  -H "Content-Type: application/json" \
  -d '{
    "connector": "claude",
    "prompt": "Analyze this project",
    "workDir": "/path/to/my-project"
  }'
```

---

## 에러 처리

| HTTP Status | 설명 | 대응 |
|-------------|------|------|
| 400 | 잘못된 요청 | 요청 파라미터 확인 |
| 404 | 프로세스 없음 | processId 확인 |
| 429 | 동시 실행 초과 | 잠시 후 재시도 |
| 500 | 서버 오류 | 로그 확인 |
