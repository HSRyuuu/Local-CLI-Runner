.PHONY: help swagger build run clean

# 기본 명령어 (help)
help:
	@echo "사용 가능한 명령어:"
	@echo "  make swagger  - Swagger 문서 생성/업데이트"
	@echo "  make build    - Swagger 생성 후 빌드"
	@echo "  make run      - Swagger 생성 후 빌드 후 실행"
	@echo "  make clean    - 빌드 파일 삭제"

# Swagger 문서 생성
swagger:
	@echo "🔄 Swagger 문서 생성 중..."
	@$(shell go env GOPATH)/bin/swag init
	@echo "✅ Swagger 문서 생성 완료!"

# 빌드 (Swagger 자동 생성)
build: swagger
	@echo "🔨 빌드 중..."
	@go build -o cli-runner
	@echo "✅ 빌드 완료!"

# 실행
run: build
	@echo "🚀 서버 실행 중..."
	@./cli-runner

# 정리
clean:
	@echo "🧹 정리 중..."
	@rm -f cli-runner
	@echo "✅ 정리 완료!"
