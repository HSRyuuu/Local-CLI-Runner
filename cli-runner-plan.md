# CLI Runner 구현 계획

## 1. 프로젝트 개요

| 항목 | 내용 |
|------|------|
| **서비스명** | CLI Runner |
| **언어** | Go 1.22+ |
| **프레임워크** | Gin |
| **목적** | AI CLI(Claude, Gemini) 실행 및 SSE 스트리밍 |

---

## 2. 프로젝트 구조

```
cli-runner/
├── main.go                 # 진입점
├── go.mod
├── go.sum
├── Dockerfile
├── config/
│   └── config.go           # 설정 로드 (Viper)
├── api/
│   ├── server.go           # Gin 서버 초기화
│   ├── handlers.go         # HTTP 핸들러
│   └── sse.go              # SSE 스트리밍 헬퍼
├── runner/
│   ├── runner.go           # 프로세스 실행 핵심
│   ├── process.go          # Process 구조체
│   ├── buffer.go           # 이벤트 버퍼 (ring buffer)
│   └── manager.go          # 프로세스 매니저 (싱글톤)
├── connector/
│   ├── connector.go        # Connector 인터페이스
│   ├── claude.go           # Claude CLI 커넥터
│   └── gemini.go           # Gemini CLI 커넥터 (추후)
└── pkg/
    └── logger/
        └── logger.go       # Zerolog 설정
```

---

## 3. 핵심 컴포넌트

### 3.1 Process 구조체

```go
type Process struct {
    ID           string
    Connector    string
    Status       string        // pending, running, completed, failed, stopped
    Cmd          *exec.Cmd
    StartedAt    time.Time
    CompletedAt  *time.Time

    // 버퍼링
    Events       *RingBuffer[Event]
    Result       *Result

    // 구독자 관리
    subscribers  map[string]chan Event
    mu           sync.RWMutex
}
```

### 3.2 Event 구조체

```go
type Event struct {
    Type      string          // stream, result, error, done
    Data      json.RawMessage
    Timestamp time.Time
}
```

### 3.3 RingBuffer

```go
type RingBuffer[T any] struct {
    data  []T
    size  int
    head  int
    count int
    mu    sync.RWMutex
}

func (rb *RingBuffer[T]) Push(item T)
func (rb *RingBuffer[T]) ToSlice() []T
```

---

## 4. API 엔드포인트

| Method | Path | 설명 |
|--------|------|------|
| POST | /api/v1/run | 프로세스 실행 (즉시 processId 반환) |
| GET | /api/v1/stream/:id | SSE 스트림 구독 (버퍼 재생 포함) |
| GET | /api/v1/process/:id | 프로세스 상태 조회 |
| GET | /api/v1/result/:id | 최종 결과 조회 |
| DELETE | /api/v1/process/:id | 프로세스 종료 |
| GET | /api/v1/processes | 프로세스 목록 |
| GET | /api/v1/connectors | 사용 가능한 커넥터 목록 |
| GET | /health | 헬스체크 |
| GET | /ready | 레디체크 |

---

## 5. 구현 단계

### Phase 1: 기본 구조 

- [ ] 프로젝트 초기화 (go mod init)
- [ ] 디렉토리 구조 생성
- [ ] 설정 파일 로드 (Viper)
- [ ] Zerolog 로거 설정
- [ ] Gin 서버 기본 설정
- [ ] 헬스체크 엔드포인트

### Phase 2: 프로세스 관리 

- [ ] RingBuffer 구현
- [ ] Process 구조체 구현
- [ ] ProcessManager 싱글톤 구현
- [ ] 프로세스 생성 (spawn)
- [ ] stdout/stderr 라인 버퍼링
- [ ] 프로세스 종료 처리
- [ ] 자동 정리 (cleanup)

### Phase 3: 커넥터 시스템 

- [ ] Connector 인터페이스 정의
- [ ] Claude 커넥터 구현
  - [ ] 커맨드 빌드
  - [ ] stream-json 파싱
  - [ ] result 이벤트 감지
- [ ] 커넥터 등록/조회

### Phase 4: API 구현 

- [ ] POST /run 핸들러
- [ ] GET /stream/:id 핸들러 (SSE)
  - [ ] 버퍼 재생
  - [ ] 실시간 구독
  - [ ] 연결 종료 처리
- [ ] GET /process/:id 핸들러
- [ ] GET /result/:id 핸들러
- [ ] DELETE /process/:id 핸들러
- [ ] GET /processes 핸들러
- [ ] GET /connectors 핸들러

### Phase 5: 안정화

- [ ] 에러 처리 통합
- [ ] 타임아웃 처리
- [ ] Graceful shutdown
- [ ] 동시성 테스트
- [ ] 메모리 누수 체크

### Phase 6: 배포
- [ ] Dockerfile 작성
- [ ] README.md

---

## 6. 설정 파일

```yaml
# config.yaml
server:
  port: 4001
  host: "0.0.0.0"
  readTimeout: 5s
  writeTimeout: 0     # SSE는 타임아웃 없음

process:
  defaultTimeout: 1800s     # 30분
  maxConcurrent: 10
  cleanupDelay: 300s        # 5분
  bufferSize: 1000          # 이벤트 버퍼

connectors:
  claude:
    command: "claude"
    args:
      - "--dangerously-skip-permissions"
      - "--output-format"
      - "stream-json"
      - "--verbose"
    available: true

logging:
  level: "info"
  format: "json"
```

---

## 7. 핵심 흐름

### 7.1 실행 요청

```
POST /api/v1/run
     │
     ▼
┌─────────────────┐
│ processId 생성   │
│ (UUID)          │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Process 생성    │
│ status: pending │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Manager에 등록  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐     ┌─────────────────┐
│ 응답 반환       │     │ goroutine:      │
│ { processId }   │     │ spawn & stream  │
└─────────────────┘     └─────────────────┘
```

### 7.2 SSE 구독

```
GET /api/v1/stream/:id
     │
     ▼
┌─────────────────┐
│ Process 조회    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 버퍼된 이벤트    │ ──▶ SSE 전송 (재생)
│ 모두 전송       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 구독자 등록     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ 채널 대기       │ ──▶ 실시간 SSE 전송
│ (실시간)        │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ done 이벤트     │ ──▶ 연결 종료
└─────────────────┘
```

---

## 8. 의존성

```go
// go.mod
module cli-runner

go 1.22

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/spf13/viper v1.18.2
    github.com/rs/zerolog v1.32.0
    github.com/google/uuid v1.6.0
)
```
