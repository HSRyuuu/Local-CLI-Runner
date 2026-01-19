# CLI Runner

AI CLI (Claude, Gemini)를 HTTP API로 실행하고 SSE 스트리밍으로 결과를 받아볼 수 있는 서비스입니다.

## 📦 설치 및 실행

### 사전 요구사항

1. **Go 설치** (1.21 이상)
   ```bash
   # macOS
   brew install go
   
   # 설치 확인
   go version
   ```

2. **Swagger 도구 설치** (API 문서 생성용)
   ```bash
   go install github.com/swaggo/swag/cmd/swag@latest
   ```

3. **Claude CLI 설치** (AI CLI 사용 시)
   ```bash
   npm install -g @anthropic-ai/claude-cli
   ```

### 프로젝트 실행

```bash
# 1. 저장소 클론
git clone <repository-url>
cd Local-CLI-Runner

# 2. 의존성 다운로드
go mod download

# 3. 빌드 및 실행 (Swagger 문서 자동 생성)
make run
```

### 기타 Makefile 명령어

```bash
make help     # 사용 가능한 명령어 목록
make build    # Swagger 생성 + 빌드
make swagger  # Swagger 문서만 생성
make clean    # 빌드 파일 삭제
```

## 📚 문서

- **[API 명세 (API_SPEC.md)](./API_SPEC.md)** - REST API 엔드포인트 상세 문서
- **[사용 가이드 (USE_GUIDE.md)](./USE_GUIDE.md)** - CLI 요청 흐름 및 사용 예시

## 🔗 추가 리소스

- **Swagger UI**: `http://localhost:4001/swagger/index.html` (서버 실행 후)
- **Health Check**: `http://localhost:4001/health`

## ⚙️ 설정

`config.yaml` 파일에서 서버 및 프로세스 설정을 변경할 수 있습니다.

| 설정 | 기본값 | 설명 |
|------|--------|------|
| `server.port` | 4001 | 서버 포트 |
| `process.maxConcurrent` | 10 | 최대 동시 실행 수 |
| `process.defaultTimeout` | 30분 | 프로세스 타임아웃 |
