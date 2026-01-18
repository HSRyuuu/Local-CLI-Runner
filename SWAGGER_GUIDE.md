# CLI Runner - Swagger 문서 자동 생성 가이드

## 🎯 요약

**답변: 아니요!** Go에서는 코드 주석만 작성하면 `swag` 도구가 자동으로 Swagger 문서를 생성합니다.

---

## 🚀 사용 방법

### 1️⃣ 일일 작업 (코드 수정 후)

```bash
# 방법 A: Makefile 사용 (추천)
make build   # Swagger 자동 생성 + 빌드
make run     # Swagger 자동 생성 + 빌드 + 실행

# 방법 B: 직접 명령어 실행
$(go env GOPATH)/bin/swag init  # Swagger만 생성
go build -o cli-runner           # 빌드
./cli-runner                     # 실행
```

### 2️⃣ 새 API 엔드포인트 추가할 때

1. **핸들러에 주석 추가**
```go
// GetResultDataHandler handles GET /api/v1/result-data/:id
// @Summary 캐시된 result 데이터 조회
// @Description 10분간 메모리에 저장된 result 이벤트 데이터를 조회합니다
// @Tags process
// @Produce json
// @Param id path string true "프로세스 ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} ErrorResponse
// @Router /result-data/{id} [get]
func (h *Handlers) GetResultDataHandler(c *gin.Context) {
    // 구현...
}
```

2. **빌드만 하면 끝!**
```bash
make build  # Swagger 자동 생성됨!
```

---

## 📋 Swagger 주석 패턴

### 필수 주석
- `@Summary` - 짧은 설명
- `@Description` - 상세 설명
- `@Tags` - 그룹화 (예: process, connector)
- `@Router` - 경로와 HTTP 메서드

### 선택 주석
- `@Param` - 파라미터 정의
- `@Success` - 성공 응답
- `@Failure` - 에러 응답
- `@Produce` - 응답 타입 (json, xml 등)
- `@Accept` - 요청 타입

---

## 📊 생성되는 파일

```
docs/
├── docs.go        # Go 코드
├── swagger.json   # JSON 형식
└── swagger.yaml   # YAML 형식
```

---

## 🌐 Swagger UI 접속

서버 실행 후:
```
http://localhost:3001/swagger/index.html
```

---

## 💡 주요 명령어

```bash
make help      # 도움말
make swagger   # Swagger만 생성
make build     # Swagger + 빌드
make run       # Swagger + 빌드 + 실행
make clean     # 빌드 파일 삭제
```

---

## ✅ 체크리스트

- [x] `swag` CLI 도구 설치됨
- [x] `Makefile` 생성됨
- [x] Swagger 주석 추가 방법 알게됨
- [x] 앞으로는 `make build`만 하면 자동 생성!

---

## 🔥 핵심 포인트

**일일이 업데이트할 필요 없음!**
- 코드 주석만 작성
- `make build` 실행
- Swagger 자동 생성! ✨
