# API 명세

**Base URL**: `http://localhost:3001/api/v1`

---

## 프로세스 실행

### POST /run
AI CLI 프로세스를 실행하고 processId를 반환합니다.

**Request Body**
```json
{
  "connector": "claude",
  "prompt": "Hello, how are you?",
  "workDir": "/path/to/project"  // optional
}
```

**Response** `202 Accepted`
```json
{
  "processId": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Error Responses**
| 상태 | 설명 |
|------|------|
| 400 | 잘못된 요청 (필수 필드 누락) |
| 429 | 최대 동시 실행 수 초과 |
| 500 | 서버 오류 |

---

## SSE 스트림

### GET /stream/{id}
프로세스의 실시간 이벤트를 SSE로 스트리밍합니다.

**Event Types**
| Type | 설명 |
|------|------|
| `output` | 표준 출력 데이터 |
| `result` | 최종 결과 (JSON) |
| `error` | 에러 발생 |
| `done` | 프로세스 완료 |

**Event Format**
```
event: result
data: {"type":"result","data":{...},"timestamp":"..."}
```

---

## 프로세스 관리

### GET /process/{id}
프로세스 상태를 조회합니다.

**Response** `200 OK`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "connector": "claude",
  "prompt": "Hello",
  "status": "running",
  "startedAt": "2024-01-01T12:00:00Z",
  "completedAt": null
}
```

### GET /result/{id}
완료된 프로세스의 결과를 조회합니다.

**Response** `200 OK` (완료 시)
```json
{
  "exitCode": 0,
  "output": "..."
}
```

**Response** `202 Accepted` (실행 중)
```json
{
  "status": "running",
  "message": "Process is still running"
}
```

### GET /result-data/{id}
캐시된 result 이벤트 데이터를 조회합니다. (10분간 보관)

SSE 스트림에서 `result` 이벤트를 놓친 경우 이 API로 조회할 수 있습니다.

**Response** `200 OK`
```json
{
  "result": "...",
  "usage": {...}
}
```

**Response** `404 Not Found`
```json
{
  "error": "Result data not found or expired",
  "message": "Result data is only cached for 10 minutes"
}
```

### DELETE /process/{id}
실행 중인 프로세스를 종료하고 삭제합니다.

**Response** `200 OK`
```json
{
  "message": "Process deleted successfully"
}
```

### GET /processes
모든 프로세스 목록을 조회합니다.

**Response** `200 OK`
```json
{
  "processes": [...],
  "count": 5
}
```

---

## 커넥터

### GET /connectors
사용 가능한 AI CLI 커넥터 목록을 조회합니다.

**Response** `200 OK`
```json
{
  "connectors": ["claude"],
  "count": 1
}
```

---

## 헬스 체크

### GET /health
서버 상태를 확인합니다.

**Response** `200 OK`
```json
{
  "status": "healthy"
}
```

### GET /ready
서버 준비 상태를 확인합니다.

**Response** `200 OK`
```json
{
  "status": "ready"
}
```
