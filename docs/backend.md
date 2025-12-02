# backend

## 폴더 구조
```
apps/backend/
├── .air.toml               # Live-reloading 설정
├── .gitignore
├── build.js                # 프로덕션 빌드 스크립트
├── dev.js                  # 개발 서버 실행 스크립트
├── embed_dev.go
├── embed_prod.go
├── go.mod
├── go.sum
├── main.go
├── package.json            # pnpm & turbo 모노레포 구성용
│
├── config/                 # 환경별 설정 파일
│   ├── config.dev.yaml
│   └── config.prod.yaml
│
├── internal/
│   │
│   ├── config/             # 설정 로딩 및 관리
│   │   ├── config_model.go
│   │   └── handler.go
│   │
│   ├── webdav/             # WebDAV 서버
│   │   └── server.go       #    - WebDAV 요청을 받아 핵심 서비스 호출
│   │
│   ├── ftp/                # FTP 서버
│   │   └── server.go       #    - FTP 명령어를 받아 핵심 서비스 호출
│   │
│   ├── file/               # 파일 도메인 로직
│   │   ├── api.go
│   │   ├── file.go
│   │   └── service.go
│   │
│   ├── share/              # 폴더 공유 도메인 직
│   │   ├── api.go
│   │   ├── share.go
│   │   └── service.go
│   │
│   ├── user/               # 유저 도메인 로직
│   │   ├── api.go
│   │   ├── user.go
│   │   └── service.go
│   │
│   ├── storage/            # 실제 파일 저장소 (추상화 계층)
│   │   ├── interface.go
│   │   ├── filesystem.go
│   │   └── s3.go
│   │
│   ├── platform/           # 공통 기반 기술
│   │   ├── database/
│   │   │   └── db.go       # DB 초기화
│   │   └── auth/           #    - 인증/인가 로직
│   │
│   └── spa/                # SPA 프론트엔드 서빙 핸들러
│       └── handler.go
```

## 실행 구조

백엔드 앱은 개발 시와 프로덕션(서비스) 시의 빌드 및 실행 방식에 차이가 있음. 설정 관리는 `Viper` 라이브러리를 사용함.

### 공통: `Viper`로 설정 파일 읽기

어떤 환경이든 앱 시작 시 `internal/config/handler.go`의 `SetConfig(goEnv)` 함수가 먼저 호출됨.

1.  이 함수는 `goEnv` 값('development' 또는 'production')에 따라 `config.dev.yaml` 또는 `config.prod.yaml` 파일을 읽도록 `viper`를 세팅해야 함.
2.  `viper`는 `config/` 폴더 안에서 설정 파일을 찾아 읽음.
3.  읽어온 설정(서버 포트, DB 정보 등)은 `Conf` 전역 변수에 저장됨. 이를 통해 앱 어디서든 `config.Conf.Server.Port`처럼 쉽게 설정 값을 사용 가능함.

---

### 1. 개발 환경에서 실행

1.  **시작**: `dev.js` 스크립트가 `air` 라이브 리로더를 실행함. `air`는 코드 변경 시 자동으로 재빌드 및 재시작을 수행함.
2.  **빌드**: `air`는 `.air.toml` 설정에 따라 `go build -o ./tmp/main.exe .` 명령으로 앱을 컴파일함.
    -   `goEnv` 변수는 코드 상의 기본값인 `"development"`를 유지함.
    -   프론트엔드 파일은 바이너리에 포함되지 않음.
3.  **실행**: 컴파일된 `tmp/main.exe`를 실행함.
4.  **서버 동작**:
    -   `SetConfig("development")`가 호출되어 `viper`가 `config.dev.yaml` 파일을 로드함.
    -   개발용 설정(포트, DB 정보)으로 서버가 동작함.
    -   서버는 API 요청(`_API 경로`)만 처리. 프론트엔드 UI는 `vite` 개발 서버가 별도로 담당함.

---

### 2. 프로덕션 환경에서 실행

1.  **시작**: `build.js` 스크립트를 실행해야 함.
2.  **프론트엔드 통합**: `frontend/dist`의 빌드 결과물을 `backend/dist/web`으로 복사함.
3.  **빌드**: `go build` 시 아래의 특정 플래그를 사용해야 함.
    -   `-ldflags "-X main.goEnv=production"`: 이 플래그로 `goEnv` 변수 값을 `"production"`으로 강제 변경하여 컴파일함.
    -   `-tags=production`: 이 태그로 `embed_prod.go`를 빌드에 포함시켜, `dist/web` 폴더의 프론트엔드 파일들을 바이너리 안에 내장시킴.
4.  **결과물**: API 서버와 프론트엔드 UI가 통합된 단일 실행 파일 `dist/main.exe`가 생성됨.
5.  **서버 동작** (`dist/main.exe` 실행 시):
    -   `SetConfig("production")`이 호출되어 `viper`가 `config.prod.yaml` 파일을 로드함.
    -   프로덕션용 설정으로 서버가 동작함.
    -   이 서버 하나가 API 요청(`_API 경로`)과 웹 UI(`/` 경로)를 모두 처리하는 독립 앱으로 기능함.